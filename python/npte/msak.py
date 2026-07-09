"""Command-line builders for ``msak-server`` and ``msak-client``."""

# SPDX-License-Identifier: GPL-3.0-or-later

import dataclasses
import uuid

from .ports import MSAK_CLEARTEXT_PORT, MSAK_TLS_PORT


@dataclasses.dataclass(frozen=True)
class NpteMsakServer:
    """Configuration for an msak-server (cleartext + TLS)."""

    binary: str
    datadir: str
    cert: str
    key: str
    cleartext_port: int = MSAK_CLEARTEXT_PORT
    tls_port: int = MSAK_TLS_PORT

    def server_argv(self) -> list[str]:
        return [
            self.binary,
            "-ws_addr",
            f":{self.cleartext_port}",
            "-wss_addr",
            f":{self.tls_port}",
            "-prometheusx.listen-address",
            "127.0.0.1:9991",
            "-cert",
            self.cert,
            "-key",
            self.key,
            "-datadir",
            self.datadir,
        ]


@dataclasses.dataclass(frozen=True)
class NpteMsakClient:
    """Configuration for an msak-client download test."""

    binary: str
    cc: str = "cubic"
    scheme: str = "ws"
    duration: str = "10s"
    port: int = MSAK_CLEARTEXT_PORT
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
            "-streams",
            "1",
            "-cc",
            self.cc,
            "-duration",
            self.duration,
            "-download",
            "-upload=false",
            "-mid",
            str(uuid.uuid7()),
        ]
