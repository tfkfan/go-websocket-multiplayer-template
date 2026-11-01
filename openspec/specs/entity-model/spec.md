# entity-model Specification

## Purpose

Defines how a room, its spatial index, and its outbound delivery treat world objects beyond the
single player implementation: heterogeneous membership, connection-less objects, per-instance
behavior and appearance, deterministic ticking across kinds, snapshot contribution, and a
reusable layer that carries no Circles rules.

## Requirements

### Requirement: Heterogeneous room membership

A room SHALL hold world objects of more than one kind, keyed by a stable identifier, as long as
each object exposes a shape and solidity for the spatial index and can be advanced by one tick.
Admission SHALL reject a second object that reuses an existing identifier.

#### Scenario: Non-player object joins

- **WHEN** a scripted world object is admitted to a room that already holds players
- **THEN** it becomes a member and is present in the spatial index

#### Scenario: Duplicate identifier is rejected

- **WHEN** two objects with the same identifier are admitted
- **THEN** the second admission fails and membership is unchanged

### Requirement: Connection-backed and connection-less members

A member MAY have an outbound connection. Targeted delivery SHALL address only members that have
one, and addressing a member without a connection SHALL fail without removing it from the room
or affecting ticking and indexing.

#### Scenario: Connection-less object is not addressable

- **WHEN** a targeted message is sent to a member that has no connection
- **THEN** the send fails and the member remains a room member

#### Scenario: Connection-less object still participates

- **WHEN** a tick runs with a connection-less solid object in the room
- **THEN** the object is updated, indexed, and able to contact other members

### Requirement: Per-instance behavior and appearance

Movement behavior, speed, size, and appearance SHALL be selected per member instance rather than
by the room knowing a concrete kind, and control input SHALL be replaceable per instance. Two
members of the same role SHALL be able to differ in every one of those properties without a room
change.

#### Scenario: Same role with different profiles

- **WHEN** two players are admitted with different speeds, radii, and colors
- **THEN** each moves at its own speed and reports its own size and color

#### Scenario: Scripted control replaces keyboard control

- **WHEN** a player instance is driven by scripted input instead of keyboard state
- **THEN** it moves through the same movement path as a keyboard-driven instance

### Requirement: Deterministic tick over mixed kinds

Every tick SHALL advance every member exactly once in ascending stable-key order regardless of
kind, and the spatial index SHALL be rebuilt from all solid members before contact handling. A
tick's outcome SHALL NOT depend on the order in which members were admitted.

#### Scenario: Mixed kinds advance once per tick

- **WHEN** a room holds players and a scripted object and a tick runs
- **THEN** every member is advanced exactly once

#### Scenario: Mixed kinds are order independent

- **WHEN** the same members are admitted in different orders and the same ticks run
- **THEN** every member ends in the same state

### Requirement: Snapshot contribution

Only members that provide a snapshot contribution SHALL appear in an outbound snapshot payload.
A member that provides no contribution SHALL NOT alter the payload, and contribution SHALL be
selected per kind rather than by the room asserting concrete types.

#### Scenario: Players contribute to UPDATE

- **WHEN** an UPDATE payload is built for a room that holds players
- **THEN** every player appears in the payload exactly once

#### Scenario: Non-contributing member is excluded

- **WHEN** a scripted object that does not contribute is a room member during a tick
- **THEN** the UPDATE payload is unchanged by its presence

### Requirement: Game-rule-free reusable layer

The layer that provides membership, snapshot staging, outbox, writer, ticker, timeout, close
ordering, room storage, and matchmaking SHALL NOT depend on the player, game-room, or game-event
packages, or on their rules. A dependency check SHALL fail when that boundary is crossed.

#### Scenario: Boundary is enforced

- **WHEN** the dependency check runs and a reusable package imports a game-rule package
- **THEN** the check fails

#### Scenario: Reusable layer is exercised without game code

- **WHEN** the reusable primitive, room, session, and matchmaker tests run
- **THEN** they use only test doubles and no game-rule package

### Requirement: Preserved connection behavior

Separating connection state from members SHALL NOT change delivery behavior: one writer per
connection, latest-per-type coalescing, close codes, and frame ordering SHALL remain exactly as
specified for the connection layer.

#### Scenario: Frames are unchanged

- **WHEN** the same client sequence runs after the restructure
- **THEN** JOIN_SUCCESS, UPDATE, and close codes are identical to the previous behavior

#### Scenario: One writer per connection

- **WHEN** a member with a connection is admitted
- **THEN** exactly one writer goroutine drains that connection's outbox
