# Spec Delta

## Purpose

Defines the shared generic min-heap used by the scheduler, including construction, ordering,
and empty-safe operations.

## ADDED Requirements

### Requirement: Construction

`NewHeap(less)` SHALL require a non-nil `less` function implementing a strict weak ordering
and SHALL initialize the heap before it is used; the zero value SHALL NOT be used for pushes.

#### Scenario: Nil ordering is rejected

- **WHEN** a heap is created with a nil `less` function
- **THEN** construction fails instead of returning an unusable heap

#### Scenario: New heap is empty

- **WHEN** a heap is created with a valid `less` function
- **THEN** its length is zero and peek reports no element

### Requirement: Ordering

`Push` SHALL insert an element and `Pop` SHALL remove and return the minimum according to
`less`; elements that compare equal SHALL be ordered consistently by `less`.

#### Scenario: Elements pop in ascending order

- **WHEN** unsorted values are pushed and then popped until empty
- **THEN** the values come out in non-decreasing order

#### Scenario: Ties follow the ordering function

- **WHEN** elements compare equal by their primary key but differ in a tie-break field used by `less`
- **THEN** they pop in the order defined by the tie-break

### Requirement: Empty-safe operations

`Pop` and `Peek` SHALL return the zero value and `false` when the heap is empty; `Len` SHALL
return zero. These operations SHALL be safe on a constructed heap and SHALL NOT panic.

#### Scenario: Empty heap reports no element

- **WHEN** `Pop` or `Peek` is called on an empty heap
- **THEN** it returns the zero value with `false`

#### Scenario: Length reflects contents

- **WHEN** elements are pushed and popped
- **THEN** `Len` matches the number of elements currently stored

### Requirement: Encapsulated typed API

The heap SHALL expose typed `Push`, `Pop`, `Peek`, and `Len` operations and SHALL keep the
generic `container/heap` machinery an internal detail, so callers do not import or drive
`container/heap` themselves.

#### Scenario: Generic element types are supported

- **WHEN** a caller instantiates the heap for a custom element type with its own ordering
- **THEN** push and pop work without any type-specific code in the heap
