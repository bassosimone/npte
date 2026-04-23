# Running a browser through a namespace

The previous chapters built a shaped `client — router — server` star
and a vocabulary for reasoning about how it behaves under load. This
chapter steps away from synthetic traffic and runs a real web browser
inside `client` against the real internet. No new `npte` primitives —
just the two wrinkles that matter in practice: forwarding display
environment variables through `netns run` so the browser can reach
your screen, and picking a browser that plays nicely with network
namespaces in the first place. By the end you will have Epiphany,
Chrome, and Firefox all reachable from inside the namespace, and the
pattern for adding a fourth will be obvious.

We still use `sudo` to gain the required `root` privileges.

## 1. A star and a caveat

Build the topology:

    sudo npte star create <uplink>

`star create` turns `router` into a NATing gateway, so `client` has
internet access out of the box — this is the same shape chapter 4
used, minus any traffic shaping. We deliberately apply no netem in
this chapter. The goal is to get a real browser rendering inside the
namespace; shaping it afterwards with `npte star netem --profile
<name>` or raw `netem apply` is a separate concern and changes
nothing about how the browser is launched.

Before we launch anything, one caveat that rules out the obvious
candidates. On Ubuntu, the default Firefox and Chromium both ship
as snaps, and snaps do **not** work inside our namespaces. The
mechanism is mechanical: `snap-confine` creates its own mount
namespace on launch and overwrites `/etc/resolv.conf` with a
snap-generated version pointing at `127.0.0.53` — `systemd-resolved`'s
stub listener on the host. Inside a network namespace there is no
such listener, and every DNS lookup the browser makes fails
silently. The browser will start, load a blank tab, and never
resolve a single hostname.

The three browsers below are all reachable as non-snap packages:
Epiphany from the Ubuntu archive as a plain `.deb`, Chrome as a
Google-signed `.deb`, and Firefox as a self-contained tarball from
Mozilla. Each reads the namespace's `/etc/resolv.conf` normally and
reaches the internet through `router`.

## 2. Epiphany — the minimum-viable launch

Epiphany is the WebKit-based browser maintained as part of GNOME.
It installs in one `apt` call from the default Ubuntu archive, with
no external repository or key to manage:

    sudo apt install epiphany-browser

Epiphany is not what most users run. Its performance characteristics
do not match Firefox or Chrome, and you should not use it to draw
conclusions about how a real user experiences a link. But it is the
shortest path to *a* browser rendering inside the namespace, which
makes it the right place to introduce the environment-forwarding
pattern the other two will extend.

Launch it inside `client`:

    sudo npte netns run \
        -e DISPLAY=$DISPLAY \
        -e WAYLAND_DISPLAY=$WAYLAND_DISPLAY \
        -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
        -e DBUS_SESSION_BUS_ADDRESS=$DBUS_SESSION_BUS_ADDRESS \
        client epiphany

The four `-e` flags matter because `netns run` does not carry your
shell's environment through to the target. Internally it runs
`ip netns exec <ns> runuser -u $SUDO_USER -- env K=V... <cmd>` —
add `--dry-run` to any of the commands in this chapter to see the
exact pipeline. `runuser` drops privileges and installs its own
default environment for the target user; the `env K=V...` segment
is where the `-e` flags materialise, after the privilege drop.
Anything the target needs has to be forwarded explicitly. The
four variables below are the display stack:

- `DISPLAY` locates the X server (or XWayland proxy).
- `WAYLAND_DISPLAY` locates the Wayland compositor socket.
- `XDG_RUNTIME_DIR` is where both of those sockets live on disk.
- `DBUS_SESSION_BUS_ADDRESS` is the user session bus — browsers
  talk to it for notifications, media keys, and the like.

Not every one is needed in every setup — a pure X11 session often
only needs `DISPLAY`. But the four together cover both X11 and
Wayland without having to know which your session is using.

`netns run` drops privileges to `$SUDO_USER` before launching the
command, so Epiphany runs as you, not as root. This matches what
you want for browser testing: the browser's profile directory lives
under your home, same as it would outside the namespace.

Close the window (or `Ctrl-C` the launching terminal) when you are
done.

## 3. Chrome — one more environment variable

Google distributes Chrome as a regular `.deb`, not a snap, so the
resolver problem from §1 does not apply. Download and install the
current stable build:

    wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
    sudo apt install ./google-chrome-stable_current_amd64.deb

