# SPDX-License-Identifier: MIT
"""A SunSpec device: pysunspec2 encodes the registers, this file serves them.

pysunspec2 has no Modbus server. The transport here is the smallest one that
carries the register map: Modbus TCP with functions 3, 6 and 16. Function 4 is
answered with an exception, as a device that keeps its map in holding registers
does.
"""

import asyncio
import os
import signal
import struct

from . import fixture

MAX_READ = 125
MAX_WRITE = 123
ILLEGAL_FUNCTION = 1
ILLEGAL_ADDRESS = 2
ILLEGAL_VALUE = 3
GATEWAY_NO_TARGET = 0x0B


class Device:
    def __init__(self, fx):
        dev = fixture.build(fx)
        self.unit_id = fx["unitId"]
        self.base = fx["baseAddress"]
        self.data = bytearray(fixture.image(dev))
        self.writable = fixture.writable(dev)

    def _span(self, addr, count):
        off = addr - self.base
        if off < 0 or (off + count) * 2 > len(self.data):
            return None
        return off

    def read(self, addr, count):
        if not 1 <= count <= MAX_READ:
            return None, ILLEGAL_VALUE
        off = self._span(addr, count)
        if off is None:
            return None, ILLEGAL_ADDRESS
        return bytes(self.data[off * 2:(off + count) * 2]), 0

    def write(self, addr, payload):
        count = len(payload) // 2
        off = self._span(addr, count)
        if off is None or not self.writable.issuperset(range(off, off + count)):
            return ILLEGAL_ADDRESS
        self.data[off * 2:(off + count) * 2] = payload
        return 0

    def handle(self, pdu):
        """The response PDU of a request PDU."""
        func = pdu[0]

        def exception(code):
            return bytes((func | 0x80, code))

        if func == 3:
            if len(pdu) != 5:
                return exception(ILLEGAL_VALUE)
            addr, count = struct.unpack(">HH", pdu[1:5])
            data, err = self.read(addr, count)
            return exception(err) if err else bytes((func, len(data))) + data
        if func == 6:
            if len(pdu) != 5:
                return exception(ILLEGAL_VALUE)
            err = self.write(struct.unpack(">H", pdu[1:3])[0], pdu[3:5])
            return exception(err) if err else pdu
        if func == 16:
            if len(pdu) < 6:
                return exception(ILLEGAL_VALUE)
            addr, count, size = struct.unpack(">HHB", pdu[1:6])
            if not 1 <= count <= MAX_WRITE or size != count * 2 or len(pdu) != 6 + size:
                return exception(ILLEGAL_VALUE)
            err = self.write(addr, pdu[6:])
            return exception(err) if err else pdu[:5]
        return exception(ILLEGAL_FUNCTION)


async def serve_connection(device, reader, writer):
    try:
        while True:
            header = await reader.readexactly(7)
            tid, proto, length, unit = struct.unpack(">HHHB", header)
            if length < 2:
                break
            pdu = await reader.readexactly(length - 1)
            if proto != 0:
                break
            if unit != device.unit_id:
                answer = bytes((pdu[0] | 0x80, GATEWAY_NO_TARGET))
            else:
                answer = device.handle(pdu)
            writer.write(struct.pack(">HHHB", tid, 0, len(answer) + 1, unit) + answer)
            await writer.drain()
    except (asyncio.IncompleteReadError, ConnectionError):
        pass
    finally:
        writer.close()


async def run(device, address, port, ready_file):
    stop = asyncio.Event()
    loop = asyncio.get_running_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, stop.set)

    server = await asyncio.start_server(lambda r, w: serve_connection(device, r, w), address, port)
    with open(ready_file, "w", encoding="ascii") as f:
        f.write("ready\n")
    print(f"serving {len(device.data) // 2} registers at {device.base}, unit {device.unit_id}, on {address}:{port}", flush=True)
    try:
        await stop.wait()
    finally:
        server.close()
        try:
            os.unlink(ready_file)
        except OSError:
            pass
