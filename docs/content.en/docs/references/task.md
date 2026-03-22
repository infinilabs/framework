---
title: "Task Scheduling"
weight: 25
---

# Task Scheduling

The INFINI Framework provides a built-in task scheduling system for running recurring, cron-based, and one-off background tasks. Tasks are managed through a central registry that handles scheduling, lifecycle tracking, and graceful shutdown. The task system is defined in `core/task/task.go`.

## Overview

The task system supports three types of tasks:

| Type | Description |
|------|-------------|
| `interval` | Executes repeatedly at a fixed time interval (e.g., every 10 seconds). |
| `crontab` | Executes according to a cron expression (e.g., every day at midnight). |
| `transient` | Executes once immediately and is removed after completion. |

Tasks are stored in a global thread-safe registry (`sync.Map`) and can be started, stopped, or deleted at any time.

## ScheduleTask Struct

The `ScheduleTask` struct defines a scheduled task:

```go
type ScheduleTask struct {
    ID          string              `config:"id" json:"id,omitempty"`
    Group       string              `config:"group" json:"group,omitempty"`
    Description string              `config:"description" json:"description,omitempty"`
    Type        string              `config:"type" json:"type,omitempty"`
    Interval    string              `config:"interval" json:"interval,omitempty"`
    Crontab     string              `config:"crontab" json:"crontab,omitempty"`
    Singleton   bool                `config:"singleton" json:"singleton,omitempty"`
    Task        func(ctx context.Context) `config:"-" json:"-"`
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `string` | Unique identifier. Auto-generated (UUID) if left empty. |
| `Group` | `string` | Logical grouping for the task (e.g., `"metrics"`, `"pipeline"`). |
| `Description` | `string` | Human-readable description of what the task does. |
| `Type` | `string` | Task type: `"interval"`, `"crontab"`, or `"transient"`. Auto-detected from `Interval`/`Crontab` fields if omitted. |
| `Interval` | `string` | Execution interval using Go duration syntax (e.g., `"10s"`, `"5m"`, `"1h"`). Used when `Type` is `"interval"`. |
| `Crontab` | `string` | Cron expression for scheduling (e.g., `"0 0 * * *"`). Used when `Type` is `"crontab"`. |
| `Singleton` | `bool` | When `true`, prevents overlapping executions. If the previous run has not finished, the next scheduled run is skipped. |
| `Task` | `func(ctx context.Context)` | The function to execute on each run. Receives a context for cancellation support. |

## Task States

Each task transitions through the following states during its lifecycle:

| State | Value | Description |
|-------|-------|-------------|
| Pending | `"PENDING"` | Task is registered but has not started executing yet. |
| Running | `"STARTED"` | Task is currently executing. |
| Canceled | `"CANCELED"` | Task has been explicitly stopped or canceled. |
| Finished | `"FINISHED"` | Task has completed its execution. |

```
PENDING ──▶ STARTED ──▶ FINISHED
                │
                └──▶ CANCELED
