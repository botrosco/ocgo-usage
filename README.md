# ocgo-usage

A small dependency-free CLI that polls the OpenCode Zen **Go usage** endpoint
(`GET /zen/go/v1/usage`, [anomalyco/opencode#16513](https://github.com/anomalyco/opencode/pull/16513))
and shows the rolling / weekly / monthly rate-window state for your OpenCode Go
subscription.

## Endpoint

```
GET https://opencode.ai/zen/go/v1/usage
Authorization: Bearer <api-key>
```

The console answers with the same data as the zen workspace dashboard:

```json
{
  "useBalance": false,
  "rollingUsage": { "status": "ok", "resetInSec": 12400, "usagePercent": 24 },
  "weeklyUsage":  { "status": "ok", "resetInSec": 345000, "usagePercent": 41 },
  "monthlyUsage": { "status": "ok", "resetInSec": 1040000, "usagePercent": 12 }
}
```

`status` is `"ok"` or `"rate-limited"`; `usagePercent` is 0–100 of the window
limit; `resetInSec` counts down to the window reset.

## API key resolution (in order)

1. `--api-key` flag
2. `OPENCODE_API_KEY` environment variable
3. **pi agent auth** — `~/.pi/agent/auth.json`, entry `opencode-go` (then `opencode`),
   for `api_key`/`api` type entries the `key` field, for `oauth` entries the `access` token
4. **opencode CLI auth** — `~/.local/share/opencode/auth.json`, entry `opencode`

Override the pi auth file location with `--auth-file`.

## Usage

```sh
# one-shot report (default: one-line summary, colour-coded percents on a TTY)
ocgo-usage

# full multi-line report (endpoint, key, resets, alerts)
ocgo-usage --full

# machine-readable
ocgo-usage --json

# poll every 60s
ocgo-usage --watch --interval 60

# alert exit code when any window hits 80% (for cron/scripts)
ocgo-usage --limit 80

# explicit key / different console
ocgo-usage --api-key sk-... --url https://opencode.ai/zen/go/v1/usage
```

Exit codes: `0` ok · `1` usage at/over `--limit` or rate-limited · `2` error
(no key, auth failure, network, bad response).

## Install

Requires Go ≥ 1.24 (no external dependencies).

```sh
go build -o ~/.local/bin/ocgo-usage .   # single binary on PATH
```

Rebuild from the repo and re-copy to upgrade.

```sh
go build -o ocgo-usage . && cp ocgo-usage ~/.local/bin/
go test ./...
```