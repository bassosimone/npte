# Traffic Shaping with netem

This chapter explains how to simulate network conditions (latency, losses,
rate limiting) using the Linux `tc` and `netem` subsystems.

npte does not have a built-in traffic shaping command. Instead, you run `tc`
inside the appropriate namespace using `npte netns run --user root`.

## TL;DR

The properties we want to emulate are the following:

1. Packet loss rate in the upload direction

2. Packet loss rate in the download direction

3. Last-mile upload line rate

4. Last-mile download line rate

5. Last-mile upload propagation delay

6. Last-mile download propagation delay

The following diagram shows the proper interface to use for
emulating each of the above properties. Note that (1) is
applied either on `lab-router-i` or `lab-server-r` depending
on whether the server is on the internet or in a namespace.

```
                            ┌──────────┐
                            │   host   │
                            └────*─────┘
                                 │
                                 │
                                 │ <veth:lab-router-i> (1)
                            ┌────*─────┐
                            │  router  │
                            └──*────*──┘
 <veth:lab-client-r> (2, 4, 6) │    │ <veth:lab-server-r> (1)
                               │    │
                               │    │
                               │    │
    <veth:lab-client-s> (3, 5) │    │
                      ┌────────*┐  ┌*────────┐
                      │ client  │  │ server  │
                      └─────────┘  └─────────┘
```

As a reminder, you can find the interface names with:

    npte netns show lab

The following sections explain each property and its `tc`/`netem` options.

## Adding delay

Delay models **propagation latency**: the time it takes a packet to
traverse the network path (fiber length, processing at intermediate
hops, etc.). Each direction is configured independently; the round-trip
time (RTT) is the sum of both directions.

Add 25ms one-way delay to the client's download path:

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem delay 25ms

And to the upload path:

    sudo npte netns run --user root lab client \
        tc qdisc add dev lab-client-s root netem delay 25ms

In practice, delay is never constant. Add **jitter** (random variation)
with `delay BASE JITTER distribution paretonormal`. The `paretonormal`
distribution models real networks well: most packets arrive near the
base delay, but some are delayed significantly (right-skewed, heavy tail).

Add 25ms base delay with 5ms jitter:

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem \
            delay 25ms 5ms distribution paretonormal

## Packet loss

Loss models **congestion and impairments elsewhere in the network**:
intermediate routers dropping packets under load, wireless interference,
shared links under contention. These are things outside the access link
that manifest as random packet drops from the client's perspective.

Add 1% random packet loss to the download path:

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem loss 1%

And to the upload path (server on the internet):

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-router-i root netem loss 1%

If the server is in a namespace, use `lab-server-r` instead of `lab-router-i`.

In practice, loss is **bursty**: when a router buffer overflows or a
wireless link degrades, several consecutive packets are lost, not random
individual ones. The **Gilbert-Elliott model** (`loss gemodel`) captures
this with a two-state Markov chain (good state and bad state):

    loss gemodel p r 1-h 1-k

- `p` = probability of transitioning good → bad (e.g., `1%`)
- `r` = probability of transitioning bad → good (e.g., `30%`)
- `1-h` = loss probability in the bad state (e.g., `80%`)
- `1-k` = loss probability in the good state (usually `0%`)

Example: occasional burst losses on a degraded link:

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem \
            loss gemodel 1% 30% 80% 0%

You can combine delay and loss in a single netem rule when they are on
the same interface. For example, download delay + loss on `lab-client-r`:

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem delay 25ms loss 1%

## Rate limiting

The `rate` option simulates a physical interface by **pacing packets** at
the configured speed. This produces correct inter-packet gaps (IPG), which
matters for congestion control algorithms like BBR that estimate bottleneck
bandwidth from packet arrival timing.

Limit the client's download to 30 Mbit/s with 25ms delay:

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem delay 25ms rate 30mbit

To simulate a realistic client access link, condition both directions
on the client's veth pair. Example: 4G-like link (50ms RTT, 30/10 Mbit/s):

    # Download: 25ms delay + 30 Mbit/s
    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem delay 25ms rate 30mbit

    # Upload: 25ms delay + 10 Mbit/s
    sudo npte netns run --user root lab client \
        tc qdisc add dev lab-client-s root netem delay 25ms rate 10mbit

## Child qdiscs and bufferbloat

By default, netem uses a simple FIFO buffer for queued packets (controlled
by the `limit` parameter, in packets). When the buffer fills, excess
packets are tail-dropped. This models a **dumb buffer** — the oversized
FIFOs found in cheap modems and routers that cause bufferbloat.

netem supports **child qdiscs** that replace the default FIFO with a
smarter queuing policy. The child handles how packets are queued and
which packets are dropped; netem handles the timing (delay, rate, loss).

**Simulating bufferbloat** — use a large FIFO buffer:

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root handle 1: \
            netem delay 25ms rate 30mbit

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r parent 1:1 handle 10: \
            pfifo limit 1250

With 1250 full-size packets at 30 Mbit/s, the queue can hold ~500ms of
data — latency spikes by half a second under load.

**Well-managed network** — use `fq_codel` as the child qdisc. It provides
per-flow fair queuing and CoDel-based active queue management, keeping
latency bounded even under load:

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root handle 1: \
            netem delay 25ms rate 30mbit

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r parent 1:1 handle 10: fq_codel

This is what modern routers typically use and post Linux 4.20 [BBR should
be able to interoperate with `fq_codel` just fine](https://groups.google.com/g/bbr-dev/c/4jL4ropdOV8).

## Removing rules

Remove download conditioning:

    sudo npte netns run --user root lab router \
        tc qdisc del dev lab-client-r root

Remove upload conditioning:

    sudo npte netns run --user root lab client \
        tc qdisc del dev lab-client-s root

## Common profiles

Each profile is applied per direction. The one-way delay is half the RTT.

| Profile     | RTT   | Download | Upload   |
|-------------|-------|----------|----------|
| 2g          | 600ms | 200kbit  | 50kbit   |
| 3g          | 200ms | 3mbit    | 1mbit    |
| 4g          | 60ms  | 30mbit   | 10mbit   |
| 5g          | 20ms  | 100mbit  | 30mbit   |
| broadband   | 50ms  | 100mbit  | 20mbit   |
| ftth-100    | 10ms  | 100mbit  | 50mbit   |
| ftth-1g     | 10ms  | 1gbit    | 500mbit  |
| server      | 2ms   | (none)   | (none)   |

The "server" profile adds delay only — data center links run at
10-100 Gbps, beyond what `tc` can meaningfully shape.

Add a child qdisc (e.g., `fq_codel`) to control the buffering
behavior. Without a child, netem uses its default FIFO.
