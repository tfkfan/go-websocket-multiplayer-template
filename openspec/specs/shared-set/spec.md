# shared-set Specification

## Purpose
Defines the generic, concurrency-safe keyed set used for room membership: construction, add and
lookup, key-based removal, enumeration, bulk clear, and concurrent access.

## Requirements

### Requirement: Construction

`NewSet` SHALL create an empty set for any comparable key type and any value type; the returned
set SHALL be ready for use, and the zero value SHALL NOT be used.

#### Scenario: New set is empty
- **WHEN** a set is created
- **THEN** its length is zero, its listing is empty, and every lookup reports absence

#### Scenario: Any comparable key and value types are supported
- **WHEN** sets are instantiated for distinct key and value types, such as UUID to player and
  string to integer
- **THEN** membership operations work for each instance without type-specific code

### Requirement: Add-if-absent

`Add(key, value)` SHALL insert the pair only when the key is absent, returning true on insertion
and false when the key already exists; an existing entry SHALL NOT be replaced.

#### Scenario: New key is inserted
- **WHEN** a value is added under a key not present in the set
- **THEN** the add reports true and the value is retrievable under that key

#### Scenario: Duplicate key is rejected
- **WHEN** a value is added under a key that already exists
- **THEN** the add reports false and the original value remains retrievable

### Requirement: Lookup

`Get(key)` SHALL return the stored value with true when the key is present, and the value type's
zero value with false otherwise.

#### Scenario: Present key returns its value
- **WHEN** a key in the set is looked up
- **THEN** the stored value and true are returned

#### Scenario: Missing key reports absence
- **WHEN** a key not in the set is looked up
- **THEN** the zero value and false are returned

### Requirement: Key-based removal

`Remove(key)` SHALL delete the entry for the key and return its value with true; when the key is
absent it SHALL return the zero value with false and leave the set unchanged. Removal SHALL match
by key alone and SHALL NOT compare the stored value.

#### Scenario: Present key is removed
- **WHEN** a key in the set is removed
- **THEN** its value and true are returned and later lookups of that key report absence

#### Scenario: Missing key is ignored
- **WHEN** a key not in the set is removed
- **THEN** the zero value and false are returned and the length is unchanged

#### Scenario: Removal does not depend on the stored value
- **WHEN** a key holding a composite or non-comparable value is removed
- **THEN** the entry is deleted and its value returned, because only the key is compared

### Requirement: Enumeration and length

`Len` SHALL report the number of entries. `List` SHALL return a newly allocated slice of the
current values in unspecified order; changing that slice SHALL NOT change the set.

#### Scenario: Listing reflects current membership
- **WHEN** entries are added and removed and then listed
- **THEN** the listing contains exactly the currently stored values with no duplicates

#### Scenario: Listing is a snapshot
- **WHEN** a caller overwrites or reorders the slice returned by a listing
- **THEN** subsequent lookups and listings still reflect the stored entries

### Requirement: Bulk clear

`Clear` SHALL remove every entry, return the removed values in unspecified order, and leave the
set empty; clearing an empty set SHALL return an empty result.

#### Scenario: Clear returns and removes everything
- **WHEN** a populated set is cleared
- **THEN** every prior value is returned and the set reports zero length

#### Scenario: Clearing an empty set is safe
- **WHEN** an empty set is cleared
- **THEN** it returns no values and remains empty

### Requirement: Concurrent access

All set operations SHALL be safe for concurrent use by multiple goroutines without external
synchronization; each operation SHALL be individually atomic, and no operation SHALL deadlock
when callers mix reads, writes, listings, and clears.

#### Scenario: Concurrent mixed operations are race-free
- **WHEN** many goroutines add, look up, remove, list, and read the length at the same time
- **THEN** no data race or deadlock occurs and every surviving entry is consistent

### Requirement: Room membership integration

`GameRoom` SHALL keep its players in a shared set keyed by player UUID and SHALL use it for
duplicate detection, capacity checks, targeted delivery lookup, snapshot and movement iteration,
disconnect reaping, and close-time clearing.

#### Scenario: Membership drives admission and delivery
- **WHEN** a player is admitted and later targeted for a message
- **THEN** duplicate identifiers are rejected, capacity is enforced, and the targeted player is
  found through the same membership set

#### Scenario: Snapshots and movement cover current members
- **WHEN** a tick publishes a snapshot or advances movement
- **THEN** it iterates exactly the players currently in the set

#### Scenario: Disconnect and close update membership
- **WHEN** a player disconnects or the room closes
- **THEN** the disconnected player is removed, freeing capacity, and close clears the set and
  returns every player for shutdown

#### Scenario: A stale leave removes a reused identifier
- **WHEN** a leave command for an old connection is processed after a newer player reused the
  same UUID
- **THEN** the newer player is removed from membership, because removal matches by key alone and
  does not check identity
