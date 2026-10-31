# SPDX-License-Identifier: MIT
# sunspec-interop adapter for pysunspec2.
#
# Build from the repository root:
#   docker build -f adapters/pysunspec2/Dockerfile -t sunspec-interop-pysunspec2:dev .
#
# pysunspec2 is Apache-2.0 and pure Python; its one dependency, pyserial, is
# BSD-3-Clause. The adapter is MIT like the rest of the repository. Pins are
# mirrored in versions.yaml.

# Where the base image comes from. It is pinned by digest, so a mirror of
# Docker Hub serves exactly the same image; CI uses one to stay clear of
# Docker Hub's pull rate limit (see the Makefile).
ARG BASE_REGISTRY=docker.io/library
# python:3.14.8-slim-trixie, multi-arch index (linux/amd64 and linux/arm64).
ARG PYTHON_DIGEST=sha256:a2b82f3c48559aa0a8446d9af49826b6e2b2016f4cd2afabfe6013ec53729170

# ---------------------------------------------------------------------------
# Stage 1: pysunspec2 from PyPI, the adapter, byte-compiled
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/python:3.14.8-slim-trixie@${PYTHON_DIGEST} AS build

# The upstream release. requirements.txt pins the same version with the hash
# of its wheel, and pyserial. A candidate build overrides UPSTREAM_VERSION: the hash of
# another release cannot be known here, so it is installed without hash
# checking, and the hash of what was downloaded is what print-capabilities
# reports as the revision.
ARG UPSTREAM_VERSION=1.3.7
ARG ADAPTER_VERSION=dev

ENV PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_ROOT_USER_ACTION=ignore

COPY adapters/pysunspec2/requirements.txt /src/requirements.txt
# The pyserial wheel has no licence file: this is LICENSE.txt of its sdist.
COPY adapters/pysunspec2/licenses/pyserial-LICENSE.txt /src/pyserial-LICENSE.txt

WORKDIR /wheels
RUN set -eu; \
    pinned="$(sed -nE 's/^pysunspec2==([^ \\]+).*/\1/p' /src/requirements.txt)"; \
    test -n "${pinned}"; \
    if [ "${UPSTREAM_VERSION}" = "${pinned}" ]; then \
        pip download --no-deps --only-binary :all: --require-hashes -r /src/requirements.txt -d /wheels; \
    else \
        echo "candidate build: pysunspec2 ${UPSTREAM_VERSION} instead of the pin ${pinned}, no hash to check" >&2; \
        grep -A1 '^pyserial' /src/requirements.txt > /src/pyserial.txt; \
        pip download --no-deps --only-binary :all: --require-hashes -r /src/pyserial.txt -d /wheels; \
        pip download --no-deps --only-binary :all: "pysunspec2==${UPSTREAM_VERSION}" -d /wheels; \
    fi; \
    test "$(ls /wheels | wc -l)" = 2; \
    wheel="$(ls /wheels/pysunspec2-*.whl)"; \
    sha="$(sha256sum "${wheel}" | cut -d' ' -f1)"; \
    pip install --no-index --no-deps --no-compile --target /opt/sunspec-interop/site-packages /wheels/*.whl; \
    installed="$(ls -d /opt/sunspec-interop/site-packages/pysunspec2-*.dist-info | sed -E 's/.*pysunspec2-(.*)\.dist-info/\1/')"; \
    test "${installed}" = "${UPSTREAM_VERSION}"; \
    mkdir -p /licenses/pysunspec2 /licenses/pyserial /licenses/python /licenses/sunspec-interop; \
    cp /opt/sunspec-interop/site-packages/pysunspec2-*.dist-info/LICENSE* /licenses/pysunspec2/; \
    cp /src/pyserial-LICENSE.txt /licenses/pyserial/LICENSE.txt; \
    cp /usr/local/lib/python3.14/LICENSE.txt /licenses/python/LICENSE.txt; \
    printf '{"adapterVersion":"%s","upstreamVersion":"%s","upstreamRevision":"sha256:%s","upstreamArtifact":"%s"}\n' \
        "${ADAPTER_VERSION}" "${installed}" "${sha}" "$(basename "${wheel}")" > /opt/sunspec-interop/build-info.json

COPY adapters/pysunspec2/sunspec_interop /opt/sunspec-interop/sunspec_interop
COPY adapters/pysunspec2/sunspec-interop /opt/sunspec-interop/bin/sunspec-interop
COPY LICENSE /licenses/sunspec-interop/LICENSE

# The runtime user cannot write byte code next to the sources: compile now.
RUN python3 -m compileall -q /opt/sunspec-interop \
    && chmod 0755 /opt/sunspec-interop/bin/sunspec-interop

# ---------------------------------------------------------------------------
# Stage 2: runtime
# ---------------------------------------------------------------------------
FROM ${BASE_REGISTRY}/python:3.14.8-slim-trixie@${PYTHON_DIGEST} AS runtime

ARG ADAPTER_VERSION=dev

RUN useradd --uid 10001 --no-create-home --shell /usr/sbin/nologin interop \
    && mkdir -p /run/sunspec-interop \
    && chown interop /run/sunspec-interop

COPY --from=build /opt/sunspec-interop /opt/sunspec-interop
COPY --from=build /opt/sunspec-interop/bin/sunspec-interop /usr/local/bin/sunspec-interop
COPY --from=build /licenses /licenses
COPY fixtures /fixtures

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

LABEL org.opencontainers.image.title="sunspec-interop-pysunspec2" \
      org.opencontainers.image.description="SunSpec reference client and device built on pysunspec2" \
      org.opencontainers.image.source="https://github.com/otfabric/sunspec-interop" \
      org.opencontainers.image.licenses="MIT AND Apache-2.0 AND BSD-3-Clause" \
      org.opencontainers.image.version="${ADAPTER_VERSION}"

USER interop
EXPOSE 1502

HEALTHCHECK --interval=2s --timeout=2s --start-period=10s --retries=15 \
  CMD test -f /run/sunspec-interop/ready

ENTRYPOINT ["/usr/local/bin/sunspec-interop"]
CMD ["server"]
