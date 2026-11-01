# server-runtime Specification

## Purpose

Defines how the process boots, acquires its listener, serves HTTP and WebSocket traffic, keeps
the room registry, and shuts down when a signal, the parent context, or a serve error ends it.

## Requirements

### Requirement: Startup and listener acquisition

Startup SHALL read configuration, bind the configured port before serving, and return an error
without serving when binding fails. After binding it SHALL report the HTTP address and WebSocket
endpoint and start serving in the background.

#### Scenario: Port already in use aborts startup

- **WHEN** the configured port is already bound by another process
- **THEN** startup returns an error and no HTTP server serves traffic

#### Scenario: Startup reports its endpoints

- **WHEN** the listener is acquired
- **THEN** the process logs the HTTP address and the WebSocket URL built from the endpoint setting

### Requirement: HTTP server limits

The HTTP server SHALL apply a 10 second read timeout and a 10 second write timeout to every
request, and SHALL dispatch through the configured routes.

#### Scenario: Requests are bounded

- **WHEN** the server handles an HTTP request
- **THEN** its read and write timeouts are the compiled 10 second limits

#### Scenario: Routes dispatch

- **WHEN** a request arrives for an asset, the room endpoint, or the WebSocket endpoint
- **THEN** the matching handler produces the response

### Requirement: Room registry

The server SHALL keep rooms in a registry keyed by room UUID, protected for concurrent access.
A created room SHALL be registered before its identifier is returned to the caller, and a room
SHALL be removed from the registry after its close completes.

#### Scenario: Concurrent creation is safe

- **WHEN** several room-creation requests run at the same time
- **THEN** each returns a distinct identifier and all rooms are registered without a data race

#### Scenario: Closed room leaves the registry

- **WHEN** a room's close sequence completes
- **THEN** its identifier no longer resolves to a room and later joins report an unknown room

### Requirement: Shutdown triggers

The process SHALL begin shutting down when it receives `SIGINT` or `SIGTERM`, when its parent
context is done, or when serving returns an error. A serve error SHALL be logged and SHALL NOT
skip the shutdown sequence.

#### Scenario: Termination signal starts shutdown

- **WHEN** the process receives `SIGINT` or `SIGTERM`
- **THEN** shutdown begins

#### Scenario: Parent context cancellation starts shutdown

- **WHEN** the parent context is canceled
- **THEN** shutdown begins

#### Scenario: Serve failure is reported and shutdown continues

- **WHEN** serving returns an error
- **THEN** the error is logged and the shutdown sequence still runs

### Requirement: Shutdown sequence and deadline

Shutdown SHALL cancel the server context so every room closes through its canceled context,
then shut down the HTTP server within a 10 second deadline. Server shutdown SHALL stop accepting
new requests, and room close handshakes SHALL NOT extend the deadline past the wait limit.

#### Scenario: Rooms are closed by shutdown

- **WHEN** shutdown cancels the server context
- **THEN** each room closes through its context-canceled branch

#### Scenario: Deadline bounds shutdown

- **WHEN** the shutdown deadline elapses before the HTTP server drains
- **THEN** shutdown returns the deadline error rather than waiting indefinitely

#### Scenario: New requests are refused after shutdown

- **WHEN** the HTTP server has shut down
- **THEN** new connections are no longer accepted

### Requirement: Process exit status

The process SHALL exit with a non-zero status and report the error when startup fails, including
configuration errors and listener acquisition failures. A shutdown that returns without error
SHALL exit with status zero.

#### Scenario: Startup failure exits non-zero

- **WHEN** configuration cannot be read or the listener cannot bind
- **THEN** the process reports the error and exits with a non-zero status

#### Scenario: Clean shutdown exits zero

- **WHEN** shutdown returns without error
- **THEN** the process exits with status zero
