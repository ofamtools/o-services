package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type cappedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("command output exceeded limit")
	}
	return b.Buffer.Write(p)
}

type commandResult struct {
	Output     []byte
	Diagnostic string
	Err        error
	Code       int
}

func command(ctx context.Context, name string, args ...string) commandResult {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = time.Second
	out, diagnostic := cappedBuffer{limit: 8 << 20}, cappedBuffer{limit: 16 << 10}
	cmd.Stdout, cmd.Stderr = &out, &diagnostic
	err := cmd.Run()
	code := 0
	if err != nil {
		code = -1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		}
	}
	if ctx.Err() != nil {
		err = ctx.Err()
		code = -1
	}
	return commandResult{Output: out.Bytes(), Diagnostic: strings.TrimSpace(diagnostic.String()), Err: err, Code: code}
}

func (r commandResult) error(name string) error {
	if r.Err != nil {
		return fmt.Errorf("%s: %w (%s)", name, r.Err, r.Diagnostic)
	}
	if r.Diagnostic != "" {
		return fmt.Errorf("%s: %s", name, r.Diagnostic)
	}
	return nil
}
