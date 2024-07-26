package clean

import (
	"path"
	"strings"
)

// Clean cleans the path p so that the following conditions are met:
//
// - It is always non-empty
// - It always starts with a '/'
// - It never contains any '.' or '..' segments
// - It always ends with a '/'
//
// This means it can be safely used in URL checks.
// The code was adapted from the 'net/http' package.
func Clean(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}

	np := path.Clean(p)

	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		// Fast path for common case of p being the string we want:
		if len(p) == len(np)+1 && strings.HasPrefix(p, np) {
			np = p
		} else {
			np += "/"
		}
	}
	return np
}
