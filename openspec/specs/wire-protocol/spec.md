# wire-protocol Specification

## Purpose

Defines the HTTP and WebSocket surface the client depends on: routing and static asset delivery,
room creation, the JSON envelope and message numbers, payload shapes, handshake limits, player
identity, and close codes.

## Requirements

### Requirement: HTTP endpoints

The server SHALL serve static files at `/`, room creation at `ROOM_ENDPOINT`, and WebSocket
upgrades at `WS_ENDPOINT`. Exact handler paths SHALL take precedence over the `/` catch-all.

#### Scenario: Static assets are served

- **WHEN** a client requests a path that exists under the assets directory
- **THEN** the server returns that file from disk

#### Scenario: Unknown asset yields 404

- **WHEN** a client requests a path that does not exist under the assets directory
- **THEN** the response is 404

### Requirement: Web export asset delivery

Static delivery SHALL serve the Godot Web export: `/` SHALL return the exported entry document,
`.wasm` files SHALL be served as `application/wasm`, and `.pck` files SHALL be served as
`application/octet-stream`.

#### Scenario: Root serves the export entry document

- **WHEN** the asset root contains `index.html` and a client requests `/`
- **THEN** the response is that document as HTML

#### Scenario: WebAssembly module has its content type

- **WHEN** a client requests a `.wasm` file
- **THEN** the response content type is `application/wasm`

#### Scenario: Game pack has its content type

- **WHEN** a client requests a `.pck` file
- **THEN** the response content type is `application/octet-stream`

### Requirement: Room creation response

`POST` to `ROOM_ENDPOINT` SHALL create a room and return HTTP 200 with the new room UUID as
plain text. The request body SHALL be ignored. Any other method SHALL return 405 with an empty
body and no `Allow` header.

#### Scenario: Room creation returns a UUID

- **WHEN** a client posts to the room endpoint
- **THEN** the response is 200 with a parseable UUID in the body

#### Scenario: Other methods are rejected

- **WHEN** a client sends a non-POST request to the room endpoint
- **THEN** the response is 405 with an empty body and no `Allow` header

### Requirement: Room selection

The room SHALL be selected by the `room` query parameter of the WebSocket URL, which SHALL parse
as a UUID. No client JOIN message SHALL be required, and a client message of type 2 SHALL be
ignored like any other unhandled type.

#### Scenario: Connection joins through the query parameter

- **WHEN** a client opens the WebSocket endpoint with a valid `room` parameter
- **THEN** the connection is admitted to that room when its join command is applied

#### Scenario: JOIN message is not needed

- **WHEN** a client never sends a message of type 2 and only sends key events
- **THEN** the connection still receives join data and updates

#### Scenario: Invalid room parameter

- **WHEN** the `room` parameter is missing or not a UUID
- **THEN** the upgrade attempt ends with close status 1014 after the socket is accepted

### Requirement: JSON envelope

Client messages SHALL be JSON text frames shaped `{"type": <number>, "data": ...}`. Frames that
do not parse SHALL be logged and ignored. Types other than 200 SHALL be ignored, and unknown
directions inside a key event SHALL be ignored without an error response.

#### Scenario: Malformed client JSON is ignored

- **WHEN** a client sends a frame that is not valid JSON
- **THEN** the server logs the message and keeps the connection open

#### Scenario: Unknown message type is ignored

- **WHEN** a client sends a valid envelope with a type the server does not handle
- **THEN** the server ignores the frame and keeps the connection open

### Requirement: Client input message

A client message of type 200 SHALL carry `{key, state}` where `key` is `UP`, `DOWN`, `LEFT`, or
`RIGHT` and `state` is a boolean. The server SHALL apply the state to that player's movement
bitmask.

#### Scenario: Key event is applied

- **WHEN** a client sends a key event with a valid direction and state
- **THEN** the player's movement state for that direction matches the sent state

#### Scenario: Unknown key is ignored

- **WHEN** a client sends a key event with a direction that is not one of the four
- **THEN** the player's movement state is unchanged and the connection stays open

### Requirement: Server message types

