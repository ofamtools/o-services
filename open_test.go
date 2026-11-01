package main

import (
	"context"
	"errors"
	"testing"
)

func TestBrowserURLRequiresExplicitWebScheme(t *testing.T) {
	for _, tc := range []struct{ address, family, scheme, want string }{
		{"0.0.0.0", "ipv4", "http", "http://127.0.0.1:8080/"},
		{"::", "ipv6", "https", "https://[::1]:8080/"},
		{"fe80::1%en0", "ipv6", "http", "http://[fe80::1%25en0]:8080/"},
		{"192.0.2.4", "ipv4", "https", "https://192.0.2.4:8080/"},
	} {
		got, err := browserURL(Endpoint{Transport: "tcp", Family: tc.family, Address: tc.address, Port: 8080}, tc.scheme)
		if err != nil || got != tc.want {
			t.Errorf("URL=%q error=%v want=%q", got, err, tc.want)
		}
	}
	for _, e := range []Endpoint{{Transport: "udp", Family: "ipv4", Address: "127.0.0.1", Port: 80}, {Transport: "tcp", Family: "ipv4", Address: "evil.example", Port: 80}, {Transport: "tcp", Family: "ipv4", Address: "127.0.0.1"}} {
		if _, err := browserURL(e, "http"); err == nil {
			t.Errorf("accepted %+v", e)
		}
	}
	for _, scheme := range []string{"", "file", "javascript", "http://evil"} {
		if _, err := browserURL(Endpoint{Transport: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 80}, scheme); err == nil {
			t.Errorf("accepted scheme %q", scheme)
		}
	}
}

type fixtureScanner struct {
	snapshot Snapshot
	process  Process
}

func (f fixtureScanner) Discover(context.Context) Snapshot { return f.snapshot }
func (f fixtureScanner) Process(pid int) (Process, error) {
	if f.process.PID != pid {
		return Process{}, errors.New("gone")
	}
	return f.process, nil
}

func TestBrowserRejectsReplacementAndIncompleteInventory(t *testing.T) {
	birth := Birth{PID: 40, BootID: "boot", StartKind: "linux-ticks", StartValue: "10"}
	service := Service{ID: birth.Key(), Process: Process{Birth: birth}}
	endpoint := Endpoint{Service: service.ID, Transport: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 8080}
	f := fixtureScanner{snapshot: Snapshot{Complete: true, Services: []Service{service}, Endpoints: []Endpoint{endpoint}}, process: service.Process}
	if got, err := validatedURL(context.Background(), service, endpoint, "http", f); err != nil || got != "http://127.0.0.1:8080/" {
		t.Fatalf("valid endpoint: %q %v", got, err)
	}
	f.process.StartValue = "11"
	if _, err := validatedURL(context.Background(), service, endpoint, "http", f); err == nil {
		t.Fatal("same PID replacement opened")
	}
	f.process = service.Process
	f.snapshot.Complete = false
	if _, err := validatedURL(context.Background(), service, endpoint, "http", f); err == nil {
		t.Fatal("incomplete validation opened")
	}
	f.snapshot.Complete = true
	f.snapshot.Endpoints = nil
	if _, err := validatedURL(context.Background(), service, endpoint, "http", f); err == nil {
		t.Fatal("closed endpoint opened")
	}
	service.Stale = true
	if _, err := validatedURL(context.Background(), service, endpoint, "http", f); err == nil {
		t.Fatal("stale selected row opened")
	}
}
