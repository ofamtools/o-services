package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"ofam.dev/ochazuke"
	ocli "ofam.dev/ochazuke/cli"
)

type page struct {
	a     *App
	m     *ochazuke.Model
	v     *ocli.PageView
	dirty bool
}

// loadedMsg carries the initial discovery, run off the loop.
type loadedMsg struct{ s Snapshot }

func mountPage(a *App) (*page, error) {
	data, err := ocli.Catalog(a.r, "o-services", version)
	if err != nil {
		return nil, err
	}
	var doc ocli.CatalogDoc
	if err = json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	m := ochazuke.NewModel(a.r)
	m.Title = "o-services"
	v, err := ocli.MountPage(m, doc, servicesPage(), ocli.PageConfig{})
	if err != nil {
		return nil, err
	}
	p := &page{a: a, m: m, v: v}
	a.tui = true
	a.changed = func() { p.dirty = true }
	quit := ochazuke.Do("app.quit")
	quit.Instance, quit.Mode, quit.Keys = pageID, "normal", "q"
	if err = a.r.Bind(quit); err != nil {
		return nil, err
	}
	if err = a.r.OpenPage(pageID); err != nil {
		return nil, err
	}
	if err = p.reload(); err != nil {
		return nil, err
	}
	if err = v.List(listID).Focus(); err != nil {
		return nil, err
	}
	return p, nil
}

// reload shows the snapshot's rows, keeping the selection by endpoint id
// (the list keeps a selected id that is still present).
func (p *page) reload() error {
	if !p.a.loaded {
		p.v.SetStatus("Not loaded: services.load-on-open is off. Press r to discover.")
		return p.v.SetRows(listID, nil)
	}
	s := p.a.snapshot
	status := fmt.Sprintf("Observed %s: %d endpoints of %d processes", s.At.Format("15:04:05"), len(s.Endpoints), len(s.Services))
	if s.Restricted > 0 {
		status += fmt.Sprintf("; %d processes not readable by this user", s.Restricted)
	}
	if !s.Complete {
		status += "; INCOMPLETE: " + strings.Join(s.Problems, "; ")
	}
	p.v.SetStatus(status)
	return p.v.SetRows(listID, endpointRows(s))
}

func runTUI(a *App, args []string) error {
	fs := flag.NewFlagSet("view", flag.ContinueOnError)
	fs.SetOutput(a.errOut)
	smoke := fs.Bool("smoke", false, "Discover once, render one frame and exit")
	w := fs.Int("width", 140, "Frame width")
	h := fs.Int("height", 30, "Frame height")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return errors.New("unexpected view arguments")
	}
	p, err := mountPage(a)
	if err != nil {
		return err
	}
	if *smoke {
		if a.r.Settings().Bool("services.load-on-open") {
			a.apply(a.collect(context.Background(), Snapshot{}, a.r.Settings().Bool("services.provenance")))
			if err := p.reload(); err != nil {
				return err
			}
		}
		p.m.Width, p.m.Height = *w, *h
		_, err = fmt.Fprintln(a.out, strings.TrimRight(ansi.Strip(p.m.View().Content), "\n"))
		return err
	}
	_, err = tea.NewProgram(p, tea.WithInput(a.in), tea.WithOutput(a.out)).Run()
	return err
}

func (p *page) Init() tea.Cmd {
	cmds := []tea.Cmd{p.m.Init(), p.a.r.PendingCommands()}
	if p.a.r.Settings().Bool("services.load-on-open") {
		provenance := p.a.r.Settings().Bool("services.provenance")
		cmds = append(cmds, func() tea.Msg {
			return loadedMsg{p.a.collect(context.Background(), Snapshot{}, provenance)}
		})
	}
	return tea.Batch(cmds...)
}

func (p *page) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if l, ok := msg.(loadedMsg); ok {
		// A manual refresh that landed first wins; the initial load only
		// fills a page that has nothing yet.
		if !p.a.loaded {
			p.a.apply(l.s)
		}
		msg = nil
	}
	var cmd tea.Cmd
	if msg != nil {
		_, cmd = p.m.Update(msg)
	}
	if p.dirty {
		p.dirty = false
		if err := p.reload(); err != nil {
			p.a.r.UI().Error(err.Error())
		}
		cmd = tea.Batch(cmd, p.a.r.PendingCommands())
	}
	return p, cmd
}

func (p *page) View() tea.View { return p.m.View() }
