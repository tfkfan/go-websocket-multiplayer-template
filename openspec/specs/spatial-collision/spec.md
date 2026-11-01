# spatial-collision Specification

## Purpose

Defines the collision primitives: vector and rectangle geometry, circle and AABB shapes,
penetration computation, exact contact detection over the spatial index, and the read-only
reporting contract callers use to respond to contacts.

## Requirements

### Requirement: Circle and AABB geometry

The spatial package SHALL provide a 2D vector with add, subtract, scale, dot, length, distance,
and normalization operations; a rectangle with width, height, center, validity, containment,
intersection, containment, clamping, translation, and quadrant operations; and a shape interface
exposing bounds, center point, point containment, and distance. Circle and AABB SHALL implement
that interface, with distance zero inside the shape.

#### Scenario: Distances match the shape

- **WHEN** a point lies inside a shape
- **THEN** its distance is zero; outside, it is the distance to the shape boundary

#### Scenario: Bounding rectangles are exact

- **WHEN** a shape reports its bounds
- **THEN** the bounds enclose the shape with no padding

### Requirement: Exact contact detection

Contact detection SHALL report a contact for two distinct solid-capable bodies only when their
shapes actually penetrate, not when they merely touch. Detection SHALL support circle-circle,
circle-AABB, and AABB-AABB pairs with a shared penetration depth and normal.

#### Scenario: Two circles overlap

- **WHEN** two circle centers are closer than the sum of their radii
- **THEN** a contact is reported with that penetration depth

#### Scenario: Two circles touch

- **WHEN** the distance between centers equals the sum of radii
- **THEN** no contact is reported

#### Scenario: Circle overlaps a box face

- **WHEN** a circle penetrates an AABB side
- **THEN** the contact normal points from the box toward the circle center

#### Scenario: Circle center lies inside a box

- **WHEN** a circle center is inside an AABB
- **THEN** the contact uses the nearest side and a depth of that side distance plus the radius

#### Scenario: Two boxes overlap

- **WHEN** two AABBs overlap
- **THEN** the contact uses the axis of smaller overlap

#### Scenario: Coincident circle centers

- **WHEN** two circles share a center
- **THEN** the normal is the positive X axis and the depth is the sum of radii

### Requirement: Contact orientation

A contact SHALL name bodies `A` and `B` with `A.Key() <= B.Key()`, and its normal SHALL point
from `B` toward `A`. A body SHALL NOT be reported in contact with itself.

#### Scenario: Pair is ordered by key

- **WHEN** contact detection receives bodies in the reverse key order
- **THEN** the reported pair and normal are swapped consistently

### Requirement: Read-only collision reporting

Detection SHALL NOT move or mutate any body. It SHALL report every penetrating pair of solid
bodies exactly once, ignore non-solid bodies, and require no mutation to answer. A nil index
SHALL report nothing.

#### Scenario: Callback receives every contact once

- **WHEN** several players overlap
- **THEN** the callback receives each unordered pair once

#### Scenario: Detection does not move bodies

- **WHEN** detection runs
- **THEN** all body positions are unchanged

#### Scenario: Non-solid bodies are ignored

- **WHEN** a body reports itself non-solid
- **THEN** it produces no contacts

#### Scenario: Nil inputs are safe

- **WHEN** detection receives a nil index or a nil body
- **THEN** it reports nothing instead of panicking

### Requirement: Body-focused contact reporting

A body-focused query SHALL report contacts for one target body only, skipping itself and
non-solid others, and SHALL return nothing for a nil or non-solid target.

#### Scenario: Only the target body's contacts are reported

- **WHEN** a focused query runs for one body in a crowded index
- **THEN** every reported contact names that body

#### Scenario: Non-solid target reports nothing

- **WHEN** the queried body is not solid
- **THEN** the result is empty

### Requirement: Deterministic contact order

Contacts SHALL be ordered by ascending `A` key and then ascending `B` key, independent of
insertion order, and one pass SHALL report every contact present at detection time.

#### Scenario: Order is independent of insertion

- **WHEN** the same bodies populate two indexes in different orders
- **THEN** both report contacts in the same order

#### Scenario: One pass reports current contacts

- **WHEN** a caller corrects a body after a report
- **THEN** the same pass does not re-report the pair unless the correction still penetrates
