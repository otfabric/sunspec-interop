# Compatibility

No known differences. The self-test of every release runs every client against every device, pysunspec2 1.3.7 and cB-Sigma (sunspec v0.3.0, sunspec-sim v0.3.0), on amd64 and arm64, and all cases pass; `results.json` of a run is its evidence. Only a consumer's own suite shows anything about that consumer.

## pysunspec2 and cB-Sigma

The two references are independent of each other, and each reads what the other serves on every fixture: model discovery at bases 40000, 0 and 50000, raw values of every type, 64-bit accumulators, not-implemented values, scale factors from -3 to 3 (nested groups taking theirs from the model's top level), groups that occur once, and model 705's curves (repeating groups nested in repeating groups, counted by top-level points). Each writes DER settings, a signed setpoint, a point of a group that occurs once, a curve point and a curve setting on the other's device, and the other reads the value back.

Where their devices differ without a case failing:

- A `pad` point: sunspec-sim encodes 0x8000, SunSpec's not-implemented value for `pad`; pysunspec2 encodes 0. `print-registers` differs in those registers only (the last register of model 1, and that of model 120 in `baseline`). go-sunspec encodes 0x8000 too.

Where their clients differ:

- A write of a read-only point: cB-Sigma's `Write.Encode` refuses it before sending anything, pysunspec2 sends it and the device refuses it (exception 2). The refused-write cases pass for both: the device keeps its value.

## OT Fabric's own stack

`otfabric` (go-sunspec, `own/go-sunspec/`) is run by hand against the references of the latest release, and is not evidence about go-sunspec ([own/go-sunspec/README.md](../own/go-sunspec/README.md)). Its registers equal sunspec-sim's on every fixture; it differs from pysunspec2's in the `pad` registers above. Its client does not declare `write-group` (go-sunspec's `WritePoint` writes top-level points), so those cases are `unsupported` for it.

## Upstream observations

- pysunspec2 1.3.7 has no Modbus server, and the wheel of its dependency pyserial 3.5 carries no licence file.
- pysunspec2 1.3.7: a point of a group has no `write()` of its own (only top-level points are client points); the adapter writes it through its group (`Group.write`).
- cB-Sigma sunspec and sunspec-sim are tagged on GitHub without releases; the pins are tags and their commits.
- cB-Sigma's `Find` tries 40000, 50000, 0 and then neighbouring addresses; the adapter calls `FindAt` with the contract's order.
- `scan` tries base addresses 40000, 0 and 50000 in that order.
