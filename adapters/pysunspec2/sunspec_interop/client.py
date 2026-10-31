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


def write(args):
    mb = _device(args)
    mb.scan(connect=True)
    point = _find(mb, args.model, args.instance).points.get(args.point)
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


OPERATIONS = {"scan": scan, "write": write}
CONNECT_ERRORS = (ConnectionError, TimeoutError, OSError, modbus.ModbusClientError)
