package main

import (
	"testing"
	"time"
)

func TestInventoryMembershipAndIdentity(t *testing.T) {
	birth := Birth{PID: 42, BootID: "boot", StartKind: "linux-ticks", StartValue: "100"}
	old := Snapshot{Complete: true, Services: []Service{{ID: birth.Key(), Process: Process{Birth: birth}, Origin: &Origin{Pane: "tuios:server/pane", Source: "verified-ancestry"}}}}
	next := Snapshot{Complete: true, Services: []Service{{ID: birth.Key(), Process: Process{Birth: birth}}}}
	got := reconcile(old, next, true)
	if got.Services[0].Origin == nil || got.Services[0].Origin.Pane != "tuios:server/pane" {
		t.Fatal("live detached process lost observed origin")
	}
	birth.StartValue = "101"
	reused := Snapshot{Complete: true, Services: []Service{{ID: birth.Key(), Process: Process{Birth: birth}}}}
	if reconcile(old, reused, true).Services[0].Origin != nil {
		t.Fatal("PID reuse inherited origin")
	}
	if got := reconcile(old, Snapshot{Complete: true}, true); len(got.Services) != 0 {
		t.Fatal("successful empty refresh retained exited services")
	}
	partial := reconcile(old, Snapshot{At: time.Unix(200, 0), Problems: []string{"permission denied"}}, true)
	if len(partial.Services) != 1 || !partial.Services[0].Stale || partial.Complete {
		t.Fatalf("partial refresh hid uncertainty: %+v", partial)
	}
	if reconcile(old, next, false).Services[0].Origin != nil {
		t.Fatal("disabled provenance retained origin")
	}
}

func TestEndpointIdentityAndDeduplication(t *testing.T) {
	b := Birth{PID: 42, BootID: "boot", StartKind: "linux-ticks", StartValue: "100"}
	s := Snapshot{At: time.Unix(1, 0), Complete: true}
	p := Process{Birth: b}
	s.add(p, []Endpoint{{Transport: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 8080}, {Transport: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 8080}})
	b.PID = 43
	s.add(Process{Birth: b}, []Endpoint{{Transport: "tcp", Family: "ipv4", Address: "127.0.0.1", Port: 8080}})
	if len(s.Endpoints) != 2 || s.Endpoints[0].ID == s.Endpoints[1].ID {
		t.Fatalf("fd duplicate or shared port identity lost: %+v", s.Endpoints)
	}
	unknown := Snapshot{At: time.Unix(2, 0)}
	unknown.add(Process{Birth: Birth{PID: 42}}, []Endpoint{{Port: 8080}})
	later := Snapshot{At: time.Unix(3, 0)}
	later.add(Process{Birth: Birth{PID: 42}}, []Endpoint{{Port: 8080}})
	if unknown.Services[0].ID == later.Services[0].ID {
		t.Fatal("unknown birth became persistent PID identity")
	}
}
