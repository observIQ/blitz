#!/bin/bash
# This script generates test TLS certificates for use in local testing only.
# These certificates are NOT suitable for production use.
# 
# The certificates are valid for:
# - localhost
# - 127.0.0.1
# - ::1 (IPv6 localhost)

set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KEY_FILE="${DIR}/test_server.key"
CERT_FILE="${DIR}/test_server.crt"

# Generate private key
openssl genrsa -out "${KEY_FILE}" 2048

# Generate self-signed certificate with SANs
openssl req -new -x509 -key "${KEY_FILE}" -out "${CERT_FILE}" -days 365 \
  -subj "/CN=localhost/O=Blitz Test/C=US" \
  -addext "subjectAltName=DNS:localhost,DNS:*.localhost,IP:127.0.0.1,IP:::1"

echo "Test certificates generated successfully:"
echo "  Private key: ${KEY_FILE}"
echo "  Certificate: ${CERT_FILE}"
echo ""
echo "WARNING: These certificates are for TESTING ONLY and should NOT be used in production."

