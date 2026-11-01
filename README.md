# o-services

`o-services` lists the local processes that listen on a TCP port or hold a
bound UDP socket: one row per endpoint, with the program, PID, address and
port, and the process's working directory. When TUIOS can prove which pane or
agent started a process, the row shows that pane, agent and the pane's mtm
project; otherwise it says `unknown` and why. A selected TCP endpoint opens in
the browser after you choose `http` or `https`.

It is an o-family app: oden dispatches the CLI, ochazuke supplies the TUI,
settings, bindings and catalog. Nothing runs in the background: discovery
happens when you run a command, when the TUI opens, and when you refresh.

Build with the Go version in [go.mod](go.mod) or newer:

```sh
make build          # ./o-services
```

## Commands

```sh
o-services                                  # the Services page (TUI)
o-services view --smoke --width 140         # discover once, print one frame
o-services endpoint list --format table     # every endpoint, fresh
o-services endpoint list                    # the same rows as JSON
o-services endpoint show ID                 # one endpoint, its process and provenance
o-services endpoint open ID http            # asks, re-checks, then opens http://ADDR:PORT/
o-services endpoint open ID https --yes     # accept the question non-interactively
o-services service list --format table      # one row per process, its endpoints joined
o-services service show ID
o-services services refresh                 # discover and print a summary
```

`ID` is the `id` of a row from `endpoint list` (or `service list`). An
endpoint id is the process's birth key followed by transport, family, address
and port, so it stays the same across refreshes while the same process holds
the same socket, and never names a different process.

On the Services page:

| Key | Verb | What it does |
| --- | --- | --- |
| `r` | `services.refresh` | Discover again, off the UI loop; the selection is kept by endpoint id |
| `o` | `endpoint.open` | Choose http or https, confirm the exact URL, re-check, open |
| `q` | `app.quit` | Quit |

List navigation, filtering, the palette (`:`) and help (`?`) are ochazuke's.
Every key is a binding row: rebind or remove them in your `bindings.json`; the
verbs stay reachable from the palette and the CLI.

## What is listed

- TCP sockets in LISTEN and UDP sockets bound to a nonzero local port, of the
  processes the current user can see. UDP rows say `bound`: a bound socket is
  not a claim that anything serves it.
- macOS: `lsof -F` machine fields for sockets (one call per protocol), one
  batched PID-targeted `lsof -d cwd` call for working directories, and the
  kernel's process table (`sysctl kern.proc`) for PID, parent, start time and
  short name.
- Linux: `/proc/net/{tcp,tcp6,udp,udp6}`, socket inodes matched to
  `/proc/PID/fd` in the current network namespace only, and `/proc/PID/cwd`.
  Processes of other users count as `restricted`, outside the listing's scope.
- Other platforms: an explicit "unsupported" problem, never an empty success.
- The program column is the kernel's short process name. o-services never reads
  a process's environment or full command line.

A scan reports `complete`, any `problems`, the `restricted` count and when it
ran. A failed or partial scan never looks like an empty one: the page status
says `INCOMPLETE`, and rows kept from an earlier scan are marked stale and
cannot be opened until a refresh confirms them. A stopped process, or one that
closed all its sockets, drops off at the next scan; nothing is persisted.

## Identity

A process is identified by boot id + PID + kernel start time (Linux clock
ticks since boot, macOS microseconds since the epoch). Each is read before and
after the scan gathers sockets and cwd; a process that exited or changed in
between is dropped. A process whose start time cannot be read is still listed,
with `identity_status: unavailable` and an id that lasts one scan only: it gets
no provenance and cannot be opened. There is no PID-only fallback.

## Provenance

Provenance needs a TUIOS daemon that publishes, on `list-agents` rows:

- `pane_process`: the pane's root process as the daemon spawned it (pid, boot
  id, start time);
- `agent_process`: an agent's root process, published only after the daemon
  verified the agent hook's process lineage inside that pane.

o-services walks each service's ancestry with the kernel process table,
re-reads it, and attributes:

- the **pane** whose `pane_process` birth is on the ancestry;
- the **agent** whose `agent_process` birth is on the ancestry, in that same
  pane (a shell sibling of the agent in the same pane is not the agent's);
- the **project** from mtm's typed pane assignment (`mtm pane list --all`),
  never from the cwd or a name.

The agent's current conversation is shown as current context only: it is not
the conversation that started the service. An origin seen once is kept for that
same process incarnation for as long as the TUI runs, even if the pane closes
later; a restart does not reconstruct it.

Anything that cannot be proven stays `unknown` with the reason: TUIOS not
running, an older TUIOS without `pane_process`/`agent_process`, an unreadable
ancestry, a process started outside every pane. Older TUIOS daemons still give
a complete listing; only provenance is unknown.

## Opening in the browser

A port does not say which protocol it speaks, so `endpoint.open` always needs
`http` or `https`; the TUI asks with a chooser, the CLI takes it as an
argument. There is no probe and no guess from the port number.

The URL is built from the endpoint's parsed address and port, with path `/`:
`0.0.0.0` opens `127.0.0.1`, `::` opens `::1`, other addresses stay as bound
(IPv6 bracketed, zones escaped). UDP rows, stale rows, rows without a verified
birth, vanished endpoints and other schemes are refused. Before the handoff the
question names the exact URL; after you accept, the process birth and the socket
are checked again with a fresh scan, so another process that took the port is
not opened as the one you selected. The opener is `open -- URL` on macOS and
`xdg-open URL` on Linux. A race between that last check and the browser's
connection remains and cannot be closed from here.

## Settings

| Setting | Default | Off means |
| --- | --- | --- |
| `services.load-on-open` | `true` | The TUI opens with a "not loaded" page until `r` |
| `services.provenance` | `true` | No TUIOS or mtm calls; every row says provenance `disabled` |

CLI list, show, open and refresh always discover, whatever `load-on-open` says.
Settings live in ochazuke's settings file for `o-services` and on the Settings
page.

## Development

`make check` runs gofmt, vet and the tests with `GOWORK=off`. Tests use fake
scanners and openers and a temp HOME/XDG; none opens a browser or reads your
config. See [AGENTS.md](AGENTS.md) for the rules.
