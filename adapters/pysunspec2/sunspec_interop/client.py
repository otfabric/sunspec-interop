# SPDX-License-Identifier: MIT
"""The contract's client operations on pysunspec2's Modbus TCP client."""

from sunspec2.modbus import client as sunspec
from sunspec2.modbus import modbus

META = ("mid", "error", "model_id")


def _document(mb, base):
    models = []
    for m in mb.model_list:
        values = m.get_dict(computed=False)
        scaled = m.get_dict(computed=True)
        entry = {
            "id": m.model_id,
            "name": m.gname,
            "address": m.model_addr,
            "length": m.model_len,
            "values": {k: v for k, v in values.items() if k not in META},
            "scaled": {k: v for k, v in scaled.items() if k not in META},
        }
        if m.error_info:
            entry["error"] = m.error_info.strip()
        models.append(entry)
    return {"baseAddress": base, "models": models}


def _device(args):
    return sunspec.SunSpecModbusClientDeviceTCP(
        slave_id=args.unit_id, ipaddr=args.host, ipport=args.port, timeout=args.timeout
    )


def _find(mb, model_id, instance):
    found = [m for m in mb.model_list if m.model_id == model_id]
    if instance >= len(found):
        raise LookupError(f"no instance {instance} of model {model_id}")
    return found[instance]


def scan(args):
    mb = _device(args)
    mb.scan(connect=True)
    return _document(mb, mb.base_addr)


def _resolve(group, path):
    """The group and the point at a path of the contract: "PFWInj.PF" (a group
    that occurs once), "Crv[1].Pt[0].V" (occurrences from 0)."""
    parts = path.split(".")
    for part in parts[:-1]:
        name, _, index = part.partition("[")
        child = group.groups.get(name)
        if child is None:
            raise LookupError(f"no group {name} in {path}")
        if index:
            if not isinstance(child, list):
                raise LookupError(f"{name} occurs once, it takes no index in {path}")
            try:
                child = child[int(index.rstrip("]"))]
            except (ValueError, IndexError) as e:
                raise LookupError(f"no occurrence {index.rstrip(']')} of {name} in {path}") from e
        elif isinstance(child, list):
            raise LookupError(f"{name} repeats, it needs an index in {path}")
        group = child
    point = group.points.get(parts[-1])
    if point is None:
        raise LookupError(f"no point {path}")
    return group, point


def write(args):
    mb = _device(args)
    mb.scan(connect=True)
    model = _find(mb, args.model, args.instance)
    if args.operation == "write-group":
        # A point of a group is no client point of its own: the group writes
        # its changed points (Group.write) and reads itself back.
        if "." not in args.point:
            raise LookupError(f"{args.point} is not the path of a group's point")
        group, point = _resolve(model, args.point)
        point.value = args.value
        mb.connect()
        try:
            group.write()
            group.read()
        finally:
            mb.disconnect()
        return {"model": args.model, "point": args.point, "value": point.value}
    point = model.points.get(args.point)
    if point is None:
        raise LookupError(f"model {args.model} has no point {args.point}")
    point.value = args.value
    mb.connect()
    try:
        point.write()
        point.read()
    finally:
        mb.disconnect()
    return {"model": args.model, "point": args.point, "value": point.value}


OPERATIONS = {"scan": scan, "write": write, "write-group": write}
CONNECT_ERRORS = (ConnectionError, TimeoutError, OSError, modbus.ModbusClientError)
