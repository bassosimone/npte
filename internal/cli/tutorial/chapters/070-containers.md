# Containers — attaching a filesystem namespace

Every chapter so far has swapped out only the network stack.
`npte netns run` enters a namespace but leaves the host filesystem
underneath — curl, ping, iperf3, even the browser all read from
`/usr` on the host. That is the right default for tools already
installed on your machine, but it breaks the moment you want to
install server software per-namespace. Putting nginx on the host
to serve one namespace pollutes the host; reaching for Docker
breaks the performance-testing persistence the rest of this
tutorial relies on. The fix is a second, orthogonal namespace: a
private filesystem tree, bound to the existing network namespace
by `systemd-nspawn`. This chapter walks the pattern end to end —
a lab, a debootstrap tree, a booted container, nginx inside,
and a curl from `client` that hits it.

We still use `sudo` to gain the required `root` privileges.

## 1. A lab and a tree

Build the topology, plus a gateway on top so the booted container
in `server` can reach Ubuntu's apt mirrors when we install nginx in
§5. Find your host's internet-facing interface with `ip route show
default`, then:

    sudo npte lab create
    sudo npte gateway create router 172.16.1.0/24 <uplink>

Same lab as chapter 4 plus a `router` uplink: `client` at
`172.16.3.2`, `server` at `172.16.2.2`, `router` NATing to the
host. No traffic shaping — shaping is orthogonal to the container
story.

Splitting `lab` and `gateway` keeps the password-prompt cost
focused: `lab` is allowlisted via `npte sudoers`, so only
`gateway create` and `gateway destroy` actually prompt. The
`debootstrap` step that follows is a host-side fetch and does not
need the gateway; only the in-container `apt install nginx` later
does.

Now build the filesystem tree. The location is up to you; this
chapter uses `$HOME/containers/noble` so the tree lives alongside
your user data rather than under `/var/local`. Create the parent
directory and debootstrap into the leaf:

    mkdir -p $HOME/containers
    sudo npte container create noble $HOME/containers/noble

`container create` is a thin wrapper over `debootstrap`. It runs
`debootstrap noble $HOME/containers/noble`, which fetches the
Ubuntu 24.04 LTS base system, unpacks it, and leaves a minimal
root filesystem sitting on disk. It takes a few minutes on a
first run. The result is a plain directory tree — nothing is
mounted, nothing is running, and the kernel does not know it
exists as anything special.

Two properties of this step matter. First, the tree is **not**
tied to the network namespace. `container create` takes a suite
and a path; the word `server` never appears. Second, once the
tree exists you can point it at any namespace, destroy and
recreate the namespace without re-bootstrapping, or poke at the
tree outside a namespace entirely. The filesystem namespace and
the network namespace are orthogonal primitives, and `npte`
keeps them that way.

## 2. Setting a root password

A bare debootstrap tree has `root` with a locked password — the
`!` in `/etc/shadow` means no password string will ever match.
Once we boot the tree, `systemd` starts a login getty on the
console and there is no usable credential to type in. Fix that
once, now, before booting:

    sudo npte container run --netns server $HOME/containers/noble

`container run` (no `--boot`) drops you straight into a root
shell inside the tree with no login step — `systemd-nspawn`
launches `/bin/bash` as PID 1 directly rather than `systemd`.
Inside that shell, set a root password:

    passwd

Type something you will remember for the duration of the chapter
(`npte` is fine). Then exit:

    exit

The password is now baked into the tree's `/etc/shadow`. The next
time anything asks for it — including the login prompt on a
booted container — `root` plus that password will work.

The `--netns server` is not strictly needed here: we are not
using the network during `passwd`. It is included so the mental
model stays clean: every `container run` and `container boot` in
this chapter targets `server`. The filesystem tree itself does
not care.

## 3. Booting the tree into `server`

Two terminals from here. **Terminal A** holds the booted
container. **Terminal B** drives the client side and stays free
for `curl` and inspection.

In terminal A:

    sudo npte container boot $HOME/containers/noble --netns server

This runs `systemd-nspawn --boot -D $HOME/containers/noble
--network-namespace-path=/run/netns/server` — add `--dry-run` to
see the exact command. Two namespace kinds come together here:

- The **filesystem namespace** is everything under
  `$HOME/containers/noble`. `nspawn` pivots into it: `/`, `/usr`,
  `/etc`, and every process's view of the filesystem comes from
  this tree, not the host.

- The **network namespace** is `/run/netns/server`, already built
  by `lab create`. `nspawn` does not create or address it; it
  joins it as-is. From inside the container, `ip link show` lists
  `lo` and `if-router` — the same interfaces `npte netns run
  server ip link show` would print on the host.

`systemd` runs as PID 1 inside the container, mounts its own
`/proc`, `/sys`, and `/dev`, and starts the tree's default
target. After a few seconds you get a `login:` prompt. Log in as
`root` with the password you set in the previous section.

You are now a root shell inside an Ubuntu 24.04 userspace that
happens to be running on your kernel, through the `server`
namespace's network stack. `ip addr show` reports
`172.16.2.2/24` on `if-router`, which is exactly what the rest
of the topology expects.

## 4. Fixing DNS

Before installing anything, one detour. Try:

    apt update

It fails: every mirror hostname returns `SERVFAIL` immediately.
The cause is specific and worth understanding rather than
papering over, because the same shape will recur any time you
attach a freshly-booted container to a namespace that has no
DHCP running in it.

Inside the container:

    resolvectl status

The `Global` block has no `Fallback DNS:` line, and `if-router`
reports `Current Scopes: none`. Two things are missing:

- **No per-link DNS on `if-router`.** That slot is normally filled
  by a DHCP lease (via NetworkManager or systemd-networkd). Our
  IP was assigned statically by `npte netns assign-addr`; nothing
  ran DHCP inside this namespace, so the link has no DNS
  associated with it.

- **No global fallback DNS.** Debian and Ubuntu build the
  `systemd` package with `--with-dns-servers=""`, deliberately
  stripping the compile-time fallback list so the OS does not
  silently leak queries to Google or Cloudflare without the user
  opting in. The fallback you might expect to be there on
  upstream `systemd` is simply not there in the packaged build.

Empty per-link + empty fallback = nothing to resolve against.
Every lookup returns immediately with `SERVFAIL`, which is what
`apt update` surfaces as a flurry of "Temporary failure resolving"
lines.

Configure a global resolver and restart `systemd-resolved`:

    mkdir -p /etc/systemd/resolved.conf.d
    printf '[Resolve]\nDNS=8.8.8.8\n' > \
        /etc/systemd/resolved.conf.d/npte.conf
    systemctl restart systemd-resolved

`resolvectl status` now shows the global `DNS Servers: 8.8.8.8`,
and `apt update` succeeds.

You could also bypass the stub resolver entirely by overwriting
`/etc/resolv.conf` with a plain file pointing at a nameserver:
glibc reads whatever is at that path and never asks
`systemd-resolved` anything. The drop-in above is the more
surgical fix because it keeps the stub in place and matches how
a real Ubuntu install is configured; for the rest of this
chapter the two are interchangeable.

## 5. Installing nginx

    apt install -y nginx

The install pulls the package through the newly-working
resolver, and — this is the reason we booted rather than ran —
nginx's post-install script enables and starts `nginx.service`.
There is no "now start nginx" step: the service unit is part of
the deb, `systemd` is PID 1 inside the container, and the unit
comes up before `apt` returns. Confirm:

    systemctl status nginx
    ss -ltn

You should see nginx active and a listener on `0.0.0.0:80`. The
bind address is what matters: because nginx is bound to the
wildcard, it accepts traffic on `lo`, `if-router`, and any other
interface in the namespace — including the `172.16.2.2` address
the rest of the topology reaches.

Leave terminal A alone. The container keeps running, `systemd`
keeps `nginx.service` healthy, and we switch to terminal B.

## 6. Fetching from `client`

From terminal B:

    sudo npte netns run client curl -v http://172.16.2.2/

The request leaves `client` on `if-router` (`172.16.3.2`), hits
`router` at `172.16.3.1`, forwards out `if-server` to
`172.16.2.2`, and reaches the `server` namespace. Inside that
namespace the packet lands on the only listener on port 80 —
nginx, running inside the booted container. Nginx serves the
default "Welcome to nginx!" HTML and the response retraces the
path back.

The `-v` output makes the handshake visible: a single TCP
connection, HTTP/1.1, `200 OK`, a short response body. No TLS,
no DNS, no proxy — one HTTP call across one hop of the topology,
landing in a userspace that lives in a different filesystem than
the caller's.

To see the same fetch from the other side, back in terminal A:

    tail -n5 /var/log/nginx/access.log

Each curl shows up as a line with source `172.16.3.2` —
`client`'s address, as the `server` namespace sees it. nginx has
no idea a namespace exists; it sees a perfectly ordinary HTTP
request from a perfectly ordinary IP address, which happens to
be meaningful only inside this topology.

## 7. Teardown

Inside the booted container (terminal A):

    systemctl poweroff

`systemd-nspawn` exits as soon as PID 1 halts and returns
terminal A to your host shell. `Ctrl-]` three times is the
brute-force alternative when an inner process is wedged, but
`poweroff` is the polite path and runs every service's shutdown
hook on the way down.

Then, from terminal B:

    sudo npte gateway destroy router
    sudo npte lab destroy

`gateway destroy` removes the host-side NAT/FORWARD rules and the
uplink veth; `lab destroy` removes the three namespaces and the
veth pairs. The **filesystem tree does not go**: it still sits at
`$HOME/containers/noble`, nginx still installed, the resolved.conf
drop-in still in place, root password still set. Attach it to a
different namespace in the next experiment, or remove it by hand
with `sudo rm -rf $HOME/containers/noble` when you are done.

## Recap

- Network namespaces and filesystem namespaces are orthogonal.
  `npte netns` builds the first, `npte container` builds the
  second, and `systemd-nspawn` binds them together — one kernel,
  two decoupled views.

- `npte container create <suite> <rootfs>` is a thin wrapper over
  `debootstrap`. It knows nothing about namespaces — the tree is
  a plain directory you can reuse, destroy, or populate
  independently of any topology.

- A fresh debootstrap tree has `root` locked. Drop in with
  `container run` (no `--boot`) and `passwd` once, before you
  boot; the booted container's login prompt will then accept the
  password you set.

- `npte container boot <rootfs> --netns <ns>` runs
  `systemd-nspawn --boot -D <rootfs>
  --network-namespace-path=/run/netns/<ns>`. `systemd` starts as
  PID 1 inside and shares the namespace's network stack.

- Booting (rather than one-shot `container run`) matters when you
  want services to start on their own. `apt install nginx` inside
  a booted container starts `nginx.service` via its systemd unit,
  with no extra command.

- DNS inside a freshly-booted container in a no-DHCP namespace
  does **not** work out of the box on Ubuntu. `systemd` is
  packaged with `--with-dns-servers=""` so there is no fallback;
  nothing in the namespace runs DHCP to populate a per-link DNS;
  and `systemd-resolved` ends up with no server to ask. Drop
  `DNS=<nameserver>` into `/etc/systemd/resolved.conf.d/` and
  restart `systemd-resolved`.

- Destroying the topology leaves the filesystem tree on disk.
  The two primitives outlive each other, which is the payoff of
  keeping them orthogonal.
