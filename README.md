# hop

Start and resume tmux sessions on a remote machine over ssh, so you can close
your laptop, come back tomorrow, and pick up exactly where you left off.

```
hop --host mac-mini.local              start a new session under a generated name
hop --host mac-mini.local --continue   resume the most recently active session
hop --host mac-mini.local --resume     pick a session from an interactive list
hop --host mac-mini.local --list       print sessions and exit
```

The picker is keyboard driven:

```
Pick a session

❯ brisk-harbor   1 window   just now
  dapper-delta   1 window   3m ago

↑/↓ navigate · enter attach · q quit
```

## Install

```sh
go install github.com/sorodrigo/hop/cmd/hop@latest
```

## Per-machine aliases

`hop` is a library with a thin CLI on top; binding it to one machine is just an
alias. The host can also come from `HOP_HOST`.

```sh
alias macmini="hop --host mac-mini.local"

macmini              # new session
macmini --continue   # resume the latest
macmini --resume     # pick from a list
```

## Flags

| Flag | Env | Default | Meaning |
| --- | --- | --- | --- |
| `--host` | `HOP_HOST` | — | ssh alias, hostname, or `user@host` |
| `--tmux` | `HOP_TMUX` | `tmux` | tmux executable on the remote host |
| `--continue` | | | resume the most recently active session |
| `--resume` | | | open the interactive picker |
| `--list` | | | print sessions and exit |
| `--no-login-shell` | | | do not wrap the remote command in a login shell |

## How it works

`hop` shells out to `ssh`; it does not speak the ssh protocol itself, so your
`~/.ssh/config`, agent, and keys all apply unchanged.

Attaching uses `syscall.Exec` to **replace** the hop process with `ssh`. Handing
the terminal over rather than proxying it keeps tmux's key handling, window
resizing, and exit status intact.

Two details that are easy to get wrong, and which the tests pin down:

- **The remote command runs through a login shell.** `ssh host cmd` runs
  non-interactively, where `PATH` is typically just `/usr/bin:/bin:/usr/sbin:/sbin`
  — no Homebrew, so a bare `tmux` is not found. Pass `--no-login-shell` if your
  profile prints anything to stdout.
- **Session names are shell-quoted.** Names may contain spaces and quotes, and
  the remote side parses the command with a shell. `remote` has tests that run
  the generated command through a real `/bin/sh` with a stub tmux and assert on
  the argv that arrives, including names like `` `touch /tmp/pwned` ``.

### The tmux defaults hop turns around

An attached session should feel like you just ssh'd in. Several tmux defaults
work against that, so attaching sends a command list rather than a bare
`new-session`:

| default | | why it goes |
| --- | --- | --- |
| `status on` | → `off` | a green bar across the bottom row, on a session that is one window |
| `mouse off` | → `on` | the scroll wheel reaches nothing at all |
| `set-titles off` | → `on` | the terminal tab title goes stale |
| `set-clipboard external` | → `on` | a yank on the far side never reaches your clipboard |
| `focus-events off` | → `on` | vim's `autoread` never fires |
| `allow-passthrough off` | → `on` | no inline images, no shell-integration marks |

Two are worth spelling out.

**The scroll wheel.** tmux draws in the alternate screen, so your terminal's own
scrollback freezes the moment you attach — the wheel reaches the text from
*before* tmux, not what is on screen. `mouse on` points it at the pane's history
instead. Dragging then selects inside tmux, so hold Option (iTerm2, Ghostty,
Terminal.app) or Shift (most others) to select natively.

**The clipboard.** `external`, the default, means tmux sets the system clipboard
outward but refuses OSC 52 *from applications inside it* — which is the
direction that matters when you yank in an editor three machines away.

Session options use `-t` rather than `-g`, so sessions you started by hand keep
their own. `set-clipboard` and `focus-events` have no per-session scope and are
set server-wide anyway: each grants a capability rather than changing how
anything looks, so another session can only gain from them. `allow-passthrough`
is set last, because it is the newest option of the group and tmux abandons the
rest of a command list when one command fails.

One default hop leaves alone: `history-limit` is still tmux's 2000 lines. It
only applies to panes created *after* it is set, and by then `new-session` has
already made the only pane hop has.

### Why `COLORTERM` is forwarded

tmux reads `COLORTERM` from its client's environment to decide whether the
terminal can do 24-bit colour ([`tty-term.c`][tty-term]). ssh forwards `LANG`
and `LC_*` but not that, so the remote tmux never learns your terminal does
truecolor and quantises every 24-bit escape an application inside emits down to
256 colours. hop passes your local value through:

```sh
COLORTERM=truecolor exec tmux -u new-session -A -s NAME ...
```

Only on attach — listing sessions draws nothing — and only when your terminal
set it, so hop never claims a capability the terminal did not.

[tty-term]: https://github.com/tmux/tmux/blob/master/tty-term.c

### One default that is already fine

`escape-time` is the tweak every tmux config carries, and on tmux 3.5+ it is
unnecessary: upstream lowered the default from 500ms to 10ms. hop does not set
it.

### Why `|` and not tab

tmux 3.6+ sanitizes control characters in `-F` format output with `vis(3)`, so a
tab separator comes back as `_` and nothing parses:

```
$ tmux display-message -p "A$(printf '\t')B"
A_B
```

`tmux.ListFormat` therefore uses a printable delimiter. Since a session name may
legally contain it, `ParseSessions` reads the three trailing numeric fields from
the right and treats everything before them as the name.

This same bug breaks [tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect),
whose delimiter is `$'\t'` — on tmux 3.6+ it writes empty snapshots.

## Layout

```
hop.go        Decide(): mode + sessions -> what to do
naming/       generated "adjective-noun" session names
tmux/         Session, ListFormat, ParseSessions, Latest
remote/       ssh argv construction, List, Attach
picker/       bubbletea model for --resume
cmd/hop/      flag parsing and wiring
```

Navigation and parsing are pure functions, so the whole interaction is tested
without a terminal:

```sh
go test ./...
```
