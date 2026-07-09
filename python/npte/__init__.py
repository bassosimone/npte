"""
Drive ``npte`` from Python scripts.

Import all public symbols from this package::

    from npte import NpteLab, NpteTools, NpteGrid, ...

Do not import from internal modules (``npte.lab``, ``npte.sweep``, etc.)
— module names may change without notice.  The public API is the set of
``Npte``-prefixed classes and ``npte_``-prefixed functions exported here.

Prerequisite: install passwordless sudoers rules with ``npte sudoers``.
See ``python/README.md`` for setup details.

Example — build tools, sweep rates/RTTs/CCs across multiple servers::

    from npte import (
        NpteCell,
        NpteGrid,
        NpteLab,
        NpteTools,
        npte_symmetric_shaping_matrix,
    )

    tools = NpteTools()
    tools.generate_certs()
    tools.build_msak()
    tools.build_ndt7()

    with NpteLab() as lab:
        grid = NpteGrid(lab)
        grid.add_server(tools.iperf3_server())
        grid.add_server(tools.msak_server())
        grid.add_server(tools.ndt7_server())

        for shaping in npte_symmetric_shaping_matrix(
            rates=["100mbit"],
            rtts_ms=[5, 25],
        ):
            for cc in ["bbr", "cubic"]:
                cell = NpteCell()
                cell.set_download(shaping)
                cell.set_upload(shaping)
                cell.add_client(tools.iperf3_client(cc=cc))
                cell.add_client(tools.msak_client(cc=cc))
                cell.add_client(tools.ndt7_client())
                grid.add_cell(cell)

        for result in grid.run():
            print(f"exit={result.exitcode} dir={result.proc_dir}")
"""

# SPDX-License-Identifier: GPL-3.0-or-later

from .executor import NpteSudoExecutor
from .iperf3 import NpteIperf3Client, NpteIperf3Server
from .lab import NpteLab
from .msak import NpteMsakClient, NpteMsakServer
from .ndt7 import NpteNdt7Client, NpteNdt7Server
from .powersaving import npte_energy_performance_preferences
from .sweep import (
    NpteCell,
    NpteClientConfig,
    NpteGrid,
    NpteServerConfig,
    NpteShaping,
    npte_symmetric_shaping_matrix,
)
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
    "npte_symmetric_shaping_matrix",
]
