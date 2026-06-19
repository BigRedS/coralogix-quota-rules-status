# quota-rules-status web UI

A tiny localhost web UI for [quota-rules-status](../README.md): pick a region, paste
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

The build context is the **repo root** (the webui imports `internal/quotarules`).

```sh
# from the repo root
docker build -f webui/Dockerfile -t quota-rules-status-webui .
docker run --rm --read-only --cap-drop=ALL --security-opt=no-new-privileges \
    -p 8765:8765 quota-rules-status-webui
```

Or with compose (from this `webui/` directory):

```sh
docker compose up --build
```

## Deep links

The page reads `region` and `key` (or `api_key`) from the URL query string,
prefills the form, and — if a key is present — runs the check immediately:

```
http://localhost:8765/?region=eu2&key=cxup_xxxxxxxx
```

Handy for bookmarks. The key still reaches the server only via the form's POST,
not as a server-side GET parameter.

## Endpoints

| Method | Path           | Purpose                                            |
|--------|----------------|----------------------------------------------------|
| `GET`  | `/`            | the HTML page (accepts `?region=&key=` to prefill) |
| `GET`  | `/api/regions` | region codes + hosts for the dropdown              |
| `POST` | `/api/run`     | `{region, api_key}` → the report as JSON           |

## Security notes

- The API key is sent to this server and used only to call Coralogix. It is
  never stored or written to disk. Trust the host you point it at.
- A key in the URL (`?key=...`) ends up in browser history and may appear in any
  reverse-proxy access logs in front of the server. Prefer typing it for a
  hosted instance; deep links are most useful for a local server.
- The key needs `team-quota-rules:Read` plus data-usage/metrics read access.
