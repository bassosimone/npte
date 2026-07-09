"""Command-line builders for ``ndt-server`` and ``ndt7-client``."""

# SPDX-License-Identifier: GPL-3.0-or-later

import dataclasses

from .ports import NDT7_CLEARTEXT_PORT, NDT7_TLS_PORT


@dataclasses.dataclass(frozen=True)
class NpteNdt7Server:
    """Configuration for an ndt-server (cleartext + TLS)."""

    binary: str
    datadir: str
    cert: str
    key: str
    cleartext_port: int = NDT7_CLEARTEXT_PORT
    tls_port: int = NDT7_TLS_PORT

    def server_argv(self) -> list[str]:
        return [
            self.binary,
            "-ndt7_addr_cleartext",
            f":{self.cleartext_port}",
            "-ndt7_addr",
            f":{self.tls_port}",
            "-ndt5_addr",
            "127.0.0.1:9301",
            "-ndt5_ws_addr",
            "127.0.0.1:9302",
            "-ndt5_wss_addr",
            "127.0.0.1:9310",
            "-health_addr",
            "127.0.0.1:9800",
            "-prometheusx.listen-address",
            "127.0.0.1:9990",
            "-cert",
            self.cert,
            "-key",
            self.key,
            "-datadir",
            self.datadir,
            "-compress-results=false",
        ]


@dataclasses.dataclass(frozen=True)
class NpteNdt7Client:
    """Configuration for an ndt7-client download test."""

    binary: str
    scheme: str = "ws"
    port: int = NDT7_CLEARTEXT_PORT
    timeout: float = 30

    def client_argv(self, server_addr: str) -> list[str]:
        return [
            "/usr/bin/time",
            "-p",
            self.binary,
            "-server",
            f"{server_addr}:{self.port}",
            "-scheme",
            self.scheme,
            "-no-verify",
            "-download",
            "-upload=false",
            "-format=json",
        ]
