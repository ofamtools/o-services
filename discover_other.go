//go:build !darwin && !linux

package main

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

type system struct{}

func newSystem() scanner { return &system{} }
func (*system) Process(int) (Process, error) {
	return Process{}, fmt.Errorf("process discovery is unsupported on %s", runtime.GOOS)
}
func (*system) Discover(context.Context) Snapshot {
	return Snapshot{At: time.Now(), Problems: []string{fmt.Sprintf("service discovery is unsupported on %s", runtime.GOOS)}}
}
