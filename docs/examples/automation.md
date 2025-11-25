# Automation Service Quickstart

Schedule recurring function executions using cron expressions or interval-based triggers.

## Overview

The Automation service provides:
- **Cron Scheduling**: Standard cron expressions for precise timing
- **Interval Triggers**: Simple `@every` syntax for periodic execution
- **Job Management**: Enable/disable jobs without deletion
- **Execution History**: Track job runs and outcomes

## Prerequisites

```bash
export TOKEN=dev-token
export TENANT=tenant-a
export API=http://localhost:8080
```

## Quick Start

### 1. Create an Account and Function

```bash
# Create account
ACCOUNT_ID=$(curl -s -X POST $API/accounts \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-ID: $TENANT" \
  -H "Content-Type: application/json" \
  -d '{"owner":"scheduler"}' | jq -r .id)

# Create function to be scheduled
FUNC_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/functions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "price-refresher",
    "runtime": "js",
    "source": "(params) => ({ refreshed: true, timestamp: new Date().toISOString() })"
  }' | jq -r .ID)
```

### 2. Create Scheduled Job

```bash
# Run every 5 minutes
JOB_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/automation/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "function_id": "'"$FUNC_ID"'",
    "schedule": "*/5 * * * *",
    "payload": {"action": "refresh"},
    "enabled": true
  }' | jq -r .ID)

echo "Job ID: $JOB_ID"
```

### 3. List and Manage Jobs

```bash
# List all jobs
curl -s -H "Authorization: Bearer $TOKEN" \
  $API/accounts/$ACCOUNT_ID/automation/jobs | jq

# Disable job
curl -s -X PATCH $API/accounts/$ACCOUNT_ID/automation/jobs/$JOB_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}'

# Re-enable job
curl -s -X PATCH $API/accounts/$ACCOUNT_ID/automation/jobs/$JOB_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'
```

## Schedule Formats

### Cron Expressions

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, Sun=0)
│ │ │ │ │
* * * * *
```

| Expression | Description |
|------------|-------------|
| `*/5 * * * *` | Every 5 minutes |
| `0 * * * *` | Every hour at :00 |
| `0 0 * * *` | Daily at midnight |
| `0 9 * * 1-5` | Weekdays at 9 AM |
| `0 0 1 * *` | First of each month |

### Interval Syntax

| Expression | Description |
|------------|-------------|
| `@every 30s` | Every 30 seconds |
| `@every 5m` | Every 5 minutes |
| `@every 1h` | Every hour |
| `@every 24h` | Every day |
| `@hourly` | Every hour |
| `@daily` | Every day at midnight |
| `@weekly` | Every week |
| `@monthly` | Every month |

## API Reference

### Create Job

```http
POST /accounts/{account}/automation/jobs
```

```json
{
  "function_id": "func-uuid",
  "schedule": "*/5 * * * *",
  "payload": {"key": "value"},
  "enabled": true,
  "metadata": {"env": "prod"}
}
```

### Update Job

```http
PATCH /accounts/{account}/automation/jobs/{id}
```

```json
{
  "schedule": "@every 10m",
  "enabled": false
}
```

### Job Response

```json
{
  "ID": "job-uuid",
  "AccountID": "account-uuid",
  "FunctionID": "func-uuid",
  "Schedule": "*/5 * * * *",
  "Payload": {"key": "value"},
  "Enabled": true,
  "LastRun": "2025-01-15T10:00:00Z",
  "NextRun": "2025-01-15T10:05:00Z",
  "CreatedAt": "2025-01-01T00:00:00Z",
  "UpdatedAt": "2025-01-15T10:00:00Z"
}
```

## CLI Usage

```bash
# List jobs
slctl automation jobs list --account $ACCOUNT_ID

# Create job
slctl automation jobs create --account $ACCOUNT_ID \
  --function $FUNC_ID \
  --schedule "*/5 * * * *" \
  --payload '{"action":"refresh"}'

# Disable job
slctl automation jobs update --account $ACCOUNT_ID \
  --job $JOB_ID \
  --enabled false
```

## Use Cases

### Price Feed Refresh

```bash
curl -s -X POST $API/accounts/$ACCOUNT_ID/automation/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "function_id": "price-fetcher-func",
    "schedule": "@every 1m",
    "payload": {"feeds": ["NEO/USD", "GAS/USD"]},
    "enabled": true
  }'
```

### Daily Report Generation

```bash
curl -s -X POST $API/accounts/$ACCOUNT_ID/automation/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "function_id": "report-generator",
    "schedule": "0 8 * * *",
    "payload": {"report_type": "daily_summary"},
    "enabled": true
  }'
```

### Hourly Cleanup

```bash
curl -s -X POST $API/accounts/$ACCOUNT_ID/automation/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "function_id": "cleanup-func",
    "schedule": "@hourly",
    "payload": {"max_age_hours": 24},
    "enabled": true
  }'
```

## Best Practices

1. **Use Intervals for Simple Cases**: Prefer `@every 5m` over `*/5 * * * *` for readability
2. **Set Reasonable Frequencies**: Avoid schedules more frequent than every 30 seconds
3. **Handle Failures Gracefully**: Functions should be idempotent
4. **Monitor Job Health**: Check `LastRun` and execution logs
5. **Disable Before Updating**: Disable jobs before making significant changes

## Related Documentation

- [Functions Service](../service-catalog.md#2-functions-service)
- [Triggers Service](triggers.md)
- [Service Catalog](../service-catalog.md)