The second command installs Chrome and, as a side effect, registers
Google's apt repository and signing key on the host — future
`apt upgrade` runs will keep Chrome current with no further work.

Chrome needs one thing Epiphany did not: X11 authentication. Unlike
Epiphany, which is content to talk to XWayland without an explicit
auth cookie, Chrome verifies access to the X server through the
file pointed to by `XAUTHORITY` and refuses to start if it cannot
read one. Forward it with a fifth `-e` flag:

    sudo npte netns run \
        -e DISPLAY=$DISPLAY \
        -e WAYLAND_DISPLAY=$WAYLAND_DISPLAY \
        -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
        -e DBUS_SESSION_BUS_ADDRESS=$DBUS_SESSION_BUS_ADDRESS \
        -e XAUTHORITY=$XAUTHORITY \
        client google-chrome

Chrome defaults to XWayland rather than native Wayland, which is
why `XAUTHORITY` matters even on a Wayland desktop. You can force
native Wayland with `--ozone-platform=wayland`, but at the cost of
Vulkan GPU acceleration — the XWayland path is the one Chrome
treats as production, and the one this chapter recommends.

## 4. Firefox — the tarball path

Mozilla distributes Firefox as a self-contained tarball that runs
without installation. Because it is not a snap, it uses the system
resolver and sees our namespace-scoped `/etc/resolv.conf`. Because
it is self-contained, it uses a profile directory separate from the
Ubuntu snap Firefox — useful for clean testing, since the snap's
cached state does not leak into the namespaced run.

Download a recent release from the Mozilla release archive. The
example below uses `148.0`; substitute whatever the current version
is when you read this:

    https://releases.mozilla.org/pub/firefox/releases/148.0/linux-x86_64/en-US/

The directory contains `firefox-148.0.tar.xz` and
`firefox-148.0.tar.xz.asc` — the tarball and its PGP signature.
Download both, verify the signature against Mozilla's signing key,
and unpack under `/opt` so the binary has a stable, out-of-home
path:

    sudo mkdir -p /opt/firefox-148.0
    sudo tar -C /opt/firefox-148.0 -xf firefox-148.0.tar.xz

Launch it through the namespace. No `XAUTHORITY` is needed: Firefox
uses native Wayland when `WAYLAND_DISPLAY` is set, and Wayland does
its own compositor-level authorisation rather than X11 cookies.

    sudo npte netns run \
        -e DISPLAY=$DISPLAY \
        -e WAYLAND_DISPLAY=$WAYLAND_DISPLAY \
        -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
        -e DBUS_SESSION_BUS_ADDRESS=$DBUS_SESSION_BUS_ADDRESS \
        client /opt/firefox-148.0/firefox/firefox

Substitute the version path you actually unpacked.

## 5. Teardown

    sudo npte star destroy

The three namespaces go, the gateway state on `router` goes. The
three browsers stay on the host — they never lived inside the
namespaces in the first place. They ran as processes that happened
to have a different network stack attached; once the namespaces are
gone, the binaries sit on disk unchanged, ready for the next run.

## Recap

- `netns run` enters the namespace with
  `ip netns exec <ns> runuser -u $SUDO_USER -- env K=V... <cmd>`
  (`--dry-run` prints the exact pipeline). `runuser` resets to a
  default environment on the privilege drop, so the target starts
  with essentially nothing from your shell — forward what it needs
  with `-e KEY=VALUE`.

- The display stack is four variables: `DISPLAY`,
  `WAYLAND_DISPLAY`, `XDG_RUNTIME_DIR`, `DBUS_SESSION_BUS_ADDRESS`.
  Chrome additionally needs `XAUTHORITY` for XWayland.

- Snap-packaged browsers (Ubuntu's default Firefox and Chromium)
  do not work inside the namespace: `snap-confine` overwrites
  `/etc/resolv.conf` with a file pointing at the host's
  `systemd-resolved` stub, and the namespace has no such stub.

- Three non-snap browsers cover the common cases: Epiphany
  (`apt install epiphany-browser`), Chrome (Google's direct
  `.deb`), and Firefox (Mozilla's tarball, unpacked under `/opt`).

- This chapter applies no traffic shaping — `star create` alone is
  enough to run a browser against the real internet. Layer
  `npte star netem --profile <name>` (chapter 5) or raw
  `netem apply` (chapter 4) on top when you want the browser to
  experience a realistic access link.
