package http

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/andi-frame/lockedin/apps/server/internal/auth"
)

// metrics are per app instance (not the global registry) so tests can build many.
type metrics struct {
	reg      *prometheus.Registry
	duration *prometheus.HistogramVec
	inflight prometheus.Gauge
}

func newMetrics() *metrics {
	m := &metrics{
		reg: prometheus.NewRegistry(),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "tepati_http_request_duration_seconds",
			Help: "HTTP request latency by method, route pattern and status.",
			// Dense around the SLO (SPEC §10: p95 150 ms reads, 300 ms writes).
			Buckets: []float64{.005, .01, .025, .05, .1, .15, .25, .3, .5, 1, 2.5, 5},
		}, []string{"method", "route", "status"}),
		inflight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "tepati_http_requests_in_flight", Help: "Requests being served right now.",
		}),
	}
	m.reg.MustRegister(m.duration, m.inflight)
	return m
}

// routeSet remembers the registered route patterns, so a label is only ever one of
// them. Fiber leaves c.Route() pointing at a middleware or catch-all entry when a
// request never reached a handler (404, 401, 429), and those must not become labels.
type routeSet map[string]struct{}

func (rs routeSet) fill(app *fiber.App) {
	for _, r := range app.GetRoutes(true) {
		rs[r.Method+" "+r.Path] = struct{}{}
	}
}

// routeLabel is the matched route pattern, never the raw path: ids and invite tokens
// in paths would explode metric cardinality and leak secrets into logs.
func (rs routeSet) label(c fiber.Ctx) string {
	if r := c.Route(); r != nil {
		if _, ok := rs[c.Method()+" "+r.Path]; ok {
			return r.Path
		}
	}
	return "unrouted"
}

// clientIP is c.IP(), which is empty when a trusted proxy header is configured but a
// request arrives without it. An empty key would put every client in one rate-limit bucket.
func clientIP(c fiber.Ctx) string {
	if ip := c.IP(); ip != "" {
		return ip
	}
	return c.RequestCtx().RemoteIP().String()
}

// observe logs and times each request after its final status is known. It converts
// a returned error into the response itself so it can see that status. It sits
// outside recover on purpose, so panics are logged as the 500 they became.
func observe(log *slog.Logger, m *metrics, routes routeSet) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		m.inflight.Inc()
		err := c.Next()
		m.inflight.Dec()
		if err != nil {
			if herr := c.App().Config().ErrorHandler(c, err); herr != nil {
				return herr
			}
		}

		elapsed := time.Since(start)
		status := c.Response().StatusCode()
		route := routes.label(c)
		m.duration.WithLabelValues(c.Method(), route, strconv.Itoa(status)).Observe(elapsed.Seconds())

		attrs := []any{
			"request_id", requestid.FromContext(c),
			"method", c.Method(),
			"route", route,
			"status", status,
			"duration_ms", float64(elapsed.Microseconds()) / 1000,
			"ip", clientIP(c),
		}
		if user := auth.UserID(c); user != uuid.Nil {
			attrs = append(attrs, "user_id", user.String())
		}
		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case opsPath(c.Path()):
			level = slog.LevelDebug
		}
		log.Log(c.Context(), level, "request", attrs...)
		return nil
	}
}
