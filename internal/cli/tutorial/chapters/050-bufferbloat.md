# Bufferbloat and child qdiscs

The previous chapter put a hard rate cap on the access link with
`--rate` and stopped. We never asked an obvious question: when
packets arrive at a rate-limited interface faster than the cap can
drain them, where do they go? They queue. And the discipline of
that queue — its depth, whether it drops anything before filling
up, whether it shares fairly between flows — is what determines
whether the link feels merely *slow* under load or actively
*broken*. This chapter turns the queue into the subject. We will
make a textbook bufferbloat picture appear on a loaded link, then
fix it with a single `--child` flag.

The chapter assumes `iperf3` is installed on the host (the
previous chapter said the same). `ping` is universal and needs no
introduction.

We still use `sudo` to gain the required `root` privileges.

## 1. Three terminals, a star, and a server

Bufferbloat is a property of a link **under load** that you only
see by observing a *separate* small flow at the same time as the
load — a fat iperf3 download fills the queue, and a thin ping
flow measures how deep the queue got. So this chapter wants three
terminals, not two:

- **Terminal A** — the iperf3 server (started once, left running).
- **Terminal B** — the iperf3 client, run in long bursts to load
  the link.
- **Terminal C** — a continuous `ping` from `client` to `server`,
  watching the loaded RTT in real time.

Build the topology in any of them:

    sudo npte star create <uplink>

In **terminal A**, start the throughput server inside `server` and
leave it running:

    sudo npte netns run server iperf3 -s

In **terminal C**, start a continuous ping from `client` to
`server` and leave it running:

    sudo npte netns run client ping 172.16.2.2

The ping output is the dashboard for the rest of the chapter. With
no shaping and no load it should report sub-millisecond RTTs —
this is the unshaped fast path from the previous chapter,
restated. Keep this terminal in view; we will check it after every
shaping or load change.

Use **terminal B** for everything that follows.

If the ping reports anything larger than a sub-millisecond RTT,
some shaping is left over from a previous experiment. List the
qdiscs in each namespace with `tc qdisc show` to find the residue:

    sudo npte netns run --user root router tc qdisc show
    sudo npte netns run --user root client tc qdisc show
    sudo npte netns run --user root server tc qdisc show

A clean interface shows `qdisc noqueue` (the kernel default for an
interface with no shaping). Anything else — typically a stray
`qdisc netem ... delay ...` — is residue. Clear it with `npte
netem clear <ns> <iface>` and confirm the ping returns to
sub-millisecond RTTs.

## 2. The idle baseline

Reinstall the access-link profile from the previous chapter — the
4G-ish 50ms / 30 Mbit/s shape, both directions, both qdiscs on
`router`:

    sudo npte netem apply router if-client --delay 25ms --rate 30mbit
    sudo npte netem apply router if-server --delay 25ms --rate 10mbit

Look at terminal C. The ping RTT immediately jumps from
sub-millisecond to ~50ms and stays there, perfectly flat: 25ms on
the way out, 25ms on the way back, no jitter, no surprises.

This is the link's **idle latency** — what a user would call its
"ping time", what a speedtest reports as latency, what every
diagnostic tool measures with no other traffic running. It is the
number the link is rated for. It is also the number that almost
nothing on the internet sees in practice, because almost nothing
on the internet is the only flow on the link. The next section
shows what the link feels like when something else is using it.

Leave the ping running, leave the shaping in place, and switch to
terminal B.

## 3. Bufferbloat under load

Run a thirty-second download from terminal B:

    sudo npte netns run client iperf3 -c 172.16.2.2 -R -t 30

Now watch terminal C. Within a second or two of iperf3 starting,
the ping RTTs climb — 50ms, 100ms, 200ms, often well past 400ms
— and stay there for the duration of the iperf3 run. When iperf3
finishes, the RTTs drop back to 50ms within a couple of seconds.
The link's loaded latency is **an order of magnitude worse** than
its idle latency. Same link, same shaping, same kernel: the only
thing that changed is that one other flow is using it.

This is bufferbloat. The mechanism is mechanical and has nothing
to do with anything clever:

1. iperf3 fills the link with bytes flowing from `server` to
   `client`. They reach `router` and need to egress
   `if-client`, which is rate-capped at 30 Mbit/s.
