// Package remote builds and runs the ssh invocations that drive tmux on
// another machine.
package remote

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/sorodrigo/hop/tmux"
)

// Host describes how to reach a machine and run tmux on it.
type Host struct {
	// Addr is anything ssh accepts: a hostname, an alias from ~/.ssh/config,
	// or user@host.
	Addr string

	// TmuxPath is the tmux executable on the remote side. "tmux" is enough
	// when LoginShell is set; an absolute path skips PATH resolution.
	TmuxPath string

	// LoginShell wraps the remote command in a login shell so PATH matches
	// what an interactive session would see. Without it, `ssh host cmd` runs
	// with a bare PATH that typically excludes Homebrew.
	LoginShell bool

	// ColorTerm is exported as COLORTERM on the far side, where tmux reads it
	// to decide whether the client can do 24-bit colour. ssh forwards LANG and
	// LC_* but not this, so without it tmux quantises every truecolor escape
	// an application inside emits down to 256. Empty forwards nothing, which
	// is right for a terminal that never claimed truecolor in the first place.
	ColorTerm string
}

// New returns a Host with defaults that work for a stock remote machine.
func New(addr string) Host {
	return Host{
		Addr:       addr,
		TmuxPath:   "tmux",
		LoginShell: true,
		ColorTerm:  os.Getenv("COLORTERM"),
	}
}

// tmux -u forces UTF-8 output. It is not optional here: the remote command
// runs under `sh -lc`, a non-interactive login shell that never sources the
// zsh config where LANG and LC_ALL are exported. tmux then sees no UTF-8
// locale, marks the client non-UTF-8, and rewrites every non-ASCII character
// it draws as "_" -- which turns any TUI on the far side into line noise.

// ListArgs returns the argv for listing sessions on the host.
func (h Host) ListArgs() []string {
	command := h.remoteCommand([]string{h.tmux(), "-u", "list-sessions", "-F", tmux.ListFormat}, false, "")
	return append(h.ssh(false), command)
}

// AttachArgs returns the argv for creating-or-attaching the named session.
// -A is what lets a single command both start a session and resume it.
//
// The `set` commands that follow undo tmux defaults that otherwise announce,
// constantly, that you are not in a plain shell:
//
//	status off           the green bar across the bottom row
//	mouse on             a scroll wheel that reaches nothing, because tmux
//	                     draws in the alternate screen and freezes the
//	                     terminal's own scrollback
//	set-titles           tmux swallows the title escapes the remote shell
//	                     sends, so the terminal tab goes stale
//	-T clipboard         tells this client that its terminal accepts OSC 52,
//	                     even when its TERM name is not one tmux recognises
//	set-clipboard on     the default, external, sets the clipboard outward but
//	                     refuses OSC 52 from applications inside, so a yank in
//	                     the far side's editor never reaches your machine
//	focus-events on      vim's autoread and anything else watching for focus
//	allow-passthrough    inline images and shell-integration marks
//
// Session options take -t so hop never rewrites sessions it did not open.
// The other two have no per-session scope; they are set anyway because each
// grants a capability rather than changing how anything looks, so a session
// someone else started can only gain from them.
func (h Host) AttachArgs(session string) []string {
	argv := []string{h.tmux(), "-u", "-T", "clipboard", "new-session", "-A", "-s", session}

	for _, option := range [][2]string{
		{"status", "off"},
		{"mouse", "on"},
		{"set-titles", "on"},
		// tmux's own default here reads "#S:#I:#W - "#T"", which no local
		// shell would ever put in a title bar. #T alone is what the remote
		// shell set, which is what plain ssh would have shown.
		{"set-titles-string", "#T"},
	} {
		argv = append(argv, ";", "set", "-t", session, option[0], option[1])
	}

	argv = append(argv,
		";", "set", "-s", "set-clipboard", "on",
		";", "set", "-s", "focus-events", "on",
	)

	// Last on purpose: allow-passthrough is the newest option here, and tmux
	// abandons the rest of a command list when one command fails. Everything
	// above it is worth more than it is.
	argv = append(argv, ";", "set", "-w", "-t", session, "allow-passthrough", "on")

	return append(h.ssh(true), h.remoteCommand(argv, true, h.colorEnv()))
}

// colorEnv renders the COLORTERM assignment that precedes tmux on the remote
// side, or "" when there is nothing to claim. It is deliberately not quoted as
// a whole: a quoted word is a command name to the shell, not an assignment.
func (h Host) colorEnv() string {
	if h.ColorTerm == "" {
		return ""
	}
	return "COLORTERM=" + shellQuote(h.ColorTerm) + " "
}

// List returns the sessions currently on the host. A host whose tmux server
// is not running yields no sessions rather than an error.
func (h Host) List() ([]tmux.Session, error) {
	args := h.ListArgs()

	cmd := exec.Command(args[0], args[1:]...)
	// Keep stderr out of stdout: ssh writes host-key notices there, and they
	// would otherwise be parsed as malformed session lines.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()

	if err != nil {
		if tmux.IsNoServer(stderr.String()) {
			return nil, nil
		}
		return nil, fmt.Errorf("hop: listing sessions on %s: %w: %s",
			h.Addr, err, strings.TrimSpace(stderr.String()))
	}

	return tmux.ParseSessions(string(stdout))
}

// Attach hands this process's terminal to ssh and does not return on success.
// Replacing the process rather than proxying keeps tmux's own key handling,
// resize signals, and exit status intact.
func (h Host) Attach(session string) error {
	args := h.AttachArgs(session)

	path, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("hop: %w", err)
	}

	return syscall.Exec(path, args, os.Environ())
}

func (h Host) tmux() string {
	if h.TmuxPath == "" {
		return "tmux"
	}
	return h.TmuxPath
}

func (h Host) ssh(tty bool) []string {
	args := []string{"ssh"}
	if tty {
		// Without -t there is no pseudo-terminal, and tmux refuses to attach.
		args = append(args, "-t")
	}
	return append(args, h.Addr)
}

// remoteCommand renders argv into the single string sshd hands to the remote
// shell, quoting every word so session names may contain spaces or quotes.
// env is prepended verbatim, already quoted by its caller.
func (h Host) remoteCommand(argv []string, replaceShell bool, env string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = shellQuote(arg)
	}
	command := strings.Join(quoted, " ")

	if replaceShell {
		// Drop the intermediate shell so tmux owns the TTY directly.
		command = "exec " + command
	}
	// Assignments come first, and survive the exec into tmux's environment.
	command = env + command
	if h.LoginShell {
		command = "sh -lc " + shellQuote(command)
	}

	return command
}

// shellQuote wraps s in single quotes, ending and reopening the quoted run
// around any embedded single quote. Everything else is literal to the shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
