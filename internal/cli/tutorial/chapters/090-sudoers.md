# Passwordless sudo for the common case

Most of what `npte` does has nothing to do with the actual internet:
creating network namespaces, wiring veth pairs, assigning private IP
addresses, and applying traffic shaping with `tc`/`netem`. These
operations manipulate private kernel state inside namespaces you
own. They never affect anything outside your machine.

If you are running `npte` interactively to develop or debug a
client, typing your sudo password every few seconds is friction
without payoff. `npte sudoers` emits a sudoers snippet that
allowlists exactly those subcommand classes for NOPASSWD execution.

## What the snippet covers

Allowlisted (no password):

- `npte netns *` — create, destroy, connect, address, route inside
  namespaces.
- `npte netem *` — apply or clear traffic shaping on interfaces
  inside namespaces.

Still password-protected:

- `npte gateway *` — installs NAT and forwarding rules in the host
  network namespace, which can give a private namespace egress to
  the actual internet.
- `npte star *` — composes namespace primitives but typically also
  brings up a gateway, so it inherits the same boundary.
- `npte container *` — runs `systemd-nspawn` with capabilities that
  reach beyond the namespace.

## Installing the snippet

The snippet binds the allowlist to `/usr/local/sbin/npte`. Sudoers
matches commands by absolute path, so the binary must live there
for the rule to apply. If your `npte` binary is somewhere else
(`$GOPATH/bin`, a build tree, …), copy or symlink it to
`/usr/local/sbin/npte` before installing the snippet.

Print the snippet (run without `sudo`):

    npte sudoers

The output is itself valid sudoers syntax, tailored to your `$USER`.
Paste it into `/etc/sudoers` via `visudo`:

    sudo visudo

`visudo` validates the snippet's syntax before activation.

A note on drop-in files: `sudo visudo -f /etc/sudoers.d/npte-<user>`
would also work, and the resulting drop-in is easier to revoke
(delete the file). We do not recommend it because `visudo -f`
inspects only the file being edited and will print an alarming "you
have removed your ability to run 'sudo visudo' again" warning when
the file does not itself grant visudo. The warning is a false
positive — sudoers files are additive, and your existing privileges
in `/etc/sudoers` continue to apply — but it is scary enough that
editing `/etc/sudoers` directly is the friendlier path.

After installation, `sudo npte netns …` and `sudo npte netem …`
invocations run without prompting. `sudo npte gateway …`,
`sudo npte star …`, and `sudo npte container …` invocations still
prompt — those commands reach beyond your private namespaces.
