package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type system struct {
	boot    string
	bootErr error
}

func newSystem() scanner {
	boot, err := unix.Sysctl("kern.bootsessionuuid")
	return &system{boot: strings.TrimSpace(boot), bootErr: err}
}
func (s *system) fromKernel(k *unix.KinfoProc) Process {
	start := k.Proc.P_starttime
	value := ""
	if start.Sec > 0 {
		value = strconv.FormatInt(start.Sec*1000000+int64(start.Usec), 10)
	}
	return Process{Birth: Birth{PID: int(k.Proc.P_pid), BootID: s.boot, StartKind: "darwin-epoch-microseconds", StartValue: value}, Parent: int(k.Eproc.Ppid), Command: kernelName(k.Proc.P_comm[:]), CWDStatus: "unavailable"}
}
func (s *system) Process(pid int) (Process, error) {
	k, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return Process{}, err
	}
	p := s.fromKernel(k)
	if p.PID != pid {
		return Process{}, errProcessGone
	}
	return p, nil
}

func (s *system) Discover(ctx context.Context) Snapshot {
	result := Snapshot{At: time.Now(), Complete: true}
	if s.bootErr != nil {
		result.problem(fmt.Errorf("boot identity: %w", s.bootErr))
	}
	processes, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		result.problem(fmt.Errorf("process inventory: %w", err))
		return result
	}
	before := make(map[int]Process, len(processes))
	for i := range processes {
		p := s.fromKernel(&processes[i])
		if p.PID > 0 {
			before[p.PID] = p
		}
	}
	endpoints := make(map[int][]Endpoint)
	for _, args := range [][]string{{"-nP", "-iTCP", "-sTCP:LISTEN", "-F0pcfPtnT"}, {"-nP", "-iUDP", "-F0pcfPtnT"}} {
		if err := ctx.Err(); err != nil {
			result.problem(err)
			break
		}
		answer := command(ctx, "lsof", args...)
		// lsof returns 1 without diagnostics when no selected files exist.
		if !(answer.Code == 1 && len(answer.Output) == 0 && answer.Diagnostic == "") {
			result.problem(answer.error("lsof sockets"))
		}
		rows, err := parseLSOF(answer.Output)
		result.problem(err)
		for pid, rows := range rows {
			endpoints[pid] = append(endpoints[pid], rows...)
		}
	}
	// One batched, PID-targeted cwd query for every listening process.
	cwds := map[int]string{}
	cwdProblem := ""
	if len(endpoints) > 0 && ctx.Err() == nil {
		pids := make([]string, 0, len(endpoints))
		for pid := range endpoints {
			pids = append(pids, strconv.Itoa(pid))
		}
		answer := command(ctx, "lsof", "-nP", "-a", "-p", strings.Join(pids, ","), "-d", "cwd", "-F0pfn")
		// lsof exits 1 when one of the PIDs has exited or is not readable;
		// that PID's cwd is then reported unavailable, not the whole scan.
		if answer.Err != nil && answer.Code != 1 {
			cwdProblem = answer.error("lsof cwd").Error()
		}
		cwds = parseLSOFCWDs(answer.Output)
	}
	for pid, rows := range endpoints {
		if err := ctx.Err(); err != nil {
			result.problem(err)
			break
		}
		p, observedBefore := before[pid]
		if !observedBefore {
			p = Process{Birth: Birth{PID: pid}, Command: "unknown", CWDStatus: "unavailable"}
		}
		if cwd, ok := cwds[pid]; ok {
			p.CWD, p.CWDStatus = cwd, "available"
		} else if cwdProblem != "" {
			p.CWDStatus = "unavailable: " + cwdProblem
		} else {
			p.CWDStatus = "unavailable: not readable by this user or exited"
		}
		after, err := s.Process(pid)
		if err != nil {
			if errors.Is(err, errProcessGone) || errors.Is(err, unix.ESRCH) || errors.Is(err, unix.EIO) {
				continue
			}
			result.problem(fmt.Errorf("PID %d identity: %w", pid, err))
			p.Birth = Birth{PID: pid}
		} else if observedBefore {
			if p.Birth != after.Birth {
				continue
			}
			p.Parent = after.Parent
		} else {
			p.Command = after.Command
			p.Parent = after.Parent
		}
		result.add(p, rows)
	}
	result.sort()
	return result
}

// kernelName is the process's kernel short name up to its first NUL; the
// bytes after it are stale and not part of the name.
func kernelName(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}
