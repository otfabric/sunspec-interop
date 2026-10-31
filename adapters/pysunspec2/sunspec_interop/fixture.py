# SPDX-License-Identifier: MIT
"""Fixtures: what a device serves, encoded by pysunspec2."""

import json
import os
import struct

from sunspec2 import device

MARKER = b"SunS"
END_MODEL = struct.pack(">HH", 0xFFFF, 0)
FIXTURES = "/fixtures"


class FixtureError(Exception):
    """The fixture cannot be served."""


def path_of(name):
    return os.path.join(FIXTURES, name, "fixture.json")


def names():
    if not os.path.isdir(FIXTURES):
        return []
    return sorted(n for n in os.listdir(FIXTURES) if os.path.isfile(path_of(n)))


def load(path):
    try:
        with open(path, encoding="utf-8") as f:
            fx = json.load(f)
    except (OSError, ValueError) as e:
        raise FixtureError(f"{path}: {e}") from e
    if not isinstance(fx, dict):
        raise FixtureError(f"{path}: not an object")
    for key, kind in (("name", str), ("unitId", int), ("baseAddress", int), ("models", list)):
        if not isinstance(fx.get(key), kind):
            raise FixtureError(f"{path}: {key} is missing or has the wrong type")
    if os.path.basename(os.path.dirname(os.path.abspath(path))) != fx["name"]:
        raise FixtureError(f"{path}: name {fx['name']!r} is not the name of its directory")
    if not fx["models"]:
        raise FixtureError(f"{path}: no models")
    return fx


def build(fx):
    """The pysunspec2 device of a fixture."""
    dev = device.Device()
    try:
        dev._set_dict({"name": fx["name"], "models": fx["models"]})  # pylint: disable=protected-access
    except Exception as e:  # pysunspec2 raises several types on a bad document
        raise FixtureError(f"pysunspec2 rejects the fixture: {e}") from e
    if len(dev.model_list) != len(fx["models"]):
        raise FixtureError("pysunspec2 has no definition for a model of the fixture")
    for m in dev.model_list:
        if m.error_info:
            raise FixtureError(f"model {m.model_id}: {m.error_info.strip()}")
    return dev


def image(dev):
    """The whole register map from the base address, as bytes."""
    return MARKER + dev.get_mb() + END_MODEL


def _points(group):
    yield from group.points.values()
    for g in group.groups.values():
        for instance in g if isinstance(g, list) else [g]:
            yield from _points(instance)


def writable(dev):
    """Offsets from the base address of the registers of read-write points."""
    out = set()
    start = len(MARKER) // 2
    for m in dev.model_list:
        for p in _points(m):
            if p.pdef.get("access") == "RW":
                out.update(range(start + p.offset, start + p.offset + p.len))
        start += len(m.get_mb()) // 2
    return out
