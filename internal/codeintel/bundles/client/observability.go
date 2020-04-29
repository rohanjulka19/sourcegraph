package client

import (
	"context"
	"io"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sourcegraph/sourcegraph/internal/trace"
)

// ErrorLogger captures the method required for logging an error.
type ErrorLogger interface {
	Error(msg string, ctx ...interface{})
}

// OperationMetrics contains three common metrics for any operation.
type OperationMetrics struct {
	Duration *prometheus.HistogramVec // How long did it take?
	Count    *prometheus.CounterVec   // How many things were processed?
	Errors   *prometheus.CounterVec   // How many errors occurred?
}

// Observe registers an observation of a single operation.
func (m *OperationMetrics) Observe(secs, count float64, err error, lvals ...string) {
	if m == nil {
		return
	}

	m.Duration.WithLabelValues(lvals...).Observe(secs)
	m.Count.WithLabelValues(lvals...).Add(count)
	if err != nil {
		m.Errors.WithLabelValues(lvals...).Add(1)
	}
}

// MustRegister registers all metrics in OperationMetrics in the given prometheus.Registerer.
// It panics in case of failure.
func (m *OperationMetrics) MustRegister(r prometheus.Registerer) {
	r.MustRegister(m.Duration)
	r.MustRegister(m.Count)
	r.MustRegister(m.Errors)
}

// ClientMetrics encapsulates the Prometheus metrics of a Client.
type ClientMetrics struct {
	SendUpload  *OperationMetrics
	GetUpload   *OperationMetrics
	SendDB      *OperationMetrics
	QueryBundle *OperationMetrics
}

// NewClientMetrics returns ClientMetrics that need to be registered in a Prometheus registry.
func NewClientMetrics(subsystem string) ClientMetrics {
	return ClientMetrics{
		SendUpload: &OperationMetrics{
			Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "src",
				Subsystem: subsystem,
				Name:      "bundle_client_query_send_upload_duration_seconds",
				Help:      "Time spent performing send upload queries",
			}, []string{}),
			Count: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "src",
				Subsystem: subsystem,
				Name:      "bundle_client_query_send_upload_total",
				Help:      "Total number of send upload queries",
			}, []string{}),
			Errors: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "src",
				Subsystem: subsystem,
				Name:      "bundle_client_query_send_upload_errors_total",
				Help:      "Total number of errors when performing send upload queries",
			}, []string{}),
		},
		GetUpload: &OperationMetrics{
			Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "src",
				Subsystem: subsystem,
				Name:      "bundle_client_query_get_upload_duration_seconds",
				Help:      "Time spent performing get upload queries",
			}, []string{}),
			Count: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "src",
				Subsystem: subsystem,
				Name:      "bundle_client_query_get_upload_total",
				Help:      "Total number of get upload queries",
			}, []string{}),
			Errors: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "src",
				Subsystem: subsystem,
				Name:      "bundle_client_query_get_upload_errors_total",
				Help:      "Total number of errors when performing get upload queries",
			}, []string{}),
		},
		SendDB: &OperationMetrics{
			Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "src",
				Subsystem: subsystem,
				Name:      "bundle_client_query_send_db_duration_seconds",
				Help:      "Time spent performing send db queries",
			}, []string{}),
			Count: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "src",
				Subsystem: subsystem,
				Name:      "bundle_client_query_send_db_total",
				Help:      "Total number of send db queries",
			}, []string{}),
			Errors: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "src",
				Subsystem: subsystem,
				Name:      "bundle_client_query_send_db_errors_total",
				Help:      "Total number of errors when performing send db queries",
			}, []string{}),
		},
		QueryBundle: &OperationMetrics{
			Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Namespace: "src",
				Subsystem: subsystem,
				Name:      "bundle_client_query_bundle_duration_seconds",
				Help:      "Time spent performing bundle queries",
			}, []string{}),
			Count: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "src",
				Subsystem: subsystem,
				Name:      "bundle_client_query_bundle_total",
				Help:      "Total number of bundle queries",
			}, []string{}),
			Errors: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: "src",
				Subsystem: subsystem,
				Name:      "bundle_client_query_bundle_errors_total",
				Help:      "Total number of errors when performing bundle queries",
			}, []string{}),
		},
	}
}

// An ObservedClient wraps another Client with error logging, Prometheus metrics, and tracing.
type ObservedClient struct {
	base    ClientBase
	logger  ErrorLogger
	metrics ClientMetrics
	tracer  trace.Tracer
}

var _ ClientBase = &ObservedClient{}

// NewObservedClient wraps the given ClientBase with error logging, Prometheus metrics, and tracing.
func NewObservedClient(base ClientBase, logger ErrorLogger, metrics ClientMetrics, tracer trace.Tracer) Client {
	return &ObservedClient{
		base:    base,
		logger:  logger,
		metrics: metrics,
		tracer:  tracer,
	}
}

func (c *ObservedClient) BundleClient(bundleID int) BundleClient {
	// Override the default so we get the instrumented QueryBundle
	return &bundleClientImpl{base: c, bundleID: bundleID}
}

func (c *ObservedClient) SendUpload(ctx context.Context, bundleID int, r io.Reader) (err error) {
	tr, ctx := c.tracer.New(ctx, "Client.SendUpload", "")
	defer func(began time.Time) {
		secs := time.Since(began).Seconds()
		c.metrics.SendUpload.Observe(secs, 1, err)
		log(c.logger, "client.send-upload", err)
		tr.SetError(err)
		tr.Finish()
	}(time.Now())

	return c.base.SendUpload(ctx, bundleID, r)
}

func (c *ObservedClient) GetUpload(ctx context.Context, bundleID int, dir string) (_ string, err error) {
	tr, ctx := c.tracer.New(ctx, "Client.GetUpload", "")
	defer func(began time.Time) {
		secs := time.Since(began).Seconds()
		c.metrics.GetUpload.Observe(secs, 1, err)
		log(c.logger, "client.get-upload", err)
		tr.SetError(err)
		tr.Finish()
	}(time.Now())

	return c.base.GetUpload(ctx, bundleID, dir)
}

func (c *ObservedClient) SendDB(ctx context.Context, bundleID int, r io.Reader) (err error) {
	tr, ctx := c.tracer.New(ctx, "Client.SendDB", "")
	defer func(began time.Time) {
		secs := time.Since(began).Seconds()
		c.metrics.SendDB.Observe(secs, 1, err)
		log(c.logger, "client.send-db", err)
		tr.SetError(err)
		tr.Finish()
	}(time.Now())

	return c.base.SendDB(ctx, bundleID, r)
}

func (c *ObservedClient) QueryBundle(ctx context.Context, bundleID int, op string, qs map[string]interface{}, target interface{}) (err error) {
	tr, ctx := c.tracer.New(ctx, "Client.QueryBundle", "")
	defer func(began time.Time) {
		secs := time.Since(began).Seconds()
		c.metrics.QueryBundle.Observe(secs, 1, err)
		log(c.logger, "client.query-bundle", err)
		tr.SetError(err)
		tr.Finish()
	}(time.Now())

	return c.base.QueryBundle(ctx, bundleID, op, qs, &target)
}

func log(lg ErrorLogger, msg string, err error, ctx ...interface{}) {
	if err == nil {
		return
	}

	lg.Error(msg, append(append(make([]interface{}, 0, len(ctx)+2), "error", err), ctx...)...)
}
