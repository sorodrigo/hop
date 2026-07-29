// Package hop resumes and starts tmux sessions on a remote machine over ssh.
package hop

import (
	"github.com/sorodrigo/hop/naming"
	"github.com/sorodrigo/hop/tmux"
)

// Mode is what the user asked for on the command line.
type Mode string

const (
	// ModeNew starts a fresh session under a generated name.
	ModeNew Mode = "new"
	// ModeContinue resumes the most recently active session.
	ModeContinue Mode = "continue"
	// ModeResume opens the interactive picker.
	ModeResume Mode = "resume"
)

// Plan is what the CLI should do once it knows which sessions exist.
type Plan struct {
	// Session is the session to attach to, or "" when there is nothing to do.
	Session string
	// Picker asks the caller to open the interactive list instead.
	Picker bool
	// Note explains any fallback, for printing before handing over to ssh.
	Note string
}

// Decide maps a mode plus the host's current sessions onto a plan.
func Decide(mode Mode, sessions []tmux.Session) Plan {
	switch mode {
	case ModeContinue:
		if latest, ok := tmux.Latest(sessions); ok {
			return Plan{Session: latest.Name}
		}
		// A host with no sessions is the normal state after a reboot, so
		// start one rather than failing on nothing to continue.
		return Plan{
			Session: naming.Name(tmux.Names(sessions)),
			Note:    "No sessions to continue — starting a new one.",
		}

	case ModeResume:
		if len(sessions) == 0 {
			return Plan{Note: "No sessions to resume — run hop with no flags to start one."}
		}
		return Plan{Picker: true}

	default:
		return Plan{Session: naming.Name(tmux.Names(sessions))}
	}
}
