# npte Tutorial

npte (Network Performance Testing Environment) creates isolated network
namespaces for testing client network performance.

## What npte is for

npte is designed for a specific workflow: you have a **client** that talks to
1+ **servers**, and you want to measure how the client performs under different
network conditions (latency, bandwidth, bufferbloat). The servers either live
in local network namespaces on the same machine or on the internet. The **router**
is the namespace where we implement shaping.

```
                        DL delay+rate
┌────────┐                    ┌────────┐                    ┌────────┐
│        │ ◄───────────────── │        │ ◄───────────────── │        │
│ client │                    │ router │                    │ server │
│        │ ─────────────────► │        │ ─────────────────► │        │
└────────┘                    └────────┘                    └────────┘
                                    UL delay+rate
```

Traffic shaping (`tc`) works on **egress**. Delay and rate limiting are
applied separately in each direction on the router's interfaces. The
client's access link is shaped to simulate real-world conditions (2G, 4G,
broadband, fiber, etc.). The servers are left unshaped — you're testing
the client, not the network itself.

**Primary goal**: maximize single-client throughput and minimize per-client
CPU usage. These reinforce each other: a faster client uses less CPU per
byte transferred, which means you can either push more bandwidth or serve
more concurrent clients on the same hardware.

## How npte works

npte is **project-scoped**. Per-project configuration is stored under
`/var/local/npte/<project>/` and survives reboots. Kernel resources
(namespaces, interfaces, routes) are ephemeral; recreate them with
`npte netns up` after a reboot.

Within a project, npte creates Linux network namespaces arranged in
a **star topology** around a **router** namespace:

- The **router** namespace has internet access via host NAT.

- **Endpoint** namespaces (client, server, etc.) connect to the router
  through veth pairs and route all traffic through it.

- Traffic shaping happens on the router's interfaces, simulating the
  bottleneck link between client and server(s).

You can run commands directly in a namespace (via `npte netns run`) or
inside a lightweight container backed by systemd-nspawn (via
`npte container run`). The namespace approach is lighter and lets you
test local binaries without copying them; containers are useful when you
need an isolated filesystem (e.g., for installing packages or running
services like a browser).

## Chapters

1. [quickstart](quickstart.md) — Create a project, add namespaces, run a command.

2. [namespaces](namespaces.md) — Star topology, addressing, interface naming.

3. [containers](containers.md) — Lightweight containers with systemd-nspawn.

4. [netem](netem.md) — Simulate network conditions with `tc` and `netem`.

5. [browser](browser.md) — Run a web browser in a shaped namespace.

Read a chapter with:

    npte tutorial <chapter>

For example:

    npte tutorial quickstart
