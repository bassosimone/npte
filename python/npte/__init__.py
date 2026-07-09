"""
Drive ``npte`` from Python scripts.

Import all public symbols from this package directly::

    from npte import NpteLab, NpteTools, NpteGrid, ...

Do not rely on the internal module layout (``npte.lab``, ``npte.sweep``,
etc.) — module names may change without notice. The public API is the
set of ``Npte``-prefixed classes and ``npte_``-prefixed functions
re-exported here.

The main assumption is that you have used ``npte sudoers`` to install
sudoers(5) rules that allow executing the following commands:

1. ``npte netns *``
2. ``npte netem *``
3. ``npte lab *``

with sudo(8) without requiring a password prompt.

Minimal example::

    from npte import NpteLab

    with NpteLab() as lab:
        server = lab.server.start(["./httpserver"])
        lab.client.shape_download(rate="100mbit", delay="25ms")
        lab.client.shape_upload(rate="20mbit", delay="25ms")
        lab.client.run(["curl", f"http://{lab.server.addr}:8080/"])
        server.kill()
        server.wait()
"""

# SPDX-License-Identifier: GPL-3.0-or-later

from .executor import NpteSudoExecutor
from .iperf3 import NpteIperf3Client, NpteIperf3Server
from .lab import NpteLab
from .msak import NpteMsakClient, NpteMsakServer
from .ndt7 import NpteNdt7Client, NpteNdt7Server
from .powersaving import npte_energy_performance_preferences
from .sweep import NpteCell, NpteClientConfig, NpteGrid, NpteServerConfig, NpteShaping
from .tools import NpteTools

__all__ = [
    "NpteCell",
    "NpteClientConfig",
    "NpteGrid",
    "NpteIperf3Client",
    "NpteIperf3Server",
    "NpteLab",
    "NpteMsakClient",
    "NpteMsakServer",
    "NpteNdt7Client",
    "NpteNdt7Server",
    "NpteServerConfig",
    "NpteShaping",
    "NpteSudoExecutor",
    "NpteTools",
    "npte_energy_performance_preferences",
]
