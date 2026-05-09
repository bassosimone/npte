# Routing between namespaces

This chapter builds on the previous chapter. You will compose three
namespaces into a `client — router — server` topology, watch the
routing table decide what can reach what, and finish by turning
the middle namespace into an internet gateway for both leaves. By
the end you will know what `npte netns add-route` is for and how
it composes with `npte gateway create`.

We still use `sudo` to gain the required `root` privileges.

## 1. Three namespaces in a row

Create the three namespaces:

    sudo npte netns create client
    sudo npte netns create router
    sudo npte netns create server

Wire them into a chain with a veth pair on each link:

    sudo npte netns connect client router
    sudo npte netns connect server router

The `router` namespace now has two interfaces: `if-client` (toward
`client`) and `if-server` (toward `server`). Each leaf has a
single interface, `if-router`, facing the middle. No addresses
yet.

Assign addresses — one per interface, a distinct `/24` per link.
We draw specifically from `172.16.0.0/16`, the quietest corner of
RFC1918 in practice: home routers and ISP CPE almost always hand
out `192.168.0.0/24` or something inside `10/8`, and Docker's
default bridge lands in `172.17.0.0/16` — one `/16` over from
ours. The second octet tags the link — `.2` for server-router,
`.3` for client-router:

    sudo npte netns assign-addr client if-router 172.16.3.2/24
    sudo npte netns assign-addr router if-client 172.16.3.1/24
    sudo npte netns assign-addr server if-router 172.16.2.2/24
    sudo npte netns assign-addr router if-server 172.16.2.1/24

Two disjoint subnets matter. If both links shared a single wider
prefix — say a careless `/8` — the router would end up with two
connected routes covering the same destination, forwarding
decisions would become ambiguous, and strict reverse-path
filtering would silently drop traffic on whichever interface lost
the tie. Remember: one subnet per link; no overlaps.

Verify each link in isolation:

    sudo npte netns run client ping -c1 172.16.3.1
    sudo npte netns run server ping -c1 172.16.2.1

Both succeed. The kernel installed a connected route for each
`/24` when the address was assigned, and nothing else needs to be
true for a leaf to talk to its directly attached router interface.

## 2. Crossing the router

Now try to reach across:

    sudo npte netns run client ping -c1 172.16.2.2
    sudo npte netns run server ping -c1 172.16.3.2

Both fail with `connect: Network is unreachable`. The failure is
local — the packet never leaves the namespace. `client`'s routing
table has `172.16.3.0/24` (connected) and nothing else; when the
target is `172.16.2.2`, no route matches and the kernel refuses to
build the packet at the socket layer. The router is never
consulted. This is a different failure from the previous chapter "no
route out of a sealed box": the namespace does not know that
anything beyond its own `/24` is reachable.

