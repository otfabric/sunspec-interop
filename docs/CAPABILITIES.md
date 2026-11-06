# Capabilities

`print-capabilities` is the authority for an image. This page explains what it means.

| Field | Meaning |
|---|---|
| `upstream` | The implementation inside: name, exact version, revision, artifact |
| `upstream.independentEngine` | `false` for OT Fabric's own stack (`own/`): results against it do not count as an independent implementation. Absent (contract 1.0 images) means `true` |
| `roles` | Which of `server` and `client` the image provides |
| `clientOperations` | Operations of `client`; an operation that is not listed is a usage error (exit 2) |
| `serverFunctions` | Function codes the device answers with something other than exception 1 |
| `baseAddresses` | What `scan` tries, in order |

| | pysunspec2 1.3.7 | otfabric (`own/go-sunspec`, not a reference) |
|---|---|---|
| Independent engine | yes | no |
| Register encoding from values | pysunspec2 (`Device.get_mb`) | go-sunspec (`NewServer`) |
| Device transport | The adapter's Modbus TCP layer: functions 3, 6, 16 | go-modbus: functions 3, 6, 16 |
| Client | pysunspec2 (`SunSpecModbusClientDeviceTCP`): scan, point read and write | go-sunspec (`Discover`, `ReadModels`): scan only |
| Base addresses scanned | 40000, 0, 50000 | 40000, 0, 50000 |
| Models | Those of the model definitions in the pysunspec2 wheel | Those of go-sunspec's registry |

Not covered:

- Models without a definition. pysunspec2 skips them in its model list, so `scan` does not report them.
- Modbus RTU, TLS and input registers.
- pysunspec2 as a Modbus server: it has none, so the transport of the device role is not evidence about pysunspec2.
- `write` in the client of `otfabric`: go-sunspec's client does not write (its LIMITATIONS.md), and the adapter does not write registers on its own, so `clientOperations` is `["scan"]` and the write cases are `unsupported` for it as a client. Its device accepts writes, so a pysunspec2 client writes it.
