"""Types for describing and running measurement sweeps."""

# SPDX-License-Identifier: GPL-3.0-or-later

import dataclasses
import time
from collections.abc import Callable, Iterator

from .executor import RunningProcess, TerminatedProcess
from .lab import NpteLab


@dataclasses.dataclass(frozen=True)
class NpteShaping:
    """Shaping parameters for one direction."""

    rate: str
    delay: str


def symmetric_shaping_matrix(
    rates: list[str],
    rtts_ms: list[float],
) -> Iterator[NpteShaping]:
    """Yield an NpteShaping for each (rate, rtt) combination with delay = rtt/2."""
    for rtt_ms in rtts_ms:
        for rate in rates:
            yield NpteShaping(rate=rate, delay=f"{rtt_ms / 2}ms")


class NpteCell:
    """A single point in the grid: shaping conditions plus clients to run."""

    def __init__(self) -> None:
        self._download: NpteShaping | None = None
        self._upload: NpteShaping | None = None
        self._clients: list[Callable[[], TerminatedProcess]] = []

    def set_download(self, shaping: NpteShaping) -> None:
        self._download = shaping

    def set_upload(self, shaping: NpteShaping) -> None:
        self._upload = shaping

    def add_client(self, client: Callable[[], TerminatedProcess]) -> None:
        self._clients.append(client)


class NpteGrid:
    """Collects servers and cells, runs the sweep."""

    def __init__(self, lab: NpteLab) -> None:
        self._lab = lab
        self._servers: list[Callable[[], RunningProcess]] = []
        self._cells: list[NpteCell] = []

    def add_server(self, server: Callable[[], RunningProcess]) -> None:
        self._servers.append(server)

    def add_cell(self, cell: NpteCell) -> None:
        self._cells.append(cell)

    def run(self) -> list[TerminatedProcess]:
        running: list[RunningProcess] = []
        results: list[TerminatedProcess] = []
        try:
            for start_server in self._servers:
                running.append(start_server())
            time.sleep(0.5)
            for proc in running:
                if not proc.is_running():
                    raise RuntimeError("server exited early")
            for cell in self._cells:
                self._apply_shaping(cell)
                for client in cell._clients:
                    results.append(client())
        finally:
            for proc in running:
                proc.kill()
            for proc in running:
                proc.wait(timeout=5)
        return results

    def _apply_shaping(self, cell: NpteCell) -> None:
        self._lab.client.shape_download(
            rate=cell._download.rate if cell._download is not None else None,
            delay=cell._download.delay if cell._download is not None else None,
        )
        self._lab.client.shape_upload(
            rate=cell._upload.rate if cell._upload is not None else None,
            delay=cell._upload.delay if cell._upload is not None else None,
        )
