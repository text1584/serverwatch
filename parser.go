package main

import (
	"html"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var (
	tagRE     = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(script|style)>|<[^>]+>`)
	requestRE = regexp.MustCompile(`(?m)\b([A-Z][A-Z0-9-]{2,15})\s+(\S+)\s+HTTP/([0-9]+(?:\.[0-9]+)?)\b`)
)

var validMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true,
	"DELETE": true, "HEAD": true, "OPTIONS": true,
	"CONNECT": true, "TRACE": true, "PROPFIND": true,
	"MKCOL": true, "COPY": true, "MOVE": true,
}

func htmlToText(body string) string {
	body = tagRE.ReplaceAllString(body, " ")
	body = html.UnescapeString(body)
	body = strings.ReplaceAll(body, "\r", "\n")
	return strings.Join(strings.Fields(body), " ")
}

func absoluteURL(baseURL, requestTarget string) (string, string, string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", "", "", err
	}

	// Absolute-form request target.
	if u, err := url.Parse(requestTarget); err == nil && u.IsAbs() {
		return u.String(), u.EscapedPath(), u.RawQuery, nil
	}

	ref, err := url.Parse(requestTarget)
	if err != nil {
		return "", "", "", err
	}

	resolved := base.ResolveReference(ref)
	return resolved.String(), resolved.EscapedPath(), resolved.RawQuery, nil
}

func endpointCategory(path string) string {
	p := strings.ToLower(path)

	staticExt := []string{
		".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg",
		".ico", ".webp", ".woff", ".woff2", ".ttf", ".map",
		".mp4", ".webm", ".mp3", ".wav", ".pdf",
	}

	for _, ext := range staticExt {
		if strings.HasSuffix(p, ext) {
			return "static"
		}
	}

	switch {
	case strings.Contains(p, "/api/"),
		strings.HasPrefix(p, "/api"),
		strings.HasPrefix(p, "/graphql"):
		return "api"
	case strings.HasSuffix(p, ".php"),
		strings.HasSuffix(p, ".asp"),
		strings.HasSuffix(p, ".aspx"),
		strings.HasSuffix(p, ".jsp"),
		strings.HasSuffix(p, ".do"),
		strings.HasSuffix(p, ".action"):
		return "dynamic"
	default:
		return "route"
	}
}

func ParseServerStatus(body, targetURL string) []SnapshotRequest {
	text := htmlToText(body)

	matches := requestRE.FindAllStringSubmatch(text, -1)
	seen := make(map[string]SnapshotRequest)

	now := nowUTC()

	for _, m := range matches {
		method := strings.ToUpper(m[1])
		requestTarget := strings.TrimSpace(m[2])
		protocol := "HTTP/" + m[3]

		if !validMethods[method] {
			continue
		}

		// Apache request targets are origin-form paths or absolute-form URLs.
		if requestTarget == "" || requestTarget[0] != '/' {
			if !strings.HasPrefix(requestTarget, "http://") &&
				!strings.HasPrefix(requestTarget, "https://") {
				continue
			}
		}

		fullURL, path, query, err := absoluteURL(targetURL, requestTarget)
		if err != nil || path == "" {
			continue
		}

		// Normalize root path and remove fragments.
		fullParsed, err := url.Parse(fullURL)
		if err != nil {
			continue
		}
		fullParsed.Fragment = ""
		fullURL = fullParsed.String()

		category := endpointCategory(path)
		key := method + " " + path
		if query != "" {
			key += "?" + query
		}

		// Deduplicate identical method+target entries within one snapshot.
		seen[key] = SnapshotRequest{
			Timestamp: now,
			Method:    method,
			URL:       fullURL,
			Path:      path,
			Query:     query,
			Protocol:  protocol,
			Category:  category,
		}
	}

	out := make([]SnapshotRequest, 0, len(seen))
	for _, req := range seen {
		out = append(out, req)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Method != out[j].Method {
			return out[i].Method < out[j].Method
		}
		return out[i].URL < out[j].URL
	})

	return out
}
