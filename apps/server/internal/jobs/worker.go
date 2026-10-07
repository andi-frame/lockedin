package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
)

// Options configure Run. Only Redis, Svc and Log are required.
type Options struct {
	Redis           asynq.RedisConnOpt
	Svc             Settlement
	Mail            *Mail   // nil runs without email; a nil Mail.Queue gets a client on Redis
	Uploads         Uploads // nil runs without media processing
	MediaWorkers    int     // media:process jobs at once, default 2 (ARCHITECTURE §6)
	Log             *slog.Logger
	Metrics         *Metrics      // default: a fresh set
	Concurrency     int           // default 10
	MetricsAddr     string        // "host:port" for /metrics and /healthz; empty disables the listener
	Schedule        []Periodic    // default Schedule(); tests pass faster ticks
	ShutdownTimeout time.Duration // how long a stopping worker waits for running jobs, default 25s
}

// Run starts the asynq server, the scheduler and the metrics listener, and blocks until ctx is
// cancelled. Stopping is graceful: the scheduler stops enqueueing first, then running jobs get
// ShutdownTimeout to finish; unfinished tasks go back to the queue and are retried.
//
// ARCHITECTURE §3 gives media its own concurrency cap. asynq has one worker pool per server, so
// media:process runs on a second server that serves only the media queue.
func Run(ctx context.Context, o Options) error {
	if o.Metrics == nil {
		o.Metrics = NewMetrics()
	}
	if o.Concurrency <= 0 {
		o.Concurrency = 10
	}
	if o.Schedule == nil {
		o.Schedule = Schedule()
	}
	if o.ShutdownTimeout <= 0 {
		o.ShutdownTimeout = 25 * time.Second
	}
	if o.MediaWorkers <= 0 {
		o.MediaWorkers = mediaConcurrency
	}
	log := asynqLogger{o.Log}

	handlers := NewHandlers(o.Svc, o.Log, o.Metrics)
	if o.Mail != nil {
		mail := *o.Mail
		if mail.Queue == nil {
			client := asynq.NewClient(o.Redis)
			defer client.Close()
			mail.Queue = client
		}
		handlers.WithMail(&mail)
	}
	if o.Uploads != nil {
		handlers.WithUploads(o.Uploads)
	}

	// Settlement and email share one pool. Media gets a server of its own so that its small
	// concurrency cap is real and a backlog of transcodes can never occupy the settlement workers.
	srv := asynq.NewServer(o.Redis, asynq.Config{
		Concurrency:     o.Concurrency,
		Queues:          map[string]int{QueueCritical: QueueWeights[QueueCritical], QueueDefault: QueueWeights[QueueDefault]},
		ShutdownTimeout: o.ShutdownTimeout,
		Logger:          log,
	})
	mediaSrv := asynq.NewServer(o.Redis, asynq.Config{
		Concurrency:     o.MediaWorkers,
		Queues:          map[string]int{QueueMedia: 1},
		ShutdownTimeout: o.ShutdownTimeout,
		Logger:          log,
		// Once the retries are used up the attachment is rejected, so the uploader is told.
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, t *asynq.Task, _ error) {
			retried, _ := asynq.GetRetryCount(ctx)
			if max, ok := asynq.GetMaxRetry(ctx); ok && retried >= max {
				handlers.mediaFailed(ctx, t)
			}
		}),
	})
	sched := asynq.NewScheduler(o.Redis, &asynq.SchedulerOpts{
		Location: time.UTC,
		Logger:   log,
		// Two worker replicas both schedule; the unique lock makes the second enqueue a
		// duplicate, which is the intended outcome and not worth an error line.
		EnqueueErrorHandler: func(task *asynq.Task, _ []asynq.Option, err error) {
			if !errors.Is(err, asynq.ErrDuplicateTask) {
				o.Log.Error("enqueue periodic task", "task", task.Type(), "err", err)
			}
		},
	})
	for _, p := range o.Schedule {
		// MaxRetry 0: the next tick is the retry. Unique stops replicas from queueing the same
		// tick twice; its TTL is shorter than the period so it never swallows the next one.
		if _, err := sched.Register(p.Spec, asynq.NewTask(p.Type, nil),
			asynq.Queue(p.Queue), asynq.Timeout(p.Timeout), asynq.MaxRetry(0), asynq.Unique(uniqueTTL(p.Spec))); err != nil {
			return fmt.Errorf("schedule %s: %w", p.Type, err)
		}
	}

	insp := asynq.NewInspector(o.Redis)
	defer insp.Close()
	o.Metrics.WatchQueues(insp)

	if err := srv.Start(handlers.Mux()); err != nil {
		return fmt.Errorf("start asynq server: %w", err)
	}
	if err := mediaSrv.Start(handlers.Mux()); err != nil {
		srv.Shutdown()
		return fmt.Errorf("start media server: %w", err)
	}
	if err := sched.Start(); err != nil {
		mediaSrv.Shutdown()
		srv.Shutdown()
		return fmt.Errorf("start scheduler: %w", err)
	}
	o.Log.Info("worker started", "concurrency", o.Concurrency, "periodic", len(o.Schedule))

	var ops *http.Server
	opsDone := make(chan error, 1)
	if o.MetricsAddr != "" {
		ln, err := net.Listen("tcp", o.MetricsAddr)
		if err != nil {
			sched.Shutdown()
			mediaSrv.Shutdown()
			srv.Shutdown()
			return fmt.Errorf("metrics listener: %w", err)
		}
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", o.Metrics.Handler())
		mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
		ops = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		go func() { opsDone <- ops.Serve(ln) }()
		o.Log.Info("worker metrics listening", "addr", ln.Addr().String())
	}

	<-ctx.Done()
	o.Log.Info("worker stopping")
	sched.Shutdown()
	mediaSrv.Shutdown() // each blocks until its running jobs finish or ShutdownTimeout passes
	srv.Shutdown()
	if ops != nil {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := ops.Shutdown(sctx); err != nil {
			return err
		}
		if err := <-opsDone; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	return nil
}

// uniqueTTL is just under the cron period.
func uniqueTTL(spec string) time.Duration {
	var d time.Duration
	if len(spec) > len("@every ") {
		d, _ = time.ParseDuration(spec[len("@every "):])
	}
	if d <= time.Second {
		return time.Second
	}
	return d - time.Second
}

// asynqLogger adapts slog to asynq's Logger.
type asynqLogger struct{ l *slog.Logger }

func (a asynqLogger) Debug(args ...any) { a.l.Debug(fmt.Sprint(args...)) }
func (a asynqLogger) Info(args ...any)  { a.l.Info(fmt.Sprint(args...)) }
func (a asynqLogger) Warn(args ...any)  { a.l.Warn(fmt.Sprint(args...)) }
func (a asynqLogger) Error(args ...any) { a.l.Error(fmt.Sprint(args...)) }
func (a asynqLogger) Fatal(args ...any) { a.l.Error(fmt.Sprint(args...)) }
