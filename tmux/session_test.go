package tmux_test

import (
	"testing"
	"time"

	"github.com/sorodrigo/hop/tmux"
)

func TestParseSessionsReadsEveryField(t *testing.T) {
	raw := "quiet-otter|2|1753800000|0\nbold-cypress|1|1753790000|1\n"

	got, err := tmux.ParseSessions(raw)
	if err != nil {
		t.Fatalf("ParseSessions returned error: %v", err)
	}

	want := []tmux.Session{
		{Name: "quiet-otter", Windows: 2, Activity: time.Unix(1753800000, 0), Attached: false},
		{Name: "bold-cypress", Windows: 1, Activity: time.Unix(1753790000, 0), Attached: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d sessions, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Windows != want[i].Windows ||
			!got[i].Activity.Equal(want[i].Activity) || got[i].Attached != want[i].Attached {
			t.Errorf("session %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseSessionsTreatsMultipleClientsAsAttached(t *testing.T) {
	// tmux reports a client count, not a boolean.
	got, err := tmux.ParseSessions("busy-harbor|1|1753800000|3\n")
	if err != nil {
		t.Fatalf("ParseSessions returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d sessions, want 1", len(got))
	}
	if !got[0].Attached {
		t.Error("session with 3 clients reported as not attached")
	}
}

func TestParseSessionsHandlesEmptyOutput(t *testing.T) {
	for _, raw := range []string{"", "\n", "   \n"} {
		got, err := tmux.ParseSessions(raw)
		if err != nil {
			t.Errorf("ParseSessions(%q) returned error: %v", raw, err)
		}
		if len(got) != 0 {
			t.Errorf("ParseSessions(%q) = %+v, want no sessions", raw, got)
		}
	}
}

func TestParseSessionsKeepsNamesContainingSpaces(t *testing.T) {
	got, err := tmux.ParseSessions("my project|1|1753800000|0\n")
	if err != nil {
		t.Fatalf("ParseSessions returned error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "my project" {
		t.Errorf("got %+v, want a single session named %q", got, "my project")
	}
}

// tmux replaces control characters in format output with "_", so the
// separator has to be printable — and therefore legal inside a session name.
// The three trailing fields are always numeric, so the name is whatever
// precedes them however many separators it contains.
func TestParseSessionsKeepsNamesContainingTheDelimiter(t *testing.T) {
	got, err := tmux.ParseSessions("api|worker|2|1753800000|0\n")
	if err != nil {
		t.Fatalf("ParseSessions returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d sessions, want 1: %+v", len(got), got)
	}
	if got[0].Name != "api|worker" {
		t.Errorf("Name = %q, want %q", got[0].Name, "api|worker")
	}
	if got[0].Windows != 2 {
		t.Errorf("Windows = %d, want 2", got[0].Windows)
	}
}

func TestListFormatAvoidsControlCharacters(t *testing.T) {
	// A tab here would come back from tmux as "_" and every line would fail
	// to parse. This guards the fix from being quietly reverted.
	for _, r := range tmux.ListFormat {
		if r < 0x20 || r == 0x7f {
			t.Errorf("ListFormat contains control character %q; tmux rewrites those to _", r)
		}
	}
}

func TestParseSessionsRejectsMalformedLines(t *testing.T) {
	cases := map[string]string{
		"too few fields":        "quiet-otter|2\n",
		"windows not a number":  "quiet-otter|many|1753800000|0\n",
		"activity not a number": "quiet-otter|2|yesterday|0\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := tmux.ParseSessions(raw); err == nil {
				t.Errorf("ParseSessions(%q) succeeded, want an error", raw)
			}
		})
	}
}

func TestIsNoServerRecognisesTmuxMessage(t *testing.T) {
	noServer := []string{
		"no server running on /private/tmp/tmux-501/default",
		"no server running on /private/tmp/tmux-501/default\n",
		"error connecting to /private/tmp/tmux-501/default (No such file or directory)",
	}
	for _, output := range noServer {
		if !tmux.IsNoServer(output) {
			t.Errorf("IsNoServer(%q) = false, want true", output)
		}
	}

	if tmux.IsNoServer("quiet-otter|2|1753800000|0") {
		t.Error("IsNoServer reported true for real session output")
	}
	if tmux.IsNoServer("ssh: connect to host mac-mini.local port 22: Host is down") {
		t.Error("IsNoServer swallowed an ssh connection failure")
	}
}

func TestLatestPicksMostRecentlyActive(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "old", Activity: time.Unix(1000, 0)},
		{Name: "newest", Activity: time.Unix(3000, 0)},
		{Name: "middle", Activity: time.Unix(2000, 0)},
	}

	got, ok := tmux.Latest(sessions)
	if !ok {
		t.Fatal("Latest reported no session for a non-empty list")
	}
	if got.Name != "newest" {
		t.Errorf("Latest = %q, want %q", got.Name, "newest")
	}
}

func TestLatestOnEmptyList(t *testing.T) {
	if _, ok := tmux.Latest(nil); ok {
		t.Error("Latest(nil) reported a session")
	}
}

func TestLatestBreaksTiesDeterministically(t *testing.T) {
	// Equal activity timestamps are common right after a reboot restore.
	sessions := []tmux.Session{
		{Name: "beta", Activity: time.Unix(1000, 0)},
		{Name: "alpha", Activity: time.Unix(1000, 0)},
	}

	first, _ := tmux.Latest(sessions)
	for i := 0; i < 10; i++ {
		got, _ := tmux.Latest(sessions)
		if got.Name != first.Name {
			t.Fatalf("Latest is non-deterministic on ties: got %q then %q", first.Name, got.Name)
		}
	}
}

func TestLatestDoesNotReorderInput(t *testing.T) {
	sessions := []tmux.Session{
		{Name: "old", Activity: time.Unix(1000, 0)},
		{Name: "newest", Activity: time.Unix(3000, 0)},
	}

	tmux.Latest(sessions)

	if sessions[0].Name != "old" {
		t.Errorf("Latest mutated caller's slice: now starts with %q", sessions[0].Name)
	}
}

func TestNamesExtractsEverySessionName(t *testing.T) {
	sessions := []tmux.Session{{Name: "quiet-otter"}, {Name: "bold-cypress"}}

	got := tmux.Names(sessions)

	if len(got) != 2 || got[0] != "quiet-otter" || got[1] != "bold-cypress" {
		t.Errorf("Names = %v, want [quiet-otter bold-cypress]", got)
	}
}
