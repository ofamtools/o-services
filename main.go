// Command o-services lists the local processes that listen on TCP ports or
// hold bound UDP sockets, with each process's PID, cwd and, where TUIOS can
// prove it, the pane, agent and project that started it. A selected TCP
// endpoint opens in the browser with an explicit http or https scheme.
package main

import (
	"context"
	"io"
	"os"

	ocli "ofam.dev/ochazuke/cli"
	"ofam.dev/oden"
)

var version = "dev"

func main() {
	os.Exit(oden.Report(os.Stderr, "o-services", Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)))
}

// Main runs one CLI call or the TUI against the real host.
func Main(args []string, in io.Reader, out, errOut io.Writer) error {
	a, err := newApp(in, out, errOut, newSystem(), readMetadata, openBrowser)
	if err != nil {
		return err
	}
	return dispatch(a, args)
}

func dispatch(a *App, args []string) error {
	app := oden.New("o-services", args, oden.WithBuiltins(), oden.WithStdio(a.in, a.out, a.errOut), oden.WithDefault(func(_ context.Context, args []string) error {
		if len(args) > 0 {
			return oden.ErrNotFound
		}
		return runTUI(a, nil)
	}))
	app.Add(oden.Command{Name: "view", Summary: "open the Services page", Usage: "[--smoke] [--width N] [--height N]", Run: func(_ context.Context, args []string) error { return runTUI(a, args) }})
	ocli.Mount(app, a.shell())
	app.HelpText += "\nendpoint list and service list discover the current state on every call; nothing is cached between calls." +
		"\nendpoint open ID http|https asks before opening (--yes accepts) and checks the process and socket again first." +
		"\nProvenance is unknown unless TUIOS proves the process descends from a pane (pane_process) or agent (agent_process)."
	return app.Run(context.Background())
}
