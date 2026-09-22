package main

import "time"

type Endpoint struct {
	Key       string    `json:"key"`
	Method    string    `json:"method"`
	URL       string    `json:"url"`
	Path      string    `json:"path"`
	Query     string    `json:"query,omitempty"`
	Protocol  string    `json:"protocol"`
	Category  string    `json:"category"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	SeenCount uint64    `json:"seen_count"`
	Live      bool      `json:"live"`
}

type SnapshotRequest struct {
	Timestamp time.Time `json:"timestamp"`
	Method    string    `json:"method"`
	URL       string    `json:"url"`
	Path      string    `json:"path"`
	Query     string    `json:"query,omitempty"`
	Protocol  string    `json:"protocol"`
	Category  string    `json:"category"`
}

type Finding struct {
	Key         string    `json:"key"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Evidence    string    `json:"evidence"`
	Description string    `json:"description"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Count       uint64    `json:"count"`
}

type State struct {
	TargetURL string              `json:"target_url"`
	LastPoll  time.Time           `json:"last_poll"`
	Endpoints map[string]Endpoint `json:"endpoints"`
	Findings  map[string]Finding  `json:"findings"`
	LastLive  map[string]bool     `json:"last_live"`
}
