#!/usr/bin/env bash
# Shared helpers for the self-test scripts. Source, do not execute.
#
# Servers and clients run as containers on a private Docker network and reach
# each other by container name (scripts/selftest.py), so no host port is
# published and parallel runs do not collide.

REGISTRY="${REGISTRY:-ghcr.io/otfabric}"
VERSION="${VERSION:-dev}"
export REGISTRY VERSION

log() { echo "[$(basename "$0" .sh)] $*" >&2; }

# image_of <adapter>: ADAPTER_IMAGE_<adapter> names an image outside the release
# set, which this repository's scripts know nothing about.
image_of() {
    local override="ADAPTER_IMAGE_$1"
    if [[ -n "${!override:-}" ]]; then echo "${!override}"; else echo "${REGISTRY}/sunspec-interop-$1:${VERSION}"; fi
}

require_tools() {
    local t
    for t in docker python3; do
        command -v "$t" >/dev/null || { log "missing required tool: $t"; exit 2; }
    done
}
