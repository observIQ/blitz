# Test TLS Certificates

**WARNING: These certificates are for TESTING ONLY and should NOT be used in production.**

The files `test_server.key` and `test_server.crt` are self-signed test certificates generated for local development and testing purposes only.

## Certificate Details

- **Valid for**: localhost, 127.0.0.1, ::1 (IPv6 localhost)
- **Valid period**: 365 days from generation date
- **Key size**: 2048 bits

## Regenerating Certificates

To regenerate the test certificates, run:

```bash
./generate_test_certs.sh
```

The script will regenerate both the private key and certificate files.

