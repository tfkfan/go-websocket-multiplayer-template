# godot-client Specification

## Purpose
Defines the Godot client that players run in the browser: its project and export layout, lobby and
room flow, protocol session, field rendering, movement input, status reporting, and responsive
layout.

## Requirements

### Requirement: Godot client project

The client SHALL be a Godot 4.7 project rooted at `client/` with `project.godot`, scene files,
GDScript sources, and a committed Web export preset. Application HTML, JavaScript, and CSS sources
SHALL NOT exist in the repository.

#### Scenario: Project opens in Godot 4.7
- **WHEN** the `client/` directory is opened with Godot 4.7
- **THEN** the project resolves its main scene and reports no missing resources

#### Scenario: Legacy web sources are gone
- **WHEN** the repository is inspected for the client implementation
- **THEN** no hand-written HTML, JavaScript, or CSS application sources remain

### Requirement: Web export build

The client SHALL be exportable as a single-threaded Godot Web build named `index.html` into
`client/build/web`, and the generated export output SHALL NOT be tracked in version control.

#### Scenario: Export produces the served tree
- **WHEN** the documented Web export command runs against `client/`
- **THEN** `client/build/web` contains `index.html` and its matching `.js`, `.wasm`, and `.pck`
  files

#### Scenario: Build output stays untracked
- **WHEN** generated export files exist under `client/build/web`
- **THEN** the repository ignores them

### Requirement: Served entry point and room link

The client SHALL load from `/` and SHALL accept a room UUID from the `room` query parameter. It
SHALL keep that parameter in the address while a room is active and SHALL display a shareable room
link.

#### Scenario: Shared link joins the room
- **WHEN** the page is opened with a `room` query parameter holding a valid UUID
- **THEN** the client connects to that room without further user action

#### Scenario: Creating a room updates the address
- **WHEN** the player creates a room
- **THEN** the address carries the new `room` parameter and the room link is shown

### Requirement: Lobby actions

The client SHALL offer Create room, Copy link, and Leave actions. Create room SHALL be disabled
while a request is in flight and SHALL report a visible error when the request fails.

#### Scenario: Create room succeeds
- **WHEN** the player activates Create room and the server returns a room UUID
- **THEN** the client shows the room and switches to the in-room controls

#### Scenario: Create room fails
- **WHEN** room creation returns a non-success response or no room UUID
- **THEN** the client shows an error status and leaves Create room enabled

#### Scenario: Copy link reports the outcome
- **WHEN** the player activates Copy link
- **THEN** the client copies the room URL and reports whether copying succeeded

#### Scenario: Leave closes the session
- **WHEN** the player activates Leave
- **THEN** the client closes the socket, clears the room view, and reports that the room was left

### Requirement: Room session

The client SHALL connect to `WS_ENDPOINT` with the active room UUID as the `room` query parameter
and SHALL exchange JSON envelopes carrying numeric message types. It SHALL consume JOIN_SUCCESS
(4) and UPDATE (100), keep the latest UPDATE as the rendered state, and handle close codes 1001
and 1014 as specified.

#### Scenario: Session starts
- **WHEN** a room is active and the socket opens
- **THEN** the client reports the connection and displays world dimensions and tick rate from
  JOIN_SUCCESS

#### Scenario: Updates drive the render
- **WHEN** an UPDATE arrives
- **THEN** the client renders that snapshot and reports the current player count and tick

#### Scenario: Room timeout closes the session
- **WHEN** the socket closes with code 1001
- **THEN** the client clears the room and reports that the room closed

#### Scenario: Admission failure is reported
- **WHEN** the socket closes with a code other than 1001 while not leaving
- **THEN** the client reports the disconnect with its code and reason

### Requirement: Field rendering

The client SHALL draw the shared 1200 × 800 field fitted into the available stage while preserving
its aspect ratio. It SHALL draw every player from the latest UPDATE at its server position with
its radius and color, and SHALL mark the local player with a distinct outline and a `you` label.

#### Scenario: Field fits the stage
- **WHEN** the window is resized
- **THEN** the field keeps its 3:2 aspect ratio and stays fully visible

#### Scenario: Players render from the snapshot
- **WHEN** an UPDATE lists players
- **THEN** each is drawn once at its position, radius, and color

#### Scenario: Local player is identified
- **WHEN** the snapshot includes the joined player
- **THEN** that circle carries the distinct outline and the `you` label

#### Scenario: Empty state is shown
- **WHEN** no snapshot has been received
- **THEN** the field shows the waiting or create-a-room prompt instead of players

### Requirement: Movement input

The client SHALL treat a direction as pressed while any of its bound keys is held, map W and Up to
UP, S and Down to DOWN, A and Left to LEFT, and D and Right to RIGHT, and SHALL send exactly one
press on activation and one release on deactivation. It SHALL release every held direction when
the window loses focus, the page is hidden, or the session ends.

#### Scenario: Direction transition is sent once
- **WHEN** a player holds a movement key
- **THEN** the client sends one press event for that direction and no repeats

#### Scenario: Overlapping keys keep the direction
- **WHEN** both keys bound to a direction are held and only one is released
- **THEN** the direction stays pressed and no release event is sent

#### Scenario: Focus loss releases input
- **WHEN** the window loses focus or the page becomes hidden while directions are held
- **THEN** the client sends a release for each held direction

### Requirement: Status and statistics

The client SHALL show the connection or error status, and while a room is active SHALL show the
room identifier, player count, local player identifier, local position, and latest tick. It SHALL
clear room statistics when no state is available.

#### Scenario: Statistics follow the snapshot
- **WHEN** an UPDATE is applied
- **THEN** the statistics reflect the room, player count, local position, and tick from that
  snapshot

#### Scenario: Statistics clear on close
- **WHEN** the session closes or the room is left
- **THEN** the statistics no longer show stale room state

### Requirement: Responsive layout

The client SHALL place the field and the side panel side by side when the window is at least 900
pixels wide and SHALL stack them vertically, field first, below that width.

#### Scenario: Wide window shows the side panel
- **WHEN** the window is at least 900 pixels wide
- **THEN** the field and panel appear side by side

#### Scenario: Narrow window stacks the layout
- **WHEN** the window is narrower than 900 pixels
- **THEN** the panel moves below the field and both remain visible
