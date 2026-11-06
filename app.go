package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"ofam.dev/ochazuke"
	ocli "ofam.dev/ochazuke/cli"
)

const (
	pageID = "services"
	listID = "services/endpoints"
)

// App owns the last discovery snapshot. A CLI call collects a fresh one on
// first use (each call is a fresh process); the TUI keeps the snapshot it
// last landed and replaces it only through services.refresh.
type App struct {
	r           *ochazuke.Runtime
	in          io.Reader
	out, errOut io.Writer
	scan        scanner
	metadata    func(context.Context) metadata
	opener      func(context.Context, string) error
	snapshot    Snapshot
	loaded      bool
	tui         bool
	// changed tells a mounted page to reload its rows from the snapshot.
	changed func()
}

// Columns drop from the largest Priority first; 0 never drops.
var endpointColumns = []ochazuke.Column{
	{Title: "Program", Max: 18},
	{Title: "PID", Width: 7, Align: ochazuke.AlignRight},
	{Title: "Endpoint", Max: 30},
	{Title: "Cwd", Max: 40, Priority: 1},
	{Title: "Pane", Max: 22, Priority: 2},
	{Title: "Agent", Max: 14, Priority: 3},
	{Title: "Project", Max: 18, Priority: 4},
}

var serviceColumns = []ochazuke.Column{
	{Title: "Program", Max: 18},
	{Title: "PID", Width: 7, Align: ochazuke.AlignRight},
	{Title: "Endpoints", Max: 40, Priority: 1},
	{Title: "Cwd", Max: 40, Priority: 2},
	{Title: "Provenance", Max: 40, Priority: 3},
}

var detailFields = []string{
	"program", "pid", "url_hint", "transport", "family", "address", "port", "state",
	"cwd", "cwd_status", "identity_status", "stale", "observed_at",
	"origin_pane", "origin_agent", "origin_harness", "origin_project", "origin_source", "origin_observed_at",
	"current_pane", "current_project", "current_conversation", "current_status",
	"provenance_status", "scan_complete", "scan_restricted", "scan_problems",
}

func newApp(in io.Reader, out, errOut io.Writer, scan scanner, meta func(context.Context) metadata, opener func(context.Context, string) error) (*App, error) {
	a := &App{in: in, out: out, errOut: errOut, scan: scan, metadata: meta, opener: opener}
	a.r = ochazuke.NewRuntime()
	a.r.Config().App = "o-services"
	if err := a.r.DeclareSetting(
		ochazuke.Setting{Name: "services.load-on-open", Type: ochazuke.SettingBoolean, Default: true, Description: "Discover services when the TUI opens; off opens a not-loaded page until services.refresh"},
		ochazuke.Setting{Name: "services.provenance", Type: ochazuke.SettingBoolean, Default: true, Description: "Read TUIOS pane/agent and mtm project metadata to attribute services; off makes no metadata calls and shows provenance disabled"},
	); err != nil {
		return nil, err
	}
	id := func(noun string) ochazuke.Parameter {
		return ochazuke.Parameter{Type: "string", Required: true, Noun: noun, Description: "The exact " + noun + " id (from " + noun + " list); defaults to the selected row", From: ochazuke.SourceRow}
	}
	a.r.Define("services").Verb("refresh", a.refresh, "Discover local TCP listeners and bound UDP sockets again")
	a.r.Define("endpoint").Verb("list", a.listEndpoints, "List local endpoints: one row per listening TCP or bound UDP socket of one process").Rows("endpoint").Fetch().Quiet()
	a.r.Define("endpoint").Verb("show", a.showEndpoint, "Show an endpoint with its process, cwd and provenance").Positional("id")
	a.r.Define("endpoint").Verb("open", a.openEndpoint, "Open a TCP endpoint in the browser with an explicit http or https scheme, after revalidating its process").Positional("id", "scheme")
	a.r.Define("service").Verb("list", a.listServices, "List processes with at least one listening or bound socket").Rows("service").Fetch().Quiet()
	a.r.Define("service").Verb("show", a.showService, "Show a service process with its endpoints and provenance").Positional("id")
	a.setParams("endpoint.show", map[string]ochazuke.Parameter{"id": id("endpoint")})
	a.setParams("endpoint.open", map[string]ochazuke.Parameter{
		"id":     id("endpoint"),
		"scheme": {Type: "string", Required: true, Enum: []string{"http", "https"}, Description: "Web scheme to open; a port does not say which protocol it speaks"},
	})
	a.setParams("service.show", map[string]ochazuke.Parameter{"id": id("service")})
	if err := a.r.AddVerbCheck("endpoint.open", ochazuke.VerbCheck{Name: "browser-target", Check: a.confirmOpen}); err != nil {
		return nil, err
	}
	a.r.Define("endpoint").Columns(endpointColumns...).Props(factProps("endpoint", endpointFields)...).Store(a.endpointStore())
	a.r.Define("service").Columns(serviceColumns...).Props(factProps("service", serviceFields)...).Store(a.serviceStore())
	// Every key is a binding row; help and the footer read them.
	a.r.Define("endpoint").Mode("normal").
		Bind("r", ochazuke.Do("services.refresh")).
		Bind("o", ochazuke.Do("endpoint.open"))
	if err := ocli.DeclarePage(a.r, servicesPage()); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) setParams(key string, params map[string]ochazuke.Parameter) {
	op := a.r.Operations[key]
	op.Parameters = params
	a.r.Operations[op.Key()] = op
}

