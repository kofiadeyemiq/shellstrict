package shellstrict

// flagState tracks which of the three strict-mode flags are enabled at a
// given point in a script.
type flagState struct {
	errexit  bool
	nounset  bool
	pipefail bool
}

func (s flagState) has(name string) bool {
	switch name {
	case "errexit":
		return s.errexit
	case "nounset":
		return s.nounset
	case "pipefail":
		return s.pipefail
	default:
		return false
	}
}

func (s flagState) set(name string, enable bool) flagState {
	switch name {
	case "errexit":
		s.errexit = enable
	case "nounset":
		s.nounset = enable
	case "pipefail":
		s.pipefail = enable
	}
	return s
}

// missing returns which of required are not enabled in s, preserving the
// order of required.
func (s flagState) missing(required []string) []string {
	var out []string
	for _, r := range required {
		if !s.has(r) {
			out = append(out, r)
		}
	}
	return out
}

// applySetWords updates fs according to the arguments of a `set` command
// (everything after the literal "set"). It recognises every combination of
// short flags ("-e", "-u", "-eu", "-Eeuo"), the long form ("-o errexit", and
// the glued "-oerrexit"), and "+" to disable a flag. Any letter or long-opt
// name it does not track (x, v, verbose, ...) is ignored rather than
// rejected, since shellstrict only cares about errexit/nounset/pipefail.
func applySetWords(words []string, fs *flagState) {
	i := 0
	for i < len(words) {
		w := words[i]
		i++
		if w == "--" {
			break
		}
		if len(w) < 2 || (w[0] != '-' && w[0] != '+') {
			// Not an option token (e.g. a positional parameter after
			// "set --"). Nothing left to learn from this word.
			continue
		}
		enable := w[0] == '-'
		body := w[1:]
		for j := 0; j < len(body); j++ {
			c := body[j]
			switch c {
			case 'e':
				*fs = fs.set("errexit", enable)
			case 'u':
				*fs = fs.set("nounset", enable)
			case 'o':
				var name string
				if j+1 < len(body) {
					// Glued long option, e.g. "-oerrexit".
					name = body[j+1:]
					j = len(body)
				} else if i < len(words) {
					name = words[i]
					i++
				}
				switch name {
				case "errexit", "nounset", "pipefail":
					*fs = fs.set(name, enable)
				}
			}
		}
	}
}
