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
- `npte lab *` — composes only the `netns` primitives above to
  build the canonical `client/router/server` topology. It installs
  no host-namespace state, so it sits on the same side of the
  boundary as the primitives it wraps.

Still password-protected:

- `npte gateway *` — installs NAT and forwarding rules in the host
  network namespace, which can give a private namespace egress to
  the actual internet. Layer this on top of `lab` when a chapter
  needs internet access.
- `npte container *` — runs `systemd-nspawn` with capabilities that
  reach beyond the namespace.

## Why this is safe to allowlist

NOPASSWD on a verb class means anyone with write access to the
allowlisted user's account — a malicious script, a compromised
shell rc file, a rogue dependency in a build tree — can run those
verbs as root without authenticating. That sounds alarming, and it
would be, were it not bounded.

The bound is the registry. Every `netns` and `netem` verb other
than `create` checks for a marker file at `/run/npte/netns/<name>`
before doing any kernel work, and refuses if the marker is
missing. `create` writes the marker only after `ip netns add`
succeeds; `destroy` removes it only after `ip netns del` succeeds.
So a caller who has the NOPASSWD grant cannot point any of these
verbs at namespaces `npte` did not create — Docker's, libvirt's,
the host root namespace, or any namespace an operator built by
hand with `ip netns add`. The privileged surface is bounded to
the set `npte` itself owns.

A second bound matters for `npte netns run`, the one allowlisted
verb that runs user-supplied code inside a namespace. `ip netns
exec` shares the host's mount namespace — a root shell launched
inside a fresh netns still sees the host's `/etc`, `/root`,
`/home`, `/var/lib/docker`, and so on. So if `run` simply
exec'd the user's command as root, NOPASSWD on `netns *` would
collapse to NOPASSWD on `/bin/bash`. To prevent that, `npte
netns run` drops privilege to `$SUDO_USER` via `runuser(1)`
before exec'ing the inner command: the command lands inside the
namespace but with the invoking user's identity, not root's.
The grant lets you cross the namespace boundary; it does not let
you escalate while crossing it.

For workloads you want to confine further — third-party binaries,
freshly-built prototypes, or commands driven by an autonomous
agent — `npte netns run --sandbox` adds a bubblewrap layer on top
of the privilege drop: the host filesystem becomes read-only, the
current directory the only writable path, and `/tmp` a fresh
tmpfs per invocation. This is opt-in defense-in-depth — the bound
above already prevents privilege escalation through the NOPASSWD
grant, while `--sandbox` constrains what the dropped-to user can
damage. The sandbox chapter has the full policy.

`gateway` and `container` are kept out of the allowlist precisely
because neither bound applies to them: `gateway` installs rules
in the host namespace (the registry cannot fence it in), and
`container` runs `systemd-nspawn` with capabilities that escape
any single namespace (the privilege drop would not bound the
inner workload). Asking for the sudo password on each invocation
is the right friction for operations these bounds cannot cover.

## Installing the snippet

The snippet binds the allowlist to a fixed absolute path: typically
`/usr/local/sbin/npte` for source installs and `/usr/sbin/npte` for
the `.deb`. Sudoers matches commands by absolute path, so the binary
must live at the path the snippet names. Run `npte sudoers` to see
which path applies to your build; if your `npte` binary is somewhere
else (`$GOPATH/bin`, a build tree, …), copy it to that path before
installing the snippet.

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

After installation, `sudo npte netns …`, `sudo npte netem …`, and
`sudo npte lab …` invocations run without prompting. `sudo npte
gateway …` and `sudo npte container …` invocations still prompt —
those commands reach beyond your private namespaces.
