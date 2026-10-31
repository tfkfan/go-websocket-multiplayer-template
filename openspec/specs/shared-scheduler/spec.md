# shared-scheduler Specification

## Purpose
Defines the shared timer facility: one-shot, absolute-time, and periodic scheduling, callback
execution, graceful shutdown, and the room timeout integration.

## Requirements

### Requirement: Scheduler lifecycle

`NewScheduler` SHALL return a running scheduler with one dispatcher goroutine; the zero value
SHALL NOT be used; scheduling methods SHALL be safe for concurrent use, including calls made from
inside callbacks.

#### Scenario: Concurrent schedules are accepted
- **WHEN** multiple goroutines schedule tasks on the same scheduler concurrently
- **THEN** every accepted task is eventually dispatched

### Requirement: One-shot scheduling

`Schedule(timeout, callback)` SHALL run the callback once after `timeout`; a negative timeout
SHALL fire as soon as possible.

#### Scenario: Callback runs once
- **WHEN** a one-shot task is scheduled and its timeout elapses
- **THEN** the callback runs exactly once and never repeats

#### Scenario: Negative timeout fires immediately
- **WHEN** a task is scheduled with a negative timeout
- **THEN** the callback runs as soon as the dispatcher can execute it

### Requirement: Absolute-time scheduling

`ScheduleAt(time, callback)` SHALL run the callback once at the specified absolute time; a past
time SHALL fire as soon as possible.

#### Scenario: Callback runs at the requested time
- **WHEN** a task is scheduled for a time in the future
- **THEN** the callback does not run before that time and runs once shortly after

#### Scenario: Past time fires immediately
- **WHEN** a task is scheduled for a time that has already passed
- **THEN** the callback runs as soon as possible

### Requirement: Periodic scheduling

`SchedulePeriodic(initialDelay, period, callback)` SHALL first run the callback after
`initialDelay` and then every `period`; a non-positive period SHALL degrade the schedule to a
one-shot; missed deadlines SHALL be skipped without a burst while preserving the original phase.

#### Scenario: Callback repeats
- **WHEN** a periodic task is scheduled with a positive period
- **THEN** the callback runs repeatedly at that period until shutdown

#### Scenario: Missed deadlines do not burst
- **WHEN** the scheduler falls behind and several periods elapse before a firing
- **THEN** skipped firings are not replayed in a burst and the schedule continues from the next
  future deadline

### Requirement: Callback execution

Each firing SHALL run the callback in its own goroutine; callbacks SHALL NOT be serialized and
MAY overlap; panics SHALL NOT be recovered.

#### Scenario: Slow callback does not block other tasks
- **WHEN** one callback blocks for a long time
- **THEN** other scheduled tasks still fire on time

#### Scenario: Periodic callbacks may overlap
- **WHEN** a periodic callback takes longer than its period
- **THEN** a later firing may start before the earlier callback finishes

### Requirement: Shutdown

`Shutdown` SHALL be idempotent, drop pending tasks, stop the dispatcher, and wait for in-flight
callbacks; it SHALL return the context error when the context expires first, a nil context SHALL
wait without a deadline, scheduling after shutdown SHALL be a no-op, and shutdown from inside a
callback SHALL require a context that expires.

#### Scenario: Pending tasks are dropped
- **WHEN** shutdown happens before a scheduled task fires
- **THEN** the callback never runs

#### Scenario: In-flight callbacks are awaited
- **WHEN** shutdown happens while a callback is running
- **THEN** shutdown returns only after the callback returns, unless its context expires first

#### Scenario: Deadline is reported
- **WHEN** the shutdown context expires before running callbacks finish
- **THEN** shutdown returns the context error

#### Scenario: Repeated shutdown is safe
- **WHEN** shutdown is called more than once with a live context
- **THEN** every call returns nil once the scheduler is idle

#### Scenario: Scheduling after shutdown is ignored
- **WHEN** a task is scheduled after shutdown completed
- **THEN** the callback never runs

#### Scenario: Shutdown from a callback needs a deadline
- **WHEN** a callback calls shutdown with a context that never expires
- **THEN** shutdown waits for that same callback and does not return

### Requirement: Room timeout integration

`GameRoom` SHALL create a scheduler, schedule `ROOM_TIMEOUT` once when its actor starts, and
deliver the timeout to the actor as a buffered event; close SHALL shut the scheduler down so no
scheduler goroutine outlives the room.

#### Scenario: Room closes through its scheduler
- **WHEN** a room's `ROOM_TIMEOUT` elapses
- **THEN** the room actor receives the timeout event, closes the room, and returns

#### Scenario: Scheduler does not outlive the room
- **WHEN** the room closes
- **THEN** the room scheduler is shut down and its goroutine exits
