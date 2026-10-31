# Contributing to sunspec-interop

## Ground rules

- **Nothing here may know about a library under test.** No imports, no build steps, no expectations and no names from any consumer.
- **The contract is the product.** A change to [docs/CONTAINER_CONTRACT.md](docs/CONTAINER_CONTRACT.md) or [docs/FIXTURES.md](docs/FIXTURES.md) lands with the adapter change.
- **Declare only what is tested.** A capability is listed when the self-test exercises it.
- **Pin everything.** Upstream by exact version and wheel hash, base images by digest, all mirrored in [versions.yaml](versions.yaml).

## Setup

Docker (with buildx), GNU make and Python 3.

```sh
make ci     # validate, lint, build, self-test, document schemas
```

## Moving the upstream pin

1. Change `ARG UPSTREAM_VERSION` in the Dockerfile, `adapters/pysunspec2/requirements.txt` and `versions.yaml`; `scripts/check-versions.sh` must pass.
2. `make ci`.
3. Record the new version in [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md), [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) and [RELEASE.md](RELEASE.md).

The **Upstream** workflow checks the pin and the base image every day and self-tests a newer release as a candidate (`make candidate-pysunspec2 CANDIDATE_VERSION=<version>`).