2. The kernel's TCP stack on `server` does not know about that
   cap; it sends as fast as the receive window allows. Bytes
   arrive at `router/if-client` faster than 30 Mbit/s can drain
   them.
3. The excess sits in the netem qdisc's queue. netem's default
   queue is a 1000-packet FIFO. At full-size 1500-byte packets
   and 30 Mbit/s, that is ~400ms of buffered data.
4. Every echo-reply from `server` (the second leg of the ping
   you are watching) hits the same egress and queues behind all
   the iperf3 bytes already there. The ping sees the queue
   depth as added RTT.

Watch terminal C for a few more seconds. The climb is not
monotonic — once the RTT hits its peak, it drops sharply, then
grows again. The sawtooth is the entire bufferbloat dynamic in
miniature, played out by CUBIC and the FIFO together:

1. CUBIC ramps up its window. Bytes arrive at the bottleneck
   faster than 30 Mbit/s can drain. The queue grows. The ping
   RTT grows with it.
2. The queue eventually overflows and the FIFO drops a packet
   at the tail — the only feedback signal a dumb FIFO has.
3. The drop reaches `server`'s TCP stack as duplicate ACKs (or
   an RTO if it took long enough). CUBIC interprets it as
   congestion and slow-starts: the sending rate collapses to
   roughly nothing.
4. The queue drains, the ping RTT drops back toward 50ms, and
   CUBIC starts ramping again. Back to step 1.

This is the worst of both worlds. Throughput is mediocre because
TCP keeps backing off; latency is awful because the buffer is
deep enough to hold close to a second of data; and the only
reason TCP backs off at all is that the buffer eventually
overflows. The buffer is "doing its job" — losing nothing under
transient bursts — and that is exactly what makes the link feel
broken.

The queue is the variable. The cap forces one to exist; the
*discipline* of the queue — how deep it is allowed to get,
whether anything is dropped before it fills, whether a small
flow gets to skip past a bulk flow — is what determines whether
the loaded link feels tolerable or broken. A dumb FIFO that
grows until it overflows and drops only at the tail does nothing
to help an interactive flow squeezed between bulk packets, and
nothing to tell the bulk flow to slow down before the queue is
deep. To fix the link we need a queue that *does* something —
that signals congestion early, before the buffer fills, and that
shares the link fairly. That is the next section.

Wait for iperf3 to finish (or interrupt it) and confirm the ping
RTTs settle back to 50ms before continuing.

## 4. cake

`npte netem apply` exposes the queue beneath netem with `--child`:
the flag attaches a child qdisc, replacing the default FIFO with
whatever you ask for. The obvious move is to hand it an
active-queue-management qdisc — `fq_codel`, say — and expect
bloat to vanish.

It does not. The bloat queue we measured in the previous section
lives **inside netem**: when `--rate` caps the link at 30 Mbit/s,
netem holds incoming packets in its own internal FIFO until each
one's release time arrives, and only then hands them downstream.
A child sitting beneath that bottleneck never sees a backlog —
packets reach it one at a time at exactly 30 Mbit/s, which is no
congestion at all. Whatever AQM cleverness the child knows about
is wasted on a queue that never grows.

The fix is to put the rate cap where the AQM is. **cake**
(Common Applications Kept Enhanced) combines rate-limiting,
per-flow fair queueing, and CoDel-style early dropping in a
single qdisc — the bottleneck and the queue manager are the same
code. Demote netem to delay-only and let cake be both the cap
and the queue:

    sudo npte netem clear router if-client
    sudo npte netem apply router if-client \
        --delay 25ms --child "cake bandwidth 30mbit"

The qdisc tree is now: netem (25ms delay, no rate cap, no
bottleneck queue) → cake (30 Mbit/s rate cap with managed AQM
queue). Re-run the loaded test:

    sudo npte netns run client iperf3 -c 172.16.2.2 -R -t 30

Watch terminal C. iperf3 still pulls close to 30 Mbit/s. The
ping RTT stays close to 50ms — typically within a few ms of
idle — for the entire run. No sawtooth, no spikes into the
seconds, no oscillation. The link feels essentially the same
loaded as unloaded, even though one bulk flow is using all the
bandwidth.

