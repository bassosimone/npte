# npte Tutorial

npte (Network Performance Testing Environment) creates isolated network
namespaces connected through a central router, with optional lightweight
containers backed by systemd-nspawn.

All namespaces are project-scoped. Per-project configuration is stored under
`/var/local/npte/<project>/` and survives reboots. Kernel resources
(namespaces, interfaces, routes) are ephemeral. You create namespaces with
`npte netns up` and destroy them with `npte netns down`.

## Chapters

1. [quickstart](quickstart.md) — Create a project and add namespaces.

2. [namespaces](namespaces.md) — Understand namespaces.

3. [containers](containers.md) — Create lightweight containers.

4. [netem](netem.md) — Simulate network conditions using `tc` and `netem`.

5. [browser](browser.md) — Run a web browser in a shaped namespace.

Read a chapter with:

    npte tutorial <chapter>

For example:

    npte tutorial quickstart
