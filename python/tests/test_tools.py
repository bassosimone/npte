# SPDX-License-Identifier: GPL-3.0-or-later

import os

from npte import (
    NpteIperf3Client,
    NpteIperf3Server,
    NpteMsakClient,
    NpteMsakServer,
    NpteNdt7Client,
    NpteNdt7Server,
    NpteTools,
)

# --- Factory methods ---


def test_iperf3_server(tmp_path):
    tools = NpteTools(root_dir=str(tmp_path))
    assert tools.iperf3_server() == NpteIperf3Server()


def test_iperf3_client(tmp_path):
    tools = NpteTools(root_dir=str(tmp_path))
    assert tools.iperf3_client(cc="bbr") == NpteIperf3Client(cc="bbr")


def test_iperf3_client_default_cc(tmp_path):
    tools = NpteTools(root_dir=str(tmp_path))
    assert tools.iperf3_client() == NpteIperf3Client(cc="cubic")


def test_msak_server_custom_datadir(tmp_path):
    custom = os.path.join(str(tmp_path), "custom-data")
    tools = NpteTools(root_dir=str(tmp_path), data_dir=custom)
    bindir = os.path.join(str(tmp_path), ".npte", "bin")
    certdir = os.path.join(str(tmp_path), ".npte", "certs")
    assert tools.msak_server() == NpteMsakServer(
        binary=os.path.join(bindir, "msak-server"),
        datadir=custom,
        cert=os.path.join(certdir, "cert.pem"),
        key=os.path.join(certdir, "key.pem"),
    )


def test_msak_server(tmp_path):
    tools = NpteTools(root_dir=str(tmp_path))
    bindir = os.path.join(str(tmp_path), ".npte", "bin")
    certdir = os.path.join(str(tmp_path), ".npte", "certs")
    datadir = os.path.join(str(tmp_path), ".npte", "data")
    assert tools.msak_server() == NpteMsakServer(
        binary=os.path.join(bindir, "msak-server"),
        datadir=datadir,
        cert=os.path.join(certdir, "cert.pem"),
        key=os.path.join(certdir, "key.pem"),
    )


def test_msak_client(tmp_path):
    tools = NpteTools(root_dir=str(tmp_path))
    bindir = os.path.join(str(tmp_path), ".npte", "bin")
    assert tools.msak_client(cc="bbr") == NpteMsakClient(
        binary=os.path.join(bindir, "msak-client"),
        cc="bbr",
    )


def test_msak_client_default_cc(tmp_path):
    tools = NpteTools(root_dir=str(tmp_path))
    bindir = os.path.join(str(tmp_path), ".npte", "bin")
    assert tools.msak_client() == NpteMsakClient(
        binary=os.path.join(bindir, "msak-client"),
        cc="cubic",
    )


def test_ndt7_server(tmp_path):
    tools = NpteTools(root_dir=str(tmp_path))
    bindir = os.path.join(str(tmp_path), ".npte", "bin")
    certdir = os.path.join(str(tmp_path), ".npte", "certs")
    datadir = os.path.join(str(tmp_path), ".npte", "data")
    assert tools.ndt7_server() == NpteNdt7Server(
        binary=os.path.join(bindir, "ndt-server"),
        datadir=datadir,
        cert=os.path.join(certdir, "cert.pem"),
        key=os.path.join(certdir, "key.pem"),
    )


def test_ndt7_client(tmp_path):
    tools = NpteTools(root_dir=str(tmp_path))
    bindir = os.path.join(str(tmp_path), ".npte", "bin")
    assert tools.ndt7_client() == NpteNdt7Client(
        binary=os.path.join(bindir, "ndt7-client"),
    )


# --- Build/generate methods ---


def test_generate_certs(tmp_path, monkeypatch):
    calls = []
    monkeypatch.setattr(
        "npte.tools.subprocess.run",
        lambda argv, **kw: calls.append((argv, kw)),
    )
    tools = NpteTools(root_dir=str(tmp_path))
    certdir = os.path.join(str(tmp_path), ".npte", "certs")
    tools.generate_certs()

    assert len(calls) == 1
    argv, kwargs = calls[0]
    assert argv == [
        "/usr/sbin/npte",
        "gencerts",
        "-C",
        certdir,
        "--ip-addr",
        "172.16.2.2",
    ]
    assert kwargs["check"] is True
    assert os.path.isdir(certdir)


def test_generate_certs_custom_npte(tmp_path, monkeypatch):
    calls = []
    monkeypatch.setattr(
        "npte.tools.subprocess.run",
        lambda argv, **kw: calls.append((argv, kw)),
    )
    tools = NpteTools(root_dir=str(tmp_path))
    certdir = os.path.join(str(tmp_path), ".npte", "certs")
    tools.generate_certs(npte="/opt/bin/npte")

    assert len(calls) == 1
    argv, kwargs = calls[0]
    assert argv == [
        "/opt/bin/npte",
        "gencerts",
        "-C",
        certdir,
        "--ip-addr",
        "172.16.2.2",
    ]
    assert kwargs["check"] is True


def test_build_msak(tmp_path, monkeypatch):
    calls = []
    monkeypatch.setattr(
        "npte.tools.subprocess.run",
        lambda argv, **kw: calls.append((argv, kw)),
    )
    tools = NpteTools(root_dir=str(tmp_path))
    bindir = os.path.abspath(os.path.join(str(tmp_path), ".npte", "bin"))
    tools.build_msak()

    assert len(calls) == 2
    assert calls[0][0] == [
        "go",
        "install",
        "github.com/m-lab/msak/cmd/msak-server@latest",
    ]
    assert calls[1][0] == [
        "go",
        "install",
        "github.com/m-lab/msak/cmd/msak-client@latest",
    ]
    for _, kwargs in calls:
        assert kwargs["check"] is True
        assert kwargs["env"]["GOBIN"] == bindir
    assert os.path.isdir(bindir)


def test_build_ndt7(tmp_path, monkeypatch):
    calls = []
    monkeypatch.setattr(
        "npte.tools.subprocess.run",
        lambda argv, **kw: calls.append((argv, kw)),
    )
    tools = NpteTools(root_dir=str(tmp_path))
    bindir = os.path.abspath(os.path.join(str(tmp_path), ".npte", "bin"))
    tools.build_ndt7()

    assert len(calls) == 2
    assert calls[0][0] == ["go", "install", "github.com/m-lab/ndt-server@latest"]
    assert calls[1][0] == [
        "go",
        "install",
        "github.com/m-lab/ndt7-client-go/cmd/ndt7-client@latest",
    ]
    for _, kwargs in calls:
        assert kwargs["check"] is True
        assert kwargs["env"]["GOBIN"] == bindir
    assert os.path.isdir(bindir)
