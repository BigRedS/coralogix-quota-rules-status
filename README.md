# coralogix-quota-rules-status

A small Go tool that reports how close a Coralogix team is to its capacity
blocks. It talks straight to the Coralogix HTTP API (no `cx` CLI), so it only
needs a region name and an API key.

It fetches three things for **today** and does some simple arithmetic:

1. the **total daily quota** — from the PromQL metric `cx_data_plan_units_per_day`
2. the **quota allocation rules** — how the quota is carved up per entity type
3. **today's usage so far** — from the PromQL metric `cx_data_usage_units`,
   grouped by `entity_type`

and prints the proportion used of the total, of each rule, and of the leftover
"unassigned" quota (what the rules didn't reserve).

The usage metric is keyed by the same `entity_type` names the quota rules use
(`logs`, `metrics`, `spans`, `olly`, ...), so each rule is matched to its usage
directly — including non-pillar entity types like `olly`.

## Install

Prebuilt binaries for macOS, Linux (amd64, arm64, and armv7 for 32-bit
Raspberry Pi OS), and Windows (amd64) are attached to each
[GitHub release](../../releases); every push also builds them as workflow
artifacts. Each archive contains all three binaries: `quota-rules-status`
(CLI), `quota-rules-status-exporter`, and `quota-rules-status-webui`.

Docker images for the web UI and the exporter are published to GHCR
(`ghcr.io/<owner>/coralogix-quota-rules-status-webui` and
`-exporter`, multi-arch amd64/arm64).

## Build

```sh
go build -o quota-rules-status ./cmd/quota-rules-status
```

## Run

```sh
./quota-rules-status -region eu2 -api-key <your-api-key>
```

Or without building first:

```sh
go run ./cmd/quota-rules-status -region eu2 -api-key <your-api-key>
```

There is also a small web UI — see [webui/](webui/).

The API key can also come from the `CX_API_KEY` environment variable:

```sh
export CX_API_KEY=<your-api-key>
./quota-rules-status -region eu2
```


### Required API key permissions

- `team-quota-rules:Read` — to read the quota allocation rules
- read access to data usage and metrics (for usage and the total quota)

## Example output

```
Quota-rules status report (today)
=================================================

Total quota: 1.17 units used of 15.00 (7.8%)

Per quota-rule:
  browserLogs/v2               0.00 used of 4.38 (0.0%)
  engineQueries                0.00 used of 3.62 (0.0%)
  logs                         0.62 used of 1.00 (61.5%)
  metrics                      0.55 used of 3.00 (18.5%)
  olly                         0.00 used of 3.00 (0.0%)

Unassigned: rules reserve the whole quota; no unassigned headroom (0.00 units used outside any rule)

Blocked today: 0.00 units
```

## Notes

- A rule's `allocation` is read as **units** when its `allocationType` is
  `LOCKED_UNITS`, and as a **percentage of the total quota** when `PERCENTAGE`.
- Only **enabled** rules count toward reservations.
- `cx_data_usage_units` is a running daily total (it resets at the start of the
  day), so the instant value is "today so far".
- When the rules reserve the whole quota (percentages summing to ~100%), there
  is no unassigned headroom, and the tool says so rather than dividing by zero.

## Emitting metrics

There is also an exporter that pushes these figures to Coralogix as OTLP metrics
(per-rule, plus `_total` and `_unassigned`), runnable from cron or AWS Lambda —
see [cmd/quota-rules-status-exporter/](cmd/quota-rules-status-exporter/).

## How it fits together

| Path                        | Job                                                       |
|-----------------------------|-----------------------------------------------------------|
| `cmd/quota-rules-status/`       | the CLI: flags, orchestration, printing                   |
| `cmd/quota-rules-status-exporter/` | pushes the report to Coralogix as OTLP metrics (cron/Lambda) |
| `internal/quotarules/client.go` | the HTTP calls (PromQL metrics + quota rules) and types |
| `internal/quotarules/calc.go`   | the proportion math + the metric series it emits   |
| `internal/quotarules/regions.go`| region name → API host and ingress endpoint       |
| `internal/metricemit/`      | the OTLP push (the only package that imports the OTel SDK) |
| `webui/`                    | a small web UI that reuses `internal/quotarules`       |