func servicesPage() ocli.CatalogPage {
	return ocli.CatalogPage{
		ID: pageID, Title: "Services", Layout: ocli.LayoutSplit,
		Lists:  []ocli.CatalogList{{ID: listID, Noun: "endpoint", Source: "endpoint.list", Title: "Local endpoints"}},
		Detail: &ocli.CatalogDetail{Fields: detailFields},
	}
}

func (a *App) shell() *ocli.Shell {
	sh := ocli.New(a.r, "o-services", a.in, a.out, a.errOut)
	sh.Version = version
	return sh
}

// current is the snapshot verbs read. CLI: discover now, once per process.
// TUI: the last landed snapshot; discovery runs only off the loop.
func (a *App) current(ctx context.Context) (Snapshot, error) {
	if !a.loaded && !a.tui {
		a.apply(a.collect(ctx, Snapshot{}, a.r.Settings().Bool("services.provenance")))
	}
	if !a.loaded {
		return Snapshot{}, errors.New("services are not loaded yet; run services.refresh (r)")
	}
	return a.snapshot, nil
}

func (a *App) collect(ctx context.Context, old Snapshot, provenance bool) Snapshot {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	next := a.scan.Discover(ctx)
	var meta metadata
	if provenance {
		meta = a.metadata(ctx)
		enrich(&next, meta, a.scan.Process)
	}
	next = reconcile(old, next, provenance)
	if provenance {
		refreshRetainedContext(&next, meta)
	}
	return next
}

func (a *App) apply(s Snapshot) {
	a.snapshot, a.loaded = s, true
	if a.changed != nil {
		a.changed()
	}
}

func (a *App) refresh(c ochazuke.Ctx) ochazuke.Result {
	old, provenance := a.snapshot, a.r.Settings().Bool("services.provenance")
	if !a.tui {
		a.apply(a.collect(background(), old, provenance))
		return ochazuke.Done(a.summary())
	}
	return ochazuke.Pending(func(ctx context.Context) (any, error) {
		return a.collect(ctx, old, provenance), nil
	}).ReplacingAt("app", "services-refresh").Then(func(c ochazuke.Ctx, v any, err error) {
		if err != nil {
			c.Error(err.Error())
			return
		}
		a.apply(v.(Snapshot))
		if !a.snapshot.Complete {
			c.Error("Incomplete discovery: " + strings.Join(a.snapshot.Problems, "; "))
		}
	})
}