The server SHALL send JOIN_SUCCESS (4), UPDATE (100), and PRIVATE (110). PRIVATE SHALL only be
sent to its target. ROOM_CLOSE (9) and FAILURE (11) SHALL remain reserved and SHALL NOT be sent.
Type 200 SHALL only travel from client to server.

#### Scenario: JOIN_SUCCESS shape

- **WHEN** a player is admitted
- **THEN** the payload contains `playerId`, `roomId`, `width`, `height`, `radius`, `color`, and `tickRate`

#### Scenario: UPDATE shape

- **WHEN** a tick snapshot is published
- **THEN** the payload contains `players`, `width`, and `height`

#### Scenario: UPDATE carries no tick field

- **WHEN** a client decodes an UPDATE frame
- **THEN** the payload has no `tick` member and clients must treat it as absent

#### Scenario: PRIVATE shape

- **WHEN** a targeted payload is published
- **THEN** the PRIVATE frame carries that payload to the target only

#### Scenario: Reserved types are unsent

- **WHEN** any room behavior occurs
- **THEN** the server never sends FAILURE or ROOM_CLOSE frames

### Requirement: UPDATE player list

The UPDATE `players` array SHALL contain every current member exactly once, each with `id`,
`position` (`x`, `y`), `radius`, and `color`. The array order SHALL be unspecified, and clients
SHALL identify players by `id` rather than by position in the array.

#### Scenario: Every member appears once

- **WHEN** a tick snapshot is published
- **THEN** the player list has one entry per current member with no duplicates

#### Scenario: Membership changes are reflected

- **WHEN** a player joins or leaves between two ticks
- **THEN** the next UPDATE includes or omits that player accordingly

### Requirement: Message type numbering

Message numbers SHALL be `MsgJoin` 2, `MsgJoinSuccess` 4, `MsgRoomClose` 9, `MsgFailure` 11,
`MsgUpdate` 100, `MsgPrivate` 110, and `MsgPlayerKeyDown` 200. The Godot client SHALL mirror
these numbers.

#### Scenario: Numbers stay stable

- **WHEN** a client decodes a server frame or encodes an input event
- **THEN** the numeric type is one of the fixed values above

### Requirement: WebSocket handshake

The server SHALL accept upgrades with same-origin verification enabled and SHALL apply the
library's 32 KiB message read limit. A message that exceeds the limit SHALL close the connection
with status 1009.

#### Scenario: Cross-origin upgrade is rejected

- **WHEN** a browser page from another origin attempts the upgrade
- **THEN** the upgrade is rejected and the connection is not established

#### Scenario: Oversized message closes the socket

- **WHEN** a client sends a frame larger than 32 KiB
- **THEN** the connection is closed with status 1009

### Requirement: Player identity

A player identifier SHALL be taken from the optional `X-Player-Id` request header. When the
header is absent or not a UUID, the server SHALL allocate a new identifier. Identifiers are used
as membership keys, so an identifier already present in the room SHALL be rejected by admission.

#### Scenario: Valid header selects the identifier

- **WHEN** a non-browser client sends a valid `X-Player-Id` header
- **THEN** the admitted player uses that identifier in JOIN_SUCCESS and UPDATE

#### Scenario: Invalid header gets a new identifier

- **WHEN** the header is missing or not parseable as a UUID
- **THEN** the server generates a fresh identifier for the player

### Requirement: Close codes

The server SHALL close with 1001 Going Away when a room closes. It SHALL use 1014 Bad Gateway
when admission fails before the join command is submitted (invalid or unknown room, closed room,
or any other non-close read error). An oversized frame SHALL use 1009, and a client-initiated
normal closure SHALL end the handler without a server close frame.

#### Scenario: Admission failures use 1014

- **WHEN** a connection targets an invalid room identifier, an unknown room, or a room that is already closed
- **THEN** the socket is closed with status 1014

#### Scenario: Room expiry uses 1001

- **WHEN** a room expires or is closed by the server
- **THEN** every member socket is closed with status 1001

#### Scenario: Duplicate and full joins send no close frame

- **WHEN** a join is rejected by the room's admission checks after the command is submitted
- **THEN** the client receives neither JOIN_SUCCESS nor a close frame