You need to teach it. A default route on each leaf, pointing at the
near-side address of the router, subsumes both the specific
`172.16.2.0/24` (from `client`'s point of view) and every other
off-link destination:

    sudo npte netns add-route client default 172.16.3.1
    sudo npte netns add-route server default 172.16.2.1

The next-hop must sit on a directly connected subnet — that is
the whole point of a next-hop. `172.16.3.1` lives on `client`'s
`if-router` link, so the kernel can `ARP` for it and put a frame
on the wire. `172.16.2.1` would not work from the `client`, because
the `client` has no interface on `172.16.2.0/24`; the kernel has
no way to turn a next-hop it cannot directly reach into an frame.

Retry:

    sudo npte netns run client ping -c1 172.16.2.1
    sudo npte netns run client ping -c1 172.16.2.2
    sudo npte netns run server ping -c1 172.16.3.1
    sudo npte netns run server ping -c1 172.16.3.2

All four succeed. The second and fourth pings are the interesting
ones: the packet leaves the namespace, hits the router on the near
interface, the router consults its own routing table, sees the
far `/24` as a connected route on the far interface, and forwards.

The reply takes the same path in reverse. IPv4 forwarding inside
`router` was turned on by `npte netns create` — every namespace
gets it, inert until the namespace has more than one interface to
forward between.

## 3. Router as gateway

The leaves can now reach each other, but neither can reach the
host or the internet. Layer an uplink on top by turning `router`
into a gateway, exactly as in the previous chapter. Find your host's
internet-facing interface with `ip route show default`, then:

    sudo npte gateway create router 172.16.1.0/24 <uplink>

This adds a third interface to `router` (uplinked to the host),
installs a default route inside `router` via the host-side
address, and NATs egress on the host. From the leaves' point of
view nothing changes on-link — but their default routes already
point at `router`, so off-subnet traffic now has a fully
resolvable path all the way out:

    sudo npte netns run client curl https://www.example.com/
    sudo npte netns run server curl https://www.example.com/

Both succeed. This is the payoff of the previous section's choice
of `default` over specific `/24` routes: we added the leaves'
defaults before any internet path existed, and they extended to
cover it automatically the moment one appeared. Had we used a
pair of host-route entries instead, we would now need to revisit
each leaf and add more.

## 4. Teardown

If you lose track of what you created, `npte netns list` enumerates
the namespaces npte owns, one per line, and `npte netns show <name>`
dumps a fixed set of diagnostics for one — interfaces, addresses,
routing tables, qdiscs, neighbour table, and listening sockets.
Use `--section` to restrict the dump (e.g. `--section route
--section qdisc`). npte tracks ownership via a marker file at
`/run/npte/netns/<name>`, written by `create` and removed by
`destroy`; every other verb refuses to operate on a namespace
without a marker, so this list is also the boundary of what these
commands can affect. There is no `--foreign` flag to bypass it; if
you need to poke at a namespace npte did not create, use `ip netns
...` directly.

Destroy the gateway first, then the namespaces in any order:

    sudo npte gateway destroy router
    sudo npte netns destroy client
    sudo npte netns destroy server
    sudo npte netns destroy router

As in the previous chapter, destroying a namespace cascades to its
interfaces, the peer ends of its veth pairs, and its iptables
state; the host is left as we found it.

### Recovering from a stale marker

Marker and kernel namespace are written and removed in two
consecutive operations, not atomically. Reboot is a clean slate
(both `/run` and the kernel's namespace list go away with it), but
if `npte` is killed mid-sequence — `kill -9`, OOM, an unrecovered
panic — you can be left with the two halves out of step. The
symptoms point at which half:

- `npte: <name>: not managed by npte` while `sudo ip netns list`
  shows the namespace: an orphan kernel namespace from a crashed
  `create`. Clear it with `sudo ip netns del <name>`.

- `Cannot find network namespace "<name>"` (or similar from a
  later verb) while `/run/npte/netns/<name>` exists: an orphan
  marker from a crashed `destroy`. Clear it with
  `sudo rm /run/npte/netns/<name>`.

`npte` does not ship a `gc` verb for this on purpose. A marker
carries no metadata tying it to a specific kernel-side namespace,
so an automated reconciler cannot tell a stale marker apart from
a stale marker plus a same-named foreign namespace someone else
created out-of-band — and would risk silently bringing the
privileged surface to bear on a namespace `npte` did not create.

## Recap

- A leaf namespace with only a connected route can reach its
  near-side router interface and nothing else. "Network is
  unreachable" at the socket layer means no matching route in the
  sender's own table; the router is not consulted.

- `npte netns add-route <ns> <dest> <via>` installs a route. The
  `<via>` must be on a directly connected subnet of `<ns>`.
  `default` as `<dest>` is often the right shape: one route
  covers every off-link destination, present and future.

- IPv4 forwarding inside a namespace is enabled by `npte netns
  create`. Once the namespace has interfaces on more than one
  subnet, it is a working router.

- `npte gateway create` composes cleanly with an
  `add-route default` on each leaf: the defaults extend to the
  internet automatically the moment the router gets its own
  uplink.

The next chapter introduces `npte netem apply` to shape the
traffic on one of these links — adding latency, bandwidth caps,
and loss — so you can test a workload under a realistic
approximation of a real network.
