# Quickstart

This chapter walks you through creating a project with two namespaces
(client and server), bringing the network up, and running a command.

## Prerequisites

Run `npte doctor` to check that all required external commands are installed:

    npte doctor

If anything is MISSING, install the suggested packages and re-run.

## Create a project

    sudo npte project create lab

This creates the directory skeleton at `/var/local/npte/lab/` with `config/`
and `trees/` subdirectories and writes the initial network configuration.

By default the project uses the `10.0.0.0/16` prefix; pass `--prefix` to use a
different `/16` block. For example:

    sudo npte project create lab --prefix 10.17.0.0/16

## Add namespaces

    sudo npte netns create lab client
    sudo npte netns create lab server

Each `netns create` call allocates a `/24` subnet within the project's prefix
and records it in the configuration file. No kernel resources are created yet.

## Bring the network up

    sudo npte netns up lab

This creates:

- A central **router** namespace (`lab-router`) with host NAT.
- A **client** namespace (`lab-client`) at `10.0.1.2/24`.
- A **server** namespace (`lab-server`) at `10.0.2.2/24`.

(These addresses assume the default `10.0.0.0/16` prefix.)

All endpoints route through the router. The router routes to the host.

We configure masquerading. All namespaces have internet access.

## Verify

    sudo npte netns status lab
    sudo npte netns show lab

## Run a command

Fetch a URL from the client namespace:

    sudo npte netns run lab client curl -s https://example.com/

The command runs as your user (not root) inside the client's network
namespace. We infer the user name via `$SUDO_USER`.

## Tear down

    sudo npte netns down lab

The configuration is preserved. After a reboot or teardown:

    sudo npte netns up lab
