// Package naming generates human-friendly random names for new tmux sessions.
//
// Names are "adjective-noun" pairs: short enough to type after `--resume`,
// distinct enough to tell apart in a list, and free of the "." and ":"
// characters that tmux reserves for window and pane addressing.
package naming

import "math/rand"

var adjectives = [...]string{
	"amber", "azure", "bold", "brave", "brisk", "calm", "canny", "clever",
	"crisp", "dapper", "dusky", "eager", "fleet", "frosty", "gentle", "humble",
	"ivory", "jolly", "keen", "lucid", "mellow", "nimble", "olive", "plucky",
	"quiet", "rapid", "silent", "tidy", "upbeat", "vivid", "warm", "zesty",
}

var nouns = [...]string{
	"anchor", "beacon", "cedar", "cypress", "delta", "ember", "falcon", "fjord",
	"glade", "harbor", "heron", "isle", "juniper", "kestrel", "lagoon", "lantern",
	"marlin", "meadow", "nectar", "nomad", "onyx", "orchid", "pebble", "prairie",
	"quartz", "raven", "summit", "thicket", "umbra", "valley", "willow", "otter",
}

// randomTries bounds the cheap guessing phase before falling back to a
// deterministic sweep. With a 1024-name space, 100 guesses find a free name
// with overwhelming probability until the space is genuinely crowded.
const randomTries = 100

// Name returns a random "adjective-noun" name that is absent from taken.
//
// When every pair is in use it appends a numeric suffix ("quiet-otter-2")
// rather than giving up, so a caller can always start another session.
func Name(taken []string) string {
	used := make(map[string]struct{}, len(taken))
	for _, name := range taken {
		used[name] = struct{}{}
	}

	for i := 0; i < randomTries; i++ {
		candidate := adjectives[rand.Intn(len(adjectives))] + "-" + nouns[rand.Intn(len(nouns))]
		if _, clash := used[candidate]; !clash {
			return candidate
		}
	}

	// The random phase keeps missing, so the space is crowded enough that a
	// full sweep is both cheap and guaranteed to find any remaining gap.
	if name, ok := sweep(used, ""); ok {
		return name
	}

	for suffix := 2; ; suffix++ {
		if name, ok := sweep(used, "-"+itoa(suffix)); ok {
			return name
		}
	}
}

// sweep walks every adjective-noun pair in order and returns the first one
// that, with suffix appended, is not already used.
func sweep(used map[string]struct{}, suffix string) (string, bool) {
	for _, adjective := range adjectives {
		for _, noun := range nouns {
			candidate := adjective + "-" + noun + suffix
			if _, clash := used[candidate]; !clash {
				return candidate, true
			}
		}
	}
	return "", false
}

// itoa avoids pulling strconv in for a single small positive integer.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