What changed is not how many packets the link can drop or how
long it can hold them — what changed is *who is doing the
queueing, and how*. cake's per-flow scheduler gives the ping its
own queue that the iperf3 backlog cannot starve, and its CoDel
core signals congestion to iperf3 by dropping (or ECN-marking) a
packet as soon as the standing queue exceeds a few milliseconds —
long before the buffer is deep enough to hurt anything. CUBIC
reacts to those early signals the same way it reacted to tail
drops in the previous section, but now the signal arrives when
the queue is millimetres deep instead of metres.

This is what modern routers default to (cake itself, or the
older `fq_codel` shaped along the same idea). cake has been in
mainline Linux since 4.19 and costs nothing extra to enable; the
only trick is wiring it as the actual bottleneck rather than as
a trailing child of one.

Wait for iperf3 to finish before continuing.

## 5. The profiles, in one command each

The arc of this chapter is a comparison between two shapes of the
same access link: a dumb FIFO behind a rate cap (§3) and cake as
both the cap and the AQM (§4). Because both shapes are installed
on the same two interfaces — `router/if-client` for downlink,
`router/if-server` for uplink — they bundle cleanly behind a
single convenience command.

`npte star netem --profile <name>` clears both shaped interfaces
and re-applies a named profile. Two profiles ship:

- `4g-bloated` — the dumb-FIFO shape from §3, with `--limit`
  sized for roughly 1s of downlink and 2s of uplink bloat. The
  explicit `--limit` replaces netem's 1000-packet default so the
  sawtooth is unambiguous at 30/10 Mbit/s.
- `4g-managed` — the cake shape from §4, with the same delay and
  bandwidth numbers as `4g-bloated`.

Same access link, one knob:

    sudo npte star netem --profile 4g-bloated
    # run iperf3 from terminal B; watch terminal C climb to seconds

    sudo npte star netem --profile 4g-managed
    # re-run iperf3; terminal C stays close to 50ms

With `--profile ""` (the default, no flag), both interfaces are
cleared and nothing is re-applied — the one-line equivalent of the
`netem clear` pair you have been typing.

`npte star netem --profile 4g-managed --dry-run` prints the exact
`netem clear` / `netem apply` calls the command would run, in the
same shell-quoted form used elsewhere. Two properties follow from
this: the shortcut is pure composition of the primitives you
already know (nothing new is happening at the kernel level), and
the primitives remain the primary interface — reach for `netem
apply` directly whenever the profile table does not cover your
need.

The shortcut is hardcoded for the `client/router/server` star and
its fixed interface names. Asymmetric topologies, different names,
or shapes outside what the two profiles express all still live in
the long form from §3 and §4.

## 6. Teardown

Stop the iperf3 server in terminal A and the ping in terminal C
with `Ctrl-C`, then tear the topology down from terminal B:

    sudo npte star destroy

The qdiscs we installed go with the namespaces; the host is back
to the state it was in before the chapter started.

## Recap

- A `--rate` cap forces a queue to exist somewhere on the link.
  Without one, packets would be dropped instead of paced.

- The discipline of that queue — its depth, when it drops, how
  it shares between flows — is what determines whether the
  loaded link feels tolerable or broken. Bandwidth and idle
  latency are not the whole story.

- A dumb deep FIFO produces a textbook bufferbloat sawtooth:
  CUBIC ramps until the queue overflows, slow-starts on the
  resulting tail-drop, ramps again. Loaded RTT spikes well into
  the seconds. The buffer "doing its job" is what makes the
  link feel broken.

- `--child <qdisc>` attaches a child qdisc beneath netem. It
  only helps if the child is where the bottleneck queue
  actually lives. With `--rate` on netem, the bottleneck is
  inside netem and a downstream child sees no congestion to
  manage.

- `cake bandwidth <rate>` combines rate-limiting, per-flow fair
  queueing, and CoDel-style early dropping in one qdisc. Used
  as the child of a delay-only netem, it is the right shape for
  `--child` here: the bottleneck and the AQM are the same code,
  loaded RTT stays close to idle, and throughput meets the cap.

- `npte netem apply` commits to `root netem` plus an optional
  one-level child. For richer trees — a separate rate limiter,
  multiple classes, hierarchies — drop down to raw `tc` inside
  `npte netns run --user root <ns>`.

- `npte star netem --profile <name>` is a convenience wrapper
  around the same primitives, scoped to the canonical star: it
  expands to `netem clear` + `netem apply` calls on
  `router/if-client` and `router/if-server`. Use `--dry-run` to
  see the expansion; fall back to `netem apply` directly when the
  profile table does not fit.
