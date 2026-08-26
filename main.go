// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Exit codes: 0 = ok, 1 = usage at/over limit or rate-limited, 2 = error.
const (
	exitOK    = 0
	exitAlert = 1
	exitError = 2

	defaultEndpoint = "https://opencode.ai/zen/go/v1/usage"
	defaultInterval = 30 // seconds
	defaultLimit    = 100
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ocgo-usage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		apiKey   = fs.String("api-key", "", "OpenCode Go API key (default: $"+envKeyVar+", then pi/opencode auth files)")
		authFile = fs.String("auth-file", piAuthPath(), "pi agent auth file to read an opencode-go key from")
		url      = fs.String("url", defaultEndpoint, "usage endpoint URL")
		watch    = fs.Bool("watch", false, "poll continuously until interrupted")
		interval = fs.Int("interval", defaultInterval, "seconds between polls in --watch mode")
		limit    = fs.Int("limit", defaultLimit, "exit 1 (one-shot) when any window's usage % reaches this; 0 disables")
		jsonOut  = fs.Bool("json", false, "print the raw JSON payload (one object per poll in --watch)")
		full     = fs.Bool("full", false, "print the full multi-line report (default is a one-line summary)")
		quiet    = fs.Bool("quiet", false, "suppress human output (exit code still reflects state)")
		version  = fs.Bool("version", false, "print version and exit")
	)
	fs.Usage = func() {
		fmt.Fprintf(stderr, `ocgo-usage – poll OpenCode Go usage from the console API

Poll GET %s (Bearer auth) and report rolling/weekly/monthly usage.

Usage: ocgo-usage [flags]

Flags:
`, defaultEndpoint)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return exitError
	}
	if *version {
		fmt.Fprintf(stdout, "ocgo-usage %s\n", versionString)
		return exitOK
	}
	if *limit < 0 || *limit > 100 {
		fmt.Fprintf(stderr, "--limit must be between 0 and 100\n")
		return exitError
	}
	if *interval < 1 {
		fmt.Fprintf(stderr, "--interval must be >= 1 second\n")
		return exitError
	}

	src, err := resolveKey(*apiKey, *authFile)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	endpoint := *url
	if !strings.HasPrefix(endpoint, "http") {
		endpoint = "https://" + endpoint
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if !*watch {
		return pollOnce(ctx, stdout, stderr, endpoint, src, *limit, *jsonOut, *full, *quiet)
	}

	// Watch mode: poll every interval. Ctrl-C exits 0.
	ticker := time.NewTicker(time.Duration(*interval) * time.Second)
	defer ticker.Stop()
	for {
		code := pollOnceWatch(ctx, stdout, stderr, endpoint, src, *limit, *jsonOut, *full, *quiet)
		if code == exitError {
			return exitError
		}
		select {
		case <-ctx.Done():
			return exitOK
		case <-ticker.C:
		}
	}
}

// pollOnce performs a single fetch and report; returns the process exit code.
func pollOnce(ctx context.Context, stdout, stderr io.Writer, endpoint string, src keySource, limit int, jsonOut, full, quiet bool) int {
	u, err := fetchUsage(ctx, endpoint, src.key)
	if err != nil {
		fmt.Fprintf(stderr, "ocgo-usage: %v\n", err)
		return exitError
	}
	if jsonOut {
		if err := renderJSON(stdout, u); err != nil {
			fmt.Fprintf(stderr, "ocgo-usage: %v\n", err)
			return exitError
		}
	} else if full {
		renderHuman(stdout, u, endpoint, src, time.Now(), limit)
	} else if !quiet {
		renderOneLine(stdout, u)
	}
	if limit > 0 && (u.RateLimited() != nil || u.MaxPercent() >= limit) {
		return exitAlert
	}
	return exitOK
}

// pollOnceWatch is like pollOnce but clears the screen first (when a TTY)
// and prints its own fetch-time header per poll.
func pollOnceWatch(ctx context.Context, stdout, stderr io.Writer, endpoint string, src keySource, limit int, jsonOut, full, quiet bool) int {
	if stdoutTTY {
		fmt.Fprint(stdout, "\x1b[H\x1b[2J")
	}
	return pollOnce(ctx, stdout, stderr, endpoint, src, limit, jsonOut, full, quiet)
}

// stdoutTTY reports whether stdout is attached to a terminal.
var stdoutTTY = func() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}()

var versionString = "0.1.0 (ocgo-usage)"
