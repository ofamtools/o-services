package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type system struct {
	boot    string
	bootErr error
}

func newSystem() scanner {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	return &system{boot: strings.TrimSpace(string(data)), bootErr: err}
}
func (s *system) Process(pid int) (Process, error) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return Process{}, err
	}
	return parseProcStat(string(data), s.boot)
}

func (s *system) Discover(ctx context.Context) Snapshot {
	result := Snapshot{At: time.Now(), Complete: true}
	if s.bootErr != nil {
		result.problem(fmt.Errorf("boot identity: %w", s.bootErr))
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		result.problem(err)
		return result
	}
	namespace, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		result.problem(fmt.Errorf("network namespace: %w", err))
		return result
	}
	before := make(map[int]Process)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			result.problem(err)
			return result
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		if len(before) >= 65536 {
			result.problem(errors.New("process inventory exceeds 65536 entries"))
			break
		}
		p, err := s.Process(pid)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				result.problem(fmt.Errorf("PID %d identity: %w", pid, err))
			}
			continue
		}
		before[pid] = p
	}
	sockets := make(map[uint64]Endpoint)
	for _, table := range []struct{ name, transport, family string }{{"tcp", "tcp", "ipv4"}, {"tcp6", "tcp", "ipv6"}, {"udp", "udp", "ipv4"}, {"udp6", "udp", "ipv6"}} {
		file, err := os.Open(filepath.Join("/proc/net", table.name))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) && table.family == "ipv6" {
				continue
			}
			result.problem(err)
			continue
		}
		rows, readErr := parseProcSockets(file, table.transport, table.family)
		file.Close()
		result.problem(readErr)
		for inode, e := range rows {
			sockets[inode] = e
		}
	}
	for pid, p := range before {
		if err := ctx.Err(); err != nil {
			result.problem(err)
			break
		}
		base := filepath.Join("/proc", strconv.Itoa(pid))
		ns, err := os.Readlink(filepath.Join(base, "ns/net"))
		if errors.Is(err, os.ErrPermission) {
			result.Restricted++
			continue
		}
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				result.problem(fmt.Errorf("PID %d namespace: %w", pid, err))
			}
			continue
		}
		if ns != namespace {
			continue
		}
		fds, err := os.ReadDir(filepath.Join(base, "fd"))
		if errors.Is(err, os.ErrPermission) {
			result.Restricted++
			continue
		}
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				result.problem(fmt.Errorf("PID %d sockets: %w", pid, err))
			}
			continue
		}
		var endpoints []Endpoint
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(base, "fd", fd.Name()))
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					result.problem(fmt.Errorf("PID %d fd: %w", pid, err))
				}
				continue
			}
			if !strings.HasPrefix(link, "socket:[") || !strings.HasSuffix(link, "]") {
				continue
			}
			inode, err := strconv.ParseUint(link[len("socket:["):len(link)-1], 10, 64)
			if err != nil {
				result.problem(err)
				continue
			}
			if endpoint, ok := sockets[inode]; ok {
				endpoints = append(endpoints, endpoint)
			}
		}
		if len(endpoints) == 0 {
			continue
		}
		p.CWD, err = os.Readlink(filepath.Join(base, "cwd"))
		p.CWDStatus = "available"
		if err != nil {
			p.CWDStatus = "unavailable: " + err.Error()
		}
		after, err := s.Process(pid)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				result.problem(err)
			}
			continue
		}
		afterNS, err := os.Readlink(filepath.Join(base, "ns/net"))
		if err != nil {
			result.problem(err)
			continue
		}
		if p.Birth != after.Birth || ns != afterNS {
			continue
		}
		snapshotProcess := p
		snapshotProcess.Parent = after.Parent
		result.add(snapshotProcess, endpoints)
	}
	result.sort()
	return result
}
