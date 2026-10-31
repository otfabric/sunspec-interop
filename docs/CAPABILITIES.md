# Capabilities

`print-capabilities` is the authority for an image. This page explains what it means.

| | pysunspec2 1.3.7 |
|---|---|
| Register encoding from values | pysunspec2 (`Device.get_mb`) |
| Device transport | The adapter's Modbus TCP layer: functions 3, 6, 16 |
| Client | pysunspec2 (`SunSpecModbusClientDeviceTCP`): scan, point read and write |
| Base addresses scanned | 40000, 0, 50000 |
| Models | Those of the model definitions in the pysunspec2 wheel |

Not covered:

- Models without a definition. pysunspec2 skips them in its model list, so `scan` does not report them.
- Modbus RTU, TLS and input registers.
- pysunspec2 as a Modbus server: it has none, so the transport of the device role is not evidence about pysunspec2.
