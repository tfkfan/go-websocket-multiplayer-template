# room-lifecycle Specification

## Purpose

Defines how rooms are created and registered, admit players, assign spawn slots, publish join
data, clean up disconnects, expire on timeout, and close when the room or the server stops.

## Requirements

### Requirement: Room creation and registration

Creating a room SHALL allocate a UUID, build a room actor with the configured capacity and
timeout, a bounded spatial index for the fixed world, a scheduler, and a spawner, register it in
the server registry under its identifier, start the actor goroutine, and return the identifier
as plain text. The actor SHALL run until it closes or the server context is canceled.

#### Scenario: New room is registered

- **WHEN** a client creates a room
- **THEN** an actor starts, the identifier is returned, and a connection can target that room

#### Scenario: Closed room is unregistered

- **WHEN** a room closes
- **THEN** its identifier is removed from the registry once the close sequence completes

### Requirement: Join admission

A join SHALL be rejected when the room is closed, when the joining identifier is already a member,
or when membership is at `ROOM_CAPACITY`. Admission checks SHALL run in that order: closed,
duplicate identifier, capacity.

#### Scenario: Closed room rejects the join

- **WHEN** a join is applied to a room closed by timeout or shutdown
- **THEN** the join is rejected and the player is not added

#### Scenario: Duplicate player identifier is rejected

- **WHEN** a join uses an identifier already in the room
- **THEN** the join is rejected before any capacity check

#### Scenario: Full room rejects the join

- **WHEN** membership equals capacity and a new identifier joins
- **THEN** the join is rejected and membership is unchanged

#### Scenario: Rejected join leaves membership unchanged

- **WHEN** any join is rejected
- **THEN** the room's membership set and length are unchanged

### Requirement: Spawn slot geometry and selection

Each admitted player SHALL be assigned one of `ROOM_CAPACITY` spawn slots on a ring centered on
the world center, with radius `min(width, height) * 0.32` and slot angle
`2*pi*slot/capacity - pi/2`. A slot SHALL be considered occupied when any member is within
`2.5 * 25` world units of it. The spawner SHALL start from a rotating counter, pick the first
free slot, and fall back to the counter slot when all slots are occupied.

#### Scenario: Player spawns on the ring

- **WHEN** the first player joins an empty room
- **THEN** its position lies on the spawn ring with the configured spawn radius

#### Scenario: Occupied slots are skipped

- **WHEN** the next candidate slot is occupied by a nearby member
- **THEN** the spawner advances to the next free slot

#### Scenario: All slots occupied falls back

- **WHEN** every slot is considered occupied
- **THEN** the spawner uses the rotating counter slot without failing admission

### Requirement: Spawn color assignment

An admitted player's color SHALL come from the room palette at the index `slot` modulo palette
length.

#### Scenario: Palette repeats for large capacity

- **WHEN** capacity exceeds the palette length and a high-numbered slot is used
- **THEN** the color is the palette entry at that slot modulo the palette length

### Requirement: Join success and writer start

An admitted player SHALL receive JOIN_SUCCESS published on the system channel with `playerId`,
`roomId`, world `width` and `height`, spawn `radius` and `color`, and the compiled `tickRate`,
and the room SHALL start exactly one writer goroutine for that player.

#### Scenario: Admitted player receives join data

- **WHEN** a player is admitted
- **THEN** its first outbound frame is JOIN_SUCCESS with that player's spawn state

#### Scenario: Writer starts with the player

- **WHEN** a player is admitted
- **THEN** updates staged for that player are written by its writer goroutine

### Requirement: Rejected join disposition

A join rejected by the room's own admission checks SHALL NOT publish JOIN_SUCCESS and SHALL NOT
close the socket. Applying the failed command SHALL stop the room actor, so the room stops
ticking, keeps its registry entry, and any later join to that identifier also receives no join
data.

#### Scenario: Full room join stops the actor

- **WHEN** a join is rejected because the room is at capacity
- **THEN** no JOIN_SUCCESS or close frame is sent and the room stops publishing updates

#### Scenario: Duplicate identifier join stops the actor

- **WHEN** a join is rejected because the identifier is already a member
- **THEN** the existing member is unaffected and the new connection receives no join data

### Requirement: Disconnect cleanup

A leave command SHALL remove the player from membership and stop its context. Stopped players
SHALL also be reaped at the start of each tick, and a reaped player SHALL receive no further
broadcast snapshot. Removed players SHALL NOT be included in later snapshots.

#### Scenario: Disconnect frees capacity

- **WHEN** an admitted player disconnects
- **THEN** its identifier leaves membership and another player can join in its place

#### Scenario: Stopped players are reaped

- **WHEN** a player's context is stopped without a leave command
- **THEN** the next tick removes it from membership before movement and snapshots

### Requirement: Scheduler-driven room timeout

A room SHALL schedule a one-shot timeout of `ROOM_TIMEOUT` when its actor starts. When the
timeout fires, the room SHALL close and every member socket SHALL be closed with status 1001.
The deadline SHALL be fixed from creation; player activity SHALL NOT extend it.

#### Scenario: Room expires after the timeout

- **WHEN** `ROOM_TIMEOUT` elapses with the room open
- **THEN** the room closes, members are disconnected with 1001, and the registry entry is removed

### Requirement: Close ordering and idempotence

Closing SHALL be idempotent. On the first close the room SHALL shut down its scheduler, clear
and stop membership, close every member socket concurrently with status 1001 and reason
`room closed`, wait for those close handshakes, cancel the room context, and invoke the
registry-removal callback.

#### Scenario: Repeated close is a no-op

- **WHEN** close is invoked a second time
- **THEN** it returns without repeating cleanup or callbacks

#### Scenario: Join during close is not admitted

- **WHEN** a join is submitted while the room is closing
- **THEN** the player is never admitted and the socket ends without join data

### Requirement: Server shutdown closes rooms

Canceling the server context SHALL cancel every room context. The server SHALL stop accepting
new requests and the process SHALL wait for shutdown with a bounded deadline.

#### Scenario: Shutdown cancels rooms

- **WHEN** the server receives a termination signal or its context is canceled
- **THEN** each room closes through its context-canceled branch and members are disconnected
