package main

import (
	"strings"
	"testing"
)

func TestProcStatParenthesizedCommand(t *testing.T) {
	p, err := parseProcStat("42 (worker (child) name) S 17 1 1 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 987 0", "boot")
	if err != nil || p.PID != 42 || p.Parent != 17 || p.Command != "worker (child) name" || p.StartValue != "987" {
		t.Fatalf("process = %+v, %v", p, err)
	}
	for _, input := range []string{"42 broken", "42 (a) S 1", "42 (a) S nope"} {
		if _, err := parseProcStat(input, "boot"); err == nil {
			t.Errorf("accepted malformed stat %q", input)
		}
	}
}

func TestProcSocketTables(t *testing.T) {
	text := "sl local_address rem_address st tx_queue tr retrnsmt uid timeout inode\n0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000 501 0 123\n1: 0100007F:CAFE 0100007F:0050 01 0:0 00:0 0 501 0 124\n"
	rows, err := parseProcSockets(strings.NewReader(text), "tcp", "ipv4")
	if err != nil || len(rows) != 1 || rows[123].Address != "127.0.0.1" || rows[123].Port != 8080 {
		t.Fatalf("sockets=%+v error=%v", rows, err)
	}
	ip, err := procAddress("00000000000000000000000001000000", "ipv6")
	if err != nil || ip != "::1" {
		t.Fatalf("IPv6=%s error=%v", ip, err)
	}
}

func TestLSOFMachineRecords(t *testing.T) {
	data := "p42\x00cserver name\x00\nf7\x00tIPv6\x00PTCP\x00n[::1]:8080\x00TST=LISTEN\x00\nf8\x00tIPv4\x00PUDP\x00n*:9000->127.0.0.1:53\x00\n"
	rows, err := parseLSOF([]byte(data))
	if err != nil || len(rows[42]) != 2 {
		t.Fatalf("rows=%+v error=%v", rows, err)
	}
	if rows[42][0].Address != "::1" || rows[42][0].Port != 8080 || rows[42][1].Address != "0.0.0.0" || rows[42][1].State != "bound" {
		t.Fatalf("bad socket fields: %+v", rows[42])
	}
	if _, err := parseLSOF([]byte("p42\x00f3\x00tIPv4\x00PTCP\x00nnot-an-address\x00TST=LISTEN\x00")); err == nil {
		t.Fatal("malformed endpoint became empty success")
	}
	cwd, err := parseLSOFCWD([]byte("p42\x00\nfcwd\x00n/a folder/with\nnewline\x00\n"), 42)
	if err != nil || cwd != "/a folder/with\nnewline" {
		t.Fatalf("cwd=%q err=%v", cwd, err)
	}
	if _, err := parseLSOFCWD([]byte("p43\x00fcwd\x00n/wrong\x00"), 42); err == nil {
		t.Fatal("accepted another process cwd")
	}
}
