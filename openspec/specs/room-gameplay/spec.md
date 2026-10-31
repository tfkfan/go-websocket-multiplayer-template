# room-gameplay Specification

## Purpose
Defines the authoritative world model: input state, movement, tick cadence, and one-sided
collision response over the room's shared spatial index.

## Requirements

### Requirement: World bounds and containment

The room SHALL use a 1200 by 800 world with a player radius of 25; every player position SHALL
stay within `[radius, width-radius]` horizontally and `[radius, height-radius]` vertically.

#### Scenario: Player is clamped at a wall
- **WHEN** a player holds a direction long enough to reach the world edge
- **THEN** the position stops at the radius-inset boundary and the player remains fully visible

#### Scenario: Spawn is inside the world
- **WHEN** a player is spawned on the ring
- **THEN** its circle is fully contained in the 1200 by 800 world

### Requirement: Latest-state key input

Key events for `UP`, `DOWN`, `LEFT`, and `RIGHT` SHALL set or clear the matching movement bit;
unknown keys SHALL be ignored; repeated key events SHALL collapse to the latest state rather
than queueing.

#### Scenario: Press and release update movement bits
- **WHEN** the client sends a key event setting `RIGHT` and later clears it
- **THEN** the player moves right between the two events and stops after the clear

#### Scenario: Unknown key is ignored
- **WHEN** the client sends a key event with a key other than the four movement keys
- **THEN** no movement bit changes

#### Scenario: Repeated events collapse
- **WHEN** the client sends the same press event multiple times
- **THEN** the room keeps a single pressed state for that direction

### Requirement: Server-side movement

On each tick the room SHALL move each player at 260 units per second in the pressed directions,
cancel opposing directions, normalize diagonal movement, and clamp the result to the world
bounds.

#### Scenario: Single direction speed
- **WHEN** a player holds one direction for one second of simulated time
- **THEN** the position changes by 260 units in that direction, subject to world clamping

#### Scenario: Diagonal is normalized
- **WHEN** a player holds two perpendicular directions for one second
- **THEN** the straight-line distance moved is 260 units, not 260 times the square root of two

#### Scenario: Opposing directions cancel
- **WHEN** a player holds `LEFT` and `RIGHT` at the same time
- **THEN** the horizontal position does not change

#### Scenario: Movement is clamped
- **WHEN** a movement step would cross the world boundary
- **THEN** the position is clamped to the boundary rather than leaving the world

### Requirement: Tick cadence and time cap

The room actor SHALL advance the simulation 30 times per second, and each tick SHALL use at most
0.25 seconds of elapsed time.

#### Scenario: Delayed tick does not teleport players
- **WHEN** a tick is delayed well beyond its period
- **THEN** the elapsed time used for movement is capped at 0.25 seconds and the player moves at
  most 65 units in that tick

### Requirement: Collision pass ordering

Each tick SHALL reap stopped players, move every player, refresh the room's spatial index once,
and then detect contacts against that refreshed index; the tick's collision corrections SHALL NOT
rebuild or update the index, which catches up on the next tick.

#### Scenario: Contacts use the refreshed index
- **WHEN** two players move into contact during a tick
- **THEN** the contact is detected against the positions produced by that tick's movement

#### Scenario: Corrections enter the index on the next tick
- **WHEN** a tick's collision response moves a player
- **THEN** the published snapshot carries the corrected position and the next tick's refresh
  indexes it

### Requirement: One-sided collision correction

For each contact the room SHALL move exactly one body out of overlap along the contact normal:
the only mover when one player moved, or the later mover when both moved; the other body SHALL
NOT be repositioned, tangential movement SHALL be preserved, and the corrected position SHALL be
clamped inside the world.

#### Scenario: Mover keeps moving into contact
- **WHEN** a player moves toward another player
- **THEN** it keeps the part of its movement that does not overlap, the other player does not
  move, and the UPDATE reports both at non-overlapping positions

#### Scenario: Tangential movement is preserved
- **WHEN** a player moves diagonally into another player
- **THEN** moving the player out along the contact normal preserves the remaining movement
  component instead of reverting the whole step

#### Scenario: Stationary player is untouched
- **WHEN** a moving player contacts a stationary player
- **THEN** only the moving player is repositioned and the stationary player stays exactly where
  it was

#### Scenario: Two moving players keep their own motion
- **WHEN** two players move into contact during the same tick
- **THEN** only the later mover is repositioned by that contact, the earlier mover keeps its
  movement, and the final positions do not overlap

#### Scenario: Corrected movement stays inside the world
- **WHEN** a player is moved out of contact near a world edge
- **THEN** the corrected position is clamped inside the world

#### Scenario: Collision correction precedes the snapshot
- **WHEN** a tick detects a contact
- **THEN** the UPDATE frame carries the constrained positions

### Requirement: Deterministic collision resolution

The room SHALL move players and resolve contacts in stable key order, so a head-on pair resolves
in favor of the earlier mover and results do not depend on membership iteration order.

#### Scenario: Results are order independent
- **WHEN** the same players and inputs are applied in different membership orders
- **THEN** every player ends the tick at the same position

#### Scenario: Head-on contact favors the earlier mover
- **WHEN** two players move into a head-on contact
- **THEN** the later mover is repositioned and the earlier mover keeps its position
