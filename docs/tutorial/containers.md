# Containers

This chapter explains how to create and use lightweight containers backed
by `systemd-nspawn`. Containers provide an isolated Debian filesystem bound
to a network namespace. Note that you need to create a network namespace
before you can create a container filesystem. Once a filesystem exists, you
can create a container and shell into it.

## Why containers?

`npte netns run` uses the host filesystem. This is convenient for tools
already installed on your machine. But to install server software (nginx,
iperf3) or experiment without polluting the host, containers provide
a clean filesystem tree that you can throw away when you are done. Yet,
the nature of performance testing clients and servers require some amount
of persistency that suggests to avoid using Docker.

## Creating a container

Bootstrap a container filesystem for an endpoint:

    sudo npte container create lab server

This runs `debootstrap` to create a minimal Debian (noble) tree at
`/var/local/npte/lab/trees/server/`. It takes a few minutes.

For a different suite:

    sudo npte container create lab server --suite bookworm

## Running commands

Without arguments, `container run` spawns an interactive root shell:

    sudo npte container run lab server

From the shell, install packages and configure services:

    apt update
    apt install -y nginx
    nginx -g 'daemon off;'

Run commands directly:

    sudo npte container run lab server nginx -g 'daemon off;'

## Booting the container

    sudo npte container run lab server --boot

This boots the container, runs `systemd` inside it, and provides
a login shell. It may be useful when you need `systemd` to
start several services inside the container.

Terminate the container by pressing `Ctrl` + `]` three times.

## Container vs namespace

| Feature     | `npte netns run`            | `npte container run`          |
|-------------|-----------------------------|-------------------------------|
| Filesystem  | Host                        | Isolated (`debootstrap`)      |
| Runs as     | `$SUDO_USER` (non-root)     | Root (inside container)       |
| Network     | Namespace only              | Namespace + `systemd-nspawn`  |
| Use case    | Simple tools, browsers      | Server software, daemons      |

## Example: client/server test

Terminal 1 — start a web server in the server container:

    sudo npte container run lab server nginx -g 'daemon off;'

Terminal 2 — fetch a page from the client namespace:

    sudo npte netns run lab client curl http://10.0.2.2/

Traffic flows: client (`10.0.1.2`) → router → server (`10.0.2.2`).

(These addresses assume the default `10.0.0.0/16` prefix; use
`npte netns show lab` to find the actual addresses.)
