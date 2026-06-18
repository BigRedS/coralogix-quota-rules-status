// Package metricemit pushes the quota-rules-status figures to Coralogix as OTLP
// metrics. It is the only package that depends on the OpenTelemetry SDK, so the
// CLI and web UI stay dependency-light.
//
// We emit four gauges, each labelled with `team` and `rule`:
//   - quota_rules_usage_u      units used
//   - quota_rules_limit_u      units reserved for the rule
//   - quota_rules_available_u  units still available (limit - used)
//   - quota_rules_available_pc available headroom as a percentage
//
// The `rule` label is a quota rule's entity type (e.g. "logs", "olly"), plus
// the special buckets "_total" (the whole quota) and "_unassigned".
package metricemit

import (
	"context"
	"crypto/tls"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc/credentials"

	"coralogix-quota-rules-status/internal/quotarules"
)

// Config is everything needed to push metrics to a Coralogix team.
type Config struct {
	Endpoint        string // OTLP gRPC ingress endpoint, e.g. ingress.eu2.coralogix.com:443
	IngestKey       string // Send-Your-Data key (NOT the management API key)
	Team            string // value for the `team` label
	ApplicationName string // cx.application.name (Coralogix routing)
	SubsystemName   string // cx.subsystem.name (Coralogix routing)
}

// Emit pushes one set of gauge readings and blocks until they have been sent.
// It always flushes before returning, which is what makes it safe to call from
// AWS Lambda (the execution environment freezes the moment the handler returns,
// so background batch export would never run).
func Emit(ctx context.Context, cfg Config, rows []quotarules.MetricSeries) error {
	exp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
		otlpmetricgrpc.WithHeaders(map[string]string{"Authorization": "Bearer " + cfg.IngestKey}),
		otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{})),
	)
	if err != nil {
		return fmt.Errorf("creating OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("cx.application.name", cfg.ApplicationName),
			attribute.String("cx.subsystem.name", cfg.SubsystemName),
		),
	)
	if err != nil {
		return fmt.Errorf("building resource: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)),
	)
	// Shut the provider down no matter what, so the gRPC connection is closed.
	defer func() { _ = provider.Shutdown(ctx) }()

	meter := provider.Meter("quota-rules-status")

	usage, err := meter.Float64Gauge("quota_rules_usage_u")
	if err != nil {
		return fmt.Errorf("creating gauge: %w", err)
	}
	limit, err := meter.Float64Gauge("quota_rules_limit_u")
	if err != nil {
		return fmt.Errorf("creating gauge: %w", err)
	}
	availableU, err := meter.Float64Gauge("quota_rules_available_u")
	if err != nil {
		return fmt.Errorf("creating gauge: %w", err)
	}
	availablePc, err := meter.Float64Gauge("quota_rules_available_pc")
	if err != nil {
		return fmt.Errorf("creating gauge: %w", err)
	}

	for _, row := range rows {
		attrs := metric.WithAttributes(
			attribute.String("team", cfg.Team),
			attribute.String("rule", row.Rule),
		)
		usage.Record(ctx, row.UsageU, attrs)
		limit.Record(ctx, row.LimitU, attrs)
		availableU.Record(ctx, row.AvailableU, attrs)
		availablePc.Record(ctx, row.AvailablePc, attrs)
	}

	// Force the reader to collect and export now, then report any send error.
	if err := provider.ForceFlush(ctx); err != nil {
		return fmt.Errorf("sending metrics to %s: %w", cfg.Endpoint, err)
	}
	return nil
}
