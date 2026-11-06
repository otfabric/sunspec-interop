# Capabilities

`print-capabilities` is the authority for an image. This page explains what it means.

| Field | Meaning |
|---|---|
| `upstream` | The implementation inside: name, exact version, revision, artifact |
| `upstream.independentEngine` | `false` for OT Fabric's own stack (`own/`): results against it do not count as an independent implementation. Absent (contract 1.0 images) means `true` |
| `roles` | Which of `server` and `client` the image provides |
| `clientOperations` | Operations of `client` (`scan`, `write`, and since contract 1.2 `write-group`); an operation that is not listed is a usage error (exit 2) |
| `serverFunctions` | Function codes the device answers with something other than exception 1 |
| `baseAddresses` | What `scan` tries, in order |

| | pysunspec2 1.3.7 | cbsigma (cB-Sigma sunspec v0.3.0, sunspec-sim v0.3.0) | otfabric (`own/go-sunspec`, not a reference) |
|---|---|---|---|
| Independent engine | yes | yes | no |
| Register encoding from values | pysunspec2 (`Device.get_mb`) | sunspec-sim (`Image.Add`, which is cB-Sigma sunspec's `device.Build`) | go-sunspec (`NewServer`) |
| Device transport | The adapter's Modbus TCP layer: functions 3, 6, 16 | sunspec-sim (`StartOptions`, on simonvetter/modbus): functions 3, 6, 16; exception 2 outside the map and on registers that are not writable, 11 for another unit | go-modbus: functions 3, 6, 16 |
| Client | pysunspec2 (`SunSpecModbusClientDeviceTCP`): scan, point write, group point write | cB-Sigma sunspec (`FindAt`, `ReadModel`, `Layout`, `Decode`, `Write`, on its `mbtcp` reader): scan, point write, group point write | go-sunspec (`Discover`, `ReadModels`, `WritePoint`): scan, point write |
| Base addresses scanned | 40000, 0, 50000 | 40000, 0, 50000 (`FindAt`; cB-Sigma's own `Find` order is 40000, 50000, 0) | 40000, 0, 50000 |
| Models | Those of the model definitions in the pysunspec2 wheel | The 112 models cB-Sigma sunspec embeds | Those of go-sunspec's registry |

The cbsigma client reads raw integers with `sunspec.Raw` (a scale factor as an `int16`, the type SunSpec gives `sunssf`), takes floats, strings and addresses from `Decode`, and the scaled value from `Decode`. It writes raw values with a `sunspec.Write` (`Encode`), which refuses a read-only point before anything is sent, as go-sunspec does: `device.write.read-only-point` and the other refused writes show the device's refusal only with pysunspec2 as the client.

Not covered:

- Models without a definition. pysunspec2 skips them in its model list, so `scan` does not report them.
- Modbus RTU, TLS and input registers.
- pysunspec2 as a Modbus server: it has none, so the transport of the device role is not evidence about pysunspec2.
- A write of a read-only point by the clients of `otfabric` and `cbsigma`: both refuse it before sending anything, so the refused-write cases show the device's refusal only with pysunspec2 as the client.
- A write of a point in a group by the client of `otfabric`: go-sunspec's `WritePoint` writes top-level points only, so `otfabric` does not declare `write-group` and those cases are `unsupported`.
- Devices that act on their settings: sunspec-sim can follow its curves (`sim.Follow`), but the cbsigma device serves the fixture as it is, like the other devices.
