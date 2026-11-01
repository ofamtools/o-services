<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-animated-dark.svg">
    <img alt="o-services" src="assets/logo-animated-light.svg" width="400">
  </picture>
  <br>
  <a href="https://github.com/ofamtools/o-services/releases"><img alt="Latest release" src="https://img.shields.io/github/v/release/ofamtools/o-services"></a>
  <a href="https://pkg.go.dev/ofam.dev/o-services"><img alt="Go Reference" src="https://pkg.go.dev/badge/ofam.dev/o-services.svg"></a>
  <a href="go.mod"><img alt="Go version" src="https://img.shields.io/github/go-mod/go-version/ofamtools/o-services"></a>
</p>

<p align="center">List local processes with open listening ports, PIDs, working directories, and proven origin.</p>

`o-services` surfaces only processes that hold an open listening port (TCP sockets in `LISTEN` and bound UDP sockets): one row per endpoint with the program, PID, bound address, port, and the process's working directory. When TUIOS can prove which pane or agent spawned the process, the row attributes that pane, agent, and the pane's `mtm` project; otherwise it explicitly marks provenance `unknown` and explains why. Any selected TCP endpoint opens directly in your browser with verified scheme selection (`http` or `https`).

It is an o-family extension: `oden` dispatches the CLI, `ochazuke` supplies the TUI, settings, keybindings, and catalog. Nothing runs as a persistent background daemon: discovery runs on demand when invoking a CLI verb, when opening the TUI, or upon explicit refresh (`r`).

---

## Installation

Install using Go:

```sh
go install ofam.dev/o-services@latest
```

Or build from source:

```sh
make build          # outputs ./o-services
cp o-services ~/bin/
```

Once on `PATH`, `o-services` is discovered by `o` and `mtm` as the `services` extension.

---

## CLI Usage

```sh
o-services                                  # Interactive Services TUI
o-services endpoint list --format table     # Real-time table of all listening endpoints
o-services endpoint list                    # Endpoints as JSON
o-services endpoint show ID                 # Detailed inspection of one endpoint + provenance
o-services endpoint open ID http            # Confirm target and open in default browser
o-services endpoint open ID https --yes     # Non-interactive browser open
o-services service list --format table      # Process-grouped view (endpoints joined)
o-services service show ID                  # Inspect one process and all its listening ports
o-services services refresh                 # Re-scan system and print discovery summary
o-services view --smoke --width 140         # Headless single-frame render for testing
```

`ID` is the stable identifier from `endpoint list` or `service list`. It incorporates the process's boot id, PID, start time, transport, and socket address, ensuring it never collides or misidentifies a PID reused by an exited process.

---

## Interactive TUI

Launch with `o-services` or navigate to the **Services** tab inside `mtm`.

| Key | Verb | What it does |
| --- | --- | --- |
| `r` | `services.refresh` | Re-scan the kernel process & socket tables off the UI loop; preserves selection |
| `o` | `endpoint.open` | Choose `http` or `https`, review target URL, re-verify process birth, and open |
| `q` | `app.quit` | Exit the viewer |

Standard Ochazuke motions apply: `/` filters rows, `j`/`k` navigates, `ctrl+d`/`ctrl+u` half-page scrolls, `:` opens the command palette, and `?` displays interactive help.

---

## What Is Listed

- **TCP sockets in LISTEN** and **UDP sockets bound** to a local port visible to the current user. UDP rows note `bound`.
- **macOS:** `lsof -F` machine parsing for socket states, batched PID-targeted `lsof -d cwd` for working directories, and `sysctl kern.proc` for start times and short names.
- **Linux:** `/proc/net/{tcp,tcp6,udp,udp6}`, socket inodes matched against `/proc/PID/fd`, and `/proc/PID/cwd`.
- **No environment snooping:** Only kernel process names and verified working directories are read; environments and secret CLI arguments are never collected.

---

## Provenance Tracking

When running alongside a TUIOS daemon publishing verified process births:

- **Pane:** Proven if the pane's root process is on the process ancestry chain.
- **Agent:** Proven if the verified agent hook's process lineage matches in the same pane.
- **Project:** Attributed from `mtm`'s typed pane project assignment, never guessed from directories or names.

Processes launched outside TUIOS or without unbroken parentage remain honest: marked `unknown` with the specific reason.

---

## Settings

| Setting | Default | Description |
| --- | --- | --- |
| `services.load-on-open` | `true` | When `false`, the TUI opens unloaded until `r` is pressed |
| `services.provenance` | `true` | When `false`, disables TUIOS/mtm ancestry walks |

Configure via `o-services settings list`, `ochazuke.settings`, or in `~/.config/o-services/settings.json`.

---

## Development & Testing

```sh
make check          # runs gofmt, go vet, and unit tests
make test           # runs test suite with isolated temp XDG/HOME
```
