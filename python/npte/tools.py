"""Build tools and produce pre-configured server/client dataclasses."""

# SPDX-License-Identifier: GPL-3.0-or-later

import os
import shlex
import subprocess
import sys

from .iperf3 import NpteIperf3Client, NpteIperf3Server
from .msak import NpteMsakClient, NpteMsakServer
from .ndt7 import NpteNdt7Client, NpteNdt7Server


class NpteTools:
    """Build measurement tools from source and produce configured dataclasses.

    By default, all artifacts live under ``<root_dir>/.npte/``::

        .npte/bin/       go install target
        .npte/certs/     cert.pem, key.pem
        .npte/data/      server datadirs
    """

    def __init__(self, root_dir: str = ".", *, data_dir: str | None = None) -> None:
        self._bindir = os.path.join(root_dir, ".npte", "bin")
        self._certdir = os.path.join(root_dir, ".npte", "certs")
        self._datadir = (
            data_dir
            if data_dir is not None
            else os.path.join(root_dir, ".npte", "data")
        )

    def generate_certs(self, *, npte: str = "/usr/sbin/npte") -> None:
        """Generate TLS certificates for the lab server address."""
        os.makedirs(self._certdir, exist_ok=True)
        argv = [npte, "gencerts", "-C", self._certdir, "--ip-addr", "172.16.2.2"]
        print(f"+ {shlex.join(argv)}", file=sys.stderr)
        subprocess.run(argv, check=True)

    def build_msak(self) -> None:
        """Build msak-server and msak-client from source."""
        os.makedirs(self._bindir, exist_ok=True)
        env = {**os.environ, "GOBIN": os.path.abspath(self._bindir)}
        argv = ["go", "install", "github.com/m-lab/msak/cmd/msak-server@latest"]
        print(f"+ {shlex.join(argv)}", file=sys.stderr)
        subprocess.run(argv, env=env, check=True)
        argv = ["go", "install", "github.com/m-lab/msak/cmd/msak-client@latest"]
        print(f"+ {shlex.join(argv)}", file=sys.stderr)
        subprocess.run(argv, env=env, check=True)

    def build_ndt7(self) -> None:
        """Build ndt-server and ndt7-client-go from source."""
        os.makedirs(self._bindir, exist_ok=True)
        env = {**os.environ, "GOBIN": os.path.abspath(self._bindir)}
        argv = ["go", "install", "github.com/m-lab/ndt-server@latest"]
        print(f"+ {shlex.join(argv)}", file=sys.stderr)
        subprocess.run(argv, env=env, check=True)
        argv = [
            "go",
            "install",
            "github.com/m-lab/ndt7-client-go/cmd/ndt7-client@latest",
        ]
        print(f"+ {shlex.join(argv)}", file=sys.stderr)
        subprocess.run(argv, env=env, check=True)

    def msak_server(self) -> NpteMsakServer:
        """Return a pre-configured msak-server dataclass."""
        os.makedirs(self._datadir, exist_ok=True)
        return NpteMsakServer(
            binary=os.path.join(self._bindir, "msak-server"),
            datadir=self._datadir,
            cert=os.path.join(self._certdir, "cert.pem"),
            key=os.path.join(self._certdir, "key.pem"),
        )

    def msak_client(self, *, cc: str = "cubic") -> NpteMsakClient:
        """Return a pre-configured msak-client dataclass."""
        return NpteMsakClient(
            binary=os.path.join(self._bindir, "msak-client"),
            cc=cc,
        )

    def ndt7_server(self) -> NpteNdt7Server:
        """Return a pre-configured ndt-server dataclass."""
        os.makedirs(self._datadir, exist_ok=True)
        return NpteNdt7Server(
            binary=os.path.join(self._bindir, "ndt-server"),
            datadir=self._datadir,
            cert=os.path.join(self._certdir, "cert.pem"),
            key=os.path.join(self._certdir, "key.pem"),
        )

    def ndt7_client(self) -> NpteNdt7Client:
        """Return a pre-configured ndt7-client dataclass."""
        return NpteNdt7Client(
            binary=os.path.join(self._bindir, "ndt7-client"),
        )

    def iperf3_server(self) -> NpteIperf3Server:
        """Return a pre-configured iperf3 server dataclass."""
        return NpteIperf3Server()

    def iperf3_client(self, *, cc: str = "cubic") -> NpteIperf3Client:
        """Return a pre-configured iperf3 client dataclass."""
        return NpteIperf3Client(cc=cc)
