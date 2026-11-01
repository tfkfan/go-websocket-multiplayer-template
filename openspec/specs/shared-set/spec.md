# shared-set Specification

## Purpose

Defines the concurrent keyed set used for room membership: insertion that refuses duplicates,
lookup, key-based removal, enumeration, bulk clear, and safe concurrent access.

## Requirements

### Requirement: Construction

A set SHALL be created empty for any comparable key type and arbitrary value type.

#### Scenario: New set is empty

- **WHEN** a set is created
- **THEN** its length is zero and listing returns no values

#### Scenario: Any comparable key and value types are supported

- **WHEN** the set is instantiated with a UUID key and a pointer value
- **THEN** it stores and returns those values unchanged

### Requirement: Add-if-absent

Adding SHALL insert the pair and return true, or leave the set unchanged and return false when
the key already exists.

#### Scenario: New key is inserted

- **WHEN** an absent key is added
- **THEN** the call returns true and the value is retrievable

#### Scenario: Duplicate key is rejected

- **WHEN** an existing key is added again with a different value
- **THEN** the call returns false and the stored value is unchanged

### Requirement: Lookup

Lookup SHALL return the stored value and true for a present key, and the zero value and false
otherwise.

#### Scenario: Present key returns its value

- **WHEN** an existing key is requested
- **THEN** the original value and true are returned

#### Scenario: Missing key reports absence

- **WHEN** an unknown key is requested
- **THEN** the zero value and false are returned

### Requirement: Key-based removal

Removal SHALL delete the entry by key and return the removed value and true, or the zero value
and false when the key is absent.

#### Scenario: Present key is removed

- **WHEN** an existing key is removed
- **THEN** the value is returned and the key is no longer present

#### Scenario: Missing key is ignored

- **WHEN** an absent key is removed
- **THEN** the set is unchanged and false is returned

### Requirement: Enumeration and length

Listing SHALL return a snapshot of the current values whose order SHALL NOT be relied on, and
length SHALL report the current member count. Mutating the set after a listing SHALL NOT change
the returned slice.

#### Scenario: Listing reflects current membership

- **WHEN** entries are added and removed
- **THEN** listing returns exactly the current values

#### Scenario: Listing is a snapshot

- **WHEN** entries change after a listing is taken
- **THEN** the earlier slice is unchanged

### Requirement: Bulk clear

Clear SHALL remove every entry, return the removed values, and reset the set to empty. Clearing
an empty set SHALL return an empty slice.

#### Scenario: Clear returns and removes everything

- **WHEN** a populated set is cleared
- **THEN** every previous value is returned and the length becomes zero

#### Scenario: Clearing an empty set is safe

- **WHEN** an empty set is cleared
- **THEN** the result is empty and the set stays usable

### Requirement: Concurrent access

All operations SHALL be safe for concurrent use and SHALL NOT race under mixed reads and writes.

#### Scenario: Concurrent mixed operations are race-free

- **WHEN** goroutines read, add, remove, list, and clear the same set concurrently
- **THEN** the race detector reports no data race

### Requirement: Room membership integration

Room membership SHALL be one set keyed by player identifier. Admission SHALL use add-if-absent,
lookup SHALL drive delivery and snapshot membership, and disconnect or close SHALL remove or
clear entries.

#### Scenario: Membership drives admission and delivery

- **WHEN** a player joins and later disconnects
- **THEN** admission reflects the add, and delivery and snapshots reflect the removal

#### Scenario: Snapshots and movement cover current members

- **WHEN** a tick runs
- **THEN** movement, snapshots, and indexing use the current membership listing

#### Scenario: Disconnect and close update membership

- **WHEN** a player leaves or the room closes
- **THEN** removal or clear updates the set before later snapshots

#### Scenario: A stale leave removes a reused identifier

- **WHEN** a leave arrives for an identifier already removed or reused
- **THEN** removal is keyed by identifier and leaves other members untouched
