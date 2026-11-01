package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

func parseProcStat(text, boot string) (Process, error) {
	left, right := strings.IndexByte(text, '('), strings.LastIndexByte(text, ')')
	if left < 1 || right <= left {
		return Process{}, errors.New("malformed process stat command")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(text[:left]))
	if err != nil || pid <= 0 {
		return Process{}, errors.New("malformed process stat PID")
	}
	fields := strings.Fields(text[right+1:])
	if len(fields) < 20 {
		return Process{}, errors.New("short process stat")
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return Process{}, fmt.Errorf("process parent: %w", err)
	}
	start, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil || start == 0 {
		return Process{}, errors.New("invalid process start ticks")
	}
	return Process{Birth: Birth{PID: pid, BootID: boot, StartKind: "linux-ticks", StartValue: strconv.FormatUint(start, 10)}, Parent: parent, Command: text[left+1 : right], CWDStatus: "unavailable"}, nil
}

func procAddress(value, family string) (string, error) {
	raw, err := hex.DecodeString(value)
	if err != nil || (family == "ipv4" && len(raw) != 4) || (family == "ipv6" && len(raw) != 16) {
		return "", errors.New("invalid proc socket address")
	}
	for i := 0; i < len(raw); i += 4 {
		raw[i], raw[i+3] = raw[i+3], raw[i]
		raw[i+1], raw[i+2] = raw[i+2], raw[i+1]
	}
	addr, ok := netip.AddrFromSlice(raw)
	if !ok {
		return "", errors.New("invalid proc socket address width")
	}
	return addr.String(), nil
}

func parseProcSockets(reader io.Reader, transport, family string) (map[uint64]Endpoint, error) {
	rows := make(map[uint64]Endpoint)
	scan := bufio.NewScanner(io.LimitReader(reader, 16<<20))
	if !scan.Scan() {
		return rows, errors.New("missing proc socket table header")
	}
	for n := 0; scan.Scan(); n++ {
		if n >= 65536 {
			return rows, errors.New("socket table exceeds 65536 rows")
		}
		f := strings.Fields(scan.Text())
		if len(f) < 10 {
			return rows, errors.New("malformed proc socket row")
		}
		if transport == "tcp" && f[3] != "0A" {
			continue
		}
		address, portText, ok := strings.Cut(f[1], ":")
		if !ok {
			return rows, errors.New("missing proc socket port")
		}
		port, err := strconv.ParseUint(portText, 16, 16)
		if err != nil {
			return rows, fmt.Errorf("socket port: %w", err)
		}
		if port == 0 {
			continue
		}
		ip, err := procAddress(address, family)
		if err != nil {
			return rows, err
		}
		inode, err := strconv.ParseUint(f[9], 10, 64)
		if err != nil {
			return rows, fmt.Errorf("socket inode: %w", err)
		}
		if inode == 0 {
			continue
		}
		rows[inode] = Endpoint{Transport: transport, Family: family, Address: ip, Port: uint16(port)}
	}
	return rows, scan.Err()
}

func socketAddress(text, family string) (string, uint16, error) {
	text, _, _ = strings.Cut(text, "->")
	host, portText, err := net.SplitHostPort(text)
	if err != nil {
		return "", 0, err
	}
	// A UDP socket with no local port yet reads "*"; it has no endpoint.
	if portText == "*" {
		portText = "0"
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil {
		return "", 0, err
	}
	if host == "*" {
		if family == "ipv6" {
			host = "::"
		} else {
			host = "0.0.0.0"
		}
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "", 0, err
	}
	if (family == "ipv4") != addr.Is4() {
		return "", 0, errors.New("socket address family mismatch")
	}
	return addr.String(), uint16(port), nil
}

// parseLSOF reads lsof -F0 socket records. A malformed record is reported
// and skipped; the records around it are still read.
func parseLSOF(data []byte) (map[int][]Endpoint, error) {
	rows := make(map[int][]Endpoint)
	var problems []error
	pid, active := 0, false
	var e Endpoint
	var address, state string
	flush := func() error {
		if !active || (e.Transport != "tcp" && e.Transport != "udp") {
			return nil
		}
		if e.Transport == "tcp" && state != "LISTEN" {
			return nil
		}
		if pid <= 0 || (e.Family != "ipv4" && e.Family != "ipv6") || address == "" {
			return errors.New("incomplete lsof socket record")
		}
		var err error
		e.Address, e.Port, err = socketAddress(address, e.Family)
		if err != nil {
			return fmt.Errorf("lsof PID %d: %w", pid, err)
		}
		if e.Port == 0 {
			return nil
		}
		if e.Transport == "tcp" {
			e.State = "listen"
		} else {
			e.State = "bound"
		}
		rows[pid] = append(rows[pid], e)
		return nil
	}
	for _, raw := range bytes.Split(data, []byte{0}) {
		field := bytes.TrimLeft(raw, "\n")
		if len(field) == 0 {
			continue
		}
		value := string(field[1:])
		switch field[0] {
		case 'p':
			if err := flush(); err != nil {
				problems = append(problems, err)
			}
			active = false
			var err error
			pid, err = strconv.Atoi(value)
			if err != nil || pid <= 0 {
				return rows, errors.Join(append(problems, errors.New("invalid lsof PID"))...)
			}
		case 'f':
			if err := flush(); err != nil {
				problems = append(problems, err)
			}
			active = true
			e = Endpoint{}
			address, state = "", ""
		case 't':
			switch value {
			case "IPv4":
				e.Family = "ipv4"
			case "IPv6":
				e.Family = "ipv6"
			}
		case 'P':
			e.Transport = strings.ToLower(value)
		case 'n':
			address = value
		case 'T':
			if strings.HasPrefix(value, "ST=") {
				state = strings.TrimPrefix(value, "ST=")
			}
		}
	}
	if err := flush(); err != nil {
		problems = append(problems, err)
	}
	return rows, errors.Join(problems...)
}

// parseLSOFCWDs reads the cwd records of one batched lsof call (-d cwd
// -F0pfn), by PID. A PID with no cwd record is absent, never "".
func parseLSOFCWDs(data []byte) map[int]string {
	out := make(map[int]string)
	pid, cwd := 0, false
	for _, raw := range bytes.Split(data, []byte{0}) {
		field := bytes.TrimLeft(raw, "\n")
		if len(field) == 0 {
			continue
		}
		switch field[0] {
		case 'p':
			pid, _ = strconv.Atoi(string(field[1:]))
			cwd = false
		case 'f':
			cwd = string(field[1:]) == "cwd"
		case 'n':
			if pid > 0 && cwd && len(field) > 1 {
				out[pid] = string(field[1:])
			}
		}
	}
	return out
}

func parseLSOFCWD(data []byte, want int) (string, error) {
	if cwd, ok := parseLSOFCWDs(data)[want]; ok {
		return cwd, nil
	}
	return "", errors.New("cwd unavailable in lsof response")
}
