# Traffic Shaping with netem

This chapter explains how to simulate network conditions (latency, losses,
shaping) using the Linux `tc` and `netem` subsystems.

npte does not have a built-in traffic shaping command. Instead, you run `tc`
inside the **router** namespace using `npte netns run --user root`.

## Which interface to shape

Traffic control (`tc`) shapes **egress** — packets leaving an interface.
To understand where to apply rules, recall the topology from the
namespaces chapter (IP addresses omitted for clarity):

```
                    ┌──────────┐
                    │   host   │
                    └────*─────┘
                         │
                         │
                         │ <veth:lab-router-i>
                    ┌────*─────┐
                    │  router  │
                    └──*────*──┘
   <veth:lab-client-r> │    │ <veth:lab-server-r>
                       │    │
                       │    │
                       │    │
              ┌────────*┐  ┌*────────┐
              │ client  │  │ server  │
              └─────────┘  └─────────┘
```

Shaping happens inside the **router** namespace, on egress:

- **Client download**: shape `lab-client-r` (router → client).
- **Client upload to internet**: shape `lab-router-i` (router → host).
- **Client upload to server namespace**: shape `lab-server-r` (router → server).

Find interface names with:

    sudo npte netns show lab | grep 'netns lab-router'

## Adding delay

Add 50ms one-way delay to the client's download path:

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem delay 50ms

## Rate limiting

Limit the client's download to 10 Mbit/s with 50ms delay:

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem delay 50ms rate 10mbit

## Packet loss

Add 1% random packet loss:

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem loss 1%

## Shaping both directions

To simulate a slow client link (e.g., for browser testing), shape both
the download and upload paths. In a setup with a client using the internet:

    # Client download (internet → client)
    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem delay 50ms rate 10mbit

    # Client upload (client → internet)
    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-router-i root netem delay 50ms rate 1mbit

For asymmetric shaping between client and server:

    # Client download (router → client)
    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem delay 50ms rate 10mbit

    # Client upload (router → server)
    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-server-r root netem delay 50ms rate 1mbit

## Simulating bufferbloat

The `limit` parameter controls the queue size in packets. A large queue
relative to the bandwidth-delay product simulates bufferbloat:

    sudo npte netns run --user root lab router \
        tc qdisc add dev lab-client-r root netem \
		    delay 10ms rate 50mbit limit 1000

## Removing rules

    sudo npte netns run --user root lab router \
        tc qdisc del dev lab-client-r root

## Common profiles

| Profile        | Delay  | Rate     | Loss | Limit |
|----------------|--------|----------|------|-------|
| 2G (GPRS)      | 300ms  | 50kbit   | 2%   |       |
| 3G             | 100ms  | 2mbit    | 0.5% |       |
| 4G (LTE)       | 30ms   | 30mbit   | 0.1% |       |
| Cable          | 10ms   | 50mbit   |      |       |
| Cable + bloat  | 10ms   | 50mbit   |      | 1000  |
| FTTH 1G        | 2ms    | 1gbit    |      |       |
