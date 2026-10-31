# spatial-index Specification

## Purpose
Defines the reusable, world-bounded 2D quadtree that stores game bodies and answers shape
overlap, radius, and nearest-point queries with deterministic ordering.

## Requirements

### Requirement: World-bounded construction

The index SHALL be constructed for a finite rectangular world with a node capacity and a maximum
depth; non-positive tuning values SHALL fall back to the defaults, and the zero value SHALL NOT be
used.

#### Scenario: New index is empty
- **WHEN** an index is constructed for a world rectangle
- **THEN** its length is zero and every query returns no bodies

#### Scenario: Invalid tuning falls back to defaults
- **WHEN** an index is constructed with a non-positive node capacity or maximum depth
- **THEN** it behaves as if the default values were supplied

### Requirement: Invalid world rectangle

Construction SHALL fail with a panic for a non-finite, inverted, or zero-area rectangle instead
of returning an unusable index.

#### Scenario: Zero-area world is rejected
- **WHEN** an index is constructed for a rectangle whose width or height is zero
- **THEN** construction fails with a panic

#### Scenario: Non-finite corner is rejected
- **WHEN** an index is constructed for a rectangle with a NaN or infinite corner
- **THEN** construction fails with a panic

### Requirement: Body insertion

`Insert` SHALL store a body under its stable key exactly once and SHALL reject a duplicate key, a
nil body, a body with no shape, and any body whose bounds are not fully inside the world,
reporting false and leaving the index unchanged.

#### Scenario: Bodies are inserted once
- **WHEN** a body is inserted and then a second body with the same key is inserted
- **THEN** the second insertion reports false and the original body remains

#### Scenario: Out-of-bounds body is rejected
- **WHEN** a body whose bounds are not fully inside the world rectangle is inserted
- **THEN** the insertion reports false and the body is not queryable

### Requirement: Removal, update, and bulk replacement

`Remove` SHALL delete by key and return the removed body with true; `Update` SHALL replace the
stored body for an existing key; `Rebuild` SHALL replace all contents from a body list; `Clear`
SHALL empty the index; `Len` SHALL report the entry count. An absent key SHALL be a no-op that
reports false.

#### Scenario: Removal and update change stored bodies
- **WHEN** a stored body is removed, or its shape is updated after it moves
- **THEN** subsequent queries no longer return the removed body, or return the updated shape

#### Scenario: Rebuild replaces contents
- **WHEN** a rebuild supplies a new body list
- **THEN** the old contents are gone, the new bodies are queryable, and the length counts the
  accepted bodies

### Requirement: Shape overlap query

`QueryShape` SHALL return every stored body whose shape overlaps the query shape, using exact
shape intersection rather than bounding-box overlap alone, with no duplicates and results sorted
by ascending stable key.

#### Scenario: Only intersecting bodies are returned
- **WHEN** one body intersects the query shape and another merely sits inside the same bounding
  box but outside the exact shape
- **THEN** only the intersecting body is returned

#### Scenario: Query results are deterministic
- **WHEN** several bodies intersect the same query shape
- **THEN** their keys appear in ascending order regardless of insertion order

### Requirement: Radius query

`Radius(center, radius)` SHALL return the bodies whose shape intersects the circle at that center
and radius; a negative radius SHALL return no bodies.

#### Scenario: Bodies inside the radius are returned
- **WHEN** a radius query is made around a center
- **THEN** exactly the bodies whose shapes intersect that circle are returned

#### Scenario: Negative radius returns nothing
- **WHEN** a radius query is made with a negative radius
- **THEN** no bodies are returned

### Requirement: Nearest query

`Nearest` SHALL return up to a caller-supplied limit of bodies ordered by point-to-shape distance
(zero when the point is inside the shape), then by ascending key; it SHALL honor an inclusive
maximum distance where a non-positive maximum means unbounded, and a non-positive limit SHALL
return no bodies.

#### Scenario: Nearest bodies are closest first
- **WHEN** several bodies are placed at different distances from a query point
- **THEN** `Nearest` returns them closest first and stops at the requested limit

#### Scenario: Equal distances tie-break by key
- **WHEN** two bodies are exactly equidistant from the query point
- **THEN** the one with the smaller stable key appears first

#### Scenario: Distance limit is inclusive
- **WHEN** a body's shape distance equals the maximum distance
- **THEN** it is included, while a body just beyond the limit is excluded

#### Scenario: Inside a shape reports zero distance
- **WHEN** the query point lies inside a stored body's shape
- **THEN** that body's distance is zero and it sorts ahead of surrounding bodies

#### Scenario: Non-positive limit returns nothing
- **WHEN** `Nearest` is called with a limit of zero or less
- **THEN** no bodies are returned

### Requirement: Room integration

`GameRoom` SHALL refresh its spatial index exactly once per tick, after movement and before
collision detection, and SHALL NOT rebuild it for that tick's corrections; corrected positions
SHALL enter the index on the next tick's refresh, and players removed before the tick SHALL be
absent from the rebuilt index.

#### Scenario: Tick refreshes spatial state before detection
- **WHEN** a tick moves players and then refreshes the index
- **THEN** nearest and overlap queries return players at the moved positions used for that tick's
  collision pass

#### Scenario: Corrections wait for the next refresh
- **WHEN** a tick's collision response moves a player
- **THEN** the index is not rebuilt for that tick and the corrected position is indexed by the
  next tick's refresh

#### Scenario: Removed players leave the index
- **WHEN** a player disconnects or is reaped before a tick
- **THEN** the rebuilt index contains only the remaining players

#### Scenario: Query finds closest players
- **WHEN** gameplay code asks for the closest players to a point after a tick
- **THEN** the result is ordered by distance and includes only players within the requested limit
  and distance bound
