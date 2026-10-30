# room-gameplay Specification

## Purpose
Defines the authoritative world model: input state, movement, tick cadence, and the explicit
absence of collision handling.

## Requirements

### Requirement: World bounds and containment

The room SHALL use a 1200 by 800 world with a player radius of 25; every player position
SHALL stay within `[radius, width-radius]` horizontally and `[radius, height-radius]`
vertically.

#### Scenario: Player is clamped at a wall

- **WHEN** a player holds a direction long enough to reach the world edge
- **THEN** the position stops at the radius-inset boundary and the player remains fully visible

#### Scenario: Spawn is inside the world

- **WHEN** a player is spawned on the ring
- **THEN** its circle is fully contained in the 1200 by 800 world

### Requirement: Latest-state key input

Key events for `UP`, `DOWN`, `LEFT`, and `RIGHT` SHALL set or clear the matching movement
bit; unknown keys SHALL be ignored; repeated key events SHALL collapse to the latest state
rather than queueing.

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

On each tick the room SHALL move each player at 260 units per second in the pressed
directions, cancel opposing directions, normalize diagonal movement, and clamp the result to
the world bounds.

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

The room actor SHALL advance the simulation 30 times per second, and each tick SHALL use at
most 0.25 seconds of elapsed time.

#### Scenario: Delayed tick does not teleport players

- **WHEN** a tick is delayed well beyond its period
- **THEN** the elapsed time used for movement is capped at 0.25 seconds and the player moves at most 65 units in that tick

### Requirement: No collision handling

Players SHALL NOT collide or block each other; overlapping positions and circles are allowed.

#### Scenario: Players can overlap

- **WHEN** two players move to the same coordinates
- **THEN** both remain in the room and both appear at those coordinates in the next snapshot
