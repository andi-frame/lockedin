package jobs

import (
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics are per worker instance (not the global registry), like the API's, so tests can
// build many. ARCHITECTURE §10: queue sizes and settlement transitions by type.
type Metrics struct {
	reg         *prometheus.Registry
	transitions *prometheus.CounterVec
	relayed     prometheus.Counter
	skipped     prometheus.Counter
	reminders   prometheus.Counter
	emails      *prometheus.CounterVec
	runs        *prometheus.CounterVec
	duration    *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	m := &Metrics{
		reg: prometheus.NewRegistry(),
		transitions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tepati_settlement_transitions_total",
			Help: "Settlement work done by the worker: type is deadline (a check-in moved by a passed deadline), activated or closed (pacts).",
		}, []string{"type"}),
		relayed:   prometheus.NewCounter(prometheus.CounterOpts{Name: "tepati_outbox_relayed_total", Help: "Outbox rows turned into notifications."}),
		skipped:   prometheus.NewCounter(prometheus.CounterOpts{Name: "tepati_outbox_skipped_total", Help: "Outbox rows dropped because they were unreadable."}),
		reminders: prometheus.NewCounter(prometheus.CounterOpts{Name: "tepati_reminders_queued_total", Help: "Cutoff and review reminders written to the outbox."}),
		emails: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tepati_emails_total",
			Help: "Email outcomes: sent, skipped (already sent or nothing to send), failed, enqueue_failed.",
		}, []string{"result"}),
		runs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tepati_job_runs_total", Help: "Job runs by task type and result.",
		}, []string{"task", "result"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "tepati_job_duration_seconds", Help: "Job run time by task type.",
			Buckets: []float64{.01, .05, .1, .5, 1, 5, 15, 60},
		}, []string{"task"}),
	}
	m.reg.MustRegister(m.transitions, m.relayed, m.skipped, m.reminders, m.emails, m.runs, m.duration)
	// Series exist from the start, so a rate() over a quiet hour is 0 rather than absent.
	for _, t := range []string{"deadline", "activated", "closed"} {
		m.transitions.WithLabelValues(t)
	}
	for _, r := range []string{"sent", "skipped", "failed", "enqueue_failed"} {
		m.emails.WithLabelValues(r)
	}
	return m
}

func (m *Metrics) observe(task string, d time.Duration, err error) {
	result := "ok"
	if err != nil {
		result = "error"
	}
	m.runs.WithLabelValues(task, result).Inc()
	m.duration.WithLabelValues(task).Observe(d.Seconds())
}

// WatchQueues adds the asynq queue sizes, read from Redis when /metrics is scraped.
func (m *Metrics) WatchQueues(insp *asynq.Inspector) {
	m.reg.MustRegister(&queueCollector{
		insp:  insp,
		tasks: prometheus.NewDesc("tepati_asynq_queue_tasks", "Tasks in each asynq queue by state.", []string{"queue", "state"}, nil),
		lag:   prometheus.NewDesc("tepati_asynq_queue_latency_seconds", "Age of the oldest pending task in the queue.", []string{"queue"}, nil),
	})
}

// Handler serves /metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

type queueCollector struct {
	insp       *asynq.Inspector
	tasks, lag *prometheus.Desc
}

func (c *queueCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.tasks; ch <- c.lag }

func (c *queueCollector) Collect(ch chan<- prometheus.Metric) {
	// GetQueueInfo reports a queue asynq has not created yet with an internal error type, so
	// ask which queues exist first. A missing queue is simply empty; a Redis error leaves the
	// series out, which is what an alert on absence wants.
	existing, err := c.insp.Queues()
	if err != nil {
		return
	}
	have := make(map[string]bool, len(existing))
	for _, q := range existing {
		have[q] = true
	}
	for queue := range QueueWeights {
		info := &asynq.QueueInfo{Queue: queue}
		if have[queue] {
			if info, err = c.insp.GetQueueInfo(queue); err != nil {
				continue
			}
		}
		for state, n := range map[string]int{
			"pending": info.Pending, "active": info.Active, "scheduled": info.Scheduled,
			"retry": info.Retry, "archived": info.Archived,
		} {
			ch <- prometheus.MustNewConstMetric(c.tasks, prometheus.GaugeValue, float64(n), queue, state)
		}
		ch <- prometheus.MustNewConstMetric(c.lag, prometheus.GaugeValue, info.Latency.Seconds(), queue)
	}
}
