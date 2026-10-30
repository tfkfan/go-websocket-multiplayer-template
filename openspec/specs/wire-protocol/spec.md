# wire-protocol Specification

## Purpose
Defines the HTTP and WebSocket transport, JSON envelope, message numbers, handshake limits,
and close codes that clients depend on.

## Requirements

### Requirement: HTTP endpoints

The server SHALL serve static assets at `/`, room creation at `ROOM_ENDPOINT`, and WebSocket
upgrades at `WS_ENDPOINT`; exact paths SHALL take precedence over the `/` catch-all.

#### Scenario: Static assets are served

- **WHEN** a client requests the root path for an existing asset
- **THEN** the server returns that asset

#### Scenario: Unknown asset yields 404

- **WHEN** a client requests a path that does not exist under the assets directory
- **THEN** the server returns 404

### Requirement: Room creation response

`POST` to `ROOM_ENDPOINT` SHALL return HTTP 200 with the new room UUID as plain text and
SHALL ignore the request body; any other method SHALL return 405 with an empty body.

#### Scenario: Room creation returns a UUID

- **WHEN** a client posts to the room endpoint
- **THEN** the response is 200 with a parseable UUID in the body

#### Scenario: Other methods are rejected

- **WHEN** a client sends a non-POST request to the room endpoint
- **THEN** the response is 405 with an empty body and no Allow header

### Requirement: JSON envelope

Every WebSocket application payload SHALL be a JSON text frame of the form
`{"type": <number>, "data": ...}`; malformed JSON SHALL be logged and ignored.

#### Scenario: Malformed client JSON is ignored

- **WHEN** a client sends a text frame that is not valid JSON
- **THEN** the server logs the error and keeps the connection open

#### Scenario: Unknown message type is ignored

- **WHEN** a client sends a valid envelope with an unrecognized type
- **THEN** the server ignores the message without closing the connection

### Requirement: Client input message

Client messages of type 200 SHALL carry `{"key": ..., "state": ...}`; only `UP`, `DOWN`,
`LEFT`, and `RIGHT` SHALL affect movement.

#### Scenario: Key event is applied

- **WHEN** a client sends a type 200 message with a recognized key and state
- **THEN** the matching movement state changes for that player

#### Scenario: Unknown key is ignored

- **WHEN** a client sends a type 200 message with an unrecognized key
- **THEN** the message is ignored and the connection stays open

### Requirement: Server message types

Server messages SHALL use JOIN_SUCCESS (4) with `playerId`, `roomId`, `width`, `height`,
`radius`, `color`, and `tickRate`; UPDATE (100) with `players`, `width`, `height`, and
`tick`; and PRIVATE (110) with the publisher's payload. FAILURE (11) and ROOM_CLOSE (9)
SHALL remain reserved and SHALL NOT be sent.

#### Scenario: JOIN_SUCCESS shape

- **WHEN** a player is admitted
- **THEN** the JOIN_SUCCESS payload contains all join fields and the world dimensions

#### Scenario: UPDATE shape

- **WHEN** a tick snapshot is published
- **THEN** the UPDATE payload contains the player list with id, position, radius, and color, plus world dimensions and tick

#### Scenario: PRIVATE shape

- **WHEN** a targeted payload is published
- **THEN** the PRIVATE frame carries that payload to the target only

#### Scenario: Reserved types are unsent

- **WHEN** any room behavior occurs
- **THEN** the server never sends FAILURE or ROOM_CLOSE frames

### Requirement: Message type numbering

Message numbers SHALL be: `MsgJoin` 2, `MsgJoinSuccess` 4, `MsgRoomClose` 9, `MsgFailure`
11, `MsgUpdate` 100, `MsgPrivate` 110, `MsgPlayerKeyDown` 200; clients SHALL mirror these
numbers.

#### Scenario: Numbers stay stable

- **WHEN** a client decodes a server frame or encodes an input event
- **THEN** the numeric type is one of the fixed values above

### Requirement: WebSocket handshake

The server SHALL complete the WebSocket upgrade before validating the room; it SHALL accept
same-origin connections unless the Origin header is absent, select no subprotocol, disable
compression, limit inbound messages to 32 KiB, and impose no application ping or read
deadline.

#### Scenario: Bad room closes after upgrade

- **WHEN** a client connects with a missing, invalid, or unknown room identifier
- **THEN** the upgrade returns 101 and the server closes the socket with status 1014

#### Scenario: Oversized message closes the socket

- **WHEN** a client sends a message larger than 32 KiB
- **THEN** the server closes the socket with status 1009

#### Scenario: Cross-origin upgrade is rejected

- **WHEN** a browser sends an Origin header for a different host
- **THEN** the upgrade is rejected with an HTTP error

### Requirement: Player identity

The server SHALL use `X-Player-Id` when it is a valid UUID, generate a new UUID when the
header is absent or invalid, and reject a duplicate identifier.

#### Scenario: Valid header selects the identifier

- **WHEN** a non-browser client sends a valid `X-Player-Id`
- **THEN** the room uses that identifier for the player

#### Scenario: Invalid header gets a new identifier

- **WHEN** the header is absent or not a UUID
- **THEN** the server generates a new player identifier

### Requirement: Close codes

The server SHALL close invalid, unknown, full, or duplicate joins with 1014; room timeout with
1001 and reason `room closed`; oversized messages with 1009; and write failures with 1011. It
SHALL treat normal and going-away closures as silent cleanup. Server shutdown MAY yield 1006.

#### Scenario: Admission failures use 1014

- **WHEN** a join is invalid, targets an unknown or full room, or duplicates an identifier
- **THEN** the connection closes with status 1014 and a reason

#### Scenario: Write failure uses 1011

- **WHEN** a server write to a connection fails while the room is still open
- **THEN** the server closes that socket with status 1011
