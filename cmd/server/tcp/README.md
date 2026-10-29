# Test TLS Certificates

**WARNING: These certificates are for TESTING ONLY and should NOT be used in production.**

The `-tls` flag loads `test_server.key` and `test_server.crt` from the current directory. These self-signed test certificates are not committed to the repository, so generate them before running the server with TLS.

## Certificate Details

- **Valid for**: localhost, 127.0.0.1, ::1 (IPv6 localhost)
- **Valid period**: 365 days from generation date
- **Key size**: 2048 bits

## Generating Certificates

To generate the test certificates, run:

```bash
./generate_test_certs.sh
```

The script writes both the private key and certificate files next to itself. Rerun it to replace expired certificates.

The automated TLS tests in `output/tcp` do not use these files. They generate a short-lived certificate in memory on every run.
