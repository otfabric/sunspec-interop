# SPDX-License-Identifier: MIT
#
# sunspec-interop adapter for cB-Sigma sunspec (client) and cB-Sigma
# sunspec-sim (device).
#
# Build from the repository root:
#   docker build -f adapters/cbsigma/Dockerfile -t sunspec-interop-cbsigma:dev .
#
# cB-Sigma sunspec and sunspec-sim are Apache-2.0; they link simonvetter/modbus
# (MIT) and goburrow/serial (MIT). The adapter is MIT and links them
# statically; the image carries their LICENSE and NOTICE files under
# /licenses. Both upstreams publish git tags only (no GitHub releases): the
# pins are the tags below, in go.mod (with their checksums in go.sum) and in
# versions.yaml.

# Where the base images come from. They are pinned by digest, so a mirror of
# Docker Hub serves exactly the same images.
ARG BASE_REGISTRY=docker.io/library
# golang:1.27.2-trixie and debian:trixie-slim, multi-arch indexes
# (linux/amd64 and linux/arm64).
ARG GOLANG_DIGEST=sha256:e58d6f83b3416618d8bcac2b3dde1b7f7e3c4a77d25e88637f8bbae81536c48d
ARG DEBIAN_DIGEST=sha256:f6b08b09a05725f9431502d3b5714f5c4576e82c1a84fa1debe0c24a7e0c3ec4

# ---------------------------------------------------------------------------
# Stage 1: build the adapter (the upstream modules linked in)
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/golang:1.27.2-trixie@${GOLANG_DIGEST} AS build

# The release pin of cB-Sigma sunspec, and the commit of its tag. A
# candidate build overrides UPSTREAM_VERSION (and SIM_VERSION) to try other
# tags without touching the pin.
ARG UPSTREAM_VERSION=v0.3.0
ARG UPSTREAM_SHA=c99cbef0ba8fefb8433663db152f9fdde8c7adb1
# The pin of cB-Sigma sunspec-sim, and the commit of its tag.
ARG SIM_VERSION=v0.3.0
ARG SIM_SHA=e407ee734647776195ce40fe6bdbffcb8ee05d6e
ARG ADAPTER_VERSION=dev

ENV CGO_ENABLED=0 GOFLAGS=-mod=mod GOTOOLCHAIN=local
# The repository's layout: the tests serve the fixtures from ../../fixtures.
WORKDIR /src/adapters/cbsigma
COPY adapters/cbsigma/go.mod adapters/cbsigma/go.sum ./

# At the pins the modules must match go.sum; anything else is a candidate.
RUN set -eu; \
    lib="$(go list -m -f '{{.Version}}' github.com/cB-Sigma/sunspec)"; \
    sim="$(go list -m -f '{{.Version}}' github.com/cB-Sigma/sunspec-sim)"; \
    if [ "${lib}" = "${UPSTREAM_VERSION}" ] && [ "${sim}" = "${SIM_VERSION}" ]; then \
        GOFLAGS=-mod=readonly go mod download && go mod verify; \
    else \
        echo "candidate build: sunspec ${UPSTREAM_VERSION}, sunspec-sim ${SIM_VERSION}" >&2; \
        go get "github.com/cB-Sigma/sunspec@${UPSTREAM_VERSION}" "github.com/cB-Sigma/sunspec-sim@${SIM_VERSION}"; \
    fi

COPY fixtures /src/fixtures
COPY adapters/cbsigma/*.go ./
RUN set -eu; \
    test -z "$(gofmt -l .)"; \
    go vet ./...; \
    go test ./...; \
    lib="$(go list -m -f '{{.Version}}' github.com/cB-Sigma/sunspec)"; \
    sim="$(go list -m -f '{{.Version}}' github.com/cB-Sigma/sunspec-sim)"; \
    test "${lib}" = "${UPSTREAM_VERSION}"; \
    test "${sim}" = "${SIM_VERSION}"; \
    go build -trimpath -ldflags "-s -w \
        -X main.adapterVersion=${ADAPTER_VERSION} \
        -X main.upstreamVersion=${lib} \
        -X main.upstreamRevision=${UPSTREAM_SHA} \
        -X main.simVersion=${sim} \
        -X main.simRevision=${SIM_SHA}" \
        -o /out/sunspec-interop .; \
    for m in sunspec:github.com/cB-Sigma/sunspec sunspec-sim:github.com/cB-Sigma/sunspec-sim \
             modbus:github.com/simonvetter/modbus serial:github.com/goburrow/serial; do \
        name="${m%%:*}"; dir="$(go list -m -f '{{.Dir}}' "${m#*:}")"; \
        mkdir -p "/out/licenses/${name}"; \
        for file in "${dir}"/LICENSE* "${dir}"/NOTICE*; do \
            if [ -f "${file}" ]; then cp "${file}" "/out/licenses/${name}/"; fi; \
        done; \
        ls "${dir}"/LICENSE* > /dev/null || { echo "no licence file in ${m#*:}" >&2; exit 1; }; \
    done; \
    mkdir -p /out/licenses/go; \
    cp "$(go env GOROOT)/LICENSE" /out/licenses/go/LICENSE

# ---------------------------------------------------------------------------
# Stage 2: runtime
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/debian:trixie-slim@${DEBIAN_DIGEST} AS runtime

ARG ADAPTER_VERSION=dev

RUN useradd --uid 10001 --no-create-home --shell /usr/sbin/nologin interop \
    && mkdir -p /run/sunspec-interop \
    && chown interop /run/sunspec-interop

COPY --from=build /out/sunspec-interop /usr/local/bin/sunspec-interop
COPY --from=build /out/licenses /licenses
COPY LICENSE /licenses/sunspec-interop/LICENSE
COPY THIRD_PARTY_NOTICES.md /licenses/THIRD_PARTY_NOTICES.md
COPY fixtures /fixtures

LABEL org.opencontainers.image.title="sunspec-interop-cbsigma" \
      org.opencontainers.image.description="SunSpec reference client (cB-Sigma sunspec) and device (cB-Sigma sunspec-sim)" \
      org.opencontainers.image.source="https://github.com/otfabric/sunspec-interop" \
      org.opencontainers.image.licenses="MIT AND Apache-2.0" \
      org.opencontainers.image.version="${ADAPTER_VERSION}"

USER interop
EXPOSE 1502

HEALTHCHECK --interval=2s --timeout=2s --start-period=10s --retries=15 \
  CMD test -f /run/sunspec-interop/ready

ENTRYPOINT ["/usr/local/bin/sunspec-interop"]
CMD ["server"]
