// Command hop starts and resumes tmux sessions on a remote machine.
//
//	hop --host mac-mini.local              start a new session
//	hop --host mac-mini.local --continue   resume the latest session
//	hop --host mac-mini.local --resume     pick a session from a list
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/sorodrigo/hop"
	"github.com/sorodrigo/hop/picker"
	"github.com/sorodrigo/hop/remote"
	"github.com/sorodrigo/hop/tmux"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		host     = flag.String("host", os.Getenv("HOP_HOST"), "remote host: ssh alias, hostname, or user@host")
		tmuxPath = flag.String("tmux", envOr("HOP_TMUX", "tmux"), "tmux executable on the remote host")
		resume   = flag.Bool("resume", false, "pick a session from an interactive list")
		list     = flag.Bool("list", false, "print the host's sessions and exit")
		noLogin  = flag.Bool("no-login-shell", false, "do not wrap the remote command in a login shell")
	)
	// Declared separately: "continue" is a keyword, so it cannot be a var name.
	cont := flag.Bool("continue", false, "resume the most recently active session")

	flag.Usage = usage
	flag.Parse()

	if *host == "" {
		flag.Usage()
		return errors.New("hop: no host given; pass --host or set HOP_HOST")
	}
	if *cont && *resume {
		return errors.New("hop: --continue and --resume do different things; pick one")
	}

	machine := remote.New(*host)
	machine.TmuxPath = *tmuxPath
	machine.LoginShell = !*noLogin

	sessions, err := machine.List()
	if err != nil {
		return err
	}

	if *list {
		return printSessions(sessions)
	}

	plan := hop.Decide(modeFrom(*cont, *resume), sessions)
	if plan.Note != "" {
		// Notes go to stderr so they never mix into piped output.
		fmt.Fprintln(os.Stderr, plan.Note)
	}

	target := plan.Session
	if plan.Picker {
		chosen, err := picker.Run(sessions)
		if err != nil {
			return err
		}
		if chosen == "" {
			return nil // quit without choosing
		}
		target = chosen
	}
	if target == "" {
		return nil
	}

	// Replaces this process, so nothing below runs on success.
	return machine.Attach(target)
}

func modeFrom(cont, resume bool) hop.Mode {
	switch {
	case cont:
		return hop.ModeContinue
	case resume:
		return hop.ModeResume
	default:
		return hop.ModeNew
	}
}

func printSessions(sessions []tmux.Session) error {
	if len(sessions) == 0 {
		fmt.Fprintln(os.Stderr, "No sessions.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	for _, session := range sessions {
		state := ""
		if session.Attached {
			state = "attached"
		}
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
			session.Name, session.Windows,
			picker.RelativeTime(time.Since(session.Activity)), state)
	}
	return w.Flush()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func usage() {
	fmt.Fprint(os.Stderr, `hop — start and resume tmux sessions on a remote machine

Usage:
  hop [--host HOST]              start a new session under a generated name
  hop [--host HOST] --continue   resume the most recently active session
  hop [--host HOST] --resume     pick a session from an interactive list
  hop [--host HOST] --list       print the host's sessions and exit

The host may also come from HOP_HOST, which is how a per-machine alias is
usually set up:

  alias macmini="hop --host mac-mini.local"

Flags:
`)
	flag.PrintDefaults()
}
