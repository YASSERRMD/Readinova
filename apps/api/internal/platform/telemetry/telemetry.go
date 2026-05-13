// Package telemetry wires up OpenTelemetry traces and a Prometheus metrics exporter.
package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer      trace.Tracer
	httpCounter metric.Int64Counter
	httpLatency metric.Float64Histogram
)

// Setup initialises OTel traces (stdout exporter for dev) and a Prometheus metrics exporter.
// Returns a shutdown function that flushes and closes all providers.
func Setup(ctx context.Context, serviceName, version string) (shutdown func(context.Context) error, err error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("resource: %w", err)
	}

	// Trace provider (no-op exporter for dev; swap for OTLP in production).
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	tracer = otel.Tracer(serviceName)

	// Metric provider backed by Prometheus.
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("prometheus exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(promExporter),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	meter := otel.Meter(serviceName)
	httpCounter, err = meter.Int64Counter("http_requests_total",
		metric.WithDescription("Total HTTP requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("counter: %w", err)
	}
	httpLatency, err = meter.Float64Histogram("http_request_duration_seconds",
		metric.WithDescription("HTTP request latency"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("histogram: %w", err)
	}

	return func(ctx context.Context) error {
		_ = tp.Shutdown(ctx)
		return mp.Shutdown(ctx)
	}, nil
}

// Middleware wraps an http.Handler with OTel tracing and Prometheus metrics.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Start a span.
		ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
		)

		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))

		duration := time.Since(start).Seconds()
		attrs := []attribute.KeyValue{
			attribute.String("method", r.Method),
			attribute.String("path", r.URL.Path),
			attribute.Int("status", rw.status),
		}

		if httpCounter != nil {
			httpCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
		if httpLatency != nil {
			httpLatency.Record(ctx, duration, metric.WithAttributes(attrs...))
		}

		span.SetAttributes(attribute.Int("http.status_code", rw.status))
	})
}

// MetricsHandler returns a Prometheus /metrics HTTP handler.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
