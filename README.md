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
