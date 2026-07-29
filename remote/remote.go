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
}

// New returns a Host with defaults that work for a stock remote machine.
func New(addr string) Host {
	return Host{Addr: addr, TmuxPath: "tmux", LoginShell: true}
}

// ListArgs returns the argv for listing sessions on the host.
func (h Host) ListArgs() []string {
	command := h.remoteCommand([]string{h.tmux(), "list-sessions", "-F", tmux.ListFormat}, false)
	return append(h.ssh(false), command)
}

// AttachArgs returns the argv for creating-or-attaching the named session.
// -A is what lets a single command both start a session and resume it.
func (h Host) AttachArgs(session string) []string {
	command := h.remoteCommand([]string{h.tmux(), "new-session", "-A", "-s", session}, true)
	return append(h.ssh(true), command)
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
func (h Host) remoteCommand(argv []string, replaceShell bool) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = shellQuote(arg)
	}
	command := strings.Join(quoted, " ")

	if replaceShell {
		// Drop the intermediate shell so tmux owns the TTY directly.
		command = "exec " + command
	}
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
