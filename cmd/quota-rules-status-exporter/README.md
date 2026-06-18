# quota-rules-status-exporter

Computes the [quota-rules-status](../../README.md) report and pushes it to Coralogix
as OTLP metrics. The same binary runs as a one-shot (cron) or as an AWS Lambda
(it detects the Lambda runtime automatically).

## Metrics

Four gauges, each labelled with `team` and `rule`:

| Metric | Meaning |
|--------|---------|
| `quota_rules_usage_u`      | units used today |
| `quota_rules_limit_u`      | units reserved for the rule |
| `quota_rules_available_u`  | units still available (`limit - used`; negative if over) |
| `quota_rules_available_pc` | headroom remaining as a percentage (100 = empty, 0 = blocked) |

The `rule` label is a quota rule's entity type (`logs`, `metrics`, `olly`, …),
plus two special buckets: `_total` (the whole quota) and `_unassigned` (the
leftover after the rules).

Alerting example: `quota_rules_available_pc{rule="logs"} < 10`.

## Configuration

Reading usage and pushing metrics use **different credentials** and may target
**different regions** (so the metrics can land in a different team).

| Flag | Env | Meaning |
|------|-----|---------|
| `-region`         | `CX_REGION`           | region to **read** usage from |
| `-api-key`        | `CX_API_KEY`          | management API key for reading (needs `team-quota-rules:Read` + usage/metrics read) |
| `-emit-region`    | `CX_EMIT_REGION`      | region to **send** metrics to (defaults to `-region`) |
| `-ingest-key`     | `CX_SEND_YOUR_DATA_KEY` | Send-Your-Data key for emitting |
| `-team`           | `CX_TEAM`             | value for the `team` label |
| `-cx-application` | `CX_APPLICATION_NAME` | `cx.application.name` (default `quota-rules-status`) |
| `-cx-subsystem`   | `CX_SUBSYSTEM_NAME`   | `cx.subsystem.name` (default `quota-rules`) |
| `-dry-run`        | —                     | print the metrics instead of pushing (no ingest key needed) |

## Run one-shot (cron)

```sh
go run ./cmd/quota-rules-status-exporter \
    -region eu2 -api-key "$MGMT_KEY" \
    -ingest-key "$INGEST_KEY" -team otel-demo

# see what would be sent, without sending:
go run ./cmd/quota-rules-status-exporter -region eu2 -api-key "$MGMT_KEY" -dry-run
```

As a container (build context is the repo root):

```sh
docker build -f cmd/quota-rules-status-exporter/Dockerfile -t quota-rules-status-exporter .
docker run --rm \
    -e CX_REGION=eu2 -e CX_API_KEY=... \
    -e CX_SEND_YOUR_DATA_KEY=... -e CX_TEAM=otel-demo \
    quota-rules-status-exporter
```

## Run on AWS Lambda

Build a `bootstrap` binary for the `provided.al2023` runtime and zip it:

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
    go build -trimpath -ldflags "-s -w" -o bootstrap ./cmd/quota-rules-status-exporter
zip function.zip bootstrap
```

Create the function (architecture `arm64`, runtime `provided.al2023`, handler
name is ignored for the custom runtime), set the env vars above, and trigger it
on a schedule with an **EventBridge** rule (e.g. `rate(1 hour)`).

There is a ready-made OpenTofu snippet that does all of that — see
[deploy/tf/](../../deploy/tf/).

No code change is needed for Lambda: when `AWS_LAMBDA_RUNTIME_API` is present the
binary starts the Lambda handler instead of running once. Metrics are flushed
synchronously before the handler returns, so the frozen execution environment
never drops them.
