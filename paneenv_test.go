package main

import "testing"

func TestPaneEnv(t *testing.T) {
	m := map[string]string{"TUIOS_SESSION": "old", "MTM_SESSION": "s"}
	get := func(k string) string { return m[k] }
	if got := paneEnv(get, "SESSION"); got != "s" {
		t.Fatal("MTM_ is read", got)
	}
	delete(m, "MTM_SESSION")
	if got := paneEnv(get, "SESSION"); got != "" {
		t.Fatal("TUIOS_ is not read", got)
	}
}
