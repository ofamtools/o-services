package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// testApp runs the real runtime and CLI shell over a fixture host, with its
// config under a temp dir so no owner settings are read.
func testApp(t *testing.T, f fixtureScanner, meta func(context.Context) metadata, opened *[]string) (*App, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir+"/config")
	t.Setenv("XDG_STATE_HOME", dir+"/state")
	t.Setenv("XDG_DATA_HOME", dir+"/data")
	var out, errOut bytes.Buffer
	opener := func(_ context.Context, url string) error { *opened = append(*opened, url); return nil }
	a, err := newApp(strings.NewReader(""), &out, &errOut, f, meta, opener)
	if err != nil {
		t.Fatal(err)
	}
	return a, &out
}

func twoPortService() (fixtureScanner, Service, Endpoint, Endpoint, Endpoint) {
	birth := Birth{PID: 40, BootID: "boot", StartKind: "linux-ticks", StartValue: "10"}
	svc := Service{ID: birth.Key(), Process: Process{Birth: birth, Command: "server", CWD: "/srv", CWDStatus: "available"}, IdentityStatus: "verified"}
	a := Endpoint{ID: svc.ID + "/tcp/ipv4/127.0.0.1/8080", Service: svc.ID, Transport: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 8080, State: "listen"}
	b := Endpoint{ID: svc.ID + "/tcp/ipv6/::/9090", Service: svc.ID, Transport: "tcp", Family: "ipv6", Address: "::", Port: 9090, State: "listen"}
	u := Endpoint{ID: svc.ID + "/udp/ipv4/0.0.0.0/5353", Service: svc.ID, Transport: "udp", Family: "ipv4", Address: "0.0.0.0", Port: 5353, State: "bound"}
	f := fixtureScanner{snapshot: Snapshot{Complete: true, Services: []Service{svc}, Endpoints: []Endpoint{a, b, u}}, process: svc.Process}
	return f, svc, a, b, u
}

func noMetadata(t *testing.T) func(context.Context) metadata {
	return func(context.Context) metadata { t.Helper(); t.Error("metadata read"); return metadata{} }
}

// Opening the second port of a multi-port process opens that port, never
// the process's first one; UDP and unknown endpoints are refused.
func TestOpenUsesTheSelectedEndpoint(t *testing.T) {
	f, _, _, b, u := twoPortService()
	var opened []string
	a, _ := testApp(t, f, func(context.Context) metadata { return metadata{Problem: "no daemon"} }, &opened)
	sh := a.shell()
	if err := sh.Run(context.Background(), []string{"endpoint", "open", b.ID, "http", "--yes"}); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || opened[0] != "http://[::1]:9090/" {
		t.Fatalf("opened %v, want the selected IPv6 endpoint", opened)
	}
	for _, args := range [][]string{
		{"endpoint", "open", u.ID, "http", "--yes"},
		{"endpoint", "open", "missing/tcp/ipv4/127.0.0.1/1", "http", "--yes"},
		{"endpoint", "open", b.ID, "--yes"},
	} {
		if err := a.shell().Run(context.Background(), args); err == nil {
			t.Errorf("%v opened", args)
		}
	}
	if len(opened) != 1 {
		t.Fatalf("a refused request reached the opener: %v", opened)
	}
}

// A process that changed birth between listing and opening is refused,
// even though its PID and port are the same.
func TestOpenRefusesAReplacedProcess(t *testing.T) {
	f, _, a8080, _, _ := twoPortService()
	var opened []string
	a, _ := testApp(t, f, func(context.Context) metadata { return metadata{} }, &opened)
	if _, err := a.current(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.process.StartValue = "11"
	a.scan = f
	if err := a.shell().Run(context.Background(), []string{"endpoint", "open", a8080.ID, "http", "--yes"}); err == nil || len(opened) != 0 {
		t.Fatalf("replacement opened: err=%v opened=%v", err, opened)
	}
}

// services.provenance off makes no metadata call and says so on each row.
func TestProvenanceSettingOff(t *testing.T) {
	f, _, _, _, _ := twoPortService()
	var opened []string
	a, out := testApp(t, f, noMetadata(t), &opened)
	if err := a.r.Settings().Set("services.provenance", false); err != nil {
		t.Fatal(err)
	}
	if err := a.shell().Run(context.Background(), []string{"endpoint", "list"}); err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want one per endpoint", len(rows))
	}
	for _, r := range rows {
		if r.Data["provenance_status"] != "disabled" || r.Data["origin_pane"] != "unknown" {
			t.Fatalf("row with provenance off: %v", r.Data)
		}
	}
}

// An unreachable TUIOS leaves every row listed, with provenance unknown and
// the reason, never an empty listing.
func TestMissingProducerKeepsRowsUnknown(t *testing.T) {
	f, _, _, _, _ := twoPortService()
	var opened []string
	a, out := testApp(t, f, func(context.Context) metadata { return metadata{Problem: "tuios list-agents: not running"} }, &opened)
	if err := a.shell().Run(context.Background(), []string{"endpoint", "list"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unknown: tuios list-agents: not running") || strings.Count(out.String(), `"noun":"endpoint"`) != 3 {
		t.Fatalf("listing hid rows or the reason: %s", out)
	}
}

func TestPaneProjects(t *testing.T) {
	got, err := paneProjects([]byte(`[{"id":"w1","label":"a","project":"p1"},{"id":"pane/w2","data":{"project":"p2"}},{"id":"w3","project":""}]`))
	if err != nil || len(got) != 2 || got["w1"] != "p1" || got["w2"] != "p2" {
		t.Fatalf("projects = %v, %v", got, err)
	}
	if _, err := paneProjects([]byte(`{"error":"x"}`)); err == nil {
		t.Fatal("non-row answer accepted")
	}
}
