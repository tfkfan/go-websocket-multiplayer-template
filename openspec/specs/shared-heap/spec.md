# shared-heap Specification

## Purpose

Defines the generic typed heap used by the scheduler: construction with an ordering function,
push, pop, peek, and length, with empty-safe reads.

## Requirements

### Requirement: Construction

A heap SHALL be created from an ordering function and SHALL start empty. A nil ordering function
SHALL be rejected with a panic.

#### Scenario: Nil ordering is rejected

- **WHEN** a heap is created with a nil ordering function
- **THEN** construction panics

#### Scenario: New heap is empty

- **WHEN** a heap is created with a valid ordering function
- **THEN** its length is zero and both pop and peek report no element

### Requirement: Ordering

The heap SHALL return elements in the order defined by the ordering function, with the minimum
according to that function available first. Ties SHALL be resolved by the same function, so a
strict total order yields a stable sequence.

#### Scenario: Elements pop in ascending order

- **WHEN** several elements are pushed in arbitrary order and popped until empty
- **THEN** they come out in ascending order

#### Scenario: Ties follow the ordering function

- **WHEN** two elements compare equal
- **THEN** their relative order is decided by the ordering function's tie-breaking rule

### Requirement: Empty-safe operations

Pop and peek SHALL report false when the heap is empty and SHALL NOT panic. Length SHALL reflect
the number of stored elements after every operation.

#### Scenario: Empty heap reports no element

- **WHEN** pop or peek runs on an empty heap
- **THEN** it returns the zero value and false

#### Scenario: Length reflects contents

- **WHEN** elements are pushed and popped
- **THEN** length matches the number stored

### Requirement: Encapsulated typed API

The heap SHALL expose typed push, pop, peek, and length operations and SHALL NOT require callers
to use the standard library heap interface directly.

#### Scenario: Generic element types are supported

- **WHEN** the heap is instantiated with a scheduler task type
- **THEN** push, pop, and peek exchange values of that type without assertions at call sites

#### Scenario: Stored values are preserved

- **WHEN** an element is pushed and later popped
- **THEN** it equals the value that was pushed
