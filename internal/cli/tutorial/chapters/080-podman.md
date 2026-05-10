# Podman — attaching a prebuilt container

The previous chapter built an Ubuntu rootfs with `debootstrap` and
bound it to `server` with `systemd-nspawn`. That is the right shape
when you want to control the userspace end-to-end — pick the suite,
pick the packages, edit config files by hand. It is not the right
shape when all you want is "a real nginx" (or redis, or postgres, or
whatever) and the OCI image ecosystem already has a perfectly good
one. This chapter takes the other path: `podman` pulls a Docker Hub
image, starts it, and attaches the resulting container to the
existing `server` namespace with `--network=ns:<path>`. No rootfs to
build, no services to wire up. The filesystem namespace and the
network namespace stay orthogonal; only the thing that populates the
filesystem side changes.

The framing matters for anyone with an existing Docker workflow:
podman pulls the same images the `docker` CLI pulls from the same
registries, so a shop standardised on Docker images can drive them
from `npte` without rebuilding anything.

We still use `sudo` to gain the required `root` privileges.

## 1. A lab and a container runtime

Build the topology:

    sudo npte lab create

No gateway this time. Image pulls run as `podman` on the host (see
the explanatory note in §2), and the container we run only **serves**
HTTP on `:80` — it never dials out. The only namespace-side traffic
in this chapter is the leaf↔leaf `curl` from `client` to
`172.16.2.2`, which crosses the topology entirely through `router`.
Same shape as the previous chapter on the wire — `client` at
`172.16.3.2`, `server` at `172.16.2.2` — minus the host uplink we
needed for `apt install` last chapter.

No shaping either. Shaping is orthogonal to the container story and
belongs in a separate pass with `npte lab netem`.

Install podman on the host:

    sudo apt install podman

`podman` is a daemonless OCI container engine. For our purposes the
important property is `--network=ns:<path>`: instead of creating a
new network namespace for the container, podman joins the one at the
given path. `/run/netns/server` is exactly the path `npte netns
create server` (called by `lab create`) pinned the namespace at, so
`--network=ns:/run/netns/server` plugs the container straight into
the topology.

## 2. Running nginx in `server`

    sudo podman run \
        --rm -d --name nginx \
        --network=ns:/run/netns/server \
        docker.io/library/nginx

Walk the flags:

- `--rm` removes the container record when it stops, so repeated
  runs do not leave stopped carcasses in `podman ps -a`.

- `-d` detaches; podman prints the container id and returns the
  shell.

- `--name nginx` fixes a human name so `podman logs nginx` and
  `podman stop nginx` work without looking up an id.

- `--network=ns:/run/netns/server` is the reason we are here. podman
  does not create a netns, does not allocate a veth, does not attach
  a bridge — it joins the existing namespace as-is. From inside the
  container, `ip link show` lists `lo` and `if-router`, the same
  interfaces `npte netns run server ip link show` prints from the
  host.

- `docker.io/library/nginx` is the fully-qualified Docker Hub
  reference for the official nginx image. podman requires a
  registry prefix in commands unless one is configured in
  `registries.conf`; spelling it out makes the line copy-paste
  reproducible.

First run pulls the image layers from `docker.io` over the **host's**
network — podman's own traffic, not anything routed through the
namespace. Subsequent runs are cache hits against
`/var/lib/containers` and start the container in a fraction of a
second.

One thing worth flagging but not fixing here: the pull runs as
`root` because the whole `podman` invocation does, and neither
podman nor Docker meaningfully sandboxes the image-fetch path.
Splitting the pull (rootless, as your user) from the run (rootful,
to join the root-owned netns) is a plausible hardening step for a
future revision of this chapter.

The nginx image's entrypoint starts nginx in the foreground as the
container's PID 1, bound to `0.0.0.0:80`. Because the container
shares `server`'s netns, that listener appears on `172.16.2.2:80` —
exactly where the nspawn-hosted nginx lived in the previous chapter.

Confirm:

    sudo podman ps

One row, `nginx`, status `Up`.

## 3. Fetching from `client`

Same call as last chapter, same topology, different server
implementation:

    sudo npte netns run client curl -v http://172.16.2.2/

The request leaves `client` on `if-router` (`172.16.3.2`), hits
`router` at `172.16.3.1`, forwards out `if-server` to `172.16.2.2`,
and reaches the only listener on port 80 in that namespace: the
containerised nginx. "Welcome to nginx!" comes back over a single
HTTP/1.1 connection.

See the same request from inside the container:

    sudo podman logs nginx

Each curl shows up in the access log with source `172.16.3.2`.
Nothing in the container image is aware that a namespace exists; the
nginx process sees an ordinary HTTP request from an ordinary IP
address, the same way it would on a real machine with a real NIC.
The isolation happens one level down, in the kernel's per-namespace
routing state, not in anything the container's userspace can see.

## 4. nspawn vs. podman, in one paragraph

Both approaches end up with a server process listening on
`172.16.2.2:80` inside `server`; they differ only in how that
process's userspace got there. `nspawn` gives you a rootfs you
populated yourself — an apt-based workflow where you install, edit,
debug, and rebuild by hand. `podman` gives you whatever the image
author packaged — opinionated, reproducible, already running. For
performance measurement the distinction is invisible: both run on
the same host kernel, share the same CPU and memory, and place the
server process in the same network namespace. Throughput, latency,
and loss measured against either will match modulo server
configuration. Pick the one that matches how you got the software in
the first place.

## 5. Teardown

Stop the container:

    sudo podman stop nginx

`--rm` deletes the container record on stop, so `podman ps -a` will
be empty. Then tear down the topology:

    sudo npte lab destroy

The three namespaces go, the veth pairs are pulled. No gateway
state to clean up — we never installed any. The **image store does
not go**: the nginx layers still sit under `/var/lib/containers`,
ready for the next `podman run` to reuse at zero cost. Evict them
explicitly with `sudo podman rmi docker.io/library/nginx` when you
are done.

## Recap

- `podman run --network=ns:<path>` attaches a container to an
  existing network namespace instead of creating a new one — the
  container-engine analogue of `systemd-nspawn
  --network-namespace-path` from the previous chapter.

- `/run/netns/<ns>` is where `npte netns create` pins the namespace.
  Passing that path to podman plugs the container directly into the
  lab — no bridge, no veth, no extra NAT in the middle.

- Any OCI image on any OCI-compatible registry works the same way:
  `sudo podman run --network=ns:/run/netns/<ns> <image>`. nginx,
  redis, postgres, or a vendor's own test server behave
  identically from the namespace's point of view.

- Image pulls travel over the **host's** network; only the running
  container's runtime traffic goes through the namespace. That is
  usually what you want — nobody is measuring how long it took to
  download the base image.

- podman is Docker-image-compatible: the blobs at
  `docker.io/library/*` are the same blobs the `docker` CLI pulls.
  A Docker-first team can use npte without rebuilding their
  images, just by switching the runner.

- Destroying the topology does not touch the image store. Networks
  are ephemeral, images are cached on disk; the two outlive each
  other, which is the payoff of keeping them orthogonal.
