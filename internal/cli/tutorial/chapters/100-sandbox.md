# The --sandbox extension

`npte netns run` drops privilege from `root` to `$SUDO_USER`
before exec'ing the inner command (the sudoers chapter explains
why that bound matters). `--sandbox` adds a second, opt-in layer
of containment on top of that drop: the inner command runs
inside a [bubblewrap][bwrap] sandbox with the host filesystem
mounted read-only, the current working directory the only
writable location, `/tmp` a fresh tmpfs, and the PID, IPC, and
UTS namespaces unshared from the host.

This is defense-in-depth, not a primary boundary. It is most
useful when the inner command is something you do not fully
trust to behave: a third-party client binary, a freshly-built
prototype, or a workload driven by an autonomous agent.

[bwrap]: https://github.com/containers/bubblewrap

## 1. The policy

When `--sandbox` is set, the assembled command looks like:

    ip netns exec <ns> runuser -u $SUDO_USER -- \
        bwrap \
            --ro-bind / / \
            --tmpfs /tmp \
            --proc /proc \
            --dev /dev \
            --bind <CWD> <CWD> \
            --chdir <CWD> \
            --share-net \
            --unshare-pid --unshare-ipc --unshare-uts \
            --die-with-parent \
            -- env [KEY=VALUE...] <command> [args...]

`--dry-run --sandbox` prints exactly this script, so you can
inspect the policy your invocation will execute before running
it for real.

Each bwrap flag plays a specific role:

- `--ro-bind / /` mounts the entire host filesystem at `/`
  read-only. Standard Unix permissions still apply after the
  privilege drop, so the inner command can read whatever
  `$SUDO_USER` could read on the host — binaries, libraries,
  configs at that user's permissions — but cannot modify any
  of it.

- `--tmpfs /tmp` overlays a fresh tmpfs at `/tmp`. Anything
  written there evaporates when the sandbox exits.

- `--proc /proc` and `--dev /dev` mount fresh procfs and a
  minimal devtmpfs. The inner `/proc` reflects only the
  sandbox's PID namespace.

- `--bind <CWD> <CWD>` rebinds the current working directory
  read-write. This is the one writable host path inside the
  sandbox, with the same path on both sides so relative file
  references in build artifacts or logs continue to make sense.

- `--chdir <CWD>` starts the inner process in that directory.

- `--share-net` keeps the network namespace already entered by
  `ip netns exec`. Without this flag, bwrap would unshare the
  net namespace by default and the inner command would lose
  access to the namespace `npte netns run` was supposed to
  drop it into.

- `--unshare-pid`, `--unshare-ipc`, `--unshare-uts` give the
  inner process its own PID, System V IPC, and UTS (hostname)
  namespaces. `ps` inside shows only sandboxed processes;
  hostname changes do not leak to the host.

- `--die-with-parent` calls `prctl(PR_SET_PDEATHSIG, SIGKILL)`
  on the bwrap supervisor, so if `npte` dies (or `^C` reaches
  it) the inner process is reaped immediately rather than
  being reparented to PID 1 and lingering.

## 2. Trying it

Set up a namespace and verify the sandbox is doing something:

    sudo npte netns create alice
    sudo npte netns run --sandbox alice touch /tmp/sandboxed
    ls -l /tmp/sandboxed

The file is not on the host: it was created inside the
sandbox's fresh `/tmp` tmpfs, which evaporated when the inner
process exited. Without `--sandbox` the same `touch` would land
on the host's `/tmp`.

Now try writing outside the current directory:

    sudo npte netns run --sandbox alice touch /etc/should-fail

`Read-only file system`. The host's `/etc` is bind-mounted
read-only, so the inner command literally cannot write there.
The current directory, by contrast, is the one writable path —
files created inside it persist on the host as usual.

## 3. Mount-order subtlety

bwrap applies mount operations in argv order. The policy's
order is not arbitrary:

1. `--ro-bind / /` establishes the entire host filesystem as
   the substrate.

2. `--tmpfs /tmp`, `--proc /proc`, `--dev /dev`, and
   `--bind <CWD> <CWD>` then overlay on top of that substrate.
   Each replaces a specific path with the desired
   per-invocation contents.

If you ever flipped `--ro-bind / /` to be later, the read-only
substrate would shadow the per-path overlays and the policy
would silently degrade — `/tmp` would inherit the host's
`/tmp`, the workspace would become read-only, and so on. Keep
`--ro-bind / /` first.

## 4. Surprises

- **setuid binaries stop elevating.** bwrap sets
  `PR_SET_NO_NEW_PRIVS`, which the kernel honors by ignoring
  the setuid bit on `execve(2)`. So `sudo`, `mount`, and
  similar inside the sandbox fail with "permission denied".
  `ping` is fine on modern distros (file capabilities, not
  setuid), but if your inner command relies on a setuid
  binary to elevate, it will not under `--sandbox`. Usually
  this is what you want — banning elevation is the point.

- **`$HOME` is read-only.** Anything that wants to write
  `~/.cache/`, `~/.config/`, or similar will hit `EROFS`.
  Most browsers, language runtimes (`pip`, `cargo`), and
  shells in default config do this. Point such workloads at
  a CWD-relative state directory, or do not use `--sandbox`.

- **`/tmp` is per-invocation.** Two `npte netns run --sandbox`
  calls do not see each other's `/tmp`. Use a CWD-relative
  path for cross-invocation state.

## 5. What `--sandbox` does *not* confine

`--sandbox` is an integrity boundary, not a confidentiality
boundary. Two surfaces remain fully open:

- **The network.** `--share-net` is explicit; the whole point
  of running inside a netns is to test traffic.

- **Host filesystem reads.** Everything `$SUDO_USER` can read
  on the host is readable inside the sandbox: the user's home
  directory, the project tree, and every world-readable file.
  Files outside that user's permissions (`/etc/shadow`, other
  users' homes) remain unreadable as usual — the standard
  Unix permission bits still apply after the privilege drop.
  If you do not trust the inner command to read your home
  directory, do not use `--sandbox` — use a real container or
  VM with no host bind.

## Recap

- `--sandbox` wraps the inner command in bubblewrap: read-only
  host substrate, writable CWD, fresh `/tmp`, unshared
  PID/IPC/UTS, shared net (so the netns remains usable).

- Defense-in-depth on top of the privilege drop, not a primary
  boundary. Integrity, not confidentiality.

- Mount-order matters: `--ro-bind / /` first, overlays on top.

- `setuid` stops elevating, `$HOME` is read-only, `/tmp` is
  per-invocation. Plan accordingly.

- `npte netns run --dry-run --sandbox ...` prints the exact
  bwrap policy your invocation will execute.
