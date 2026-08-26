package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Window is the usage state for a single rate window (rolling / weekly /
// monthly), as returned by the /zen/go/v1/usage endpoint. The OpenCode console
// computes percentages server-side against the account's subscription limits
// and hands back an absolute reset timestamp.
type Window struct {
	// Status is "ok" or "rate-limited".
	Status string `json:"status"`
	// Percent is 0..100 of the window's limit (100 when rate-limited).
	Percent int `json:"percent"`
	// ResetsAt is an RFC3339 timestamp of the window reset.
	ResetsAt string `json:"resetsAt"`
}

// OK reports whether the window is not rate-limited.
func (w Window) OK() bool { return w.Status == "ok" || w.Status == "" }

// ResetsIn returns the time until the window resets (0 for missing/invalid).
func (w Window) ResetsIn() time.Duration {
	if w.ResetsAt == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, w.ResetsAt)
	if err != nil {
		return 0
	}
	d := time.Until(t)
	if d < 0 {
		return 0
	}
	return d
}

// Windows aggregates the three rate windows returned for a key.
type Windows struct {
	Rolling Window `json:"rolling"`
	Weekly  Window `json:"weekly"`
	Monthly Window `json:"monthly"`
}

// Usage is the full payload of GET /zen/go/v1/usage:
// {"usage": {"rolling": ..., "weekly": ..., "monthly": ...}}.
type Usage struct {
	Usage Windows `json:"usage"`
}

// MaxPercent returns the highest usage percentage across all windows.
func (u *Usage) MaxPercent() int {
	w := u.Usage
	m := w.Rolling.Percent
	if p := w.Weekly.Percent; p > m {
		m = p
	}
	if p := w.Monthly.Percent; p > m {
		m = p
	}
	return m
}

// RateLimited returns the first rate-limited window, if any.
func (u *Usage) RateLimited() *Window {
	w := u.Usage
	for _, wnd := range []*Window{&w.Rolling, &w.Weekly, &w.Monthly} {
		if !wnd.OK() {
			return wnd
		}
	}
	return nil
}

// LooksEmpty reports whether the payload decoded as all-zeros, which happens
// when the server renames its fields and this struct no longer matches.
func (u *Usage) LooksEmpty() bool {
	w := u.Usage
	return w.Rolling.Status == "" && w.Rolling.Percent == 0 && w.Rolling.ResetsAt == "" &&
		w.Weekly.Status == "" && w.Weekly.Percent == 0 && w.Weekly.ResetsAt == "" &&
		w.Monthly.Status == "" && w.Monthly.Percent == 0 && w.Monthly.ResetsAt == ""
}

// apiError mirrors the console's error envelope, e.g.
// {"type":"error","error":{"type":"AuthError","message":"Unauthorized"}}.
type apiError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

var (
	// ErrUnauthorized is returned on 401 responses (missing/invalid key).
	ErrUnauthorized = errors.New("unauthorized: check the API key (missing, revoked, or not an OpenCode Go/zen key)")
	// ErrNoEntitlement is returned on 403 responses.
	ErrNoEntitlement = errors.New("OpenCode Go subscription required for this key")
)

// fetchUsage performs a single GET against the usage endpoint.
func fetchUsage(ctx context.Context, endpoint, apiKey string) (*Usage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		var e apiError
		msg := strings.TrimSpace(string(body))
		if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
			msg = e.Error.Message
		}
		switch res.StatusCode {
		case http.StatusUnauthorized:
			return nil, fmt.Errorf("%w (%s)", ErrUnauthorized, msg)
		case http.StatusForbidden:
			return nil, fmt.Errorf("%w (%s)", ErrNoEntitlement, msg)
		default:
			return nil, fmt.Errorf("API returned %s: %s", res.Status, msg)
		}
	}

	var u Usage
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if u.LooksEmpty() {
		return nil, errors.New("response decoded empty: the endpoint shape may have changed (fields renamed?)")
	}
	return &u, nil
}
