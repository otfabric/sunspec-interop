# Compatibility

No known differences. The self-test of every release passes all cases against pysunspec2 1.3.7 on amd64 and arm64; `results.json` of a run is its evidence. Only a consumer's own suite shows anything about that consumer.

## Upstream observations

- pysunspec2 1.3.7 has no Modbus server, and the wheel of its dependency pyserial 3.5 carries no licence file.
- `scan` tries base addresses 40000, 0 and 50000 in that order.
