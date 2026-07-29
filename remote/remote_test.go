package remote_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sorodrigo/hop/remote"
	"github.com/sorodrigo/hop/tmux"
)

func TestNewUsesWorkableDefaults(t *testing.T) {
	host := remote.New("mac-mini.local")

	if host.Addr != "mac-mini.local" {
		t.Errorf("Addr = %q, want %q", host.Addr, "mac-mini.local")
	}
	if host.TmuxPath == "" {
		t.Error("TmuxPath is empty, want a default so callers need not set it")
	}
	if !host.LoginShell {
		t.Error("LoginShell = false; a bare ssh PATH usually cannot find tmux")
	}
}

func TestListArgsTargetsTheHost(t *testing.T) {
	args := remote.New("mac-mini.local").ListArgs()

	if len(args) == 0 || args[0] != "ssh" {
		t.Fatalf("ListArgs = %v, want it to invoke ssh", args)
	}
	if !contains(args, "mac-mini.local") {
		t.Errorf("ListArgs = %v, want it to name the host", args)
	}
	if contains(args, "-t") {
		t.Errorf("ListArgs = %v, want no TTY request for a non-interactive list", args)
	}
	if !strings.Contains(last(args), tmux.ListFormat) {
		t.Errorf("ListArgs remote command %q does not carry tmux.ListFormat", last(args))
	}
}

func TestAttachArgsRequestsATTY(t *testing.T) {
	args := remote.New("mac-mini.local").AttachArgs("quiet-otter")

	if len(args) == 0 || args[0] != "ssh" {
		t.Fatalf("AttachArgs = %v, want it to invoke ssh", args)
	}
	if !contains(args, "-t") {
		t.Error("AttachArgs omits -t; tmux cannot attach without a TTY")
	}
	if !contains(args, "mac-mini.local") {
		t.Errorf("AttachArgs = %v, want it to name the host", args)
	}
}

func TestAttachCreatesOrResumesTheSameSession(t *testing.T) {
	// -A is what makes one command serve both "start it" and "resume it".
	cmd := last(remote.New("mac-mini.local").AttachArgs("quiet-otter"))

	if !strings.Contains(cmd, "-A") {
		t.Errorf("remote command %q lacks -A, so resuming an existing session would fail", cmd)
	}
}

func TestAbsoluteTmuxPathSkipsLoginShell(t *testing.T) {
	host := remote.Host{Addr: "mac-mini.local", TmuxPath: "/opt/homebrew/bin/tmux"}

	cmd := last(host.AttachArgs("quiet-otter"))

	if !strings.Contains(cmd, "/opt/homebrew/bin/tmux") {
		t.Errorf("remote command %q does not use the configured tmux path", cmd)
	}
	if strings.Contains(cmd, "-lc") {
		t.Errorf("remote command %q wraps a login shell despite LoginShell being false", cmd)
	}
}

// The remote command is parsed by a shell on the far side, so quoting is the
// part most likely to break. Rather than assert on the string, run it through
// a real shell with a stub tmux and see what tmux actually receives.
func TestRemoteCommandSurvivesShellParsing(t *testing.T) {
	names := map[string]string{
		"plain":                "quiet-otter",
		"with spaces":          "my project",
		"with single quote":    "rodrigo's box",
		"with double quote":    `say "hi"`,
		"shell metachars":      "a;b|c&d",
		"command substitution": "$(touch /tmp/hop-pwned)",
		"backticks":            "`touch /tmp/hop-pwned`",
	}

	for label, session := range names {
		t.Run(label, func(t *testing.T) {
			stub, argvFile := stubTmux(t)
			host := remote.Host{Addr: "irrelevant", TmuxPath: stub}

			runInShell(t, last(host.AttachArgs(session)))

			got := readArgv(t, argvFile)
			want := []string{"-u", "new-session", "-A", "-s", session}
			if !equal(got, want) {
				t.Errorf("tmux received %q, want %q", got, want)
			}
		})
	}
}

func TestListCommandSurvivesShellParsing(t *testing.T) {
	stub, argvFile := stubTmux(t)
	host := remote.Host{Addr: "irrelevant", TmuxPath: stub}

	runInShell(t, last(host.ListArgs()))

	got := readArgv(t, argvFile)
	want := []string{"-u", "list-sessions", "-F", tmux.ListFormat}
	if !equal(got, want) {
		t.Errorf("tmux received %q, want %q", got, want)
	}
}

// stubTmux writes an executable that records its argv, one entry per line,
// and returns the stub path plus the file it records into.
func stubTmux(t *testing.T) (stub, argvFile string) {
	t.Helper()
	dir := t.TempDir()
	stub = filepath.Join(dir, "tmux")
	argvFile = filepath.Join(dir, "argv")

	script := "#!/bin/sh\n: > " + argvFile + "\nfor a in \"$@\"; do printf '%s\\n' \"$a\" >> " + argvFile + "; done\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("writing stub tmux: %v", err)
	}
	return stub, argvFile
}

func runInShell(t *testing.T, remoteCommand string) {
	t.Helper()
	// sshd runs the command with `$SHELL -c <string>`; /bin/sh matches its parsing.
	out, err := exec.Command("/bin/sh", "-c", remoteCommand).CombinedOutput()
	if err != nil {
		t.Fatalf("remote command %q failed: %v\n%s", remoteCommand, err, out)
	}
}

func readArgv(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("stub tmux recorded nothing: %v", err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func last(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[len(args)-1]
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
