# room-gameplay Specification

## Purpose

Defines the authoritative simulation each room runs: world containment, latest-state key input,
server-side movement, tick cadence and time capping, and the ordering and outcome of the
collision pass that feeds the published snapshot.

## Requirements

### Requirement: World bounds and containment

The room SHALL own a fixed `1200 x 800` world. Every player position SHALL stay inside the
rectangle inset by that player's radius, including after spawn, movement, and collision
correction.

#### Scenario: Player is clamped at a wall

- **WHEN** a player moves toward a world edge
- **THEN** its position stops at the edge offset by its radius

#### Scenario: Spawn is inside the world

- **WHEN** a player is admitted
- **THEN** its spawn position satisfies the inset bounds

### Requirement: Latest-state key input

Movement input SHALL be a single atomic bitmask with one bit per direction. Pressing and
releasing SHALL set and clear the corresponding bit. Repeating the same state SHALL be
idempotent, and an unknown direction SHALL leave the mask unchanged.

#### Scenario: Press and release update movement bits

- **WHEN** a client sends a press and later a release for a direction
- **THEN** the bit is set, then cleared

#### Scenario: Repeated events collapse

- **WHEN** two presses for the same direction arrive without a release
- **THEN** the movement state is the set state after both

#### Scenario: Unknown key is ignored

- **WHEN** a direction outside `UP`, `DOWN`, `LEFT`, and `RIGHT` is applied
- **THEN** the mask is unchanged

### Requirement: Server-side movement

The server SHALL move each player by `260` world units per second along the held directions,
integrating over the tick's elapsed time. Diagonal input SHALL be normalized by `sqrt(2)` so it
is not faster than axis movement, and opposite directions SHALL cancel. Positions SHALL be
clamped to the inset world bounds after integration.

#### Scenario: Single direction speed

- **WHEN** a player holds one direction for `dt` seconds
- **THEN** it moves `260 * dt` world units along that axis

#### Scenario: Diagonal is normalized

- **WHEN** a player holds two perpendicular directions
- **THEN** the resulting speed equals the single-direction speed

#### Scenario: Opposing directions cancel

- **WHEN** a player holds both directions of one axis
- **THEN** no movement occurs on that axis

#### Scenario: Movement is clamped

- **WHEN** movement would carry a player past the world edge
- **THEN** the position is clamped to the inset boundary

### Requirement: Tick cadence and time cap

Each room SHALL tick at `30` Hz and SHALL use the elapsed wall time since the previous tick as
`dt`. A `dt` greater than `0.25` seconds SHALL be capped at `0.25` seconds so a delayed tick
cannot teleport players.

#### Scenario: Delayed tick does not teleport players

- **WHEN** the actor is delayed well beyond one tick interval
- **THEN** that tick advances movement by at most `0.25` seconds of travel

### Requirement: Collision pass ordering

Each tick SHALL run in a fixed order: reap stopped players, move every member in player-key
order, rebuild the spatial index from the moved positions, detect contacts from that index,
apply at most one correction per contact, and publish the snapshot. Corrections SHALL NOT be
written into the index during the pass; they enter it on the next tick's rebuild.

#### Scenario: Contacts use the refreshed index

- **WHEN** players move into contact during a tick
- **THEN** detection for that same tick uses their post-movement positions

#### Scenario: Corrections enter the index on the next tick

- **WHEN** a correction moves a player
- **THEN** the published snapshot reflects it and the index reflects it after the next rebuild

### Requirement: One-sided collision correction

For each solid contact the room SHALL move exactly one body: the only mover when one player
moved, the later mover in movement order when both moved, and neither when neither moved. The
moved body SHALL be displaced along the contact normal by the penetration depth and clamped to
the inset world bounds. A player that did not move SHALL never be repositioned by a collision.

#### Scenario: Mover keeps moving into contact

- **WHEN** a moving player contacts a stationary player
- **THEN** the moving player is placed at the contact surface and keeps its own motion

#### Scenario: Tangential movement is preserved

- **WHEN** a player with an axis and a tangential direction contacts a body
- **THEN** it retains the tangential component after correction

#### Scenario: Stationary player is untouched

- **WHEN** two players overlap and only one moved
- **THEN** the stationary player's position is unchanged

#### Scenario: Two moving players keep their own motion

- **WHEN** two players move into each other
- **THEN** the later mover in movement order yields and the earlier mover's position is kept

#### Scenario: Corrected movement stays inside the world

- **WHEN** a correction pushes a player toward a world edge
- **THEN** the corrected position is clamped inside the world

### Requirement: Deterministic collision resolution

Movement SHALL process players in ascending key order, and contacts SHALL be deduplicated and
reported in `(first key, second key)` order. A tick's outcome SHALL NOT depend on membership
insertion order.

#### Scenario: Results are order independent

- **WHEN** the same players and input are added to two rooms in different orders and ticked
- **THEN** every player ends at the same position in both rooms

#### Scenario: Head-on contact favors the earlier mover

- **WHEN** two players move straight into each other
- **THEN** the player that moved earlier keeps its position and the later one is displaced

### Requirement: Snapshot after correction

The UPDATE published for a tick SHALL contain the positions after movement and collision
correction for every current member.

#### Scenario: Correction is visible in the same tick

- **WHEN** a correction changes a player's position during a tick
- **THEN** that tick's UPDATE reports the corrected position
