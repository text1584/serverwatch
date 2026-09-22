package main

import (
	"regexp"
	"strings"
)

type findingRule struct {
	Type        string
	Severity    string
	Pattern     *regexp.Regexp
	Description string
}

func findingRules() []findingRule {
	return []findingRule{
		{
			Type:        "internal_ip",
			Severity:    "medium",
			Pattern:     regexp.MustCompile(`\b(?:10\.(?:\d{1,3}\.){2}\d{1,3}|192\.168\.(?:\d{1,3}\.)\d{1,3}|172\.(?:1[6-9]|2\d|3[0-1])\.(?:\d{1,3}\.)\d{1,3})\b`),
			Description: "Private RFC1918 address exposed in the monitored response.",
		},
		{
			Type:        "private_key",
			Severity:    "high",
			Pattern:     regexp.MustCompile(`-----BEGIN(?: [A-Z0-9]+)? PRIVATE KEY-----`),
			Description: "Private-key material appears in the monitored response.",
		},
		{
			Type:        "jwt",
			Severity:    "high",
			Pattern:     regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]{5,}\.[a-zA-Z0-9_-]{5,}\.[a-zA-Z0-9_-]{5,}\b`),
			Description: "JWT-like bearer token appears in the monitored response.",
		},
		{
			Type:        "aws_access_key",
			Severity:    "high",
			Pattern:     regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
			Description: "AWS access-key-like value appears in the monitored response.",
		},
		{
			Type:        "credential_assignment",
			Severity:    "high",
			Pattern:     regexp.MustCompile(`(?i)\b(?:password|passwd|pwd|secret|api[_-]?key|access[_-]?token)\s*[:=]\s*["']?[^"'&\s<]{4,}`),
			Description: "Credential/token-like assignment appears in the monitored response.",
		},
		{
			Type:        "sensitive_path",
			Severity:    "medium",
			Pattern:     regexp.MustCompile(`(?i)(?:^|[\s"'=])(?:/|https?://)[^\s"'<>]*(?:/\.git(?:/|$)|/\.env(?:$|[/?])|/backup(?:[._/-]|$)|/backups(?:[._/-]|$)|/dump(?:[._/-]|$)|/swagger(?:\.json|/|$)|/openapi(?:\.json|/|$))`),
			Description: "A path commonly associated with sensitive or administrative resources appears in the monitored response.",
		},
	}
}

func Analyze(body string) []Finding {
	var findings []Finding

	for _, rule := range findingRules() {
		match := rule.Pattern.FindString(body)
		if match == "" {
			continue
		}

		// Avoid writing an unbounded amount of page content to disk.
		match = strings.TrimSpace(match)
		if len(match) > 500 {
			match = match[:500]
		}

		findings = append(findings, Finding{
			Key:         rule.Type + "|" + match,
			Type:        rule.Type,
			Severity:    rule.Severity,
			Evidence:    match,
			Description: rule.Description,
		})
	}

	return findings
}

func ExtractServerInfo(body string) map[string]string {
	text := htmlToText(body)
	lines := strings.Fields(text)

	// Server-status output has "Label: value..." pairs. Work from the
	// original text so multi-word values are preserved as much as possible.
	fields := []string{
		"Server Version:",
		"Server MPM:",
		"Server Built:",
		"Current Time:",
		"Restart Time:",
		"Server uptime:",
		"Server load:",
		"Total accesses:",
		"Total Traffic:",
		"CPU Usage:",
	}

	result := make(map[string]string)
	for i := 0; i < len(lines); i++ {
		for _, key := range fields {
			k := strings.TrimSuffix(key, ":")
			if lines[i] == k && i+1 < len(lines) {
				result[k] = lines[i+1]
			}
		}
	}

	return result
}
