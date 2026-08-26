package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{65 * time.Second, "1m 5s"},
		{time.Hour, "1h 0m"},
		{3661 * time.Second, "1h 1m"},
		{90061 * time.Second, "1d 1h 1m"},
		{1234567 * time.Second, "14d 6h 56m"},
		{-30 * time.Second, "0s"},
	}
	for _, c := range cases {
		if got := formatDuration(c.d); got != c.want {
			t.Errorf("formatDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func window(status string, pct int, resetsAt time.Time) Window {
	return Window{Status: status, Percent: pct, ResetsAt: resetsAt.UTC().Format(time.RFC3339)}
}

func TestAlertsAndExitCondition(t *testing.T) {
	now := time.Now()
	ok := &Usage{Usage: Windows{
		Rolling: window("ok", 24, now.Add(time.Hour)),
		Weekly:  window("ok", 41, now.Add(24*time.Hour)),
		Monthly: window("ok", 12, now.Add(10*24*time.Hour)),
	}}
	if rl := ok.RateLimited(); rl != nil {
		t.Fatalf("unexpected rate-limited window: %+v", rl)
	}
	if ok.MaxPercent() != 41 {
		t.Fatalf("MaxPercent = %d, want 41", ok.MaxPercent())
	}
	if ok.LooksEmpty() {
		t.Fatal("unexpected LooksEmpty for populated usage")
	}
	if got := alerts(ok, 100); len(got) != 0 {
		t.Fatalf("unexpected alerts: %v", got)
	}
	if d := ok.Usage.Rolling.ResetsIn(); d < 50*time.Minute || d > 70*time.Minute {
		t.Fatalf("ResetsIn = %v, want ~1h", d)
	}

	hot := &Usage{Usage: Windows{
		Rolling: window("ok", 95, now.Add(time.Minute)),
		Weekly:  window("rate-limited", 100, now.Add(42*time.Second)),
		Monthly: window("ok", 10, now.Add(10*24*time.Hour)),
	}}
	if rl := hot.RateLimited(); rl == nil {
		t.Fatal("expected rate-limited window")
	}
	if hot.MaxPercent() != 100 {
		t.Fatalf("MaxPercent = %d, want 100", hot.MaxPercent())
	}
	if got := alerts(hot, 90); len(got) != 1 || !strings.Contains(got[0], "RATE-LIMITED") {
		t.Fatalf("alerts = %v, want rate-limited warning", got)
	}

	empty := &Usage{}
	if !empty.LooksEmpty() {
		t.Fatal("expected LooksEmpty for zero usage")
	}
}

func TestRenderOneLine(t *testing.T) {
	now := time.Now()
	u := &Usage{Usage: Windows{
		Rolling: window("ok", 59, now.Add(time.Hour)),
		Weekly:  window("ok", 58, now.Add(4*24*time.Hour)),
		Monthly: window("ok", 32, now.Add(28*24*time.Hour)),
	}}
	var buf bytes.Buffer
	renderOneLine(&buf, u)
	if want := "rolling 59% | weekly 58% | monthly 32%\n"; buf.String() != want {
		t.Errorf("renderOneLine = %q, want %q", buf.String(), want)
	}
}

func TestRenderHuman(t *testing.T) {
	now := time.Now()
	u := &Usage{Usage: Windows{
		Rolling: window("ok", 59, now.Add(time.Hour)),
		Weekly:  window("ok", 58, now.Add(4*24*time.Hour)),
		Monthly: window("ok", 32, now.Add(28*24*time.Hour)),
	}}
	var buf bytes.Buffer
	renderHuman(&buf, u, "https://example.com/zen/go/v1/usage", keySource{key: "sk-abcdefgh", label: "test"}, time.Now(), 100)
	out := buf.String()
	for _, want := range []string{"rolling", "weekly", "monthly", "59%", "58%", "32%", "sk-…efgh", "example.com"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderHuman output missing %q:\n%s", want, out)
		}
	}
}
