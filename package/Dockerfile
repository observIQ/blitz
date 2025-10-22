# Build stage: get CA certs and passwd from a minimal image
FROM alpine:3.20 AS basefs
RUN apk add --no-cache ca-certificates
RUN adduser -D -u 10001 appuser

# Final stage: scratch with just our binary and required files
FROM scratch

ARG PROJECT_NAME

LABEL org.opencontainers.image.source="https://github.com/observiq/${PROJECT_NAME}"

COPY --from=basefs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=basefs /etc/passwd /etc/passwd

COPY bindplane-loader /bindplane-loader

USER 10001
ENTRYPOINT ["/bindplane-loader"]

