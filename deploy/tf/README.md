# OpenTofu: scheduled Lambda deploy

A small snippet that runs [quota-rules-status-exporter](../../cmd/quota-rules-status-exporter/)
as an AWS Lambda on a schedule. Defaults to `eu-north-1`. Apply it with
[OpenTofu](https://opentofu.org) (`tofu`).

## 1. Build the package (from the repo root)

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
    go build -trimpath -ldflags "-s -w" -o bootstrap ./cmd/quota-rules-status-exporter
zip function.zip bootstrap
```

## 2. Apply (from this directory)

```sh
tofu init
tofu apply \
    -var name=quota-rules-status \
    -var cx_region=eu2 \
    -var cx_team=otel-demo \
    -var cx_api_key=...   \
    -var cx_ingest_key=...
```

`name` is required and names both the Lambda function and the EventBridge
schedule rule. The secret vars are best supplied via `TF_VAR_cx_api_key` /
`TF_VAR_cx_ingest_key` env vars or a `*.tfvars` file rather than the command line.

## Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `name`                | —              | names the Lambda + the schedule rule (required) |
| `aws_region`          | `eu-north-1`   | AWS region to deploy into |
| `schedule_expression` | `rate(1 hour)` | how often to run |
| `package_file`        | `../../function.zip` | the built Lambda zip |
| `cx_region`           | —              | Coralogix region to read usage from |
| `cx_api_key`          | —              | management API key (read; sensitive) |
| `cx_ingest_key`       | —              | Send-Your-Data key (emit; sensitive) |
| `cx_team`             | —              | value for the `team` metric label |
| `cx_emit_region`      | `""`           | region to send metrics to (empty = same as `cx_region`) |
