# Browser Testing

This chapter explains how to run a web browser through a shaped namespace.

## X11/Wayland environment variables

`npte netns run` uses `systemd-run` to enter the namespace. Because
systemd-run starts a transient unit with a clean environment, you must
forward display-related variables explicitly with `-e`:

    sudo npte netns run \
        -e DISPLAY=$DISPLAY \
        -e WAYLAND_DISPLAY=$WAYLAND_DISPLAY \
        -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
        -e DBUS_SESSION_BUS_ADDRESS=$DBUS_SESSION_BUS_ADDRESS \
        lab client ...

Not all variables are needed in every setup — `DISPLAY` alone is often
enough for X11, while Wayland typically needs all four.

## Dropping privileges

The `sudo npte netns run ...` command drops privileges and runs as the
user invoking `sudo` as documented in [namespaces](namespaces.md).

## Caveat: snap browsers do not work

Snap-packaged browsers will not resolve DNS inside the namespace. This
happens because `snap-confine` creates its own mount namespace and
overwrites our `/etc/resolv.conf` with a snap generated version that
points back to `127.0.0.53` (`systemd-resolved`'s stub listener).

On Ubuntu, both Firefox and Chromium are snaps by default.

## Using Epiphany for quick tests

For quick testing without downloading a tarball, on Ubuntu you can
install the WebKit-based Epiphany browser as a regular .deb:

    sudo apt install epiphany-browser

Since Epiphany is neither Firefox nor Chrome, it may not represent
typical user performance and be potentially faster or slower.

You can run Epiphany using:

    sudo npte netns run \
        -e DISPLAY=$DISPLAY \
        -e WAYLAND_DISPLAY=$WAYLAND_DISPLAY \
        -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
        -e DBUS_SESSION_BUS_ADDRESS=$DBUS_SESSION_BUS_ADDRESS \
        lab client epiphany

## Using the Firefox tarball

Mozilla distributes Firefox as a self-contained tarball that runs
without installation. Because it is not a snap, it uses the normal
system resolver and sees our resolv.conf overlay.

Download the tarball from the Mozilla release archive (in this
example we are using `v148.0` but YMMV in the future):

    https://releases.mozilla.org/pub/firefox/releases/148.0/linux-x86_64/en-US/

The release contains `firefox-148.0.tar.xz` and `firefox-148.0.tar.xz.asc`.

Verify the PGP signature and unpack the tarball. We recommend unpacking the
compressed tarball at `/opt/firefox-148.0`:

    mkdir -p /opt/firefox-148.0
	tar -C /opt/firefox-148.0 -xf firefox-148.0.tar.xz

The tarball Firefox uses a separate profile from the snap Firefox,
which is actually useful for clean testing.

Once you are done, you can run Firefox using:

    sudo npte netns run \
        -e DISPLAY=$DISPLAY \
        -e WAYLAND_DISPLAY=$WAYLAND_DISPLAY \
        -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
        -e DBUS_SESSION_BUS_ADDRESS=$DBUS_SESSION_BUS_ADDRESS \
        lab client /opt/firefox-148.0/firefox/firefox

## Using Google Chrome

Google distributes Chrome as a regular `.deb` package (not a snap), so
it works inside namespaces. Install it from Google's APT repository
following their instructions.

Chrome needs `XAUTHORITY` forwarded for X11 authentication:

    sudo npte netns run \
        -e DISPLAY=$DISPLAY \
        -e WAYLAND_DISPLAY=$WAYLAND_DISPLAY \
        -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
        -e DBUS_SESSION_BUS_ADDRESS=$DBUS_SESSION_BUS_ADDRESS \
        -e XAUTHORITY=$XAUTHORITY \
        lab client google-chrome

Unlike Firefox, Chrome defaults to X11 (via XWayland) rather than
native Wayland. You can force native Wayland with
`--ozone-platform=wayland`, but this disables Vulkan GPU acceleration.
The XWayland path with `XAUTHORITY` is recommended.

## Minimal setup for browser testing

If you only need to test a browser against the open internet, a
single network namespace is enough:

    sudo npte project create web
    sudo npte netns create web client
    sudo npte netns up web

    # Shape both directions
    sudo npte netns run --user root web router \
        tc qdisc add dev web-client-r root netem delay 100ms rate 2mbit

    sudo npte netns run --user root web router \
        tc qdisc add dev web-router-i root netem delay 100ms rate 500kbit

    # Browse
    sudo npte netns run \
        -e DISPLAY=$DISPLAY \
        -e WAYLAND_DISPLAY=$WAYLAND_DISPLAY \
        -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
        -e DBUS_SESSION_BUS_ADDRESS=$DBUS_SESSION_BUS_ADDRESS \
        web client /opt/firefox-148.0/firefox/firefox
