# Okta System Log Generator

The Okta generator creates synthetic Okta System Log events in JSON format. It produces realistic authentication, security, user lifecycle, and administrative events that match the Okta System Log API schema.

## Description

The Okta System Log format is a JSON structure that includes event type, actor, client context, outcome, target, and security context fields. The generator produces events across multiple categories: authentication (login, SSO, MFA), security threats (brute force, credential stuffing, impossible travel), user lifecycle (create, activate, suspend, delete), password operations, application and group membership changes, policy management, and administrative actions.

## Example Logs

```json
{"uuid":"a1b2c3d4-e5f6-7890-abcd-ef1234567890","published":"2025-11-10T21:11:47.123Z","eventType":"user.session.start","version":"0","severity":"INFO","displayMessage":"User login to Okta","actor":{"id":"00u1234567890","type":"User","alternateId":"john.smith@example.com","displayName":"John Smith"},"client":{"userAgent":{"rawUserAgent":"Mozilla/5.0...","os":"Unknown","browser":"UNKNOWN"},"zone":"null","device":"Unknown","ipAddress":"192.168.1.100","geographicalContext":{"city":"San Francisco","state":"California","country":"United States","postalCode":"94102","geolocation":{"lat":37.7749,"lon":-122.4194}}},"outcome":{"result":"SUCCESS"},"target":[{"id":"0oa1234567890","type":"AppInstance","alternateId":"slack","displayName":"Slack"}],"transaction":{"type":"WEB","id":"AbCdEfGhIjKlMnOpQrSt","detail":{}},"authenticationContext":{"authenticationProvider":"OKTA_AUTHENTICATION_PROVIDER","credentialProvider":"OKTA_CREDENTIAL_PROVIDER","credentialType":"PASSWORD"},"securityContext":{"asNumber":12345,"asOrg":"example-isp","isp":"Example ISP","domain":"example.com","isProxy":false}}
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `okta` to use this generator. |
| `generator.okta.workers` | `--generator-okta-workers` | `BLITZ_GENERATOR_OKTA_WORKERS` | `1` | Number of Okta generator workers (must be >= 1) |
| `generator.okta.rate` | `--generator-okta-rate` | `BLITZ_GENERATOR_OKTA_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

## Example Configuration

```yaml
generator:
  type: okta
  okta:
    workers: 5
    rate: 100ms
```

## Event Categories

| Category | Event Types | Severity |
|----------|------------|----------|
| Authentication | `user.session.start`, `user.session.end`, `user.authentication.sso`, `user.authentication.auth_via_mfa` | INFO/WARN |
| Security | `security.threat.detected`, `security.request.blocked`, `user.session.impersonation.start` | WARN/ERROR |
| User Lifecycle | `user.lifecycle.create`, `user.lifecycle.activate`, `user.lifecycle.deactivate`, `user.lifecycle.suspend` | INFO/WARN |
| Password | `user.account.update_password`, `user.account.reset_password`, `user.credential.forgot_password` | INFO |
| Application | `app.user_membership.add`, `app.user_membership.remove`, `application.lifecycle.create` | INFO |
| Group | `group.user_membership.add`, `group.user_membership.remove`, `group.lifecycle.create` | INFO |
| Policy | `policy.lifecycle.create`, `policy.lifecycle.update`, `policy.rule.create` | INFO |
| Admin | `user.account.privilege.grant`, `system.api_token.create`, `system.api_token.revoke` | INFO/WARN |

## Metrics

The Okta generator exposes the following metrics:

- **`blitz.generator.logs.generated`** (Counter): Total number of logs generated
- **`blitz.generator.workers.active`** (Gauge): Number of active worker goroutines
- **`blitz.generator.write.errors`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics include a `component` label set to `generator_okta`.
