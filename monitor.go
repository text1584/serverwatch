package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Monitor struct {
	URL        string
	Interval   time.Duration
	Client     *http.Client
	Store      *Store
	ShowStatic bool
}

func (m *Monitor) Run(ctx context.Context) error {
	if _, err := url.ParseRequestURI(m.URL); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	for {
		if err := m.poll(ctx); err != nil {
			fmt.Printf("[ERR] %v\n", err)
		}

		timer := time.NewTimer(m.Interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func (m *Monitor) poll(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "serverwatch/2.0")
	req.Header.Set("Accept", "text/html, text/plain;q=0.9, */*;q=0.1")

	start := time.Now()
	resp, err := m.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return err
	}

	elapsed := time.Since(start)
	fmt.Printf("[%s] HTTP %d • %d bytes • %s\n",
		time.Now().Format("15:04:05"), resp.StatusCode, len(body), elapsed.Round(time.Millisecond))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server-status returned HTTP %d", resp.StatusCode)
	}

	requests := ParseServerStatus(string(body), m.URL)

	// Do not treat the monitor's own request as an application endpoint.
	filtered := requests[:0]
	for _, r := range requests {
		if sameTarget(m.URL, r.URL) {
			continue
		}
		if !m.ShowStatic && r.Category == "static" {
			continue
		}
		filtered = append(filtered, r)
	}
	requests = filtered

	newEndpoints := m.Store.ApplySnapshot(requests)
	_ = AppendNewEndpoints(m.Store.dir, newEndpoints)
	_ = AppendSnapshot(m.Store.dir, requests)
	findings := Analyze(string(body))
	m.Store.AddFindings(findings)
	_ = m.Store.Export()
	_ = writeJSONFile("output/headers.json", resp.Header)

	total, live, api := m.Store.EndpointCounts()

	fmt.Printf("[+] Requests in snapshot: %-4d  New: %-4d  Stored: %-4d  Live: %-4d  API: %-4d\n",
		len(requests), len(newEndpoints), total, live, api)

	for _, ep := range newEndpoints {
		fmt.Printf("[NEW] %-7s %-60s [%s]\n", ep.Method, ep.URL, ep.Category)
	}

	for _, f := range findings {
		fmt.Printf("[FINDING] %-7s %-24s %s\n", strings.ToUpper(f.Severity), f.Type, f.Evidence)
	}

	serverInfo := ExtractServerInfo(string(body))
	_ = writeJSONFile("output/server_info.json", map[string]any{
		"target_url": m.URL,
		"polled_at":  time.Now().UTC(),
		"status":     resp.StatusCode,
		"info":       serverInfo,
	})

	return nil
}

func sameTarget(a, b string) bool {
	ua, err1 := url.Parse(a)
	ub, err2 := url.Parse(b)
	if err1 != nil || err2 != nil {
		return a == b
	}

	return strings.EqualFold(ua.Scheme, ub.Scheme) &&
		strings.EqualFold(ua.Host, ub.Host) &&
		ua.EscapedPath() == ub.EscapedPath() &&
		ua.RawQuery == ub.RawQuery
}

func writeJSONFile(path string, value any) error {
	b, err := jsonMarshalIndent(value)
	if err != nil {
		return err
	}
	return atomicWrite(path, b)
}
