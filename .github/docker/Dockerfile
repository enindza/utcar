# Multi-platform image used by .github/workflows/docker.yml. The workflow
# builds the static binaries into dist/ first (dist/utcar-<arch><variant>).
# For a local single-platform build see the Dockerfile in the repository root.
FROM scratch

ARG TARGETARCH
ARG TARGETVARIANT

COPY dist/ca-certificates.crt /etc/ssl/certs/
COPY dist/utcar-${TARGETARCH}${TARGETVARIANT} /utcar

# Database directory, e.g. --db /data/utcar.db (or UTCAR_DB=/data/utcar.db)
VOLUME /data

EXPOSE 12300

ENTRYPOINT ["/utcar"]
