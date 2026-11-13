package main

// paneEnv reads a pane variable by its bare name (SERVER): mtm's MTM_<name>.
// The TUIOS_ names are not read.
func paneEnv(get func(string) string, name string) string {
	return get("MTM_" + name)
}
