---
name: ocgo-usage
description: Check OpenCode Go subscription usage (rolling/weekly/monthly rate windows) by polling the zen usage endpoint with the ocgo-usage CLI. Use when asked to check opencode go usage, limits, rate-limit status, how much of the Go allowance is used, when it resets, or to watch/alert on usage crossing a threshold.
---

# ocgo-usage

Checks OpenCode Go usage with the `ocgo-usage` CLI — a zero-dependency Go
binary installed at `~/.local/bin/ocgo-usage` (on PATH), polling
`GET https://opencode.ai/zen/go/v1/usage` (the merged
[anomalyco/opencode#16513](https://github.com/anomalyco/opencode/pull/16513)
endpoint). Reports the three rate windows: **rolling**, **weekly**, **monthly**,
each with status (`ok` / `rate-limited`), usage percent (0–100), and time until
reset.

API key resolution (automatic, do not ask the user for a key):
`--api-key` flag → `OPENCODE_API_KEY` → pi auth `~/.pi/agent/auth.json`
(entry `opencode-go` or `opencode`) → opencode CLI
`~/.local/share/opencode/auth.json`. Never print a raw key — the CLI displays it
masked (`sk-…WzDYm`).

## In pi (footer extension)

The repo ships `extensions/ocgo-usage.ts` (see README "pi integration"): after
key pi actions it polls the same endpoint and shows `Go R… W… M…` in the pi
footer (throttled to one poll/60s). If it's not shown in your footer, install
it per the README and `/reload`. For an immediate one-shot, use the CLI below.

## Running

The CLI is a single binary on PATH — invoke it directly, or via the wrapper
script (identical):

```bash
ocgo-usage [flags]
bash scripts/run.sh [flags]
```

### Installing / upgrading

From a checkout of this repo:

```bash
go build -o ~/.local/bin/ocgo-usage .   # or: cp ocgo-usage ~/.local/bin/
```
(Needs any Go ≥ 1.24 toolchain on PATH.)

### Flags

```bash
ocgo-usage                  # one-shot report (Rolling: x% | Weekly: x% | Monthly: x%)
ocgo-usage --full           # full multi-line report (resets, alerts, key origin)
ocgo-usage --watch          # poll continuously (Ctrl-C to stop)
ocgo-usage --interval 60    # seconds between polls (default 30)
ocgo-usage --limit 80       # exit 1 when any window's usage % >= 80
ocgo-usage --json           # raw JSON payload (machine-readable)
ocgo-usage --quiet          # no output, exit code only (cron/dashboards)
ocgo-usage --api-key sk-…   # override key (usually unnecessary)
ocgo-usage --url <base>     # non-default console (enterprise)
```

Exit codes: `0` ok · `1` at/over `--limit` or rate-limited · `2` error
(no key, 401 bad key, 403 no Go subscription, network, bad response).

## Interpreting results

- default one-line output colour-codes each percent on a TTY: green < 75%,
  orange 75–89%, red ≥ 90% (plain text when piped)

| Window | Meaning | Resets |
|--------|---------|--------|
| rolling | usage in the current sliding window (e.g. 4h) | rolling clock |
| weekly | usage in the current calendar week | week boundary |
| monthly | usage in the monthly billing cycle | cycle boundary |

- `status: rate-limited` (percent 100) means requests in that window are being
  rejected until reset — draw attention to it.
- `--limit` is a hard threshold: usage **at or above** it triggers exit 1, so a
  threshold of 100 flags only true rate-limiting.
- `--json` returns `{"usage":{"rolling|weekly|monthly":{
  "status","percent","resetsAt"}}}` — `resetsAt` is an RFC3339 timestamp.

## Typical requests

- "check usage" → `ocgo-usage`
- "watch it" / "keep polling" → `ocgo-usage --watch --interval 30`
- "alert me at 90%" (script/cron) → `ocgo-usage --limit 90`
- feed a dashboard → `ocgo-usage --json`

After running, summarize: highest window + percent, each window's reset time,
and any rate-limited/alerting state. Keep the masked key/origin line from the
header if useful.