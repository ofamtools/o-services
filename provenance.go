package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type processesBirth struct{ Birth }
type agentProcess struct {
	Birth
	Harness string `json:"harness_id"`
	Source  string `json:"source"`
}
type paneMetadata struct {
	Window       string          `json:"window_id"`
	Session      string          `json:"session"`
	Conversation string          `json:"agent_session_id"`
	PaneProcess  *processesBirth `json:"pane_process"`
	AgentProcess *agentProcess   `json:"agent_process"`
}
type metadata struct {
	Namespace      string
	Panes          []paneMetadata
	Projects       map[string]string
	Problem        string
	ProjectProblem string
}

func readMetadata(ctx context.Context) metadata {
	server := os.Getenv("TUIOS_SERVER")
	if server == "" {
		server = "default"
	}
	m := metadata{Namespace: "tuios/" + server, Projects: make(map[string]string)}
	answer := command(ctx, "tuios", "call", "list-agents", `{"all":true,"all_sessions":true}`)
	if err := answer.error("tuios list-agents"); err != nil {
		m.Problem = err.Error()
		return m
	}
	var response struct {
		Agents []paneMetadata `json:"agents"`
	}
	if err := json.Unmarshal(answer.Output, &response); err != nil {
		m.Problem = "invalid TUIOS metadata: " + err.Error()
		return m
	}
	if response.Agents == nil {
		m.Problem = "TUIOS response has no agents array"
		return m
	}
	m.Panes = response.Agents
	projects := command(ctx, "mtm", "pane", "list", "--all")
	if err := projects.error("mtm pane list"); err != nil {
		m.ProjectProblem = err.Error()
		return m
	}
	assigned, err := paneProjects(projects.Output)
	if err != nil {
		m.ProjectProblem = err.Error()
		return m
	}
	m.Projects = assigned
	return m
}

func (m metadata) current(pane paneMetadata) PaneContext {
	current := PaneContext{Pane: m.Namespace + "/" + pane.Window, Conversation: pane.Conversation, Status: "current pane context; not launch conversation"}
	if project := m.Projects[pane.Window]; project != "" {
		current.Project = "mtm/" + project
	}
	if m.ProjectProblem != "" {
		current.Status += "; project unavailable: " + m.ProjectProblem
	}
	return current
}

func traceOrigin(service Service, m metadata, read func(int) (Process, error), at time.Time) (*Origin, PaneContext, string) {
	unknown := PaneContext{Status: "unknown"}
	if !service.Verified() {
		return nil, unknown, "unknown: process birth unavailable"
	}
	if m.Problem != "" {
		return nil, unknown, "unknown: " + m.Problem
	}
	var chain [64]Process
	pid := service.PID
	var agent *agentProcess
	for n := range chain {
		p, err := read(pid)
		if err != nil || !p.Verified() {
			return nil, unknown, fmt.Sprintf("unknown: ancestry unreadable at PID %d", pid)
		}
		if n == 0 && p.Birth != service.Birth {
			return nil, unknown, "unknown: service process changed"
		}
		for _, prior := range chain[:n] {
			if prior.PID == pid {
				return nil, unknown, "unknown: ancestry cycle"
			}
		}
		chain[n] = p
		for _, pane := range m.Panes {
			if agent == nil && pane.AgentProcess != nil && pane.AgentProcess.Source == "hook-lineage" && pane.AgentProcess.Birth == p.Birth {
				agent = pane.AgentProcess
			}
			if pane.PaneProcess == nil || pane.PaneProcess.Birth != p.Birth || pane.Window == "" {
				continue
			}
			for _, observed := range chain[:n+1] {
				now, err := read(observed.PID)
				if err != nil || now.Birth != observed.Birth || now.Parent != observed.Parent {
					return nil, unknown, "unknown: ancestry changed during verification"
				}
			}
			current := m.current(pane)
			origin := &Origin{Pane: current.Pane, Project: current.Project, At: at, Source: "verified-ancestry"}
			status := "pane ancestry verified; agent origin unknown"
			// An agent root is attributable only to the pane whose root also
			// encloses the service, never another pane's current agent label.
			if agent != nil && pane.AgentProcess == agent {
				origin.AgentProcess = agent.Birth
				origin.Agent = m.Namespace + "/agent/" + agent.Key()
				origin.Harness = agent.Harness
				status = "pane and agent process ancestry verified; launch conversation unknown"
			}
			if m.ProjectProblem != "" {
				status += "; project unavailable: " + m.ProjectProblem
			}
			return origin, current, status
		}
		if p.Parent <= 1 {
			break
		}
		pid = p.Parent
	}
	return nil, unknown, "unknown: no birth-qualified pane root on observed ancestry"
}

func enrich(snapshot *Snapshot, m metadata, read func(int) (Process, error)) {
	for i := range snapshot.Services {
		s := &snapshot.Services[i]
		s.Origin, s.Current, s.ProvenanceStatus = traceOrigin(*s, m, read, snapshot.At)
	}
}

func refreshRetainedContext(snapshot *Snapshot, m metadata) {
	for i := range snapshot.Services {
		s := &snapshot.Services[i]
		if s.Origin == nil {
			continue
		}
		s.Current = PaneContext{Status: "origin pane not currently visible"}
		for _, pane := range m.Panes {
			if strings.TrimSuffix(m.Namespace, "/")+"/"+pane.Window == s.Origin.Pane {
				s.Current = m.current(pane)
				break
			}
		}
		if m.Problem != "" {
			s.Current.Status = "current context unavailable: " + m.Problem
		}
	}
}

// paneProjects reads mtm's typed pane-to-project assignments from its pane
// list rows (props flat on the row, or under data). A pane with no project
// is absent: unassociated, never guessed from its cwd.
func paneProjects(data []byte) (map[string]string, error) {
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("invalid mtm pane rows: %w", err)
	}
	out := make(map[string]string)
	for _, row := range rows {
		id, _ := row["id"].(string)
		project, _ := row["project"].(string)
		if inner, ok := row["data"].(map[string]any); ok && project == "" {
			project, _ = inner["project"].(string)
		}
		id = strings.TrimPrefix(id, "pane/")
		if id != "" && project != "" {
			out[id] = project
		}
	}
	return out, nil
}