```

## Registering Scheduled Tasks

Use `RegisterScheduleTask` to register an interval-based or cron-based task:

```go
func RegisterScheduleTask(task ScheduleTask) (taskID string)
```

The function returns the task ID (auto-generated if not provided). If a task with the same ID already exists, the old task is stopped and replaced. The `Type` field is automatically inferred from `Interval` or `Crontab` if not explicitly set.

### Interval Task Example

```go
task1 := task.ScheduleTask{
    Description: "collect CPU metrics",
    Interval:    "30s",
    Task: func(ctx context.Context) {
        collectCPUMetrics()
    },
}
task.RegisterScheduleTask(task1)
```

### Crontab Task Example

```go
task1 := task.ScheduleTask{
    Description: "daily cleanup",
    Crontab:     "0 2 * * *", // every day at 2:00 AM
    Task: func(ctx context.Context) {
        performDailyCleanup()
    },
}
task.RegisterScheduleTask(task1)
```

### Providing an Explicit ID

```go
task1 := task.ScheduleTask{
    ID:          util.GetUUID(),
    Interval:    "10s",
    Description: "detect high memory usage",
    Task: func(ctx context.Context) {
        memInfo, err := process.MemoryInfo()
        if err != nil {
            log.Error(err)
            return
        }
        // process memInfo...
    },
}
task.RegisterScheduleTask(task1)
```

## Transient (One-Off) Tasks

Transient tasks execute once immediately in a new goroutine and are automatically removed from the registry after completion.

### RegisterTransientTask

```go
func RegisterTransientTask(group, tag string, f func(ctx context.Context) error, ctxInput context.Context) (taskID string)
```

Registers and immediately runs a one-off task within a named group. The `tag` is used as the task description. The provided context allows passing values and supporting cancellation.

```go
taskCtx := context.WithValue(context.Background(), "id", clusterID)
task.RegisterTransientTask("elastic", "refresh_cluster_state", func(ctx context.Context) error {
    id := task.MustGetString(ctx, "id")
    refreshClusterState(id)
    return nil
}, taskCtx)
```

### RunWithContext

```go
func RunWithContext(tag string, f func(ctx context.Context) error, ctxInput context.Context) (taskID string)
```

A convenience wrapper around `RegisterTransientTask` that places the task in the `"default"` group.

```go
taskCtx := context.WithValue(context.Background(), "cfg", pipelineCfg)
task.RunWithContext("pipeline:"+name, func(ctx context.Context) error {
    cfg := ctx.Value("cfg")
    // process pipeline...
    return nil
}, taskCtx)
```

### RunWithinGroup

```go
func RunWithinGroup(groupName string, f func(ctx context.Context) error) (taskID string)
```

A convenience wrapper that runs a one-off task within a named group using a background context.

```go
task.RunWithinGroup("cleanup", func(ctx context.Context) error {
    removeExpiredSessions()
    return nil
})
```

## Singleton Mode

When `Singleton` is set to `true` on a `ScheduleTask`, the framework ensures that only one instance of the task runs at a time. If the previous execution has not finished when the next scheduled run triggers, the new run is skipped with a debug log message:

```
task [<id>][<description>] should be running in single instance, skipping
```

This is useful for tasks whose execution time may exceed the scheduling interval:

```go
task1 := task.ScheduleTask{
    Description: "sync remote configs",
    Interval:    "10s",
    Singleton:   true,
    Task: func(ctx context.Context) {
        // This may take longer than 10s.
        // With Singleton enabled, overlapping runs are prevented.
        syncConfigsFromRemote()
    },
}
task.RegisterScheduleTask(task1)
```

## Controlling Tasks

The task system provides functions to control individual tasks and the entire scheduler.

### Global Control

| Function | Description |
|----------|-------------|
| `RunTasks()` | Starts all registered tasks. Called by the framework during application startup. |
| `StopTasks()` | Stops all tasks, shuts down the scheduler, and closes the quit channel. Called during application shutdown. |
| `StopAllTasks()` | Stops all currently registered tasks without shutting down the scheduler itself. |

### Individual Task Control

| Function | Description |
|----------|-------------|
| `StartTask(id string)` | Starts (or restarts) a specific task by its ID. |
| `StopTask(id string)` | Stops a specific task by its ID. Calls the task's cancel function and sets its state to `CANCELED`. |
| `DeleteTask(id string)` | Stops and removes a task from the registry. |

### Example

```go
// Register a task and capture its ID
taskID := task.RegisterScheduleTask(task.ScheduleTask{
    Description: "periodic health check",
    Interval:    "1m",
    Task: func(ctx context.Context) {
        checkHealth()
    },
})

// Later, stop the task
task.StopTask(taskID)

// Restart it
task.StartTask(taskID)

// Remove it entirely
task.DeleteTask(taskID)
```

## Cron Expression Format

The `Crontab` field accepts standard cron expressions. The format follows the five-field syntax:

```
┌───────────── minute (0-59)
│ ┌───────────── hour (0-23)
│ │ ┌───────────── day of month (1-31)
│ │ │ ┌───────────── month (1-12)
│ │ │ │ ┌───────────── day of week (0-6, Sunday=0)
│ │ │ │ │
* * * * *
```

### Examples

| Expression | Description |
|------------|-------------|
| `* * * * *` | Every minute |
| `0 * * * *` | Every hour at minute 0 |
| `0 2 * * *` | Every day at 2:00 AM |
| `0 0 * * 0` | Every Sunday at midnight |
| `*/5 * * * *` | Every 5 minutes |
| `0 9-17 * * 1-5` | Every hour from 9 AM to 5 PM, Monday through Friday |

## Complete Example

Below is a complete example showing how a module registers multiple task types during its startup phase:

```go
package mymodule

import (
    "context"
    "fmt"

    log "github.com/cihub/seelog"
    "infini.sh/framework/core/task"
    "infini.sh/framework/core/util"
)

func (module *MyModule) Start() error {
    // 1. Interval task — collect metrics every 30 seconds
    metricsTask := task.ScheduleTask{
        Description: "collect application metrics",
        Interval:    "30s",
        Singleton:   true,
        Task: func(ctx context.Context) {
            module.collectMetrics()
        },
    }
    module.metricsTaskID = task.RegisterScheduleTask(metricsTask)

    // 2. Crontab task — run daily report at 1:00 AM
    reportTask := task.ScheduleTask{
        Description: "generate daily report",
        Crontab:     "0 1 * * *",
        Task: func(ctx context.Context) {
            module.generateReport()
        },
    }
    module.reportTaskID = task.RegisterScheduleTask(reportTask)

    // 3. Transient task — run one-off initialization
    task.RunWithinGroup("init", func(ctx context.Context) error {
        fmt.Println("performing one-time initialization")
        return module.initialize()
    })

    return nil
}

func (module *MyModule) Stop() error {
    // Clean up scheduled tasks
    task.DeleteTask(module.metricsTaskID)
    task.DeleteTask(module.reportTaskID)
    return nil
}
```
