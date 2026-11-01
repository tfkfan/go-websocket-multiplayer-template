# matchmaking Specification

## Purpose

Defines the Matchmaker's waiting queue: room storage, queue admission, blocking capacity-based
match formation, atomic placement of a full batch, disconnect removal from the queue, thread
safety, shutdown release, and coexistence with direct room joins.

## Requirements

### Requirement: Matchmaker room storage

The server object SHALL store every room it creates under its UUID, SHALL load a stored room for a
direct join, and SHALL remove a room once its run loop closes.

#### Scenario: Created rooms are stored

- **WHEN** a room is created directly or formed by a match
- **THEN** it is loadable by its identifier while it runs

#### Scenario: Closed rooms are removed

- **WHEN** a stored room's run loop closes
- **THEN** its identifier no longer resolves to a room

### Requirement: Queue admission

A WebSocket connection without a room parameter SHALL be queued as a pending player and SHALL
receive no join data until it is placed in a match.

#### Scenario: Queued player waits

- **WHEN** a connection opens without a room parameter
- **THEN** it is queued and receives no join data while waiting

#### Scenario: Direct join bypasses the queue

- **WHEN** a connection opens with a valid room parameter
- **THEN** it joins that room directly and the queue length is unchanged

### Requirement: Blocking match formation

Match formation SHALL block until at least the room capacity of players is queued, then atomically
take the oldest subslice of exactly that capacity and create a room. A waiting caller SHALL return
only after its room exists and it has been placed in it.

#### Scenario: Waits below capacity

- **WHEN** fewer players than capacity are queued
- **THEN** no room is created and the callers keep waiting

#### Scenario: Forms a room at capacity

- **WHEN** the capacity-th player queues
- **THEN** one room is created from the oldest queued players

#### Scenario: Excess players stay queued

- **WHEN** more players than capacity are waiting
- **THEN** the remainder stays queued in arrival order for the next match

#### Scenario: Caller returns after placement

- **WHEN** a waiting call returns
- **THEN** its room exists and contains the caller

### Requirement: Atomic batch placement

All queued players in a batch SHALL be admitted to the newly created room before any waiting
caller returns. If admission fails for any member, the room SHALL be discarded and surviving
players SHALL remain queued.

#### Scenario: Full batch is admitted before return

- **WHEN** a match forms
- **THEN** the room contains every member of the batch before callers return

#### Scenario: Failure requeues survivors

- **WHEN** a member disconnects during placement
- **THEN** the room is discarded and the surviving members stay queued for the next match

### Requirement: Disconnect removal from the queue

A pending player whose connection closes SHALL be removed from the queue wherever it sits in the
slice, SHALL NOT disturb the relative order of the remaining players, and SHALL NOT consume a
capacity slot.

#### Scenario: Middle of the queue

- **WHEN** a queued player in the middle disconnects
- **THEN** only that player is removed and the others keep their order

#### Scenario: End of the queue

- **WHEN** the last queued player disconnects
- **THEN** the queue shrinks by one and no capacity slot is consumed

#### Scenario: Replacement can form the batch

- **WHEN** a removed slot is filled by a new connection
- **THEN** the batch can reach capacity again

### Requirement: Thread-safe queue and storage

Queue mutations, room storage, and match formation SHALL be safe under concurrent joins,
disconnects, direct joins, and lookups. A player SHALL be either queued or placed, never both.

#### Scenario: Concurrent joins are consistent

- **WHEN** many players connect at once
- **THEN** no data race occurs and every player is placed exactly once

#### Scenario: Removal racing formation

- **WHEN** a disconnect races batch selection
- **THEN** the player is either still queued or placed, never duplicated

### Requirement: Shutdown releases waiting players

Shutdown SHALL release every blocked match call with an error, clear the waiting queue, and close
the stored rooms.

#### Scenario: Blocked callers return

- **WHEN** the server shuts down while players are queued
- **THEN** every blocked call returns an error instead of waiting

#### Scenario: Rooms close with the queue

- **WHEN** shutdown completes
- **THEN** the queue is empty and stored rooms are closed

### Requirement: Coexistence with direct rooms

Direct room creation and room-parameter joins SHALL remain available and SHALL NOT consume queue
slots, and matchmaking SHALL NOT modify rooms created directly.

#### Scenario: Direct play while players queue

- **WHEN** a player creates and joins a direct room while others wait
- **THEN** the direct flow works and the queue is unchanged

#### Scenario: Matched rooms join the same storage

- **WHEN** a match forms a room
- **THEN** it is stored alongside direct rooms and removed when it closes
