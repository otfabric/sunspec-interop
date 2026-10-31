# Container contract

Version: **1.0** (`contract_version` in [versions.yaml](../versions.yaml))

Consumers depend on this document, not on the adapter's source. A change that breaks it bumps the major version; an addition bumps the minor one. Documents carry `schemaVersion` `"1.0"`, which changes only with the major version.

## Image layout

| Item | Value |
|------|-------|
| Image | `ghcr.io/otfabric/sunspec-interop-<adapter>` |
| Entrypoint | `sunspec-interop` |
| Default command | `server` |
| Protocol | SunSpec over Modbus TCP |
| Port | 1502/tcp |
| User | non-root, uid 10001 |
| Fixtures | `/fixtures/<name>/fixture.json` (baked in; mount over `/fixtures` to replace) |
| Readiness file | `/run/sunspec-interop/ready` |
| Licences | `/licenses/` |

## Commands

```text
sunspec-interop server [--fixture PATH] [--bind-address ADDR] [--port N] [--ready-file PATH]
sunspec-interop client scan  --host HOST [--port N] [--unit-id N] [--timeout SECONDS]
sunspec-interop client write --host HOST [--port N] [--unit-id N] [--timeout SECONDS]
                             --model ID [--instance N] --point NAME --value VALUE
sunspec-interop print-capabilities
sunspec-interop print-fixture [NAME]
sunspec-interop print-registers [NAME]
sunspec-interop version
```

An unknown flag is a usage error. Numbers are decimal, or hexadecimal with a `0x` prefix.

## `server`

Serves one fixture until SIGTERM or SIGINT. The readiness file appears once the port listens. An invalid fixture ends the process with exit 4 before anything listens.

The device answers functions 3 (read holding registers), 6 and 16 (write). Other functions, including 4, get exception 1. An address outside the map gets exception 2, and so does a write that touches a register of a point the model does not define as read-write. Another unit ID gets exception 11. The map is held in memory: a restart restores the fixture.

## `client`

Prints one document on standard output ([schema](../schemas/client-result.schema.json)), whatever the outcome:

| Field | Meaning |
|-------|---------|
| `schemaVersion`, `adapter`, `operation`, `host`, `port`, `unitId` | The request |
| `ok` | Whether the operation succeeded |
| `result` | Present when `ok` is true |
| `error` | `kind` (`connect`, `failed`) and `message`, present when `ok` is false |

`scan` finds the base address among 40000, 0 and 50000, in that order, and reads every model. `result` holds `baseAddress` and `models`, one entry per model: `id`, `name`, `address`, `length`, `values` (raw) and `scaled`. Both are the point names of the model, with repeating groups as lists of objects. A not-implemented point is `null`. `error` appears in a model pysunspec2 flagged.

`write` scans, writes one point in raw form and reads it back. `result` holds `model`, `point` and the `value` read back.

## `print-capabilities`

The capability document ([schema](../schemas/capabilities.schema.json)).

## `print-fixture`, `print-registers`

The fixture as baked in, and the whole register map pysunspec2 encodes from it, from the base address through the end model: `fixture`, `unitId`, `baseAddress`, `registers` (numbers).

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | The operation failed: a device refused it, or the model or point does not exist |
| 2 | Usage error |
| 3 | No SunSpec device reached: connection, timeout or no marker at a known base address |
| 4 | Invalid fixture |

## Output rules

Documents go to standard output as JSON, logs and usage errors to standard error.
