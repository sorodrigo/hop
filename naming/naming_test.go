package naming_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sorodrigo/hop/naming"
)

// tmux rejects session names containing "." or ":", and a name with spaces
// would need quoting everywhere it is passed through ssh.
var legal = regexp.MustCompile(`^[a-z]+-[a-z]+(-[0-9]+)?$`)

func TestNameLooksLikeAdjectiveNoun(t *testing.T) {
	got := naming.Name(nil)

	if !legal.MatchString(got) {
		t.Errorf("Name(nil) = %q, want an adjective-noun name matching %s", got, legal)
	}
}

func TestNameAvoidsTakenNames(t *testing.T) {
	// Ask for many names while claiming every one that comes back, so a
	// generator that ignores `taken` collides almost immediately.
	var taken []string
	seen := map[string]bool{}

	for i := 0; i < 50; i++ {
		got := naming.Name(taken)
		if seen[got] {
			t.Fatalf("Name returned %q, which was already taken (iteration %d)", got, i)
		}
		seen[got] = true
		taken = append(taken, got)
	}
}

func TestNameFallsBackWhenVocabularyExhausted(t *testing.T) {
	// Claim a huge swathe of the namespace: the generator must still return
	// something usable rather than looping forever or returning "".
	var taken []string
	for i := 0; i < 3000; i++ {
		taken = append(taken, naming.Name(taken))
	}

	got := naming.Name(taken)
	if got == "" {
		t.Fatal("Name returned empty string after vocabulary exhaustion")
	}
	if !legal.MatchString(got) {
		t.Errorf("fallback name %q does not match %s", got, legal)
	}
	for _, name := range taken {
		if got == name {
			t.Fatalf("fallback name %q collides with an existing session", got)
		}
	}
}

func TestNameVariesBetweenCalls(t *testing.T) {
	// Without `taken` to force divergence, a fixed-seed generator would
	// return the same name on every run of the binary.
	seen := map[string]bool{}
	for i := 0; i < 30; i++ {
		seen[naming.Name(nil)] = true
	}

	if len(seen) < 2 {
		t.Errorf("Name(nil) produced only %d distinct name(s) in 30 calls: %v",
			len(seen), strings.Join(keys(seen), ", "))
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
