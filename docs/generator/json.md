# JSON Generator

The JSON generator creates structured JSON log entries with configurable fields. Two log types are supported: default logs with standard fields, and PII logs with personally identifiable information fields suitable for testing PII detection and redaction systems.

## Example Logs

### Default Log Type

```json
{
  "timestamp": "2024-01-15T10:30:45Z",
  "level": "INFO",
  "environment": "production",
  "location": "us-east1",
  "message": "User authentication failed for user_id=12345, ip_address=192.168.1.100, reason=invalid_password, attempt_count=3, timestamp=2024-01-15T10:30:45Z, session_id=abc123def456, user_agent=Mozilla/5.0, location=us-east-1, service=auth-service"
}
```

### PII Log Type

The PII log type generates logs containing 37 different sensitive data types for comprehensive testing:

```json
{
  "timestamp": "2024-01-15T10:30:45Z",
  "level": "INFO",
  "message": "Customer service request completed",
  "action": "processed transaction",
  "status": "successful",

  "user_id": "a1b2c3d4e5f67890-1234567890abcdef",
  "ssn": "123-45-6789",
  "iban": "US12000100001234567890",
  "phone": "+1-555-123-4567",
  "intl_phone": "+44-555-123-4567",
  "email": "john.smith42@gmail.com",
  "credit_card": "4532 1234 5678 9012",
  "dob": "03/15/1985",
  "ipv4": "192.168.1.100",
  "ipv6": "2001:db8:85a3:0:0:8a2e:370:7334",
  "mac_address": "00:1A:2B:3C:4D:5E",
  "street_addr": "123 Main St",
  "city_state": "New York, NY",
  "zip_code": "10001-1234",

  "passport": "A12345678",
  "drivers_license": "CA1234567",
  "national_id": "AB123456C",

  "bank_account": "12345678901234",
  "routing_number": "021000021",
  "crypto_wallet": "0x1234567890abcdef1234567890abcdef12345678",

  "medical_record": "MRN-12345678",
  "health_insurance": "BCBS123456789",

  "vin": "1HGBH41JXMN109186",
  "license_plate": "ABC-1234",

  "employee_id": "EMP123456",
  "student_id": "STU123456789",

  "username": "happy_coder42",
  "password_hash": "$2a$10$N9qo8uLOickgx2ZMRZoMy...",
  "api_key": "api_EXAMPLE_key_1234567890abcdef",
  "aws_access_key": "AKIAIOSFODNN7EXAMPLE",
  "private_key": "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...",
  "jwt_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIx...",

  "gps_coords": "40.712776,-74.005974",
  "geohash": "dr5regw3p",

  "full_name": "John Smith",
  "mothers_maiden": "Johnson",
  "security_answer": "Fluffy"
}
```

## PII Data Types (37 Total)

| Category | Field | Example |
|----------|-------|---------|
| **Core PII** | | |
| UUID/GUID | `user_id` | `a1b2c3d4e5f67890-1234567890abcdef` |
| SSN | `ssn` | `123-45-6789` |
| IBAN | `iban` | `US12000100001234567890` |
| US Phone | `phone` | `+1-555-123-4567` |
| International Phone | `intl_phone` | `+44-555-123-4567` |
| Email | `email` | `john.smith42@gmail.com` |
| Credit Card | `credit_card` | `4532 1234 5678 9012` |
| Date of Birth | `dob` | `03/15/1985` |
| IPv4 Address | `ipv4` | `192.168.1.100` |
| IPv6 Address | `ipv6` | `2001:db8:85a3:0:0:8a2e:370:7334` |
| MAC Address | `mac_address` | `00:1A:2B:3C:4D:5E` |
| Street Address | `street_addr` | `123 Main St` |
| City, State | `city_state` | `New York, NY` |
| Zip Code | `zip_code` | `10001-1234` |
| **Government IDs** | | |
| Passport | `passport` | `A12345678` |
| Driver's License | `drivers_license` | `CA1234567` |
| National ID | `national_id` | `AB123456C` |
| **Financial** | | |
| Bank Account | `bank_account` | `12345678901234` |
| Routing Number | `routing_number` | `021000021` |
| Crypto Wallet | `crypto_wallet` | `0x1234...` or `bc1q...` |
| **Healthcare** | | |
| Medical Record | `medical_record` | `MRN-12345678` |
| Health Insurance | `health_insurance` | `BCBS123456789` |
| **Vehicle** | | |
| VIN | `vin` | `1HGBH41JXMN109186` |
| License Plate | `license_plate` | `ABC-1234` |
| **Employment** | | |
| Employee ID | `employee_id` | `EMP123456` |
| Student ID | `student_id` | `STU123456789` |
| **Auth/Secrets** | | |
| Username | `username` | `happy_coder42` |
| Password Hash | `password_hash` | `$2a$10$...` (bcrypt) |
| API Key | `api_key` | `api_EXAMPLE_key_123...` |
| AWS Access Key | `aws_access_key` | `AKIAIOSFODNN7EXAMPLE` |
| Private Key | `private_key` | `-----BEGIN RSA PRIVATE KEY-----` |
| JWT Token | `jwt_token` | `eyJhbGciOiJIUzI1NiIs...` |
| **Location** | | |
| GPS Coordinates | `gps_coords` | `40.712776,-74.005974` |
| Geohash | `geohash` | `dr5regw3p` |
| **Personal** | | |
| Full Name | `full_name` | `John Smith` |
| Mother's Maiden | `mothers_maiden` | `Johnson` |
| Security Answer | `security_answer` | `Fluffy` |

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `json` to use this generator. |
| `generator.json.workers` | `--generator-json-workers` | `BLITZ_GENERATOR_JSON_WORKERS` | `1` | Number of JSON generator workers (must be ≥ 1) |
| `generator.json.rate` | `--generator-json-rate` | `BLITZ_GENERATOR_JSON_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |
| `generator.json.type` | `--generator-json-type` | `BLITZ_GENERATOR_JSON_TYPE` | `default` | Type of log to generate. Valid values: `default`, `pii` |

## Example Configuration

```yaml
generator:
  type: json
  json:
    workers: 2
    rate: 500ms
    type: default
```

### PII Log Type Example

```yaml
generator:
  type: json
  json:
    workers: 10
    rate: 500ms
    type: pii
```

## Metrics

The JSON generator exposes the following metrics:

- **`blitz_generator_logs_generated_total`** (Counter): Total number of logs generated
- **`blitz_generator_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_generator_write_errors_total`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics include a `component` label set to `generator_json`.
