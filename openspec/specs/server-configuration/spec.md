# server-configuration Specification

## Purpose

Defines how the server reads and validates startup configuration from `.env` and the process
environment, bounds the configurable values, wires endpoints and assets, and fixes the gameplay
constants that environment variables cannot change.

## Requirements

### Requirement: Mandatory environment file

Startup SHALL load a `.env` file from the process working directory and SHALL fail when that
file is absent or unreadable. Values already present in the process environment SHALL take
precedence over the file values.

#### Scenario: Missing file aborts startup

- **WHEN** the server starts in a directory without a readable `.env` file
- **THEN** startup fails with a configuration error and no listener is opened

#### Scenario: Process environment wins

- **WHEN** a variable is set both in `.env` and in the process environment
- **THEN** the process environment value is used

### Requirement: Required variables

Startup SHALL require `PORT`, `WS_ENDPOINT`, `ROOM_ENDPOINT`, `ASSETS_DIR`, `ROOM_TIMEOUT`, and
`ROOM_CAPACITY`, and SHALL fail naming the first missing variable.

#### Scenario: Missing variable fails startup

- **WHEN** any required variable is unset after `.env` loading
- **THEN** startup fails with an error that names that variable

### Requirement: Value validation

`ROOM_TIMEOUT` SHALL parse as a Go duration and SHALL be positive. `ROOM_CAPACITY` SHALL parse
as an integer and SHALL be positive. A value that fails to parse or is not positive SHALL abort
startup.

#### Scenario: Invalid duration fails startup

- **WHEN** `ROOM_TIMEOUT` is not a duration string
- **THEN** startup fails with a configuration error

#### Scenario: Non-positive duration fails startup

- **WHEN** `ROOM_TIMEOUT` parses to zero or a negative duration
- **THEN** startup fails with a configuration error

#### Scenario: Invalid capacity fails startup

- **WHEN** `ROOM_CAPACITY` is not an integer
- **THEN** startup fails with a configuration error

#### Scenario: Non-positive capacity fails startup

- **WHEN** `ROOM_CAPACITY` parses to zero or a negative integer
- **THEN** startup fails with a configuration error

### Requirement: Endpoint and asset wiring

`PORT` SHALL be passed to the TCP listener verbatim, `WS_ENDPOINT` SHALL register the WebSocket
upgrade handler, `ROOM_ENDPOINT` SHALL register room creation, and `ASSETS_DIR` SHALL be the
static file root. The served client hard-codes `/room` and `/ws`, so changing those variables
SHALL also require editing `client/scripts/protocol.gd`.

#### Scenario: Missing assets directory yields 404s

- **WHEN** the server serves an asset path that does not exist under `ASSETS_DIR`
- **THEN** the response is 404

#### Scenario: Changing endpoints requires a client change

- **WHEN** `WS_ENDPOINT` or `ROOM_ENDPOINT` differs from `/ws` or `/room`
- **THEN** the unmodified client no longer reaches those handlers

### Requirement: Fixed gameplay constants

World size (`1200 x 800`), player radius (`25`), tick rate (`30` Hz), movement speed (`260`
units/s), palette colors, and the latest-per-type outbound policy SHALL be compiled into the
server and SHALL NOT be overridable through configuration.

#### Scenario: Environment cannot change gameplay constants

- **WHEN** the process environment defines arbitrary additional variables
- **THEN** world size, player radius, tick rate, and movement speed stay at their compiled values
