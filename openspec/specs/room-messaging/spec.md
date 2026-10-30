# room-messaging Specification

## Purpose
Defines how outbound frames are staged per player and delivered, including broadcast
snapshots, targeted private messages, latest-per-type coalescing, and extension channels.

## Requirements

### Requirement: Typed outbox channels

Each connection SHALL have an outbox holding one latest payload per snapshot type:
`SystemType`, `PrivateType`, `BroadcastType`, and `CustomType`; the writer SHALL send pending
types in ascending numeric order.

#### Scenario: Writer drains channels in numeric order

- **WHEN** all four snapshot types have a pending payload for one player
- **THEN** the writer sends them in the order System, Private, Broadcast, Custom

### Requirement: Latest-per-type coalescing

Publishing a payload for a type that already has a pending payload SHALL replace it, so
repeated publishes before the next write collapse to the newest payload and no unbounded
queue grows.

#### Scenario: Repeated private publishes collapse

- **WHEN** two private payloads are published for the same player before the writer drains the private slot
- **THEN** the player receives one private frame containing only the newer payload

#### Scenario: Slow client skips intermediate updates

- **WHEN** a player connection cannot keep up with the tick rate
- **THEN** only the latest broadcast snapshot is pending and older snapshots are silently skipped

### Requirement: Shared broadcast snapshot

Each tick with at least one player SHALL increment the room tick, serialize one UPDATE
payload, and publish the same payload to every player's `BroadcastType` slot. A tick with no
players SHALL publish nothing.

#### Scenario: All players receive the same snapshot

- **WHEN** a room with multiple players advances one tick
- **THEN** every player receives byte-identical UPDATE payloads with the same tick number

#### Scenario: Empty room publishes nothing

- **WHEN** a tick occurs while the room has no players
- **THEN** no snapshot is serialized or published

### Requirement: Targeted private messages

`SendTo` SHALL stage a payload in the target player's `PrivateType` slot, and `PublishTo`
SHALL stage a payload in the requested snapshot slot; the payload SHALL be a JSON envelope of
the given message type and data.

#### Scenario: Only the target receives a private frame

- **WHEN** a private payload is published for one player
- **THEN** that player receives a PRIVATE frame with the payload and no other player receives it

#### Scenario: Private data never enters the broadcast

- **WHEN** a private payload is staged before a tick
- **THEN** the shared UPDATE payload does not contain that private data

### Requirement: Outbound error semantics

`PublishTo` SHALL return a room-closed error when the room is closed or its context is done,
a player-not-found error when the target is missing or stopped, and nil when the payload has
been staged. A nil result does not acknowledge delivery, and a player stopping concurrently
may cause the staged frame to be dropped.

#### Scenario: Unknown player is reported

- **WHEN** a payload is published for an identifier that is not in the room
- **THEN** the call returns a player-not-found error

#### Scenario: Stopped player is reported

- **WHEN** a payload is published for a player whose connection has stopped
- **THEN** the call returns a player-not-found error

#### Scenario: Closing room is reported

- **WHEN** a payload is published after the room context is canceled
- **THEN** the call returns a room-closed error

#### Scenario: Publish racing a stop may drop the frame

- **WHEN** a player stops between validation and staging
- **THEN** the call may return nil while no frame is delivered

### Requirement: Extension channel

A new outbound channel SHALL be added by declaring a snapshot type before `TypeCount` and
publishing through `PublishTo`; no other wiring SHALL be required. `CustomType` SHALL remain
reserved as the general extension slot.

#### Scenario: Custom channel is delivered

- **WHEN** a payload is published for a custom snapshot type
- **THEN** the writer sends it in numeric order between the built-in channels
