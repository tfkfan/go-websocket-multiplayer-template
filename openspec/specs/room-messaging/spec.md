# room-messaging Specification

## Purpose

Defines the per-player outbound mailbox, its ordered drain, coalescing rules, the shared
broadcast snapshot, the writer that delivers frames, targeted private messages, error
semantics for staged sends, and the extension channel for new snapshot types.

## Requirements

### Requirement: Typed outbox channels

Each outbound connection SHALL own an outbox holding at most one payload per snapshot type, with
the numeric order `SystemType`, `PrivateType`, `BroadcastType`, `CustomType`, bounded by
`TypeCount`. Publishing a type at or above `TypeCount` SHALL be dropped, and publishing after
the connection context is done SHALL be dropped. A publish SHALL wake a single coalesced signal.

#### Scenario: Writer drains channels in numeric order

- **WHEN** payloads of several types are waiting in one outbox
- **THEN** they are read in ascending snapshot-type order

#### Scenario: Unbounded type is dropped

- **WHEN** a payload is published with a snapshot type at or above `TypeCount`
- **THEN** the outbox is unchanged

### Requirement: Latest-per-type coalescing

Each outbox slot SHALL hold only the newest payload of that type. A newer publish of the same
type SHALL replace the pending payload, and a slow client SHALL skip intermediate snapshots
rather than queue them.

#### Scenario: Repeated private publishes collapse

- **WHEN** two private payloads are staged before the writer drains the slot
- **THEN** only the second payload is delivered

#### Scenario: Slow client skips intermediate updates

- **WHEN** several ticks publish broadcasts before a client's writer drains
- **THEN** the writer delivers the newest broadcast and skips the rest

### Requirement: Shared broadcast snapshot

Each tick SHALL marshal one UPDATE and stage the same bytes for every connected member on the
broadcast channel. An empty room SHALL publish nothing.

#### Scenario: All players receive the same snapshot

- **WHEN** a tick snapshot is published to a room with several connected members
- **THEN** every connected member's broadcast slot holds the same payload

#### Scenario: Empty room publishes nothing

- **WHEN** the room has no members
- **THEN** the tick publishes no snapshot

### Requirement: Writer delivery

Each admitted player SHALL have exactly one writer goroutine. The writer SHALL drain the next
payload, and when the outbox is empty SHALL wait on the outbox signal, the player context, or
the room context. A write SHALL be bounded by a 5 second deadline derived from the room
context. A failed write SHALL close that player's socket with status 1011 and end that writer
only; room close SHALL end every writer.

#### Scenario: Writer waits for work

- **WHEN** the outbox is empty
- **THEN** the writer blocks until a payload is published or the player or room stops

#### Scenario: Failed write closes only that player

- **WHEN** writing to one player fails while the room is still running
- **THEN** that player's socket is closed with 1011 and other players continue receiving frames

#### Scenario: Room close ends writers

- **WHEN** the room closes
- **THEN** every writer goroutine returns

### Requirement: Targeted private messages

`SendTo` SHALL publish on the private channel, and `PublishTo` SHALL accept an explicit snapshot
type. Private frames SHALL be delivered only to the named player and SHALL never be included in
the shared UPDATE.

#### Scenario: Only the target receives a private frame

- **WHEN** a private payload is staged for one member
- **THEN** only that member's outbox holds it

#### Scenario: Private data never enters the broadcast

- **WHEN** a tick publishes the shared snapshot after a private send
- **THEN** the broadcast payload does not contain the private data

### Requirement: Outbound error semantics

`SendTo` and `PublishTo` SHALL return `ErrRoomClosed` when the room is closed or its context is
done, and `ErrPlayerNotFound` when the player is not a member or has stopped. A nil return
SHALL mean the payload was staged, not delivered, and a stop racing a send SHALL drop the
frame. These calls SHALL NOT require the actor goroutine.

#### Scenario: Unknown player is reported

- **WHEN** a send targets an identifier that is not a member
- **THEN** the call returns `ErrPlayerNotFound`

#### Scenario: Stopped player is reported

- **WHEN** a send targets a member whose context has stopped
- **THEN** the call returns `ErrPlayerNotFound`

#### Scenario: Closing room is reported

- **WHEN** a send runs while the room is closing
- **THEN** the call returns `ErrRoomClosed`

#### Scenario: Publish racing a stop may drop the frame

- **WHEN** a player stops between the membership check and the publish
- **THEN** the publish is dropped and the call reports no delivery guarantee

### Requirement: Extension channel

New outbound data SHALL be published by adding a snapshot type before `TypeCount` and calling
`PublishTo` with that type and a message type. The outbox and writer SHALL pick the new type up
in numeric order without further changes.

#### Scenario: Custom channel is delivered

- **WHEN** a payload is published with the custom snapshot type
- **THEN** the target's writer delivers it after system, private, and broadcast payloads
