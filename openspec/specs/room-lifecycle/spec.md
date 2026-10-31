# room-lifecycle Specification

## Purpose
Defines how rooms are created, registered, admit players, assign spawn slots, and terminate
through timeout, server shutdown, or connection loss.

## Requirements

### Requirement: Room creation and registration

Creating a room SHALL allocate a new identifier, start a room actor with the configured
capacity and timeout, and register the room under that identifier until it closes.

#### Scenario: New room is registered
- **WHEN** a client creates a room
- **THEN** a room actor starts and the room is reachable by the returned identifier

#### Scenario: Closed room is unregistered
- **WHEN** a room closes for any reason
- **THEN** its identifier is removed from the server registry and later joins report an unknown room

### Requirement: Join admission

A join SHALL be rejected when the room is unknown, closed, at capacity, or already contains the
joining player identifier; the server SHALL close that WebSocket with status 1014 and a reason,
and the rejection SHALL NOT stop the room actor or disturb other players.

#### Scenario: Full room rejects the join
- **WHEN** a player joins a room that already has `ROOM_CAPACITY` players
- **THEN** the connection closes with status 1014 and the room keeps serving its players

#### Scenario: Duplicate player identifier is rejected
- **WHEN** a player joins with an `X-Player-Id` already present in the room
- **THEN** the connection closes with status 1014 and a reason naming the duplicate

#### Scenario: Closed room rejects the join
- **WHEN** a player attempts to join a room that is closing or closed
- **THEN** the connection closes with status 1014 and a reason naming the closed room

#### Scenario: Rejected join leaves the room running
- **WHEN** a join is rejected for any admission reason
- **THEN** the room keeps ticking, publishing snapshots, and admitting valid joins

### Requirement: Spawn slot assignment

On admission the room SHALL place the player at the first free slot of a ring of
`0.32 * min(world width, world height)` around the center, assign radius `25`, and assign the
palette color for that slot, repeating the eight palette colors.

#### Scenario: First player spawns on the ring
- **WHEN** the first player joins an empty room
- **THEN** the player's position is on the spawn ring, the radius is 25, and the color is the
  first palette entry

#### Scenario: Occupied slots are skipped
- **WHEN** a player joins while the preferred spawn slot holds another player within 2.5 player
  radii
- **THEN** the next free slot on the ring is chosen

#### Scenario: Palette wraps for large capacity
- **WHEN** a room has capacity greater than 8 and a player joins on slot 8
- **THEN** the color repeats the first palette entry

### Requirement: Join success and writer start

An admitted player SHALL receive a JOIN_SUCCESS frame with `playerId`, `roomId`, `width`,
`height`, `radius`, `color`, and `tickRate` before any UPDATE; the room SHALL then start the
player's writer and stage snapshots for that player.

#### Scenario: Admitted player receives join data
- **WHEN** a player is admitted
- **THEN** the player receives one JOIN_SUCCESS frame with all join fields and then an UPDATE frame

### Requirement: Disconnect cleanup

When a player connection ends the server SHALL stop that player's connection and remove the
player from membership, freeing capacity; each tick SHALL also reap players whose connection has
stopped.

#### Scenario: Disconnect frees capacity
- **WHEN** a connected player disconnects from a full room
- **THEN** another player can join once the leave is processed, without restarting the room

#### Scenario: Stopped players are reaped
- **WHEN** a player's connection stops before a tick
- **THEN** that tick excludes the player from movement, collision detection, and the snapshot

### Requirement: Scheduler-driven room timeout

The room SHALL schedule one one-shot event for `ROOM_TIMEOUT` when its actor starts; when the
event fires the room SHALL close and every connected player SHALL receive close status 1001 with
reason `room closed`.

#### Scenario: Room expires after the timeout
- **WHEN** a room has been running for `ROOM_TIMEOUT` without closing
- **THEN** the room closes, connected players receive status 1001 with reason `room closed`, and
  the room is unregistered

### Requirement: Close ordering and idempotence

A close SHALL run at most once: it SHALL stop the room scheduler, clear membership, stop every
player, attempt close handshakes for all players concurrently, cancel the room context, and
unregister the room. Joins during close SHALL fail as room closed.

#### Scenario: Repeated close is a no-op
- **WHEN** close is triggered again after it has started
- **THEN** no additional handshakes, context cancellations, or unregistration occur

#### Scenario: Join during close fails
- **WHEN** a join is attempted after close has started but before the room is unregistered
- **THEN** the join fails with a room-closed error and the connection closes with status 1014

### Requirement: Server shutdown closes rooms

On server shutdown the server SHALL cancel the shared context before closing rooms, so room
actors close through context cancellation and active clients may observe status 1006 rather than
1001.

#### Scenario: Shutdown cancels rooms
- **WHEN** the process receives a termination signal or its parent context is canceled
- **THEN** the server cancels the shared context, rooms close through context cancellation, and
  attached clients may observe status 1006
