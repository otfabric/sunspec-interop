# Compatibility

No known differences. The self-test of every release passes all cases against pysunspec2 1.3.7 on amd64 and arm64; `results.json` of a run is its evidence. Only a consumer's own suite shows anything about that consumer.

## OT Fabric's own stack

`otfabric` (go-sunspec, `own/go-sunspec/`) is run by hand against the references of the latest release, and is not evidence about go-sunspec ([own/go-sunspec/README.md](../own/go-sunspec/README.md)). Where its device differs from pysunspec2's without a case failing:

- A `pad` point: go-sunspec encodes 0x8000, SunSpec's not-implemented value for `pad`; pysunspec2 encodes 0. `print-registers` differs in those registers only (the last register of model 1, and that of model 120 in `baseline`).

## Upstream observations

- pysunspec2 1.3.7 has no Modbus server, and the wheel of its dependency pyserial 3.5 carries no licence file.
- `scan` tries base addresses 40000, 0 and 50000 in that order.
