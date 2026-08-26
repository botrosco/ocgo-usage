package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"
)

// renderJSON writes the usage payload as indented JSON.
func renderJSON(w io.Writer, u *Usage) error {
	data, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

// renderOneLine writes a minimal single-line summary of the three windows,
// e.g. "rolling 59% | weekly 58% | monthly 32%" — plain text, script-friendly.
func renderOneLine(w io.Writer, u *Usage) {
	ws := u.Usage
	fmt.Fprintf(w, "rolling %d%% | weekly %d%% | monthly %d%%\n",
		ws.Rolling.Percent, ws.Weekly.Percent, ws.Monthly.Percent)
}

// renderHuman writes the human-readable report. src carries key + origin.
func renderHuman(w io.Writer, u *Usage, endpoint string, src keySource, fetchedAt time.Time, limit int) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "OpenCode Go usage\n")
	fmt.Fprintf(tw, "  endpoint\t%s\n", endpoint)
	fmt.Fprintf(tw, "  api key\t%s (%s)\n", maskKey(src.key), src.label)
	fmt.Fprintf(tw, "  fetched\t%s\n", fetchedAt.UTC().Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(tw, "\n")
	fmt.Fprintf(tw, "  bucket\tstatus\tusage\tresets in\n")
	ws := u.Usage
	fmt.Fprintf(tw, "  rolling\t%s\t%3d%%\t%s\n", statusCell(ws.Rolling, limit), ws.Rolling.Percent, formatDuration(ws.Rolling.ResetsIn()))
	fmt.Fprintf(tw, "  weekly\t%s\t%3d%%\t%s\n", statusCell(ws.Weekly, limit), ws.Weekly.Percent, formatDuration(ws.Weekly.ResetsIn()))
	fmt.Fprintf(tw, "  monthly\t%s\t%3d%%\t%s\n", statusCell(ws.Monthly, limit), ws.Monthly.Percent, formatDuration(ws.Monthly.ResetsIn()))
	fmt.Fprintf(tw, "\n")
	tw.Flush()

	for _, alert := range alerts(u, limit) {
		fmt.Fprintln(w, alert)
	}
}

// statusCell renders the window status with ANSI colour when the output is a
// terminal: green for ok, yellow near the limit, red for rate-limited.
func statusCell(w Window, limit int) string {
	text := w.Status
	if !w.OK() {
		text = "rate-limited"
		return colorize("31", text)
	}
	if w.Percent >= limit || (limit < 100 && w.Percent >= 80) {
		return colorize("33", text)
	}
	return colorize("32", text)
}

// colorize wraps s in an ANSI color code; no colour when not a TTY.
func colorize(code, s string) string {
	if !stdoutTTY {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// alerts returns human-readable warning lines for the current state.
func alerts(u *Usage, limit int) []string {
	var out []string
	var parts []string
	if rl := u.RateLimited(); rl != nil {
		parts = append(parts, "a window is RATE-LIMITED (requests for this bucket are being rejected until reset)")
	}
	if m := u.MaxPercent(); m >= limit {
		parts = append(parts, fmt.Sprintf("usage at %d%% (limit %d%%)", m, limit))
	}
	if len(parts) > 0 {
		out = append(out, "⚠ "+strings.Join(parts, "; "))
	}
	return out
}

// formatDuration renders a duration as "Xd Yh Zm" (or "Ym Zs" under an hour).
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	case mins > 0:
		return fmt.Sprintf("%dm %ds", mins, secs)
	default:
		return fmt.Sprintf("%ds", secs)
	}
}
