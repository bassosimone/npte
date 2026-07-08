# Namespace basics

This chapter is a hands-on introduction to `npte netns`. You will
create a network namespace, feel what "isolation" means in practice,
give the namespace internet access, add a second namespace, and wire
the two together so they can talk to each other. By the end you will
know about the `npte netns` and `npte gateway` commands.

We will use `sudo` to gain the required `root` privileges.

## 1. An isolated namespace

Start by creating a namespace called `alice`:

    sudo npte netns create alice

`alice` is now a kernel-level network namespace: a private network
stack with its own interfaces, addresses, routes, and iptables state.

It is not *entirely* empty — `npte netns create` brings the loopback
interface up, sets a few sysctls tuned for endpoint testing, enables
IPv4 forwarding (inert until more interfaces appear), and installs a
namespace-scoped `/etc/resolv.conf` pointing at public resolvers.

Run something inside `alice`:

    sudo npte netns run alice ping -c3 127.0.0.1

This works: loopback is up. The command runs as your normal user
(inherited from `$SUDO_USER`), not as `root` — useful when the point is
to test unprivileged software under realistic conditions. For an extra
layer of confinement on top of that drop (read-only host filesystem,
writable current directory only, fresh `/tmp`), pass `--sandbox`; the
sandbox chapter covers the policy and its tradeoffs in detail.

Now try the outside world:

    sudo npte netns run alice ping -c3 8.8.8.8

This fails, and the failure is specific: `connect: Network is
unreachable`. The kernel is not even trying to put a packet on a wire —
it has no route out because `alice` has no interfaces except `lo`. That
is what isolation means: a fresh namespace is a sealed box with only
loopback inside.

## 2. Connecting alice to the internet

To get `alice` online we need four things: a link to the host, IP
addresses at both ends of that link, a default route inside `alice`,
and NAT on the host so the outside world sees the host's address
instead of `alice`'s private one.

`npte gateway create` does all of that in one step. It needs two
pieces of information: a small subnet to use for the uplink and the
name of the host interface that has real internet access. Find the
latter with:

    ip route show default

The `dev` field is the interface name (e.g. `eth0`, `wlan0`,
`wlp0s20f3`). Then:

    sudo npte gateway create alice 10.0.0.0/24 <uplink>

Replace `<uplink>` with the interface name you found. The subnet
argument's host bits are ignored: the host side of the uplink gets the
first usable address in the prefix (typically, `.1`), and `alice`'s
side gets the next one (typically, `.2`).

Now re-run the ping, and climb the stack:

    sudo npte netns run alice ping -c3 8.8.8.8
    sudo npte netns run alice dig @8.8.8.8 www.example.com +short
    sudo npte netns run alice curl https://www.example.com/

These three tests walk up successive layers. `ping` proves that IP
packets can leave `alice`, traverse host NAT, and come back. `dig`
proves UDP egress works; the explicit `@8.8.8.8` skips the system
resolver so this test isolates the egress path from name resolution.
`curl` proves TCP and TLS work, and because it resolves
`www.example.com` implicitly, it also confirms that `alice`'s
namespace-scoped `/etc/resolv.conf` is in effect. If any step fails,
the failure tells you which layer broke.

When you are done exploring, tear the gateway down:

    sudo npte gateway destroy alice

`alice` is still alive — `gateway destroy` only removes the gateway
part (the uplink veth, host-side iptables rules). Verify by re-running
the stage-1 commands: `ping 127.0.0.1` still works, `ping 8.8.8.8` is
`Network is unreachable` again. We are back to the isolation we started
from, which is the right state in which to start the next experiment.

## 3. Two namespaces talking to each other

Create a second namespace:

    sudo npte netns create bob

Now `alice` and `bob` are two sealed boxes side by side. Neither can
reach the other. To wire them together, use `npte netns connect`, which
creates a veth pair and moves one end into each namespace:

    sudo npte netns connect alice bob

`connect` names the interfaces so that `ip link show` inside each
namespace is self-describing: inside `alice`, the interface toward
`bob` is `if-bob`; inside `bob`, the interface toward `alice` is
`if-alice`. Both interfaces are brought up, but neither has an IP
address yet — addressing is a separate concern.

Assign one IP on each side. Note that you run `assign-addr` once per
interface, not once per link: the two ends of a veth pair are
independent interfaces living in different namespaces.

    sudo npte netns assign-addr alice if-bob   10.0.1.1/24
    sudo npte netns assign-addr bob   if-alice 10.0.1.2/24

The kernel auto-installs the connected route for `10.0.1.0/24` on each
side, which is enough for the two namespaces to reach each other over
this link. No explicit `npte netns add-route` is needed here.

Verify, starting with the trivial case and working up:

    sudo npte netns run alice ping -c1 10.0.1.1
    sudo npte netns run alice ping -c1 10.0.1.2

The first ping targets `alice`'s own address — it proves the address
is live on the interface, but the packet never crosses the wire. The
second ping targets `bob` — it actually traverses the veth link and
comes back. If the second one succeeds, you have a functioning
point-to-point link between two namespaces, with nothing on the host
in between.

Neither namespace can reach the outside world right now: `bob` has no
uplink, and `alice` lost its gateway in the previous section. Routing
`bob` to the internet *through* `alice`, and building larger topologies
in general, is the topic of the next chapter.

## 4. Teardown

Destroying a namespace cascades to the interfaces inside it, the peer
ends of any attached veth pairs, and the namespace's iptables state:

    sudo npte netns destroy alice
    sudo npte netns destroy bob

The host is now back to the state it was in before the chapter
started. Nothing about your topology persists: `npte netns` keeps
no description of what you built between invocations. Every
namespace you want is rebuilt with a short sequence of composable
commands, and torn down with `destroy`. (`npte` does keep one
small per-namespace bookkeeping marker on tmpfs, just so it knows
which namespaces it owns — the next chapter and the sudo chapter
say more about why that matters; you can otherwise ignore it.)

## Recap

- A fresh network namespace is isolated: only `lo` exists, and the
  only reachable address is `127.0.0.1`.

- `npte gateway create` connects a namespace to the host's internet:
  uplink veth, addressing, default route, host-side NAT.

- `npte netns connect` wires two namespaces with a veth pair.
  Addressing is a separate concern — use `assign-addr` on each end.

- No topology persists across invocations. Composition is the
  interface.

The next chapter builds a three-namespace topology — client, router,
server — with the router also acting as an internet gateway.
