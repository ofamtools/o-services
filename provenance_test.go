package main

import (
	"errors"
	"testing"
	"time"
)

func TestOriginRequiresVerifiedAncestry(t *testing.T) {
	birth := func(pid int, start string) Birth {
		return Birth{PID: pid, BootID: "boot", StartKind: "linux-ticks", StartValue: start}
	}
	processes := map[int]Process{40: {Birth: birth(40, "400"), Parent: 30}, 30: {Birth: birth(30, "300"), Parent: 20}, 20: {Birth: birth(20, "200"), Parent: 1}}
	read := func(pid int) (Process, error) {
		p, ok := processes[pid]
		if !ok {
			return Process{}, errors.New("gone")
		}
		return p, nil
	}
	pane := paneMetadata{Window: "pane", Session: "session", PaneProcess: &processesBirth{Birth: birth(20, "200")}, AgentProcess: &agentProcess{Birth: birth(30, "300"), Harness: "omp", Source: "hook-lineage"}}
	meta := metadata{Namespace: "tuios/test", Panes: []paneMetadata{pane}, Projects: map[string]string{"pane": "project"}}
	service := Service{Process: processes[40]}
	origin, _, why := traceOrigin(service, meta, read, time.Unix(1, 0))
	if origin == nil || origin.Agent == "" || origin.Pane != "tuios/test/pane" || origin.Project != "mtm/project" || origin.Harness != "omp" {
		t.Fatalf("origin=%+v reason=%s", origin, why)
	}
	processes[40] = Process{Birth: birth(40, "400"), Parent: 20}
	origin, _, _ = traceOrigin(service, meta, read, time.Unix(1, 0))
	if origin == nil || origin.Agent != "" {
		t.Fatalf("shell sibling claimed agent: %+v", origin)
	}
	pane.PaneProcess = &processesBirth{Birth: birth(20, "199")}
	meta.Panes[0] = pane
	if got, _, _ := traceOrigin(service, meta, read, time.Unix(1, 0)); got != nil {
		t.Fatal("reused pane root inherited origin")
	}
	meta.Panes[0].PaneProcess = nil
	if got, _, _ := traceOrigin(service, meta, read, time.Unix(1, 0)); got != nil {
		t.Fatal("old producer supplied PID-only origin")
	}
}

func TestAncestryMutationRefusesAttribution(t *testing.T) {
	service := Service{Process: Process{Birth: Birth{PID: 40, BootID: "b", StartKind: "linux-ticks", StartValue: "4"}, Parent: 20}}
	root := Process{Birth: Birth{PID: 20, BootID: "b", StartKind: "linux-ticks", StartValue: "2"}, Parent: 1}
	meta := metadata{Namespace: "tuios/test", Panes: []paneMetadata{{Window: "pane", PaneProcess: &processesBirth{Birth: root.Birth}}}}
	calls := 0
	read := func(pid int) (Process, error) {
		calls++
		if pid == 20 {
			return root, nil
		}
		p := service.Process
		if calls > 2 {
			p.Parent = 1
		}
		return p, nil
	}
	if got, _, _ := traceOrigin(service, meta, read, time.Unix(1, 0)); got != nil {
		t.Fatal("reparented chain retained proof")
	}
}