func (a *App) summary() map[string]any {
	return map[string]any{
		"observed_at": a.snapshot.At.Format(time.RFC3339), "complete": a.snapshot.Complete,
		"problems": nonNil(a.snapshot.Problems), "restricted": a.snapshot.Restricted,
		"services": len(a.snapshot.Services), "endpoints": len(a.snapshot.Endpoints),
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// targetID is the id argument, else the selected row's.
func targetID(c ochazuke.Ctx) string {
	id := ochazuke.StringArg(c.Args, "id", "")
	if id == "" && c.Instance != nil {
		// A page row's Data is its row id.
		id, _ = c.Instance.Data.(string)
		if id == "" {
			id = strings.TrimPrefix(c.Instance.ID, listID+"/")
		}
	}
	return id
}

func (s Snapshot) service(id string) (Service, bool) {
	for _, svc := range s.Services {
		if svc.ID == id {
			return svc, true
		}
	}
	return Service{}, false
}

func (s Snapshot) endpoint(id string) (Service, Endpoint, error) {
	for _, e := range s.Endpoints {
		if e.ID == id {
			svc, ok := s.service(e.Service)
			if !ok {
				break
			}
			return svc, e, nil
		}
	}
	return Service{}, Endpoint{}, fmt.Errorf("endpoint %q is not present; list endpoints or refresh", id)
}

func (a *App) listEndpoints(c ochazuke.Ctx) ochazuke.Result {
	s, err := a.current(background())
	if err != nil {
		return ochazuke.Failed(err)
	}
	return ochazuke.Done(endpointRows(s))
}

func (a *App) listServices(c ochazuke.Ctx) ochazuke.Result {
	s, err := a.current(background())
	if err != nil {
		return ochazuke.Failed(err)
	}
	return ochazuke.Done(serviceRows(s))
}

func (a *App) showEndpoint(c ochazuke.Ctx) ochazuke.Result {
	s, err := a.current(background())
	if err != nil {
		return ochazuke.Failed(err)
	}
	svc, e, err := s.endpoint(targetID(c))
	if err != nil {
		return ochazuke.Failed(err)
	}
	return ochazuke.Done(endpointData(s, svc, e))
}

func (a *App) showService(c ochazuke.Ctx) ochazuke.Result {
	s, err := a.current(background())
	if err != nil {
		return ochazuke.Failed(err)
	}
	svc, ok := s.service(targetID(c))
	if !ok {
		return ochazuke.Failed(fmt.Errorf("service %q is not present; list services or refresh", targetID(c)))
	}
	return ochazuke.Done(serviceData(s, svc))
}

// confirmOpen refuses what cannot be opened and asks before the OS handoff,
// naming the exact URL. The endpoint is revalidated after confirmation.
func (a *App) confirmOpen(_ *ochazuke.Runtime, _ ochazuke.Context, args ochazuke.Arguments) ochazuke.Verdict {
	s, err := a.current(background())
	if err != nil {
		return ochazuke.Refuse(err.Error())
	}
	svc, e, err := s.endpoint(ochazuke.StringArg(args, "id", ""))
	if err != nil {
		return ochazuke.Refuse(err.Error())
	}
	if svc.Stale || !svc.Verified() {
		return ochazuke.Refuse("endpoint identity is unavailable or stale; refresh before opening")
	}
	target, err := browserURL(e, ochazuke.StringArg(args, "scheme", ""))
	if err != nil {
		return ochazuke.Refuse(err.Error())
	}
	return ochazuke.AskFirst("Open " + target + " in the browser? The endpoint is checked again first.")
}

func (a *App) openEndpoint(c ochazuke.Ctx) ochazuke.Result {
	s, err := a.current(background())
	if err != nil {
		return ochazuke.Failed(err)
	}
	svc, e, err := s.endpoint(targetID(c))
	if err != nil {
		return ochazuke.Failed(err)
	}
	scheme := ochazuke.StringArg(c.Args, "scheme", "")
	work := func(ctx context.Context) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		target, err := validatedURL(ctx, svc, e, scheme, a.scan)
		if err != nil {
			return nil, err
		}
		if err := a.opener(ctx, target); err != nil {
			return nil, err
		}
		return map[string]string{"opened": target}, nil
	}
	if !a.tui {
		v, err := work(background())
		if err != nil {
			return ochazuke.Failed(err)
		}
		return ochazuke.Done(v)
	}
	return ochazuke.Pending(work)
}

func endpointLabel(e Endpoint) string {
	host := e.Address
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return e.Transport + " " + host + ":" + strconv.Itoa(int(e.Port))
}

func shortRef(ref string) string {
	if i := strings.LastIndex(ref, "/"); i >= 0 && i+1 < len(ref) {
		return ref[i+1:]
	}
	return ref
}

func originCells(svc Service) (pane, agent, project string) {
	if svc.Origin == nil {
		return "unknown", "unknown", "unknown"
	}
	pane, agent, project = shortRef(svc.Origin.Pane), "unknown", "unknown"
	if svc.Origin.Harness != "" {
		agent = svc.Origin.Harness
	}
	if svc.Origin.Project != "" {
		project = shortRef(svc.Origin.Project)
	}
	return pane, agent, project
}

func endpointData(s Snapshot, svc Service, e Endpoint) map[string]any {
	d := serviceData(s, svc)
	delete(d, "endpoints")
	d["id"], d["service"] = e.ID, svc.ID
	d["label"] = svc.Command + " " + endpointLabel(e)
	d["transport"], d["family"], d["address"], d["port"], d["state"] = e.Transport, e.Family, e.Address, int(e.Port), e.State
	if e.Transport == "tcp" {
		u, _ := browserURL(e, "http")
		d["url_hint"] = strings.TrimPrefix(u, "http://") + " (choose http or https when opening)"
	} else {
		d["url_hint"] = "not openable: UDP bound socket"
	}
	return d
}

func serviceData(s Snapshot, svc Service) map[string]any {
	d := map[string]any{
		"id": svc.ID, "label": svc.Command, "program": svc.Command, "pid": svc.PID,
		"cwd": svc.CWD, "cwd_status": svc.CWDStatus, "identity_status": svc.IdentityStatus,
		"stale": svc.Stale, "observed_at": svc.ObservedAt.Format(time.RFC3339),
		"provenance_status": svc.ProvenanceStatus,
		"scan_complete":     s.Complete, "scan_restricted": s.Restricted,
		"scan_problems": strings.Join(s.Problems, "; "),
		"origin_pane":   "unknown", "origin_agent": "unknown", "origin_harness": "unknown", "origin_project": "unknown",
		"current_pane": svc.Current.Pane, "current_project": svc.Current.Project,
		"current_conversation": svc.Current.Conversation, "current_status": svc.Current.Status,
	}
	if svc.Verified() {
		d["birth"] = map[string]any{"boot_id": svc.BootID, "start_kind": svc.StartKind, "start_value": svc.StartValue}
	}
	if o := svc.Origin; o != nil {
		d["origin_pane"], d["origin_source"], d["origin_observed_at"] = o.Pane, o.Source, o.At.Format(time.RFC3339)
		if o.Agent != "" {
			d["origin_agent"], d["origin_harness"] = o.Agent, o.Harness
		}
		if o.Project != "" {
			d["origin_project"] = o.Project
		}
	}
	var eps []string
	for _, e := range s.Endpoints {
		if e.Service == svc.ID {
			eps = append(eps, endpointLabel(e))
		}
	}
	sort.Strings(eps)
	d["endpoints"] = eps
	return d
}

func endpointRows(s Snapshot) []ocli.Row {
	rows := make([]ocli.Row, 0, len(s.Endpoints))
	for _, e := range s.Endpoints {
		svc, ok := s.service(e.Service)
		if !ok {
			continue
		}
		d := endpointData(s, svc, e)
		pane, agent, project := originCells(svc)
		cwd := svc.CWD
		if svc.CWDStatus != "available" {
			cwd = "(" + svc.CWDStatus + ")"
		}
		program := svc.Command
		if svc.Stale {
			program = "(stale) " + program
		}
		rows = append(rows, ocli.Row{Noun: "endpoint", ID: e.ID, Label: fmt.Sprint(d["label"]), Data: d,
			Cells: []string{program, strconv.Itoa(svc.PID), endpointLabel(e), cwd, pane, agent, project}})
	}
	return rows
}

func serviceRows(s Snapshot) []ocli.Row {
	rows := make([]ocli.Row, 0, len(s.Services))
	for _, svc := range s.Services {
		d := serviceData(s, svc)
		eps, _ := d["endpoints"].([]string)
		cwd := svc.CWD
		if svc.CWDStatus != "available" {
			cwd = "(" + svc.CWDStatus + ")"
		}
		rows = append(rows, ocli.Row{Noun: "service", ID: svc.ID, Label: svc.Command, Data: d,
			Cells: []string{svc.Command, strconv.Itoa(svc.PID), strings.Join(eps, ", "), cwd, svc.ProvenanceStatus}})
	}
	return rows
}

func background() context.Context { return context.Background() }
