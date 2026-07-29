package hop_test

import (
	"strings"
	"testing"
	"time"

	"github.com/sorodrigo/hop"
	"github.com/sorodrigo/hop/tmux"
)

func existing() []tmux.Session {
	return []tmux.Session{
		{Name: "old-harbor", Activity: time.Unix(1000, 0)},
		{Name: "recent-otter", Activity: time.Unix(3000, 0)},
	}
}

func TestNewModeStartsAFreshSession(t *testing.T) {
	plan := hop.Decide(hop.ModeNew, existing())

	if plan.Picker {
		t.Error("new mode asked for the picker")
	}
	if plan.Session == "" {
		t.Fatal("new mode produced no session name")
	}
	for _, session := range existing() {
		if plan.Session == session.Name {
			t.Errorf("new mode reused the existing session %q", plan.Session)
		}
	}
}

func TestNewModeWorksOnAnUntouchedHost(t *testing.T) {
	plan := hop.Decide(hop.ModeNew, nil)

	if plan.Session == "" {
		t.Error("new mode produced no session name when the host had none")
	}
}

func TestContinueModeResumesTheLatestSession(t *testing.T) {
	plan := hop.Decide(hop.ModeContinue, existing())

	if plan.Session != "recent-otter" {
		t.Errorf("Session = %q, want the most recently active %q", plan.Session, "recent-otter")
	}
	if plan.Picker {
		t.Error("continue mode asked for the picker")
	}
	if plan.Note != "" {
		t.Errorf("Note = %q, want none when a session was actually resumed", plan.Note)
	}
}

func TestContinueModeFallsBackToANewSession(t *testing.T) {
	// Nothing to continue is the normal state after a reboot, not an error.
	plan := hop.Decide(hop.ModeContinue, nil)

	if plan.Session == "" {
		t.Fatal("continue mode with no sessions produced nothing to attach to")
	}
	if plan.Picker {
		t.Error("continue mode asked for the picker")
	}
	if plan.Note == "" {
		t.Error("continue mode silently started a new session; want a Note explaining it")
	}
}

func TestResumeModeOpensThePicker(t *testing.T) {
	plan := hop.Decide(hop.ModeResume, existing())

	if !plan.Picker {
		t.Error("resume mode did not ask for the picker")
	}
	if plan.Session != "" {
		t.Errorf("Session = %q, want the picker to choose instead", plan.Session)
	}
}

func TestResumeModeWithNothingToResume(t *testing.T) {
	plan := hop.Decide(hop.ModeResume, nil)

	if plan.Picker {
		t.Error("resume mode opened an empty picker")
	}
	if plan.Session != "" {
		t.Errorf("Session = %q, want nothing to attach to", plan.Session)
	}
	if plan.Note == "" {
		t.Error("resume mode exited silently; want a Note explaining there is nothing to resume")
	}
	if !strings.Contains(strings.ToLower(plan.Note), "no session") {
		t.Errorf("Note = %q, want it to say there are no sessions", plan.Note)
	}
}
