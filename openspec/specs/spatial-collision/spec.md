# spatial-collision Specification

## Purpose
Defines circle/AABB geometry and read-only collision detection that reports solid overlaps to a
caller-supplied callback, leaving every response to game code.

## Requirements

### Requirement: Circle and AABB geometry

The spatial system SHALL support circles (center and radius) and axis-aligned boxes (minimum and
maximum corners), each providing its bounding rectangle, center, point containment, and
point-to-shape distance (zero inside the shape).

#### Scenario: Distances match the shape
- **WHEN** a point outside a circle or box is measured
- **THEN** the distance is the Euclidean gap to the shape, and it is zero for a point inside

#### Scenario: Bounding rectangles are exact
- **WHEN** a circle or box reports its bounds
- **THEN** the rectangle encloses exactly that shape

### Requirement: Exact contact detection

Contact detection SHALL report a collision only when two shapes overlap with positive
penetration, and SHALL return the contact normal and penetration depth; shapes that merely touch
SHALL NOT collide.

#### Scenario: Two circles overlap
- **WHEN** the distance between circle centers is less than the sum of their radii
- **THEN** a contact is reported with depth equal to the overlap and a normal along the center line

#### Scenario: Two circles touch
- **WHEN** the distance between circle centers equals the sum of their radii
- **THEN** no contact is reported

#### Scenario: Circle overlaps a box face
- **WHEN** a circle overlaps the side of a box
- **THEN** the contact normal is perpendicular to that face and the depth is the overlap

#### Scenario: Circle center lies inside a box
- **WHEN** a circle's center is inside a box
- **THEN** the contact reports an outward direction through the nearest face and a depth that
  includes the circle radius

#### Scenario: Two boxes overlap
- **WHEN** two boxes intersect
- **THEN** the contact uses the axis of least penetration and reports its depth

#### Scenario: Coincident circle centers
- **WHEN** two circles share a center
- **THEN** the contact normal falls back to a fixed axis so the result stays deterministic

### Requirement: Contact orientation

A reported contact SHALL order A as the body with the lexicographically smaller stable key and
SHALL point its normal from B toward A, so moving A by the normal times the depth separates the
pair.

#### Scenario: Pair is ordered by key
- **WHEN** two bodies overlap regardless of the order they are passed in
- **THEN** the returned contact has the smaller key as A and the normal points from B toward A

### Requirement: Read-only collision reporting

Collision detection SHALL report each overlapping solid pair exactly once without moving,
pushing, separating, or otherwise mutating any body, and SHALL invoke an optional callback
synchronously for every reported contact before returning the contacts.

#### Scenario: Callback receives every contact once
- **WHEN** several solid bodies overlap
- **THEN** the callback runs once per overlapping pair and the same contacts are returned

#### Scenario: Detection does not move bodies
- **WHEN** detection reports a contact
- **THEN** both bodies keep their positions and shapes unchanged

#### Scenario: Non-solid bodies are ignored
- **WHEN** a non-solid body overlaps a solid body or another non-solid body
- **THEN** no contact is reported and the callback does not run

#### Scenario: Separated and touching bodies are ignored
- **WHEN** two solid bodies are separated or merely touching
- **THEN** no contact is reported

#### Scenario: Nil inputs are safe
- **WHEN** detection runs with a nil index or a nil callback
- **THEN** a nil index reports nothing and a nil callback still returns the contacts without
  panicking

### Requirement: Body-focused contact reporting

`DetectBodyCollisions` SHALL report contacts between one solid body and the other solid bodies
around it, once per pair, without moving either body; contacts SHALL be ordered by the other
body's key and the callback SHALL receive them synchronously.

#### Scenario: Only the target body's contacts are reported
- **WHEN** a body overlaps two others while a fourth body overlaps only itself
- **THEN** the callback receives the two contacts involving the target body and nothing else

#### Scenario: Body-focused detection is read-only
- **WHEN** body-focused detection reports contacts
- **THEN** neither the target body nor any other body changes position

#### Scenario: Non-solid target reports nothing
- **WHEN** the target body is not solid
- **THEN** no contacts are reported

### Requirement: Deterministic contact order

Reported contacts SHALL be ordered by the pair of stable body keys, so callback and return order
do not depend on index traversal or insertion order, and detection SHALL complete in a single
pass without positional iterations.

#### Scenario: Order is independent of insertion
- **WHEN** the same overlapping bodies are inserted in different orders
- **THEN** the callback sees the same contact order

#### Scenario: One pass reports current contacts
- **WHEN** detection runs on a set of overlapping bodies
- **THEN** every currently overlapping solid pair is reported once and no repeated correction pass
  occurs

### Requirement: Callback-driven response

All collision response SHALL live in the caller's callback. The spatial package SHALL NOT assume
or provide separation, sliding, stopping, damage, or world-clamping behavior; the callback
receives the bodies, normal, and depth and decides what changes to make.

#### Scenario: Caller moves only the reported body
- **WHEN** a callback moves the body that caused the overlap out along the contact normal
- **THEN** that body is the only one repositioned and the other body stays where detection found it

#### Scenario: Caller leaves bodies overlapping
- **WHEN** a callback ignores a contact
- **THEN** the bodies remain exactly where detection found them
