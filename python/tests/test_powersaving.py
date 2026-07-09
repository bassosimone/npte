# SPDX-License-Identifier: GPL-3.0-or-later

import glob
import os

from npte import npte_energy_performance_preferences


def _make_cpu(base: str, cpu_id: int, value: str) -> None:
    d = os.path.join(base, f"cpu{cpu_id}", "cpufreq")
    os.makedirs(d)
    with open(os.path.join(d, "energy_performance_preference"), "w") as filep:
        filep.write(value + "\n")


def test_no_cpus(monkeypatch):
    monkeypatch.setattr("npte.powersaving.glob.glob", lambda _: [])
    assert npte_energy_performance_preferences() == set()


def test_all_same(tmp_path, monkeypatch):
    base = str(tmp_path / "sys" / "devices" / "system" / "cpu")
    _make_cpu(base, 0, "performance")
    _make_cpu(base, 1, "performance")
    pattern = os.path.join(base, "cpu*", "cpufreq", "energy_performance_preference")
    real_glob = glob.glob
    monkeypatch.setattr(
        "npte.powersaving.glob.glob",
        lambda _: sorted(real_glob(pattern)),
    )
    assert npte_energy_performance_preferences() == {"performance"}


def test_mixed_values(tmp_path, monkeypatch):
    base = str(tmp_path / "sys" / "devices" / "system" / "cpu")
    _make_cpu(base, 0, "performance")
    _make_cpu(base, 1, "balance_performance")
    _make_cpu(base, 2, "power")
    pattern = os.path.join(base, "cpu*", "cpufreq", "energy_performance_preference")
    real_glob = glob.glob
    monkeypatch.setattr(
        "npte.powersaving.glob.glob",
        lambda _: sorted(real_glob(pattern)),
    )
    assert npte_energy_performance_preferences() == {
        "performance",
        "balance_performance",
        "power",
    }
