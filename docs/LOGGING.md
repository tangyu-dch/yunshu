# Logging

Yunshu uses Go `log/slog` through `internal/infra/logging`.

## Format

Production logs should use JSON:

```yaml
logging:
  level: info
  format: json
```

Local debugging may use `text`.

## Required Fields

All logs should include:

- `service`
- `level`
- `msg`
- `time`

HTTP logs add:

- `method`
- `path`
- `status`
- `duration`
- `requestId`
- `traceId`

Telephony command logs add:

- `callId`
- `uuid`
- `fsAddr`
- `legRole`
- `commandId`
- `command`

Telephony event logs add:

- `callId`
- `uuid`
- `fsAddr`
- `legRole`
- `eventId`
- `eventName`

## Rules

- Use `internal/infra/logging.New` at service startup and install it with `logging.SetDefault`.
- Use helper functions such as `logging.TelephonyAttrs`, `logging.TelephonyEventAttrs`, and `logging.HTTPAttrs` instead of hand-writing field names.
- Log messages must be written in Chinese. Field keys remain stable English identifiers.
- Do not log raw customer phone numbers in normal runtime logs. Use masked values or business audit storage.
- Logs for retries, duplicate commands, failed FS commands, CDR repair, recording upload, callbacks, and billing must carry the idempotency key or command id.
