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
  "usage": {
    "rolling": { "status": "ok", "percent": 24, "resetsAt": "..." },
    "weekly":  { "status": "ok", "percent": 41, "resetsAt": "..." },
    "monthly": { "status": "ok", "percent": 12, "resetsAt": "..." }
  }
}
```

`status` is `"ok"` or `"rate-limited"`; `percent` is 0–100 of the window
limit; `resetsAt` is an RFC3339 timestamp of the window reset.

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

## License

[GPL-3.0](LICENSE).

## pi integration

The repo is a [pi package](https://github.com/earendil-works/pi) — it ships a
extension that shows Go usage in the pi footer plus the `ocgo-usage` skill.

### Footer status extension (`extensions/ocgo-usage.ts`)

After key pi actions — session start, completed turns, settled agent runs,
model changes, thinking-level changes — the extension polls the usage endpoint
via the `ocgo-usage` CLI and renders the three windows in the footer:

```
Go R59% W58% M32%
```

Colour-coded like the CLI (green < 75%, orange 75–89%, red ≥ 90%), `⚠` marks a
rate-limited window. Polls are throttled to one per 60s (event-driven, no
background timers), so busy turns can't hammer the endpoint. One-shot
notifications fire when the highest window crosses the alert threshold or any
window rate-limits.

Install (needs the binary on PATH, see above). Link or copy the extension
into a pi extension dir, then `/reload`:

```sh
mkdir -p ~/.pi/agent/extensions
ln -s "$(pwd)/extensions/ocgo-usage.ts" ~/.pi/agent/extensions/
# or reference it from ~/.pi/agent/settings.json:
#   "extensions": ["/absolute/path/to/extensions/ocgo-usage.ts"]
```

Reload with `/reload`. Flags:

- `--ocgo-usage-interval <seconds>` — minimum seconds between polls (default 60)
- `--ocgo-usage-alert <percent>` — notify when the highest window crosses this
  percentage (default 90)

Alternatively install the whole repo as a package to get extension + skill:

```sh
pi install .    # from a checkout of this repo
```