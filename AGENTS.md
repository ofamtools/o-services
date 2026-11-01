# o-services

Lists local listening services (PID, endpoint, cwd) with the pane, agent and
project that started them when TUIOS can prove it, and opens a selected TCP
endpoint in the browser. An o-family app: oden dispatches the CLI, ochazuke
supplies the TUI, settings, bindings and catalog.

## Plans and todos

Family plans, todos and the roadmap live in `~/hub/o-plans/` (`README.md` is
the index). The accepted design is
`~/hub/o-plans/backlog-20261031-232011/services-design.md`. Do not add a
TODO.md or docs/plans/ to this repo.

## Rules

- Identity is birth-qualified: boot id + PID + kernel start time. Never key a
  service, an origin or an open on a PID alone; a row without a verified birth
  gets a one-snapshot id and cannot be opened.
- Provenance is evidence, not a guess. A pane or agent is attributed only when
  a TUIOS `pane_process` / `agent_process` birth is on the service's re-read
  ancestry. Unknown stays unknown, with the reason. Never infer it from cwd,
  names, TTYs or the current pane's agent label; projects come only from mtm's
  typed pane assignment.
- Never read a process's environment or full command line; the kernel short
  name is the program column.
- No daemon, watcher, poller, killer, scanner of remote hosts or HTTP probe.
  Discovery runs on CLI calls, at TUI open (setting) and on `services.refresh`.
- A failed or partial scan must not look like an empty, complete one.
- Keys are binding rows; never bind F1 to F12.
- Tests run with a temp HOME/XDG (`testApp` does this) and fake openers and
  scanners; no test opens a browser or reads the owner's config.
- Build and test with `make check` (GOWORK=off, -trimpath).
