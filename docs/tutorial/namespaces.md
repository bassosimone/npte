# Namespaces

This chapter explains the network topology and how to work with namespaces.

## Star topology

npte arranges namespaces in a star around a central **router** namespace.
Each endpoint connects to the router via a veth pair. The router connects
to the host via another veth pair with NAT, giving all endpoints internet
access.

```
                    ┌──────────┐
                    │   host   │
                    │ 10.0.0.1 │
                    └────*─────┘
                         │ <veth:lab-router-h>
                         │
                         │
                         │ <veth:lab-router-i>
                    ┌────*─────┐
                    │  router  │
                    │ 10.0.0.2 │
                    └──*────*──┘
   <veth:lab-client-r> │    │ <veth:lab-server-r>
                       │    │
                       │    │
                       │    │
                       │    │
   <veth:lab-client-s> │    │ <veth:lab-server-s>
              ┌────────*┐  ┌*────────┐
              │ client  │  │ server  │
              │10.0.1.2 │  │10.0.2.2 │
              └─────────┘  └─────────┘
```

(Addresses shown assume the default `10.0.0.0/16` prefix.)

Each project uses a configurable `/16` prefix (default `10.0.0.0/16`, set via
`npte project create --prefix`). Within that prefix, `/24` subnets are
auto-allocated: index 0 (`10.0.0.0/24` with the default prefix) is the
router-to-host link, and indices 1+ are endpoint subnets. Within each `/24`,
`.1` is the router and `.2` is the endpoint (or host).

The meaning of each interface naming is the following:

- `lab-router-h`: **h**ost-facing router VETH
- `lab-router-i`: **i**nner-facing router VETH
- `lab-client-r`: **r**outer VETH connecting to client
- `lab-client-s`: **s**tub client VETH

The general format is:

    <project>-<namespace>-<descriptor>

where `descriptor` is one of `h`, `i`, `r`, and `s`.

## Inspecting the topology

    sudo npte netns show lab

Prints a grep-friendly table (addresses depend on the project's prefix):

    project lab netns lab-client addr 10.0.1.2 mask 24 veth lab-client-s
    project lab netns lab-router addr 10.0.1.1 mask 24 veth lab-client-r
    project lab netns lab-server addr 10.0.2.2 mask 24 veth lab-server-s
    project lab netns lab-router addr 10.0.2.1 mask 24 veth lab-server-r
    project lab netns lab-router addr 10.0.0.2 mask 24 veth lab-router-i
    project lab netns default addr 10.0.0.1 mask 24 veth lab-router-h

Extract an IP programmatically:

    sudo npte netns show lab | grep 'netns lab-client ' | awk '{print $6}'

## Checking status

    sudo npte netns status lab

Prints `up` or `down` and exits with code `0` or `1`. Useful in scripts:

    sudo npte netns status lab || sudo npte netns up lab

## Running commands

`npte netns run` executes a command inside a namespace as `$SUDO_USER`:

    sudo npte netns run lab client curl -s https://example.com/
    sudo npte netns run lab server python3 -m http.server 8080

You can also run commands in the **router** namespace for diagnostics:

    sudo npte netns run lab router ip addr
    sudo npte netns run lab router ping -c3 10.0.1.2

Note: `npte netns run` drops root privileges via `systemd-run --uid`. Use
`-u root` or `--user root` to avoid dropping privileges. For example:

    sudo npte netns run -u root lab router ping -c3 10.0.1.2

## Configuration

The configuration lives at `/var/local/npte/<project>/config/netns.json`.
It records the project name, the `/16` prefix, and the endpoint subnet indices:

    cat /var/local/npte/lab/config/netns.json

Configuration survives reboots. Kernel resources do not — use `npte netns up`
to recreate the namespaces if you need them again.

## TCP buffer tuning

New network namespaces inherit the kernel's default TCP buffer limits, which
are too small for high-bandwidth or high-latency paths. `npte netns up`
enlarges the TCP autotuning range in every endpoint namespace:

    net.ipv4.tcp_rmem = 4096 131072 33554432
    net.ipv4.tcp_wmem = 4096 131072 33554432

This sets the maximum TCP receive and send buffers to 32 MB (the kernel
default max is ~4–6 MB). Without this, TCP autotuning would cap throughput
well below the link rate on fast or high-latency paths.

The bottleneck is the **bandwidth-delay product** (BDP): a 1 Gbit/s link
with 50 ms RTT needs ~6.25 MB of in-flight data to fill the pipe. The
default 4–6 MB max would starve the connection. With 32 MB, npte can
saturate links up to ~5 Gbit/s at 50 ms RTT or 1 Gbit/s at 250 ms RTT.

The router namespace is not tuned — it only forwards packets.

## BBR congestion control

`npte netns up` loads the `tcp_bbr` kernel module via `modprobe tcp_bbr`.
This makes BBR available as a congestion control algorithm for any socket
in any namespace. The module stays loaded after `npte netns down` — this
is harmless and avoids needing to track whether npte loaded it.

BBR is not set as the system-wide default. Individual tools select it
per-socket: for example, iperf3 uses `-C bbr`, and Go code can set
`TCP_CONGESTION` via a `net.Dialer` control function. This keeps npte
from affecting unrelated traffic on the host.
