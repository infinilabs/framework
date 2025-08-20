---
title: "Task Module"
weight: 70
---

# Task Module

The Task module provides task scheduling and execution capabilities for the framework. It enables running background tasks, scheduled operations, and pipeline-based processing with concurrent execution control and task management APIs.

## Features

- Task scheduling and execution management
- Concurrent task execution with configurable limits
- Task lifecycle management (start, stop, delete)
- Pipeline integration for task processing
- REST API for task management
- Timezone configuration support
- Automatic task cleanup and resource management

## Configuration

Configure the Task module in your YAML configuration:

```yaml
task:
  time_zone: "UTC"
  max_concurrent_tasks: 100
```

## Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `time_zone` | string | `"UTC"` | Timezone for task scheduling |
| `max_concurrent_tasks` | int | `100` | Maximum number of concurrent tasks |

## API Endpoints

The Task module provides the following REST API endpoints:

### Task Management

- **GET `/tasks/`** - List all tasks with their status and configuration
- **POST `/task/:id/_start`** - Start a specific task by ID
- **POST `/task/:id/_stop`** - Stop a running task by ID  
- **DELETE `/task/:id`** - Delete a task configuration by ID

### Example API Usage

```bash
# List all tasks
curl http://localhost:8080/tasks/

# Start a task
curl -X POST http://localhost:8080/task/backup_task/_start

# Stop a task
curl -X POST http://localhost:8080/task/backup_task/_stop

# Delete a task
curl -X DELETE http://localhost:8080/task/backup_task
```

## Task Configuration

Tasks can be configured in the main configuration file:

```yaml
# Task definitions
tasks:
  - id: "data_backup"
    name: "Daily Data Backup"
    enabled: true
    schedule: "0 2 * * *"  # Daily at 2 AM
    pipeline: "backup_pipeline"
    
  - id: "log_rotation"
    name: "Log File Rotation"
    enabled: true
    schedule: "0 0 * * 0"  # Weekly on Sunday
    pipeline: "log_rotation_pipeline"
    
  - id: "health_check"
    name: "System Health Check"
    enabled: true
    schedule: "*/5 * * * *"  # Every 5 minutes
    pipeline: "health_check_pipeline"
```

### Task Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Unique task identifier |
| `name` | string | Human-readable task name |
| `enabled` | boolean | Enable/disable task execution |
| `schedule` | string | Cron expression for scheduling |
| `pipeline` | string | Pipeline to execute for this task |
| `timeout` | duration | Task execution timeout |
| `retry_count` | int | Number of retry attempts on failure |

## Scheduling

The Task module supports cron-style scheduling:

### Cron Expression Format
```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │
* * * * *
```

### Common Schedule Examples

```yaml
# Every minute
schedule: "* * * * *"

# Every hour
schedule: "0 * * * *"

# Daily at midnight
schedule: "0 0 * * *"

# Weekly on Monday at 9 AM
schedule: "0 9 * * 1"

# Monthly on the 1st at 6 AM
schedule: "0 6 1 * *"

# Every 15 minutes
schedule: "*/15 * * * *"
```

## Pipeline Integration

Tasks execute through the pipeline system:

```yaml
# Define pipelines for tasks
pipelines:
  - name: "backup_pipeline"
    processors:
      - copy:
          source: "./data"
          destination: "./backup"
      - compress:
          algorithm: "gzip"
      - upload:
          destination: "s3://backups/"

  - name: "health_check_pipeline"  
    processors:
      - check_disk_space:
          threshold: 90
      - check_memory:
          threshold: 85
      - send_alert:
          condition: "failure"
```

## Task Lifecycle

### Task States
- **Scheduled** - Task is configured and scheduled
- **Running** - Task is currently executing
- **Completed** - Task finished successfully
- **Failed** - Task execution failed
- **Stopped** - Task was manually stopped

### Lifecycle Management

```go
import "infini.sh/framework/core/task"

// Start a task programmatically
err := task.StartTask("backup_task")
if err != nil {
    log.Error("Failed to start task:", err)
}

// Stop a running task
err = task.StopTask("backup_task")
if err != nil {
    log.Error("Failed to stop task:", err)
}

// Get task status
status := task.GetTaskStatus("backup_task")
log.Info("Task status:", status)
```

## Concurrent Execution

The Task module manages concurrent task execution:

### Concurrency Control
- Maximum concurrent tasks configurable
- Task queuing when limit reached
- Resource-aware scheduling
- Automatic cleanup of completed tasks

### Performance Tuning
```yaml
task:
  max_concurrent_tasks: 50  # Reduce for lower resource usage
  time_zone: "America/New_York"  # Local timezone
```

## Monitoring and Logging

### Task Monitoring
- Task execution statistics
- Success/failure rates
- Execution duration tracking
- Resource usage monitoring

### Example Monitoring
```bash
# Get task list with status
curl http://localhost:8080/tasks/ | jq '.'

# Output example:
{
  "tasks": [
    {
      "id": "backup_task",
      "name": "Daily Backup",
      "status": "running",
      "last_run": "2024-01-01T02:00:00Z",
      "next_run": "2024-01-02T02:00:00Z",
      "duration": "45s"
    }
  ]
}
```

## Error Handling

### Retry Logic
```yaml
tasks:
  - id: "resilient_task"
    name: "Resilient Task"
    schedule: "0 */6 * * *"
    pipeline: "data_sync"
    retry_count: 3
    timeout: "30m"
```

### Failure Handling
- Automatic retry with configurable attempts
- Exponential backoff between retries
- Failed task notifications
- Error logging and reporting

## Integration

The Task module integrates with:

- **Pipeline system** - Executes tasks through pipelines
- **API module** - Provides REST endpoints for management
- **Stats module** - Reports task execution metrics
- **Global environment** - Uses timezone and shutdown handling

## Use Cases

### Data Backup
```yaml
tasks:
  - id: "nightly_backup"
    schedule: "0 2 * * *"  # 2 AM daily
    pipeline: "backup_databases"
```

### Log Management
```yaml
tasks:
  - id: "log_cleanup"
    schedule: "0 3 * * 0"  # 3 AM on Sundays
    pipeline: "rotate_and_compress_logs"
```

### Health Monitoring
```yaml
tasks:
  - id: "system_health"
    schedule: "*/10 * * * *"  # Every 10 minutes
    pipeline: "check_system_health"
```

### Data Synchronization
```yaml
tasks:
  - id: "data_sync"
    schedule: "0 */4 * * *"  # Every 4 hours
    pipeline: "sync_elasticsearch_data"
```

## Best Practices

1. **Schedule carefully** - Avoid overlapping resource-intensive tasks
2. **Set timeouts** - Prevent tasks from running indefinitely
3. **Monitor execution** - Use the API to track task performance
4. **Use pipelines** - Leverage the pipeline system for complex operations
5. **Plan concurrency** - Configure appropriate concurrent task limits

## Troubleshooting

### Common Issues

1. **Tasks not starting**
   - Check task configuration syntax
   - Verify pipeline references exist
   - Review cron expression format

2. **High resource usage**
   - Reduce `max_concurrent_tasks`
   - Add task timeouts
   - Monitor pipeline resource usage

3. **Schedule conflicts**
   - Review overlapping task schedules
   - Stagger resource-intensive operations
   - Use task dependencies where needed

### Debug Configuration
```yaml
task:
  debug: true           # Enable debug logging
  log_task_execution: true  # Log all task operations
```

This enables detailed logging for troubleshooting task execution issues.