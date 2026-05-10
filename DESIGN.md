# Design Notes

`README.md` describes what `npte` is and how to use it. This document
explains *why* it looks the way it does: the use cases that drove its
shape, the prior tools whose ideas informed it, and the design choices
that fall out of those two together.

## Use cases

### Run ndt8 binaries in shaped namespaces

The bread-and-butter case. The ndt8 client and server are ordinary
Linux binaries; we want to watch them under a realistic access link
without leaving the laptop. `npte lab create` wires three namespaces
into a `client — router — server` topology, and `npte lab netem
--profile <name>` shapes both directions of the access links from a
named profile. The two profiles shipped today, `4g-bloated` and
`4g-managed`, illustrate what the profile system is for: the same
30/10 Mbit/s 4G link with and without AQM, side by side, so a change
in client behaviour under bufferbloat is visible immediately.

### Run a real browser through the same shaped namespace

ndt8 has a JavaScript client, and the only way to know how it actually
behaves on a realistic access link is to run a real browser there.
`npte netns run` launches the browser inside the `client` namespace —
internally, `ip netns exec <ns> runuser -u $SUDO_USER -- env K=V... <cmd>`
— so the browser sees real TCP/IP through `router`, with no SOCKS proxy
or other intermediary in the path. `gateway` underneath gives internet
egress; `lab netem` on top gives the impairment. The browser is just
another binary the namespace runs.

### Attach a filesystem when binaries on the host aren't enough

Some experiments need more than a binary on the host can offer: an
nginx server with its own configuration, TLS certificates, and large
files that should persist across runs. `npte container` (debootstrap +
`systemd-nspawn`) and the podman chapter cover this: a writable rootfs
bound to a namespace, so the network-isolation and traffic-shaping
story is unchanged from the cases above — only the filesystem the
process sees is different.

### Inject controlled noise onto a real uplink

Sometimes the realistic link is the host's actual internet connection,
and what we want is a controlled amount of latency or loss layered on
top — to see how a client copes when a real high-bandwidth path
develops a kink. `npte gateway create` adds a host-NAT'd uplink to a
namespace; `npte netem apply` (or `lab netem`) layers the impairment.
The shaping is entirely on the namespace side, so the host's
connectivity is unaffected.

## Prior art

