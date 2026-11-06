# sunspec-interop-otfabric

[go-sunspec](https://github.com/otfabric/go-sunspec), OT Fabric's own SunSpec
stack (on [go-modbus](https://github.com/otfabric/go-modbus)), behind the
[container contract](../../docs/CONTAINER_CONTRACT.md), so that other stacks can
test against it. Anything that uses a `sunspec-interop-<adapter>` image can use
this one in its place, within what it declares. Unlike those, it is not a
reference: its tags are go-sunspec versions, and no harness release or
`manifest.json` contains it.

**For testing only.** It runs an unauthenticated Modbus TCP device that accepts
writes. Do not expose it to a network you do not control. Old images stay
available, so check
[go-sunspec advisories](https://github.com/otfabric/go-sunspec/security/advisories)
for the version you pull.

**Not an independent implementation.** The image declares
`upstream.independentEngine: false`, and `results.json` records
`independent: false`. A result against it says nothing independent about
go-sunspec when OT Fabric produces it. A result you publish, naming the
implementation and version, is independent evidence, and we would like to hear
about it.

## Use

```sh
docker run -d --name go-sunspec -p 127.0.0.1:1502:1502 ghcr.io/otfabric/sunspec-interop-otfabric:<tag> server
until docker exec go-sunspec test -f /run/sunspec-interop/ready; do sleep 0.1; done
```

`<tag>` is a go-sunspec version such as `v0.9.0`, or `latest`, the newest one
published. A rebuild of a version replaces its tag, so pin the digest if you need
an exact build. The digest is in the summary of the
[workflow run](https://github.com/otfabric/sunspec-interop/actions/workflows/own-go-sunspec.yml),
and `docker run --rm <image> version` prints the versions.

| | |
|---|---|
| Upstream | go-sunspec, the latest release when the image was built |
| Licence | MIT; every module's licence is in the image under `/licenses` |
| Adapter name | `otfabric` (the name in documents, as the schema allows) |
| Language | Go |
| Protocol engine | OT Fabric's own: go-sunspec encodes and decodes, go-modbus carries the frames |

## Why it is apart from the other adapters

`REQUIREMENTS.md` R1: nothing under `adapters/`, `fixtures/`, `schemas/`,
`scripts/`, `tests/`, the top-level `Makefile` or `compose.yaml` knows a library
under test, and CI enforces that. This directory and
`.github/workflows/own-go-sunspec.yml` are the one exception. The scripts take
the image through `ADAPTER_IMAGE_otfabric`.

No release loop can form (`otfabric/engineering` ADR-0012). This image is built
by hand, from the latest go-sunspec release, and nothing is committed: this
repository has no pin of go-sunspec to bump. No release or required check of
go-sunspec needs the image, and go-sunspec commits no pin of it. It is not in a
harness release or `manifest.json`.

## Publishing

Run the **Own stack (go-sunspec)** workflow by hand, from `main`. It finds the
latest go-sunspec release and the latest harness release, and prints them. It
builds the image, runs the smoke test and the cross-stack self-test on both
architectures, then publishes `:<go-sunspec version>` and `:latest`. Run from a
branch, it stops before publishing. Nothing runs it automatically, so after a
go-sunspec release the image stays on the old version until someone does.

```sh
make -f own/go-sunspec/Makefile image smoke interop              # from the repository root, against the latest release
make -f own/go-sunspec/Makefile image smoke interop VERSION=dev  # against references built locally (make images)
```

## How it maps the contract

- **Device.** The fixture's models become a go-sunspec `Server` (`NewServer`,
  served by go-modbus). The fixture holds raw values and go-sunspec takes
  engineering values for a scaled point, so the adapter hands it
  raw × 10^sf, written as an exact decimal, with the scale factor go-sunspec's
  encoder would resolve; go-sunspec divides it out again. Everything about the
  register layout, the marker, the end model, writes and exceptions is
  go-sunspec's, with one change: function 4 is answered with exception 1, as
  the contract says, where go-sunspec answers exception 2 (input registers are
  "the other register type" to it).
- **Client.** `scan` is go-sunspec's `Discover` with the contract's base
  addresses (40000, 0, 50000) and `ReadModels`; `values` are the decoded raw
  values, `scaled` the scaled ones, a point not implemented is `null`.
- `print-registers` reads the served map back through the `Server`'s own
  request handler.

## What it cannot do

Not declared, because the public API of go-sunspec does not offer it and the
adapter does not encode protocol itself:

- `write` (the only client operation besides `scan`): go-sunspec's client
  reads only (its LIMITATIONS.md). `client write` is a usage error, and the
  write cases of `tests/cases.tsv` are `unsupported` with this image as the
  client. As a device it accepts writes of read-write points and refuses the
  rest, which the cases check with pysunspec2 as the client.

Where the device differs from pysunspec2's without a case failing (the `pad`
value) is in [docs/COMPATIBILITY.md](../../docs/COMPATIBILITY.md).
