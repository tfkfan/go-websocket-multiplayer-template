# server-configuration Specification

## Purpose
Defines startup configuration loading, validation, and the compiled-in world constants that
operators and clients rely on.

## Requirements

### Requirement: Mandatory .env loading

Startup SHALL fail when `.env` is missing from the working directory, even when environment
variables are already set; existing process environment variables SHALL take precedence over
file values.

#### Scenario: Missing .env fails startup

- **WHEN** the process starts in a directory without `.env`
- **THEN** startup fails with a configuration error

#### Scenario: Process environment wins

- **WHEN** a variable is set both in `.env` and in the process environment
- **THEN** the process environment value is used

### Requirement: Required variables

The server SHALL require `PORT`, `WS_ENDPOINT`, `ROOM_ENDPOINT`, `ASSETS_DIR`,
`ROOM_TIMEOUT`, and `ROOM_CAPACITY`; a missing variable SHALL fail startup.

#### Scenario: Missing variable fails startup

- **WHEN** any required variable is absent
- **THEN** startup fails with an error naming the missing variable

### Requirement: Value validation

`ROOM_TIMEOUT` SHALL parse as a positive duration and `ROOM_CAPACITY` as a positive integer;
invalid or non-positive values SHALL fail startup.

#### Scenario: Invalid duration fails startup

- **WHEN** `ROOM_TIMEOUT` is not a valid duration or is zero or negative
- **THEN** startup fails with a configuration error

#### Scenario: Invalid capacity fails startup

- **WHEN** `ROOM_CAPACITY` is not an integer or is zero or negative
- **THEN** startup fails with a configuration error

### Requirement: Endpoint and asset wiring

`PORT` SHALL be passed to `net.Listen`; `WS_ENDPOINT` and `ROOM_ENDPOINT` SHALL be registered
on the HTTP mux; `ASSETS_DIR` SHALL be the static root, and a missing directory SHALL yield
404 responses. Endpoint patterns SHALL not be validated beyond the mux behavior, and the
browser client SHALL keep its own hard-coded endpoint paths.

#### Scenario: Missing assets directory yields 404s

- **WHEN** `ASSETS_DIR` points to a directory that does not exist
- **THEN** asset requests return 404 and the WebSocket and room endpoints still work

#### Scenario: Changing endpoints also requires a client change

- **WHEN** an endpoint variable differs from the value hard-coded in the browser client
- **THEN** the browser client cannot reach that endpoint until it is updated

### Requirement: Fixed gameplay constants

World width and height (1200 and 800), player radius (25), tick rate (30), movement speed
(260), room command queue depth (64), spawn ring factor (0.32), and the 8-color palette SHALL
be compiled in and SHALL NOT be configurable through environment variables.

#### Scenario: Environment cannot change gameplay constants

- **WHEN** the server starts with any environment values
- **THEN** the world dimensions, radius, tick rate, and movement speed remain the compiled-in values sent to clients
