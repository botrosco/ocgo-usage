/**
 * SPDX-License-Identifier: GPL-3.0-only
 * ocgo-usage – OpenCode Go usage in the pi footer.
 *
 * Polls the OpenCode Go usage endpoint (via the `ocgo-usage` CLI, see
 * ../README.md) after key pi actions — session start, completed turns,
 * settled agent runs, model changes, thinking-level changes — and shows the
 * rolling / weekly / monthly usage percentages in the footer
 * (`ctx.ui.setStatus("ocgo-usage", ...)`).
 *
 * Purely event-driven: polls are throttled to one per
 * `--ocgo-usage-interval` seconds (default 60), so busy multi-turn actions
 * cannot hammer the endpoint. No background timers.
 *
 * Flags:
 *   --ocgo-usage-interval <seconds>  minimum seconds between polls (default 60)
 *   --ocgo-usage-alert  <percent>    notify once when the highest window's
 *                                    usage % crosses this threshold (default 90)
 *
 * Dependencies: needs the `ocgo-usage` binary on PATH (install with
 * `go build -o ~/.local/bin/ocgo-usage .` in the repo root).
 */

import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent";

interface UsageWindow {
	/** "ok" or "rate-limited". */
	status: string;
	/** 0..100 of the window's limit (100 when rate-limited). */
	percent: number;
	/** RFC3339 timestamp of the window reset. */
	resetsAt: string;
}

interface UsagePayload {
	usage?: {
		rolling?: UsageWindow;
		weekly?: UsageWindow;
		monthly?: UsageWindow;
	};
}

const STATUS_ID = "ocgo-usage";
const CLI = "ocgo-usage";
const POLL_TIMEOUT_MS = 15_000;

function parsePositiveInt(value: string | undefined, fallback: number): number {
	const n = Number.parseInt(value ?? "", 10);
	return Number.isFinite(n) && n > 0 ? n : fallback;
}

function parsePercent(value: string | undefined, fallback: number): number {
	const n = Number.parseInt(value ?? "", 10);
	return Number.isFinite(n) && n >= 0 && n <= 100 ? n : fallback;
}

export default function (pi: ExtensionAPI) {
	pi.registerFlag("ocgo-usage-interval", { type: "string", default: "60" });
	pi.registerFlag("ocgo-usage-alert", { type: "string", default: "90" });

	const minIntervalMs = parsePositiveInt(pi.getFlag("ocgo-usage-interval"), 60) * 1000;
	const alertPercent = parsePercent(pi.getFlag("ocgo-usage-alert"), 90);

	let lastPollMs = 0;
	let inFlight = false;
	let lastMaxPercent = -1;
	let wasRateLimited = false;

	/** Compact colored rendering of a single window (percent + rate-limit flag). */
	function fmtWindow(theme: ExtensionContext["ui"]["theme"], w: UsageWindow | undefined): string {
		if (!w) return theme.fg("dim", "–");
		const pct = w.percent;
		const color = pct >= 90 ? "error" : pct >= 75 ? "warning" : "success";
		const rl = w.status === "rate-limited";
		return theme.fg(color, `${pct}%`) + (rl ? theme.fg("error", "⚠") : "");
	}

	/** Render the three windows into the footer status. */
	function render(ctx: ExtensionContext, usage: UsagePayload["usage"]): void {
		const theme = ctx.ui.theme;
		const text = [
			`R${fmtWindow(theme, usage?.rolling)}`,
			`W${fmtWindow(theme, usage?.weekly)}`,
			`M${fmtWindow(theme, usage?.monthly)}`,
		].join(" ");
		ctx.ui.setStatus(STATUS_ID, theme.fg("accent", "Go") + " " + text);
	}

	/** One-shot notifications when usage crosses the alert threshold / rate-limits. */
	function notifyOnCrossing(ctx: ExtensionContext, usage: UsagePayload["usage"]): void {
		const max = Math.max(
			usage?.rolling?.percent ?? 0,
			usage?.weekly?.percent ?? 0,
			usage?.monthly?.percent ?? 0,
		);
		const rateLimited =
			usage?.rolling?.status === "rate-limited" ||
			usage?.weekly?.status === "rate-limited" ||
			usage?.monthly?.status === "rate-limited";

		if (rateLimited && !wasRateLimited) {
			ctx.ui.notify("OpenCode Go: a usage window is rate-limited!", "error");
		}
		if (max >= alertPercent && lastMaxPercent < alertPercent) {
			ctx.ui.notify(`OpenCode Go usage at ${max}% (alert threshold ${alertPercent}%)`, "warning");
		}
		lastMaxPercent = max;
		wasRateLimited = rateLimited;
	}

	/** Compress the CLI's stderr message for a footer. */
	function shortError(stderr: string): string {
		const line = stderr.split(/\r?\n/).find((l) => l.trim()) ?? "poll failed";
		const msg = line.replace(/^ocgo-usage:\s*/, "").trim();
		if (/unauthorized|api key/i.test(msg)) return "bad key";
		if (/subscription/i.test(msg)) return "no Go sub";
		if (/no api key|no opencode|key/i.test(msg)) return "no key";
		return msg.length > 24 ? `${msg.slice(0, 24)}…` : msg;
	}

	/** Poll usage once (throttled) and refresh the footer status. */
	async function poll(ctx: ExtensionContext): Promise<void> {
		if (!ctx.hasUI || inFlight) return; // footer only exists in TUI/RPC
		const now = Date.now();
		if (now - lastPollMs < minIntervalMs) return;

		inFlight = true;
		const theme = ctx.ui.theme;
		try {
			const res = await pi.exec(CLI, ["--json"], { timeout: POLL_TIMEOUT_MS });
			if (res.code === 2) {
				// CLI error: no key / unauthorized / no entitlement / network.
				ctx.ui.setStatus(STATUS_ID, theme.fg("error", `Go ✗ ${shortError(res.stderr)}`));
				return;
			}
			const payload = JSON.parse(res.stdout) as UsagePayload;
			if (!payload.usage) {
				ctx.ui.setStatus(STATUS_ID, theme.fg("error", "Go ✗ bad payload"));
				return;
			}
			render(ctx, payload.usage);
			notifyOnCrossing(ctx, payload.usage);
		} catch (err) {
			const msg = err instanceof Error ? err.message : String(err);
			const missing = /ENOENT|spawn .*ENOENT|not installed/i.test(msg);
			ctx.ui.setStatus(
				STATUS_ID,
				missing
					? theme.fg("error", "Go: CLI missing (`go build -o ~/.local/bin/ocgo-usage .`)")
					: theme.fg("error", "Go ✗ poll failed"),
			);
		} finally {
			inFlight = false;
			lastPollMs = Date.now();
		}
	}

	// ── Key actions that trigger a (throttled) poll ─────────────────────────
	pi.on("session_start", async (_event, ctx) => {
		void poll(ctx);
	});
	pi.on("turn_end", async (_event, ctx) => {
		void poll(ctx);
	});
	pi.on("agent_settled", async (_event, ctx) => {
		void poll(ctx);
	});
	pi.on("model_select", async (_event, ctx) => {
		void poll(ctx);
	});
	pi.on("thinking_level_select", async (_event, ctx) => {
		void poll(ctx);
	});
}