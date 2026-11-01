# spatial-index Specification

## Purpose

Defines the world-bounded quadtree owned by each room: construction and tuning, body insertion,
removal, bulk replacement, and the shape, radius, and nearest-point queries used by actor-side
gameplay.

## Requirements

### Requirement: World-bounded construction

A quadtree SHALL be created with a valid world rectangle. Construction SHALL panic when the
rectangle is invalid, non-finite, or has zero or negative width or height. A non-positive node
capacity SHALL fall back to `8`, and a non-positive maximum depth SHALL fall back to `8`.

#### Scenario: New index is empty

- **WHEN** a quadtree is constructed from a valid rectangle
- **THEN** its length is zero and queries return nothing

#### Scenario: Invalid tuning falls back to defaults

- **WHEN** options set capacity or depth to zero or a negative value
- **THEN** the index uses the default `8` and `8`

#### Scenario: Zero-area world is rejected

- **WHEN** construction receives a rectangle with zero width or height
- **THEN** construction panics

#### Scenario: Non-finite corner is rejected

- **WHEN** construction receives a rectangle with a NaN or infinite coordinate
- **THEN** construction panics

### Requirement: Body contract

A body SHALL expose a stable `Key()`, a non-nil `Shape()`, and a `Solid()` flag. Keys SHALL be
unique within one index.

#### Scenario: Bodies are inserted once

- **WHEN** two bodies share a key
- **THEN** the second insertion is rejected and the index length stays one

### Requirement: Body insertion

Insertion SHALL reject a nil body, a duplicate key, a nil shape, and a shape whose bounds are
invalid or not contained by the world rectangle. An inserted body SHALL be stored where its
bounds fit; a node SHALL subdivide when it exceeds capacity and is shallower than the maximum
depth.

#### Scenario: Out-of-bounds body is rejected

- **WHEN** a body's bounds extend past the world rectangle
- **THEN** the body is not inserted

#### Scenario: Valid body is inserted

- **WHEN** a new body fits inside the world rectangle
- **THEN** insertion returns true and the index length increases by one

### Requirement: Removal, update, and bulk replacement

Removal SHALL be keyed and SHALL return the stored body and true, or false when absent. Update
SHALL reinsert an existing body after revalidating its bounds, returning false when the body is
absent or no longer fits. Rebuild SHALL replace the contents with a given body list, and Clear
SHALL reset the index to empty.

#### Scenario: Removal and update change stored bodies

- **WHEN** a body is removed and another is updated to a new position
- **THEN** queries no longer return the removed body and return the updated one

#### Scenario: Rebuild replaces contents

- **WHEN** a rebuild is given a new body list
- **THEN** the index contains exactly those bodies that fit the world

### Requirement: Shape overlap query

A shape query SHALL return every stored body whose shape intersects the query shape, with no
duplicates, sorted by body key. A nil shape or invalid query bounds SHALL return nothing.
Intersection SHALL be the exact shape test, so merely touching shapes are not returned.

#### Scenario: Only intersecting bodies are returned

- **WHEN** a query shape overlaps some stored bodies
- **THEN** only the overlapping bodies are returned

#### Scenario: Query results are deterministic

- **WHEN** the same query runs against indexes built in different insertion orders
- **THEN** the results have the same keys in the same order

### Requirement: Radius query

A radius query SHALL test a circle of the given radius and center with the same shape overlap
rule as the shape query, and SHALL return nothing for a negative radius.

#### Scenario: Bodies inside the radius are returned

- **WHEN** a radius query encloses some bodies
- **THEN** those bodies are returned with key ordering

#### Scenario: Negative radius returns nothing

- **WHEN** a radius query has a negative radius
- **THEN** the result is empty

### Requirement: Nearest query

A nearest query SHALL return up to `limit` bodies ordered by increasing shape distance from the
point, breaking equal distances by ascending key. A non-positive limit SHALL return nothing. A
positive maximum distance SHALL be an inclusive filter; a non-positive maximum distance SHALL
not filter. Distance SHALL be zero for a point inside a shape.

#### Scenario: Nearest bodies are closest first

- **WHEN** a nearest query runs with a limit
- **THEN** the result holds the closest bodies in increasing distance order

#### Scenario: Equal distances tie-break by key

- **WHEN** two bodies are equally distant
- **THEN** the lower key comes first

#### Scenario: Distance limit is inclusive

- **WHEN** a body is exactly at the maximum distance
- **THEN** it is included

#### Scenario: Inside a shape reports zero distance

- **WHEN** the query point lies inside a stored circle
- **THEN** that body's distance is zero and it sorts first

#### Scenario: Non-positive limit returns nothing

- **WHEN** a nearest query uses a limit of zero or less
- **THEN** the result is empty

### Requirement: Room integration

Each room SHALL own one index that is rebuilt from current members after movement and before
collision detection on every tick. The index SHALL be mutated on the room actor goroutine, so
callers SHALL NOT share it across goroutines.

#### Scenario: Tick refreshes spatial state before detection

- **WHEN** a tick moves players
- **THEN** the index reflects their new positions before contacts are detected

#### Scenario: Removed players leave the index

- **WHEN** a player is reaped or disconnects
- **THEN** the next rebuild omits that player and queries no longer return it

#### Scenario: Query finds closest players

- **WHEN** actor code queries nearest bodies after a tick
- **THEN** the result reflects the current member positions
