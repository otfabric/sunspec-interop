# Fixtures

A fixture is `fixtures/<name>/fixture.json` ([schema](../schemas/fixture.schema.json)). `name` equals the directory.

| Field | Meaning |
|-------|---------|
| `name`, `description` | Identity |
| `unitId` | The Modbus unit the device answers |
| `baseAddress` | 40000, 0 or 50000 |
| `models` | Models in order, in pysunspec2's device JSON: `ID`, then raw point values by name, repeating groups as lists |

Values are raw register values, before scale factors. A point that is left out takes its not-implemented value. The adapter hands the document to pysunspec2, adds the `SunS` marker and the end model, and serves the result.

| Fixture | Base | Unit | Models | What it is for |
|---------|------|------|--------|----------------|
| `baseline` | 40000 | 1 | 1, 103, 120, 123, 160 | Inverter with nameplate, writable controls and a repeating group |
| `edge` | 0 | 7 | 1, 101, 701 | Full-width strings, not-implemented values, scale factors from -3 to 3, 64-bit accumulators |
| `altbase` | 50000 | 1 | 1, 111 | 32-bit floats |

Model 123 of `baseline` holds the read-write points the self-test writes.
