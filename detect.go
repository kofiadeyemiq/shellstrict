package shellstrict

import (
	"path"
	"strings"
)

// shebangInfo is what was learned from a script's first line (or a forced
// override).
type shebangInfo struct {
	// shell is "bash", "sh", or "" if the interpreter is not recognised.
	shell string
	// present is true when the file itself starts with a "#!" line.
	present bool
	// flags are the strict-mode flags already enabled on the shebang line
	// itself, e.g. "#!/bin/bash -eu".
	flags flagState
}

// detectShebang inspects the first line of src to determine which shell a
// script targets. forced, when non-empty, overrides detection entirely (the
// --shell flag).
func detectShebang(src []byte, forced string) shebangInfo {
	if forced != "" {
		return shebangInfo{shell: forced, present: hasShebangLine(src)}
	}

	line := firstLine(src)
	if !strings.HasPrefix(line, "#!") {
		return shebangInfo{}
	}

	fields := strings.Fields(strings.TrimPrefix(line, "#!"))
	if len(fields) == 0 {
		return shebangInfo{present: true}
	}

	interp := fields[0]
	rest := fields[1:]
	if path.Base(interp) == "env" {
		// #!/usr/bin/env bash [-eu]
		// #!/usr/bin/env -S bash -eu
		if len(rest) > 0 && rest[0] == "-S" {
			rest = rest[1:]
		}
		if len(rest) == 0 {
			return shebangInfo{present: true}
		}
		interp = rest[0]
		rest = rest[1:]
	}

	base := path.Base(interp)
	var shell string
	switch base {
	case "bash":
		shell = "bash"
	case "sh", "dash", "ash":
		shell = "sh"
	default:
		shell = ""
	}

	var fs flagState
	if shell != "" {
		applySetWords(rest, &fs)
	}
	return shebangInfo{shell: shell, present: true, flags: fs}
}

func hasShebangLine(src []byte) bool {
	return strings.HasPrefix(firstLine(src), "#!")
}

func firstLine(src []byte) string {
	if i := strings.IndexByte(string(src), '\n'); i >= 0 {
		return string(src[:i])
	}
	return string(src)
}

// shellForExtension guesses a shell family from a filename when there is no
// shebang to go on. It is only ever used as a fallback.
func shellForExtension(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".bash":
		return "bash"
	case ".sh":
		return "sh"
	default:
		return ""
	}
}