`npte` did not appear in a vacuum. The strategy it implements was first
sketched in [ooni/probe#1803](https://github.com/ooni/probe/issues/1803#issuecomment-942761976)
as one possible answer to a similar testing problem at OONI. We did not
pursue it then — we went a different direction with
[ooni/netem](https://github.com/ooni/netem) — and `npte` is, in part,
that earlier idea finally built out. Beyond that lineage, several other
tools have been a source of inspiration; the rest of this section gives
credit where it is due.

### `ip netns` and `tc` shell scripts

The direct ancestor. Almost everything `npte` does could be done by
hand with `ip netns`, `ip link`, `tc qdisc`, and `iptables`, glued
together in a shell script. `npte` is what those scripts grow into
once you want argument validation, a shared vocabulary across
subcommands, and a `--dry-run` that prints the script back out for
review. The imperative-primitive style is borrowed wholesale.

### Mininet

Stanford's [Mininet](https://github.com/mininet/mininet) is the
canonical academic network emulator: hosts as processes in network
namespaces, veth pairs as links, `tc`/`netem` for shaping. It
established that one Linux box is enough for serious network
experimentation — the premise `npte` rests on. Beyond that premise,
Mininet's vocabulary of built-in named topologies (`minimal`,
`linear`, `tree`, `single`, `torus`) is the lineage `npte`'s `lab`
belongs to: a small set of named common cases is more useful in
practice than a configurable topology DSL.

### Containerlab and Kathará

Two declarative container-lab tools whose defining ergonomic is a
DSL on disk. [Containerlab](https://github.com/srl-labs/containerlab)
(Nokia / srl-labs) is a single-binary Go CLI that wires container
nodes from a YAML topology and shapes links at runtime via a
`tools netem set` subcommand: a `topology.clab.yml` file is the
authoritative artefact, versionable and reviewable, backed by an
unusually polished ecosystem.

[Kathará](https://github.com/KatharaFramework/Kathara) (Roma Tre)
inherits Netkit's 20-year-old `lab.conf` + per-device `.startup`
format, so a whole scenario — topology, boot scripts, dependency
order — lives as a plain-text directory tree; a Megalos backend
can lift the same scenarios onto Kubernetes when one host is not
enough.

`npte` deliberately stops short of either DSL. The aim is to stay
minimal — primitives that compose, no scenario format — leaving
room for higher-level tools to define topology on top if anyone
needs that. In practice, `lab` with netem on the router covers
most of what we actually do.

### tc / netem

The kernel mechanism the whole stack rests on. `npte netem` is a
thin wrapper around `tc qdisc ... netem`; the impairment vocabulary
(delay, jitter, loss, reorder, rate limit, queue size, child qdiscs
like `cake` and `fq_codel`) is what makes any of this realistic, and
none of it is `npte`'s. It is fundamental to what `npte` does.

### Mahimahi

[Mahimahi](https://github.com/ravinet/mahimahi) (Winstein / Netravali,
USENIX ATC'15) takes a different mechanism — userspace packet
scheduling through TUN devices rather than kernel `tc`/`netem`.
Three of its design choices are worth crediting in their own right.

`mm-link` is trace-driven: bandwidth varies packet-by-packet
according to a real-world capture, and eight cellular traces ship
in the box. Non-stationary conditions matter, and `npte` does not
yet model them — an important capability we expect to add.

The canonical mahimahi invocation ends with `-- firefox`: a real
browser is the test client, launched inside the shaped namespace
with privileges dropped and the display environment preserved.
`npte` agrees this matters; the browser tutorial chapter documents
how to do the equivalent through `netns run`.

Mahimahi's UX is composition by nested shells —
`mm-delay ... mm-loss ... mm-link ... -- firefox` — so the command
line itself is the topology. `npte` lives in a different mechanism
class and composes imperatively rather than by nesting, but the
underlying instinct is the same: small primitives that combine.

## Design choices

### Composable primitives over a single orchestrator

Each subcommand does one kernel operation: `netns create`, `netns
connect`, `gateway create`, `netem apply`, `container create`. There
is no orchestrator that owns the topology — you build one by running
the verbs in sequence, by hand or from a shell script. The tradeoff
is that there is no first-class "topology" object to query or
modify; the kernel state is what was created, and inspection goes
through `ip netns list` and friends.

### No daemon, no persisted state

Following from the above: there is no `npted`, no JSON config, no
state file under `/var/lib`. Every command is an imperative kernel
operation that succeeds or fails on the spot. Cleanup is explicit
(`netns destroy`, `gateway destroy`, …); nothing watches over the
state in between. If you want to know what is running, ask the
kernel — `ip netns list`, `tc qdisc show`, `iptables -t nat -L`.

### `--dry-run` as a round-trippable shell script

Every subcommand that touches the kernel accepts `--dry-run`, which
prints the equivalent shell pipeline to stdout instead of executing.
The output is meant to be runnable: paste it into a terminal as
root and you get the same result. Two payoffs follow. First,
auditability — you can read what would happen before granting any
privilege. Second, pedagogy — `--dry-run` is the easiest way to learn
the underlying `ip`, `tc`, and `iptables` invocations.

### Network namespaces as the default isolation

Among the units that give you a routable, shapeable, isolated
network stack — VMs, full containers, namespaces — namespaces are
the lightest. A namespace shares the host kernel and filesystem;
creating one costs a syscall, not a boot. That keeps the iteration
loop tight enough to run during active development of a client,
which is what `npte` is for. Containers are available through
`npte container` when an experiment also needs a non-host filesystem
(use case 3); they are opt-in, not the default.

### `lab` as the only built-in topology

Composable primitives are the rule; `lab` is the exception. It
hard-codes the `client — router — server` shape because that is
the topology we use every day, and rebuilding it from `netns` and
`netem` calls each time would be busy work. There is no `--shape`
flag, no parametrised tree or torus or chain. Anything else is
composed by hand from primitives, which keeps the surface area of
the "fixed" part of `npte` exactly one topology wide.

### `sudoers` snippet, not a privileged daemon

Every `npte` operation that touches the kernel runs as root via
`sudo` — there is no privileged daemon, no IPC, no setuid binary.
The `sudoers` subcommand prints a snippet that grants `NOPASSWD` for
`netns`, `netem`, and `lab`, so the frequent verbs — creating
isolated namespaces and shaping their internal links, none of which
disturbs the host's own routing — stop prompting for a password.

`gateway` and `container` are deliberately not on the allowlist:
`gateway` modifies the host's iptables NAT and FORWARD chains and
brings the host into the data path, while `container` manipulates
the host filesystem; both keep the password prompt.

With the snippet installed, the user is trusted (sudo authorized
them) but the bytes they supply — flag values, positionals,
environment — are not. The packages that ship allowlisted verbs
treat that boundary as a hard invariant: every value forwarded to a
subprocess must pass through a validator in `internal/validate` or
be a hardcoded literal, and a registry of marker files at
`/run/npte/netns/<name>` bounds those verbs to namespaces `npte`
itself created. Both invariants are documented in the package's
`CLAUDE.md` and re-examined when editing.

The install path matters too. `npte` is meant to live in a directory
only root can write — `/usr/local/sbin/npte` for source installs,
`/usr/sbin/npte` for the `.deb` — because the absolute path in the
sudoers snippet has to point at something an unprivileged process
cannot replace.

### External tools over Go netlink bindings

`npte` execs `ip`, `tc`, `iptables`, `sysctl`, `systemd-nspawn`,
and `runuser` instead of calling netlink directly from Go. This
makes `--dry-run` trivial — the printed commands are exactly what
would have been executed — and lets a reader copy any line into a
terminal. The cost is a fork/exec per kernel operation and a hard
dependency on the host's `iproute2` and friends; `npte doctor`
catches the missing-binary case before it bites.

### Plain-text output for LLMs and skills

A common thread through several of the choices above: every output
channel is meant to be readable by a human or an LLM with no
in-house parser. `--dry-run` and the `+ <cmd>` echo of every
executed command emit literal shell pipelines that read identically
in either context. `netns show` runs a fixed set of read-only
`ip`/`tc`/`ss` invocations and emits their raw output under
`=== <section> ===` headers — no reformatting, no JSON envelope —
so any reader already familiar with iproute2 output can interpret
the dump. The `sudoers` subcommand likewise prints a verbatim
sudoers fragment.

The privilege model is shaped to match. The verbs allowlisted by
the sudoers snippet (`netns`, `netem`, `lab`) are exactly the ones
safe for an agent to run autonomously; anything that would reach
the host's own routing or filesystem still prompts for a password,
keeping a human in the loop. The cumulative effect: an agent
driving `npte` works against the same plain-text surface a human
reads, and cannot run anything that reaches the host without
operator approval.
