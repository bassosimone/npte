"""Helpers for checking CPU energy-performance preferences."""

# SPDX-License-Identifier: GPL-3.0-or-later

import glob


def npte_energy_performance_preferences() -> set[str]:
    """Return the set of unique energy_performance_preference values across all CPUs."""
    values: set[str] = set()
    for path in sorted(
        glob.glob("/sys/devices/system/cpu/cpu*/cpufreq/energy_performance_preference")
    ):
        with open(path) as filep:
            values.add(filep.read().strip())
    return values
