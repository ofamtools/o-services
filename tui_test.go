package main

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"ofam.dev/ochazuke"
)

func press(p *page, key rune) {
	p.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
}

// o on a selected TCP endpoint asks for the scheme (never guessing one from
// the port), and nothing reaches the opener before a choice and a yes.
func TestOpenKeyAsksForTheSchemeThenConfirms(t *testing.T) {
	f, _, _, _, _ := twoPortService()
	var opened []string
	a, _ := testApp(t, f, func(context.Context) metadata { return metadata{} }, &opened)
	if err := a.r.Settings().Set("services.load-on-open", false); err != nil {
		t.Fatal(err)
	}
	p, err := mountPage(a)
	if err != nil {
		t.Fatal(err)
	}
	p.m.Width, p.m.Height = 140, 30
	if frame := ansi.Strip(p.m.View().Content); !strings.Contains(frame, "Not loaded") {
		t.Fatalf("load-on-open off did not show a not-loaded page:\n%s", frame)
	}
	p.Update(loadedMsg{a.collect(context.Background(), Snapshot{}, true)})
	press(p, 'j') // the first row is the UDP socket; move to the TCP one
	press(p, 'o')
	frame := ansi.Strip(p.m.View().Content)
	if !strings.Contains(frame, "scheme") {
		t.Fatalf("o did not ask for the scheme:\n%s", frame)
	}
	// Only the scheme is missing: the row supplied the endpoint id.
	if strings.Count(frame, "(none)") != 1 {
		t.Fatalf("the form did not take the selected endpoint:\n%s", frame)
	}
	if len(opened) != 0 {
		t.Fatalf("opened before a scheme was chosen: %v", opened)
	}
	// enter opens the scheme chooser, enter takes http, enter submits; the
	// confirmation names the exact URL, and only y hands it to the opener.
	for _, k := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyEnter}, {Code: tea.KeyEnter}} {
		_, cmd := p.Update(k)
		drain(p, cmd, 0)
	}
	if frame := ansi.Strip(p.m.View().Content); !strings.Contains(frame, "Open http://127.0.0.1:8080/ in the browser?") || len(opened) != 0 {
		t.Fatalf("no confirmation naming the URL (opened %v):\n%s", opened, frame)
	}
	_, cmd := p.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	drain(p, cmd, 0)
	if len(opened) != 1 || opened[0] != "http://127.0.0.1:8080/" {
		t.Fatalf("opened %v, want the selected endpoint once", opened)
	}
}

// drain runs a command's messages back through the page, as the program
// would, so pending work (the revalidation and the opener) completes.
func drain(p *page, cmd tea.Cmd, depth int) {
	if cmd == nil || depth > 8 {
		return
	}
	// A timer (a notice's auto-dismiss) is not part of the flow: give each
	// command a moment and drop one that is still waiting.
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(500 * time.Millisecond):
		return
	}
	switch m := msg.(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range m {
			drain(p, c, depth+1)
		}
	default:
		_, next := p.Update(m)
		drain(p, next, depth+1)
	}
}

// The id parameters declare the selected row (Parameter.From), so the
// help's run of an unbound endpoint.open, with no binding from, asks only
// for the scheme.
func TestParameterSourceFillsTheEndpointID(t *testing.T) {
	f, _, _, _, _ := twoPortService()
	var opened []string
	a, _ := testApp(t, f, func(context.Context) metadata { return metadata{} }, &opened)
	if err := a.r.Settings().Set("services.load-on-open", false); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"endpoint.open", "endpoint.show", "service.show"} {
		if got := a.r.Operations[key].Parameters["id"].From; got != ochazuke.SourceRow {
			t.Errorf("%s id From %q", key, got)
		}
	}
	p, err := mountPage(a)
	if err != nil {
		t.Fatal(err)
	}
	p.m.Width, p.m.Height = 140, 30
	p.m.View()
	p.Update(loadedMsg{a.collect(context.Background(), Snapshot{}, true)})
	press(p, 'j')
	if err := a.r.RunUnbound(a.r.BaseFocus(), ochazuke.Request{Noun: "endpoint", Verb: "open"}); err != nil {
		t.Fatal(err)
	}
	frame := ansi.Strip(p.m.View().Content)
	if !strings.Contains(frame, "scheme") || strings.Count(frame, "(none)") != 1 {
		t.Fatalf("the form did not take the selected endpoint:\n%s", frame)
	}
}
