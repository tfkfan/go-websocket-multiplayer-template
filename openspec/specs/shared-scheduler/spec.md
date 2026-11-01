# shared-scheduler Specification

## Purpose

Defines the timer scheduler that backs room timeouts: scheduling APIs, ordering by due time,
callback execution, period handling, shutdown semantics, and the scheduler's role in room
lifecycle.

## Requirements

### Requirement: Scheduler lifecycle

A scheduler SHALL be created running, with a task heap ordered by due time and then by insertion
sequence, and SHALL accept schedules concurrently from any goroutine.

#### Scenario: Concurrent schedules are accepted

- **WHEN** several goroutines schedule tasks at the same time
- **THEN** no data race occurs and every accepted task is retained

### Requirement: One-shot scheduling

`Schedule` SHALL run a callback once after the given delay. A negative delay SHALL be treated as
zero. A nil callback SHALL be ignored.

#### Scenario: Callback runs once

- **WHEN** a one-shot task becomes due
- **THEN** its callback runs exactly once

#### Scenario: Negative timeout fires immediately

- **WHEN** a task is scheduled with a negative delay
- **THEN** it fires without waiting for a further interval

### Requirement: Absolute-time scheduling

`ScheduleAt` SHALL run a callback once at an absolute time. A time in the past SHALL fire
immediately.

#### Scenario: Callback runs at the requested time

- **WHEN** a task is scheduled for a future instant
- **THEN** the callback runs no earlier than that instant

#### Scenario: Past time fires immediately

- **WHEN** a task is scheduled for an instant already elapsed
- **THEN** the callback runs without waiting

### Requirement: Periodic scheduling

`SchedulePeriodic` SHALL run a callback on a period after an initial delay. A missed deadline
SHALL be skipped so the callback resumes on the next aligned deadline rather than bursting
through every missed one.

#### Scenario: Callback repeats

- **WHEN** a periodic task is scheduled
- **THEN** its callback runs once per period

#### Scenario: Missed deadlines do not burst

- **WHEN** the scheduler is blocked past several periods
- **THEN** the callback does not run once per missed period to catch up

### Requirement: Callback execution

Callbacks SHALL run on their own goroutine so one slow callback cannot block scheduling or other
tasks, and periodic callbacks MAY overlap when an earlier run is still executing.

#### Scenario: Slow callback does not block other tasks

- **WHEN** one callback runs long and another task is due
- **THEN** the other callback still runs

#### Scenario: Periodic callbacks may overlap

- **WHEN** a periodic callback outlives its period
- **THEN** the next run is allowed to start alongside it

### Requirement: Shutdown

`Shutdown` SHALL stop accepting work, drop all pending tasks, wait for the scheduler goroutine to
exit, and wait for in-flight callbacks. With a non-nil context it SHALL return that context's
error when the deadline elapses before in-flight callbacks finish, and with a nil context it
SHALL wait indefinitely. Shutdown SHALL be idempotent. A callback that calls shutdown without a
deadline SHALL wait for its own completion and therefore never return.

#### Scenario: Pending tasks are dropped

- **WHEN** shutdown runs with future tasks queued
- **THEN** those callbacks never run

#### Scenario: In-flight callbacks are awaited

- **WHEN** shutdown runs while a callback is executing with a sufficient deadline
- **THEN** it returns only after the callback completes

#### Scenario: Deadline is reported

- **WHEN** a callback outlives the shutdown deadline
- **THEN** shutdown returns the context error

#### Scenario: Repeated shutdown is safe

- **WHEN** shutdown is called more than once
- **THEN** the later calls return without repeating cleanup

#### Scenario: Scheduling after shutdown is ignored

- **WHEN** a task is scheduled after shutdown
- **THEN** it is dropped and never runs

#### Scenario: Shutdown from a callback needs a deadline

- **WHEN** a callback calls shutdown with a nil or unbounded context
- **THEN** shutdown waits for that same callback and never returns

### Requirement: Room timeout integration

Each room SHALL schedule its one-shot `ROOM_TIMEOUT` task on its own scheduler when its actor
starts, and SHALL shut that scheduler down when the room closes so the scheduler cannot outlive
the room.

#### Scenario: Room closes through its scheduler

- **WHEN** the room's timeout task fires
- **THEN** the room runs its close sequence

#### Scenario: Scheduler does not outlive the room

- **WHEN** a room closes before its timeout
- **THEN** the pending timeout is dropped and the scheduler goroutine exits
