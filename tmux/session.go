// Package tmux models remote tmux sessions and the command output they
// are parsed from.
package tmux

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Session is a single tmux session on the remote host.
type Session struct {
	Name     string
	Windows  int
	Activity time.Time
	Attached bool
}

// Delimiter separates fields in ListFormat.
//
// It has to be printable: tmux rewrites control characters in format output
// to "_", so a tab arrives mangled and nothing parses. Printable also means
// a session name may legally contain it, which ParseSessions handles by
// reading the three trailing numeric fields from the right.
const Delimiter = "|"

// ListFormat is the -F format string ParseSessions expects.
const ListFormat = "#{session_name}" + Delimiter + "#{session_windows}" +
	Delimiter + "#{session_activity}" + Delimiter + "#{session_attached}"

const listFields = 4

// ParseSessions turns `tmux list-sessions -F ListFormat` output into sessions.
// Blank lines are skipped; any other line that does not match ListFormat is an
// error, so a surprise on stderr is never silently read as "no sessions".
func ParseSessions(raw string) ([]Session, error) {
	var sessions []Session

	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Split(line, Delimiter)
		if len(fields) < listFields {
			return nil, fmt.Errorf("tmux: expected %d %q-separated fields, got %d in %q",
				listFields, Delimiter, len(fields), line)
		}

		// The last three fields are numbers; anything before them is the
		// session name, which may itself contain the delimiter.
		split := len(fields) - (listFields - 1)
		name := strings.Join(fields[:split], Delimiter)

		windows, err := strconv.Atoi(fields[split])
		if err != nil {
			return nil, fmt.Errorf("tmux: window count %q in %q: %w", fields[split], line, err)
		}
		activity, err := strconv.ParseInt(fields[split+1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("tmux: activity time %q in %q: %w", fields[split+1], line, err)
		}
		clients, err := strconv.Atoi(fields[split+2])
		if err != nil {
			return nil, fmt.Errorf("tmux: client count %q in %q: %w", fields[split+2], line, err)
		}

		sessions = append(sessions, Session{
			Name:     name,
			Windows:  windows,
			Activity: time.Unix(activity, 0),
			Attached: clients > 0,
		})
	}

	return sessions, nil
}

// IsNoServer reports whether output is tmux's "no server running" message,
// which means zero sessions rather than a failure. Anything else — notably an
// ssh transport error — is left to surface as a real error.
func IsNoServer(output string) bool {
	lower := strings.ToLower(strings.TrimSpace(output))

	if strings.Contains(lower, "no server running") {
		return true
	}
	// tmux reports a missing socket this way when the server has never started.
	return strings.Contains(lower, "error connecting to") &&
		strings.Contains(lower, "no such file or directory")
}

// Latest returns the most recently active session. Ties break on name so the
// choice is stable across calls, which matters after a restore stamps several
// sessions with the same activity time. The input slice is not modified.
func Latest(sessions []Session) (Session, bool) {
	if len(sessions) == 0 {
		return Session{}, false
	}

	latest := sessions[0]
	for _, session := range sessions[1:] {
		switch {
		case session.Activity.After(latest.Activity):
			latest = session
		case session.Activity.Equal(latest.Activity) && session.Name < latest.Name:
			latest = session
		}
	}

	return latest, true
}

// Names returns every session name, for collision checks when naming a new one.
func Names(sessions []Session) []string {
	names := make([]string, 0, len(sessions))
	for _, session := range sessions {
		names = append(names, session.Name)
	}
	return names
}
