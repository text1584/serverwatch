# serverwatch

Long-running monitor for an Apache `mod_status` / `server-status` endpoint.

The important parser behavior is that it **does not scan arbitrary slash-delimited strings**. It only accepts request lines shaped like:

    METHOD /path HTTP/1.1

or absolute-form HTTP request targets.

That prevents values such as:

    232802/232802
    336415/336415

from becoming fake endpoints.

## Build

```bash
go test ./...
go build -o serverwatch .
```

## Run

```bash
./serverwatch -url https://target.example/server-status
```

Useful options:

```text
-interval 30s
-timeout 15s
-output output
-include-static
```

By default, static assets are parsed but omitted from the endpoint list. Use
`-include-static` when you also want JavaScript, CSS, images, fonts, and maps.

## Output

```text
output/
  state.json
  endpoints.json
  endpoints.txt
  live_endpoints.json
  live_endpoints.txt
  new_endpoints.txt
  snapshot_history.jsonl
  findings.json
  headers.json
  server_info.json
```

`live_endpoints.*` represents endpoints observed in the latest server-status
snapshot. A missing endpoint is not deleted from history; it simply stops being
marked live.

`new_endpoints.txt` is append-only and records an endpoint the first time the
monitor sees that method + path (+ query) combination.

Use this only against systems you are authorized to monitor.
