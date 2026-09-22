package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu    sync.Mutex
	state State
	dir   string
}

func NewStore(dir, target string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	s := &Store{
		dir: dir,
		state: State{
			TargetURL: target,
			Endpoints: make(map[string]Endpoint),
			Findings:  make(map[string]Finding),
			LastLive:  make(map[string]bool),
		},
	}

	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err == nil {
		_ = json.Unmarshal(data, &s.state)
		if s.state.Endpoints == nil {
			s.state.Endpoints = make(map[string]Endpoint)
		}
		if s.state.Findings == nil {
			s.state.Findings = make(map[string]Finding)
		}
		if s.state.LastLive == nil {
			s.state.LastLive = make(map[string]bool)
		}
	}

	return s, nil
}

func (s *Store) SaveState() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveStateLocked()
}

func (s *Store) saveStateLocked() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, "state.json.tmp")
	final := filepath.Join(s.dir, "state.json")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

func (s *Store) ApplySnapshot(requests []SnapshotRequest) (newEndpoints []Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := nowUTC()
	current := make(map[string]bool, len(requests))

	for _, req := range requests {
		key := req.Method + " " + req.Path
		if req.Query != "" {
			key += "?" + req.Query
		}
		current[key] = true

		ep, exists := s.state.Endpoints[key]
		if !exists {
			ep = Endpoint{
				Key:       key,
				Method:    req.Method,
				URL:       req.URL,
				Path:      req.Path,
				Query:     req.Query,
				Protocol:  req.Protocol,
				Category:  req.Category,
				FirstSeen: now,
			}
			newEndpoints = append(newEndpoints, ep)
		}

		ep.URL = req.URL
		ep.Protocol = req.Protocol
		ep.Category = req.Category
		ep.LastSeen = now
		ep.SeenCount++
		ep.Live = true
		s.state.Endpoints[key] = ep
	}

	for key, ep := range s.state.Endpoints {
		ep.Live = current[key]
		s.state.Endpoints[key] = ep
	}

	s.state.LastLive = current
	s.state.LastPoll = now
	_ = s.saveStateLocked()

	return newEndpoints
}

func (s *Store) AddFindings(findings []Finding) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := nowUTC()
	for _, f := range findings {
		existing, ok := s.state.Findings[f.Key]
		if !ok {
			f.FirstSeen = now
		} else {
			f.FirstSeen = existing.FirstSeen
			f.Count = existing.Count
		}
		f.LastSeen = now
		f.Count++
		s.state.Findings[f.Key] = f
	}

	_ = s.saveStateLocked()
}

func (s *Store) Export() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var endpoints []Endpoint
	for _, ep := range s.state.Endpoints {
		endpoints = append(endpoints, ep)
	}
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Method != endpoints[j].Method {
			return endpoints[i].Method < endpoints[j].Method
		}
		return endpoints[i].URL < endpoints[j].URL
	})

	var live []Endpoint
	for _, ep := range endpoints {
		if ep.Live {
			live = append(live, ep)
		}
	}

	var newOnDisk []Endpoint
	for _, ep := range endpoints {
		newOnDisk = append(newOnDisk, ep)
	}
	sort.Slice(newOnDisk, func(i, j int) bool {
		return newOnDisk[i].FirstSeen.Before(newOnDisk[j].FirstSeen)
	})

	writeJSON(filepath.Join(s.dir, "endpoints.json"), endpoints)
	writeJSON(filepath.Join(s.dir, "live_endpoints.json"), live)

	if err := writeEndpointText(filepath.Join(s.dir, "endpoints.txt"), endpoints); err != nil {
		return err
	}
	if err := writeEndpointText(filepath.Join(s.dir, "live_endpoints.txt"), live); err != nil {
		return err
	}

	findings := make([]Finding, 0, len(s.state.Findings))
	for _, f := range s.state.Findings {
		findings = append(findings, f)
	}
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].LastSeen.Before(findings[j].LastSeen)
	})
	writeJSON(filepath.Join(s.dir, "findings.json"), findings)

	return writeJSON(filepath.Join(s.dir, "server_info.json"), map[string]any{
		"target_url": s.state.TargetURL,
		"last_poll":  s.state.LastPoll,
	})
}

func AppendNewEndpoints(dir string, endpoints []Endpoint) error {
	if len(endpoints) == 0 {
		return nil
	}

	f, err := os.OpenFile(filepath.Join(dir, "new_endpoints.txt"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, ep := range endpoints {
		fmt.Fprintf(w, "%s %s %s", ep.Method, ep.URL, ep.Protocol)
		if ep.Category != "" {
			fmt.Fprintf(w, " [%s]", ep.Category)
		}
		fmt.Fprintln(w)
	}
	return w.Flush()
}

func AppendSnapshot(dir string, requests []SnapshotRequest) error {
	f, err := os.OpenFile(filepath.Join(dir, "snapshot_history.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, req := range requests {
		b, err := json.Marshal(req)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeEndpointText(path string, endpoints []Endpoint) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, ep := range endpoints {
		status := "seen"
		if ep.Live {
			status = "live"
		}
		line := fmt.Sprintf("%-7s %-6s %-70s %s", ep.Method, status, ep.URL, ep.Protocol)
		if ep.Category != "" {
			line += " [" + ep.Category + "]"
		}
		if strings.TrimSpace(ep.Query) != "" {
			line += " query=" + ep.Query
		}
		fmt.Fprintln(f, line)
	}
	return nil
}

func (s *Store) EndpointCounts() (total, live, api int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ep := range s.state.Endpoints {
		total++
		if ep.Live {
			live++
		}
		if ep.Category == "api" {
			api++
		}
	}
	return
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
