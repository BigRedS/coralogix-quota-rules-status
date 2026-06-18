// Command quota-rules-status-exporter computes the quota-rules-status report and pushes
// it to Coralogix as OTLP metrics. It is built to run in two ways from the same
// binary:
//
//   - cron / one-shot: run it, it emits once and exits (good for a k8s CronJob
//     or systemd timer).
//   - AWS Lambda: when AWS_LAMBDA_RUNTIME_API is set in the environment it
//     starts the Lambda handler instead, so an EventBridge schedule can drive it.
//
// Reading usage and pushing metrics use *different* credentials and can target
// *different* regions: we read with the management API key from one region, and
// send metrics with a Send-Your-Data key to another region's ingress endpoint
// (so the metrics can land in a different team).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"

	"coralogix-quota-rules-status/internal/metricemit"
	"coralogix-quota-rules-status/internal/quotarules"
)

// config holds everything the exporter needs, gathered from flags/env.
type config struct {
	readRegion string // region we read usage from (management API)
	apiKey     string // management API key (reads quota rules + usage)
	emitRegion string // region we push metrics to (ingress); defaults to readRegion
	ingestKey  string // Send-Your-Data key (pushes metrics)
	team       string // value for the `team` metric label

	appName       string // cx.application.name
	subsystemName string // cx.subsystem.name

	dryRun bool // compute and print the metrics instead of pushing them
}

func main() {
	cfg := configFromFlags()

	// One place that does the actual work, shared by both run modes.
	run := func(ctx context.Context) error { return runOnce(ctx, cfg) }

	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		// Running inside Lambda: hand control to the runtime. The handler
		// ignores its event payload — the schedule is the only trigger.
		lambda.Start(func(ctx context.Context) error { return run(ctx) })
		return
	}

	// cron / local: run once and exit.
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func configFromFlags() config {
	var cfg config
	flag.StringVar(&cfg.readRegion, "region", os.Getenv("CX_REGION"), "region to read usage from: eu1, eu2, us1, us2, us3, ap1, ap2, ap3")
	flag.StringVar(&cfg.apiKey, "api-key", os.Getenv("CX_API_KEY"), "management API key for reading (or CX_API_KEY)")
	flag.StringVar(&cfg.emitRegion, "emit-region", os.Getenv("CX_EMIT_REGION"), "region to send metrics to (defaults to -region)")
	flag.StringVar(&cfg.ingestKey, "send-your-data-key", os.Getenv("CX_SEND_YOUR_DATA_KEY"), "Send-Your-Data key for emitting metrics (or CX_SEND_YOUR_DATA_KEY)")
	flag.StringVar(&cfg.team, "team", os.Getenv("CX_TEAM"), "value for the `team` metric label")
	flag.StringVar(&cfg.appName, "application", envOr("CX_APPLICATION_NAME", "quota-rules-status"), "cx.application.name for the emitted metrics (or CX_APPLICATION_NAME)")
	flag.StringVar(&cfg.subsystemName, "subsystem", envOr("CX_SUBSYSTEM_NAME", "quota-rules"), "cx.subsystem.name for the emitted metrics (or CX_SUBSYSTEM_NAME)")
	flag.BoolVar(&cfg.dryRun, "dry-run", false, "print the metrics that would be emitted instead of pushing them (no key needed)")
	flag.Parse()

	if cfg.emitRegion == "" {
		cfg.emitRegion = cfg.readRegion
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// runOnce reads the report and pushes it to Coralogix as metrics.
func runOnce(ctx context.Context, cfg config) error {
	// A dry run needs neither an ingest key nor a team label.
	needEmit := !cfg.dryRun
	if cfg.readRegion == "" || cfg.apiKey == "" || (needEmit && (cfg.ingestKey == "" || cfg.team == "")) {
		return fmt.Errorf("need -region and -api-key (plus -send-your-data-key and -team unless -dry-run); flags or CX_* env vars")
	}

	host, err := quotarules.HostForRegion(cfg.readRegion)
	if err != nil {
		return err
	}
	endpoint, err := quotarules.IngressEndpoint(cfg.emitRegion)
	if err != nil {
		return err
	}

	client := quotarules.NewClient(host, cfg.apiKey)
	report, err := quotarules.FetchReport(client)
	if err != nil {
		return fmt.Errorf("fetching report: %w", err)
	}

	rows := report.MetricSeriesList()

	if cfg.dryRun {
		printSeries(cfg, endpoint, rows)
		return nil
	}

	err = metricemit.Emit(ctx, metricemit.Config{
		Endpoint:        endpoint,
		IngestKey:       cfg.ingestKey,
		Team:            cfg.team,
		ApplicationName: cfg.appName,
		SubsystemName:   cfg.subsystemName,
	}, rows)
	if err != nil {
		return err
	}

	log.Printf("emitted %d rules (+ _total, _unassigned) for team %q to %s", len(report.Rules), cfg.team, endpoint)
	return nil
}

// printSeries shows what a real run would push, without sending anything.
func printSeries(cfg config, endpoint string, rows []quotarules.MetricSeries) {
	fmt.Printf("dry run — would push to %s as team=%q (app=%q subsystem=%q)\n\n",
		endpoint, cfg.team, cfg.appName, cfg.subsystemName)
	fmt.Printf("%-16s %12s %12s %12s %12s\n", "rule", "usage_u", "limit_u", "available_u", "available_pc")
	for _, r := range rows {
		fmt.Printf("%-16s %12.4f %12.4f %12.4f %12.1f\n",
			r.Rule, r.UsageU, r.LimitU, r.AvailableU, r.AvailablePc)
	}
}
