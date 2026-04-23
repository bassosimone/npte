# Shaping the access link with netem

The previous chapter built a `client — router — server` star and used
`ping` and `curl` to confirm reachability. That tells you whether
packets get through, not what the link feels like under load. This
chapter shapes the client's access link with `tc`/`netem` — adding
propagation delay, capping bandwidth, sprinkling loss — and uses
`iperf3` to measure the result. By the end you will know what `npte
netem apply` and `npte netem clear` do, and how to reach for them
when you want a realistic link instead of a perfect one.

This chapter assumes `iperf3` is installed on the host. It is the
only external dependency, and `npte doctor` does not check for it —
the rest of `npte` does not need it. On Debian/Ubuntu, `apt install
iperf3` is enough.

We still use `sudo` to gain the required `root` privileges.

## 1. Two terminals, a star, and a server

Everything in this chapter runs in two terminals side by side. One
holds the `iperf3` server inside the `server` namespace; the other
shapes the link and runs the `iperf3` client. Open them now.

Build the topology in either terminal — `npte star create` is the
short form of the recipe you assembled by hand last chapter. It
creates the three namespaces, wires `client` and `server` to
`router`, addresses every link out of `172.16.0.0/16`, installs
default routes on the leaves, and turns `router` into a NATing
gateway. Find the host's internet-facing interface with `ip route
show default`, then:

    sudo npte star create <uplink>

Replace `<uplink>` with the interface name you found. After this
runs, `client` lives at `172.16.3.2`, `server` lives at
`172.16.2.2`, and the leaves can reach each other and the internet
with no further setup.

In **terminal A**, start the throughput server inside `server` and
leave it running:

    sudo npte netns run server iperf3 -s

`iperf3 -s` binds `0.0.0.0:5201` and prints a line per accepted
connection. We never touch terminal A again until teardown — the
server stays up across every shaping experiment below, so each
re-run measures only the change in the link, not setup noise.

Switch to **terminal B** for the rest of the chapter.

## 2. The unshaped baseline

Before changing anything, measure what an unshaped link looks like.
From terminal B:

    sudo npte netns run client iperf3 -c 172.16.2.2

The address is `server`'s `if-router` interface, the same one
`client` reached with `ping` last chapter. `iperf3 -c` opens a TCP
connection, fills it for ten seconds, and prints per-second and
summary throughput.

The number you get back will be large — typically several Gbit/s,
sometimes tens. There is no physical link here: the packet path is
a chain of veth pairs inside one kernel, and a veth "transmit" is a
function call that hands the skb to the peer's receive queue. No
wire, no serialisation delay, no per-bit cost. What you are
measuring is your CPU's ability to push bytes through the TCP
stack and the loopback-shaped fast path between two namespaces.

That is the baseline we will conditioning *down* from. Every later
section in this chapter takes this absurd ceiling and clamps it
into the shape of a real access link — adding the propagation
delay, the bandwidth ceiling, and the occasional dropped packet
that the kernel-internal path does not have. Keep the baseline
number in mind: each shaping rule should produce a result that is
visibly, measurably worse, and it should be worse in a specific way
that matches the rule.

## 3. Delay

Delay models **propagation latency**: the time it takes a packet
to traverse the network path — fibre length, processing at
intermediate hops, the speed of light. Latency is a property of
the path itself: every packet pays it, in either direction. A
real 50ms-RTT link adds ~25ms going out and ~25ms coming back; you
do not get to traverse it for free in one direction.

netem, however, is an **egress-only** mechanism — a qdisc shapes
the packets *leaving* the interface it is attached to, and nothing
else. So to recreate a bidirectional property like latency, you
need two qdiscs: one on each endpoint of the veth pair, each
adding the one-way share of the RTT. We will install them one at
a time so the asymmetry between "shape one direction" and "shape
both" is visible in the iperf3 numbers.

Start with the server-to-client direction. Packets flowing from
`server` to `client` leave `router` through `if-client`:

    sudo npte netem apply router if-client --delay 25ms

`npte netem apply <ns> <if>` installs a `root netem` qdisc on
interface `<if>` inside namespace `<ns>`. We attach it on `router`,
not on `client`, because `client`'s `if-router` shapes packets
egressing `client` — i.e. the *upload* direction — and we are
delaying the download half first.

Re-run iperf3 from terminal B:

    sudo npte netns run client iperf3 -c 172.16.2.2

Two things change. The throughput drops by orders of magnitude,
and `iperf3` reports a noticeable RTT (visible as a slow ramp in
per-second throughput while TCP grows its congestion window). The
throughput drop is not because the link became "slow" in any
bandwidth sense — there is still no `rate` cap. It is because TCP
throughput on a delay-only path is bounded by the bandwidth-delay
product of the receive window: a fixed window divided by a longer
RTT yields a lower steady-state rate. Delay alone is enough to
turn a, say, 34 Gbit/s link into a much smaller one.

The link is now lopsided in a way no real link is: data going one
way pays 25ms, data going back pays nothing. ACKs from `client`
to `server` race back at fast-path speed; only the forward
direction is delayed. To model a real 50ms-RTT path, install the
matching qdisc on the other endpoint of the veth pair:

    sudo npte netem apply client if-router --delay 25ms

Re-run iperf3. RTT is now around 50ms — each packet pays its
one-way delay on the way out and another on the way back — and
the throughput drops further as the BDP ceiling tightens.

In practice, propagation delay is never perfectly constant. Add
**jitter** (random variation around the base delay) by passing
the longer netem grammar through `--delay`:

    sudo npte netem clear router if-client

    sudo npte netem clear router if-server

    sudo npte netem apply router if-client \
        --delay "25ms 5ms distribution paretonormal"

    sudo npte netem apply router if-server \
        --delay "25ms 5ms distribution paretonormal"

The `paretonormal` distribution models real networks well: most
packets arrive close to the base delay, but a few are delayed
significantly (right-skewed, heavy tail). The argument is passed
verbatim to `tc` — see `man tc-netem` for the full grammar.

When you are done, clear both directions:

    sudo npte netem clear router if-client
    sudo npte netem clear client if-router

`netem clear` removes the root qdisc on the named interface;
without arguments, it is idempotent (re-running it on an already
clean interface is fine). Re-run iperf3 once more and confirm the
baseline is back: the link should once again be doing tens of
Gbit/s.

## 4. Rate limiting

Delay alone bounds throughput indirectly, through the
bandwidth-delay product. To put a hard ceiling on the link the way
a real access link does, use `--rate`. netem's `rate` option
**paces** packets at the configured speed: it spaces them out in
time so they leave the interface with the inter-packet gaps a real
physical interface of that bitrate would produce. This matters
beyond the obvious "throughput goes down" — congestion-control
algorithms like BBR estimate the bottleneck bandwidth from packet
arrival timing, so a fake link without correct pacing would lead
those algorithms to wrong conclusions.

Like delay, bandwidth applies to both directions of a real link,
and like delay, we install one qdisc per direction. We will keep
both qdiscs on `router` — one per leaf-facing interface — so all
the shaping for this link lives in a single namespace. Pick an
asymmetric profile: 30 Mbit/s download, 10 Mbit/s upload.

Cap the download direction (router → client):

    sudo npte netem apply router if-client --rate 30mbit

Re-run the upload-direction iperf3 from terminal B:

    sudo npte netns run client iperf3 -c 172.16.2.2

`iperf3 -c` sends from `client` to `server` by default — that is
the *upload* direction from `client`'s point of view, which is
still uncapped. The number you see here will still be very large.
The download cap is in place but unused by this test. To exercise
it, use `-R` to reverse the data direction:

    sudo npte netns run client iperf3 -c 172.16.2.2 -R

Now `server` sends to `client`; the bytes egress
`router/if-client`; the qdisc paces them; throughput settles at
~30 Mbit/s. The two iperf3 invocations measure the two directions
of the same link separately — useful when the directions are
shaped differently.

Cap the upload direction (router → server):

    sudo npte netem apply router if-server --rate 10mbit

Re-run iperf3 without `-R` (upload):

    sudo npte netns run client iperf3 -c 172.16.2.2

Throughput is now ~10 Mbit/s. Re-run with `-R` (download): still
~30 Mbit/s. The two ceilings are independent.

Clear both directions when you are done:

    sudo npte netem clear router if-client
    sudo npte netem clear router if-server

## 5. A realistic access link

Delay and rate are independent knobs on the same qdisc, and a
single `apply` can set both at once. Combining them in one rule
per direction yields the shape of an actual access link: a
bandwidth ceiling, a non-trivial RTT, and the asymmetry between
download and upload that most consumer connections have. We will
model a 4G-ish profile — 50ms RTT, 30 Mbit/s down, 10 Mbit/s up —
in two `apply` calls.

Cap and delay the download direction:

    sudo npte netem apply router if-client --delay 25ms --rate 30mbit

Cap and delay the upload direction:

    sudo npte netem apply router if-server --delay 25ms --rate 10mbit

The two flags accumulate into the same qdisc — the order does not
matter, and there is no extra cost over a one-knob `apply`. Each
direction now has both a propagation delay and a bandwidth ceiling,
just like a real link.

Measure each direction separately. Upload first:

    sudo npte netns run client iperf3 -c 172.16.2.2

Throughput should sit close to 10 Mbit/s, with iperf3's reported
RTT around 50ms (or however much your test takes to converge —
slow-start over a 50ms RTT takes a few seconds). Download next:

    sudo npte netns run client iperf3 -c 172.16.2.2 -R

Throughput should sit close to 30 Mbit/s at the same RTT.

This is the kind of link a workload on the host would face from a
typical mobile uplink. Replace the numbers to model whatever
profile you care about — a fibre line is roughly `--delay 5ms
--rate 1gbit` in either direction; a stressed satellite link is
roughly `--delay 300ms --rate 5mbit`. The mechanism is the same;
only the numbers change.

Leave the qdiscs in place for the next section — we will layer
loss on top of this profile and see what it does to TCP.

## 6. Packet loss

Loss is conceptually different from delay and rate. Delay and
rate are properties of the access link — the physical medium and
its serialisation rate. Loss is mostly a property of **everything
else**: congestion at intermediate routers whose buffers are
overflowing, wireless interference somewhere along the path,
shared links under contention. From the endpoint's point of view
those all manifest the same way — packets that left never
arrived — and netem's `loss` option models them all with a single
random-drop probability.

`apply` installs a fresh qdisc, so to add a knob to an existing
shape we clear and re-apply with the full combined ruleset. Layer
1% loss on the download direction of the access link from the
previous section:

    sudo npte netem clear router if-client

    sudo npte netem apply router if-client \
        --delay 25ms --rate 30mbit --loss 1%

Re-run the download iperf3:

    sudo npte netns run client iperf3 -c 172.16.2.2 -R

The throughput collapses well below the 30 Mbit/s ceiling — often
to a small fraction of it. The cause is TCP's loss-as-congestion
heuristic: CUBIC (Linux's default congestion control) interprets
every drop as a congestion signal and halves its window. On a
50ms-RTT path the window then needs many RTTs to grow back, and
because losses keep happening at 1%, it never reaches the BDP
ceiling. The longer the RTT and the higher the rate, the more
brutal this effect — small loss rates devastate TCP throughput on
high-BDP paths. This is why "1% loss" sounds tolerable in
isolation and is anything but in practice.

Real loss is rarely uniformly random. When a buffer overflows or
a wireless link degrades, several consecutive packets are lost,
not isolated ones. netem can model bursty loss with a two-state
Markov chain (Gilbert-Elliott) by passing the longer grammar
through `--loss`:

    sudo npte netem clear router if-client

    sudo npte netem apply router if-client \
        --delay 25ms --rate 30mbit \
        --loss "gemodel 1% 30% 80% 0%"

The four values are: probability of going from the good state to
the bad state (1%), probability of going back (30%), loss
probability while in the bad state (80%), and loss probability
while in the good state (0%). See `man tc-netem` for the full
parameter grammar.

Clear both directions before moving on:

    sudo npte netem clear router if-client
    sudo npte netem clear router if-server

## 7. Teardown

Stop the iperf3 server in terminal A with `Ctrl-C`, then tear the
topology down from terminal B:

    sudo npte star destroy

`star destroy` removes the gateway state on `router` and then
destroys the three namespaces. The veth pairs and any per-namespace
iptables state go with them; the host is back to where it started.
Any netem qdiscs left installed are gone too, since they live
inside the namespaces.

## Recap

- `npte netem apply <ns> <if>` installs a `root netem` qdisc on
  an interface inside a namespace. It takes pass-through flags
  (`--delay`, `--rate`, `--loss`, ...) whose values are forwarded
  verbatim to `tc`. `npte netem clear <ns> <if>` removes the
  qdisc and is idempotent.

- netem is **egress-only**. To recreate a bidirectional link
  property like delay or rate, install one qdisc per direction —
  on the two interfaces a packet egresses on its way out and on
  its way back. Keeping both qdiscs on `router` lets all the
  shaping live in one namespace.

- `apply` is not additive. To add a knob to an existing shape,
  `clear` first and re-apply with the full combined ruleset.
  Multiple netem knobs (`--delay`, `--rate`, `--loss`, ...) on the
  same `apply` accumulate into the same qdisc.

- Each knob models a different class of impairment: delay is
  propagation latency, rate is the access link's serialisation
  ceiling, loss is congestion or impairment elsewhere on the
  path. Combining them in one rule per direction yields a
  realistic access link.

- Small loss rates devastate TCP throughput on high-BDP paths.
  CUBIC interprets every drop as congestion and halves its
  window; on a long-RTT path the window cannot recover fast
  enough to fill the pipe.

The next chapter looks at what happens at the **buffer** sitting
in front of the rate-limited link — what bufferbloat is, how it
shows up under load, and how a child qdisc like `fq_codel`
changes the picture.
