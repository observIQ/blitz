# Build stage: get CA certs and passwd from a minimal image
FROM alpine:3.22 AS basefs
RUN apk add --no-cache ca-certificates
RUN adduser -D -u 10001 appuser

# Final stage: scratch with just our binary and required files
FROM scratch

ARG PROJECT_NAME
ARG VERSION
ARG COMMIT_SHA

LABEL org.opencontainers.image.source="https://github.com/observiq/${PROJECT_NAME}"
LABEL org.opencontainers.image.description="Blitz - a load generation tool for Bindplane managed collectors"
LABEL org.opencontainers.image.vendor="Bindplane"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.title="Blitz"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${COMMIT_SHA}"

COPY --from=basefs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=basefs /etc/passwd /etc/passwd

COPY blitz /blitz

USER 10001
ENTRYPOINT ["/blitz"]

