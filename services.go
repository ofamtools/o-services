package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"
)

// errProcessGone is a process that exited before it could be read again.
var errProcessGone = errors.New("process exited")

type Birth struct {
	PID        int    `json:"pid"`
	BootID     string `json:"boot_id,omitempty"`
	StartKind  string `json:"start_kind,omitempty"`
	StartValue string `json:"start_value,omitempty"`
}

func (b Birth) Verified() bool {
	return b.PID > 0 && b.BootID != "" && b.StartKind != "" && b.StartValue != "" && b.StartValue != "0"
}
func (b Birth) Key() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s\x00%s", b.BootID, b.PID, b.StartKind, b.StartValue)))
	return hex.EncodeToString(sum[:16])
}

type Process struct {
	Birth
	Parent    int    `json:"parent"`
	Command   string `json:"command"`
	CWD       string `json:"cwd,omitempty"`
	CWDStatus string `json:"cwd_status"`
}

type Origin struct {
	Pane         string    `json:"pane,omitempty"`
	Agent        string    `json:"agent,omitempty"`
	Project      string    `json:"project,omitempty"`
	AgentProcess Birth     `json:"agent_process"`
	Harness      string    `json:"harness,omitempty"`
	At           time.Time `json:"observed_at"`
	Source       string    `json:"source"`
}

type PaneContext struct {
	Pane         string `json:"pane,omitempty"`
	Project      string `json:"project,omitempty"`
	Conversation string `json:"conversation,omitempty"`
	Status       string `json:"status"`
}

type Service struct {
	ID string `json:"id"`
	Process
	IdentityStatus   string      `json:"identity_status"`
	ObservedAt       time.Time   `json:"observed_at"`
	Stale            bool        `json:"stale"`
	Origin           *Origin     `json:"origin,omitempty"`
	Current          PaneContext `json:"current"`
	ProvenanceStatus string      `json:"provenance_status"`
}

type Endpoint struct {
	ID        string `json:"id"`
	Service   string `json:"service"`
	Transport string `json:"transport"`
	Family    string `json:"family"`
	Address   string `json:"address"`
	Port      uint16 `json:"port"`
	State     string `json:"state"`
}

func (e Endpoint) sameSocket(other Endpoint) bool {
	return e.Transport == other.Transport && e.Family == other.Family && e.Address == other.Address && e.Port == other.Port
}

type Snapshot struct {
	At       time.Time `json:"observed_at"`
	Complete bool      `json:"complete"`
	Problems []string  `json:"problems"`
	// Restricted counts processes whose sockets this user may not read
	// (another user's process, hidepid). They are outside the listing's
	// scope, not a failed scan, so they do not clear Complete.
	Restricted int        `json:"restricted"`
	Services   []Service  `json:"services"`
	Endpoints  []Endpoint `json:"endpoints"`
}

type scanner interface {
	Discover(context.Context) Snapshot
	Process(int) (Process, error)
}

func (s *Snapshot) problem(err error) {
	if err == nil {
		return
	}
	s.Complete = false
	if len(s.Problems) < 32 {
		s.Problems = append(s.Problems, err.Error())
	}
}

func (s *Snapshot) add(p Process, endpoints []Endpoint) {
	if len(endpoints) == 0 {
		return
	}
	id, status := p.Birth.Key(), "verified"
	if !p.Verified() {
		id = "observation-" + strconv.FormatInt(s.At.UnixNano(), 10) + "-" + strconv.Itoa(p.PID)
		status = "unavailable"
	}
	s.Services = append(s.Services, Service{ID: id, Process: p, IdentityStatus: status, ObservedAt: s.At, ProvenanceStatus: "unknown: no verified ancestry observed", Current: PaneContext{Status: "unknown"}})
	seen := make(map[string]bool, len(endpoints))
	for _, e := range endpoints {
		e.Service = id
		e.ID = fmt.Sprintf("%s/%s/%s/%s/%d", id, e.Transport, e.Family, e.Address, e.Port)
		if seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		if e.Transport == "tcp" {
			e.State = "listen"
		} else {
			e.State = "bound"
		}
		s.Endpoints = append(s.Endpoints, e)
	}
}

func (s *Snapshot) sort() {
	sort.Slice(s.Services, func(i, j int) bool { return s.Services[i].PID < s.Services[j].PID })
	// Endpoints read by port, then address and transport; the id breaks
	// ties so two processes sharing a port keep a stable order.
	sort.Slice(s.Endpoints, func(i, j int) bool {
		a, b := s.Endpoints[i], s.Endpoints[j]
		if a.Port != b.Port {
			return a.Port < b.Port
		}
		if a.Address != b.Address {
			return a.Address < b.Address
		}
		if a.Transport != b.Transport {
			return a.Transport < b.Transport
		}
		return a.ID < b.ID
	})
}

func reconcile(old, next Snapshot, provenance bool) Snapshot {
	previous := make(map[string]Service, len(old.Services))
	for _, service := range old.Services {
		previous[service.ID] = service
	}
	present := make(map[string]bool, len(next.Services))
	for i := range next.Services {
		s := &next.Services[i]
		present[s.ID] = true
		if provenance && s.Verified() && previous[s.ID].Origin != nil {
			s.Origin = previous[s.ID].Origin
			s.ProvenanceStatus = "observed origin retained for this process incarnation"
		}
		if !provenance {
			s.Origin = nil
			s.Current = PaneContext{Status: "disabled"}
			s.ProvenanceStatus = "disabled"
		}
		// A row this scan observed is fresh even when another part of the
		// scan failed; only rows carried over from an earlier scan are stale.
		s.Stale = false
	}
	if !next.Complete {
		for _, s := range old.Services {
			if present[s.ID] {
				continue
			}
			s.Stale = true
			if !provenance {
				s.Origin = nil
				s.Current = PaneContext{Status: "disabled"}
				s.ProvenanceStatus = "disabled"
			}
			next.Services = append(next.Services, s)
			for _, e := range old.Endpoints {
				if e.Service == s.ID {
					next.Endpoints = append(next.Endpoints, e)
				}
			}
		}
	}
	next.sort()
	return next
}
