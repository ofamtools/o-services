package main

import (
	"fmt"

	"ofam.dev/ochazuke"
)

// The service and endpoint nouns are thing nouns over the discovery
// snapshot, read only (facts stores): the standard list and get answer the
// rows the named verbs list and show answer, from the same snapshot (a CLI
// call discovers once; the TUI answers the last landed one). A service is
// a process with a socket; an endpoint is one socket of it. refresh
// (services.refresh) is how the snapshot changes; nothing here writes.

func factProps(noun string, fields []struct{ name, kind, desc string }) []ochazuke.PropSpec {
	props := []ochazuke.PropSpec{
		{Name: "id", Key: true, Parameter: ochazuke.Parameter{Type: "string", Description: "The " + noun + "'s id"}},
		{Name: "label", Role: ochazuke.RoleLabel, Parameter: ochazuke.Parameter{Type: "string", Description: "The " + noun + " as lists show it"}},
	}
	for _, f := range fields {
		props = append(props, ochazuke.PropSpec{Name: f.name, ReadOnly: true, Parameter: ochazuke.Parameter{Type: f.kind, Description: f.desc}})
	}
	return props
}

var serviceFields = []struct{ name, kind, desc string }{
	{"program", "string", "The process's command"},
	{"pid", ochazuke.SettingInteger, "The process id"},
	{"cwd", "string", "The process's working directory"},
	{"cwd_status", "string", "Whether the working directory could be read"},
	{"identity_status", "string", "Whether the process identity is verified"},
	{"stale", ochazuke.SettingBoolean, "The process is no longer the one first seen"},
	{"observed_at", "string", "When the process was observed (RFC 3339)"},
	{"provenance_status", "string", "Whether the pane, agent and project that started it are known"},
	{"origin_pane", "string", "The pane that started it"},
	{"origin_agent", "string", "The agent that started it"},
	{"origin_project", "string", "The project it was started in"},
	{"current_pane", "string", "The pane it belongs to now"},
	{"current_project", "string", "The project it belongs to now"},
	{"endpoints", ochazuke.SettingList, "The sockets the process listens on or is bound to"},
}

var endpointFields = append([]struct{ name, kind, desc string }{
	{"service", "string", "The id of the service (process) the socket belongs to"},
	{"transport", "string", "tcp or udp"},
	{"family", "string", "The address family"},
	{"address", "string", "The bound address"},
	{"port", ochazuke.SettingInteger, "The port"},
	{"state", "string", "The socket's state"},
	{"url_hint", "string", "The URL a browser would open, for a TCP socket"},
}, serviceFields[:len(serviceFields)-1]...)

func (a *App) serviceStore() ochazuke.Store {
	return ochazuke.Store{Kind: ochazuke.Collection, Facts: true,
		List: func(ochazuke.Ctx, ochazuke.ListQuery) ([]ochazuke.Thing, error) {
			s, err := a.current(background())
			if err != nil {
				return nil, err
			}
			out := make([]ochazuke.Thing, 0, len(s.Services))
			for _, svc := range s.Services {
				out = append(out, ochazuke.Thing(serviceData(s, svc)))
			}
			return out, nil
		},
		Get: func(_ ochazuke.Ctx, id string) (ochazuke.Thing, error) {
			s, err := a.current(background())
			if err != nil {
				return nil, err
			}
			svc, ok := s.service(id)
			if !ok {
				return nil, fmt.Errorf("service %q is not present; list services or refresh", id)
			}
			return ochazuke.Thing(serviceData(s, svc)), nil
		},
	}
}

func (a *App) endpointStore() ochazuke.Store {
	return ochazuke.Store{Kind: ochazuke.Collection, Facts: true,
		List: func(ochazuke.Ctx, ochazuke.ListQuery) ([]ochazuke.Thing, error) {
			s, err := a.current(background())
			if err != nil {
				return nil, err
			}
			out := make([]ochazuke.Thing, 0, len(s.Endpoints))
			for _, e := range s.Endpoints {
				if svc, ok := s.service(e.Service); ok {
					out = append(out, ochazuke.Thing(endpointData(s, svc, e)))
				}
			}
			return out, nil
		},
		Get: func(_ ochazuke.Ctx, id string) (ochazuke.Thing, error) {
			s, err := a.current(background())
			if err != nil {
				return nil, err
			}
			svc, e, err := s.endpoint(id)
			if err != nil {
				return nil, err
			}
			return ochazuke.Thing(endpointData(s, svc, e)), nil
		},
	}
}
