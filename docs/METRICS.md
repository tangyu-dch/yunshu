# Metrics Plan

Metrics should be designed before adapters are implemented so dashboards stay consistent.

## Naming

Use lowercase snake case with service labels:

- `yunshu_http_requests_total`
- `yunshu_http_request_duration_seconds`
- `yunshu_workflow_events_total`
- `yunshu_workflow_event_lag_seconds`
- `yunshu_mq_messages_total`
- `yunshu_mq_message_lag_seconds`
- `yunshu_redis_operations_total`
- `yunshu_db_operations_total`
- `yunshu_fs_node_status`
- `yunshu_fs_commands_total`
- `yunshu_cdr_tasks_total`
- `yunshu_recording_uploads_total`
- `yunshu_callback_attempts_total`

## Required Labels

Keep label cardinality controlled:

- `service`
- `route`
- `method`
- `status`
- `queue`
- `event_type`
- `workflow_id`
- `state`
- `fs_addr`
- `command`
- `result`

Do not use raw phone, call id, uuid, request id, or user id as metric labels.

