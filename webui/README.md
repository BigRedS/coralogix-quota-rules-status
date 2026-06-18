# blocked-status web UI

A tiny localhost web UI for [blocked-status](../README.md): pick a region, paste
an API key, and see how close the team is to its capacity blocks today.

It serves a single embedded HTML page and one JSON endpoint. A check is three
quick API calls, so it runs synchronously — there are no background jobs, temp
files, or downloads, and the server never writes to disk.

## Run with Go

From the repo root:

```sh
go run ./webui                 # listens on localhost:8765
go run ./webui -listen :9000   # different address
```

Then open <http://localhost:8765>.

## Run with Docker

The build context is the **repo root** (the webui imports `internal/blockedstatus`).

```sh
# from the repo root
docker build -f webui/Dockerfile -t blocked-status-webui .
docker run --rm --read-only --cap-drop=ALL --security-opt=no-new-privileges \
    -p 8765:8765 blocked-status-webui
```

Or with compose (from this `webui/` directory):

```sh
docker compose up --build
```

## Endpoints

| Method | Path           | Purpose                                            |
|--------|----------------|----------------------------------------------------|
| `GET`  | `/`            | the HTML page                                      |
| `GET`  | `/api/regions` | region codes + hosts for the dropdown              |
| `POST` | `/api/run`     | `{region, api_key}` → the report as JSON           |

## Security notes

- The API key is sent to this local server and used only to call Coralogix. It
  is never stored or written to disk. Run this only on a machine you trust.
- The key needs `team-quota-rules:Read` plus data-usage/metrics read access.
