package main

import (
	"strings"
	"testing"
)

func TestParseServerStatus(t *testing.T) {
	body := `
	<html><body>
	Apache Status for example.test
	Srv | PID | Request
	0-0 | 123 | POST /lsw_admin/api/event.php HTTP/1.1
	0-0 | 123 | GET /lsw/js/chunk-vendors.c1a89960.js HTTP/1.1
	0-0 | 123 | GET /lsw/ HTTP/1.1
	0-0 | 123 | 336455/336455
	</body></html>`

	got := ParseServerStatus(body, "https://example.test/server-status")
	if len(got) != 3 {
		t.Fatalf("expected 3 requests, got %d: %#v", len(got), got)
	}

	var seenPost bool
	for _, r := range got {
		switch {
		case r.Method == "POST" && r.Path == "/lsw_admin/api/event.php":
			seenPost = true
		case strings.Contains(r.URL, "336455/336455"):
			t.Fatalf("numeric chunk incorrectly treated as endpoint: %s", r.URL)
		}
	}
	if !seenPost {
		t.Fatal("POST endpoint was not parsed")
	}
}
