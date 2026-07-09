"""Types for describing and running measurement sweeps."""

# SPDX-License-Identifier: GPL-3.0-or-later

import dataclasses
import time
from collections.abc import Iterator
from typing import Protocol

from .executor import TerminatedProcess
from .lab import NpteLab


class NpteServerConfig(Protocol):
    """Protocol for server command-line builders."""

    def server_argv(self) -> list[str]: ...


class NpteClientConfig(Protocol):
    """Protocol for client command-line builders."""

    timeout: float

    def client_argv(self, server_addr: str) -> list[str]: ...


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
        self._clients: list[NpteClientConfig] = []

    def set_download(self, shaping: NpteShaping) -> None:
        self._download = shaping

    def set_upload(self, shaping: NpteShaping) -> None:
        self._upload = shaping

    def add_client(self, client: NpteClientConfig) -> None:
        self._clients.append(client)


class NpteGrid:
    """Collects servers and cells, runs the sweep."""

    def __init__(self, lab: NpteLab) -> None:
        self._lab = lab
        self._servers: list[NpteServerConfig] = []
        self._cells: list[NpteCell] = []

    def add_server(self, server: NpteServerConfig) -> None:
        self._servers.append(server)

    def add_cell(self, cell: NpteCell) -> None:
        self._cells.append(cell)

    def run(self) -> list[TerminatedProcess]:
        running = []
        results: list[TerminatedProcess] = []
        try:
            for server in self._servers:
                running.append(self._lab.server.start(server.server_argv()))
            time.sleep(0.5)
            for proc in running:
                if not proc.is_running():
                    raise RuntimeError("server exited early")
            for cell in self._cells:
                self._apply_shaping(cell)
                for client in cell._clients:
                    results.append(
                        self._lab.client.run(
                            client.client_argv(self._lab.server.addr),
                            timeout=client.timeout,
                        )
                    )
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
