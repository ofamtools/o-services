# Changelog

## v0.5.0

- Changed: requires ochazuke v0.134.0 and oden v0.5.1 (from v0.110.1 and v0.3.0). The lexicon word list is merged with ochazuke's. `service` and `endpoint` carry the standard noun scaffold (dynamic list, show, yank, history, watch) through their facts stores; they are read-only discovery snapshots, so archive, create and edit stay unavailable. A direct run from a pane records its pane and session through the library's `MTM_*` actor reading.
- Changed, breaking: the `TUIOS_*` fallback for pane variables is gone; only `MTM_*` is read (`MTM_SERVER` names the provenance namespace).
- The CLI has one command that is not a catalog verb, `view`; it only launches the TUI, so it stays a command.
