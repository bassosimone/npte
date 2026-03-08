# Browser Testing

This chapter explains how to run a web browser through a shaped namespace.

## Snap browsers do not work

Snap-packaged browsers will not resolve DNS inside the namespace. This
happens because `snap-confine` creates its own mount namespace and
overwrites our `/etc/resolv.conf` with a snap generated version that
points back `127.0.0.53` (`systemd-resolved`'s stub listener).

For quick testing, on Ubuntu you can use a non-snap browser. For
example, you can use the WebKit-based Epiphany browser:

    sudo apt install epiphany-browser

However, since Epiphany is neither Firefox nor Chrome, using it for
testing does not represent typical user performance.

## X11/Wayland environment variables

`npte netns run` uses `systemd-run` to enter the namespace. Because
systemd-run starts a transient unit with a clean environment, you must
forward display-related variables explicitly with `-e`:

    sudo npte netns run \
        -e DISPLAY=$DISPLAY \
        -e WAYLAND_DISPLAY=$WAYLAND_DISPLAY \
        -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
        -e DBUS_SESSION_BUS_ADDRESS=$DBUS_SESSION_BUS_ADDRESS \
        lab client epiphany

Not all variables are needed in every setup — `DISPLAY` alone is often
enough for X11, while Wayland typically needs all four.

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
        web client epiphany

## Dropping privileges

The `sudo npte netns run ...` command drops privileges and runs as the
user invoking `sudo` as documented in [namespaces](namespaces.md).
