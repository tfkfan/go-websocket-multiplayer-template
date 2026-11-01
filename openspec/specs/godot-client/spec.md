# godot-client Specification

## Purpose

Defines the Godot 4.7 Web client that ships in the served export: project and export settings,
the browser bridge, lobby and session flow, field rendering, movement input, status statistics,
and responsive layout.

## Requirements

### Requirement: Godot client project

The client SHALL be a Godot 4.7 project using the GL Compatibility renderer, with the main scene
as the startup scene, the network script as an autoload, and a `1280 x 800` base viewport. The
Web preset SHALL export a single-threaded build to `build/web/index.html`.

#### Scenario: Project opens in Godot 4.7

- **WHEN** the project is opened in Godot 4.7
- **THEN** the main scene loads and the network autoload is available as `Network`

#### Scenario: Export produces the served tree

- **WHEN** the Web preset is exported
- **THEN** `build/web/index.html` and its payload files exist for the server to serve

#### Scenario: Build output stays untracked

- **WHEN** the export is generated
- **THEN** the build directory remains excluded from version control

### Requirement: Browser bridge and desktop fallback

In a browser the client SHALL derive its HTTP origin from `window.location`, its WebSocket
origin from the page protocol and host, and the requested room from the `room` query parameter.
Outside a browser it SHALL fall back to `127.0.0.1:8080`. The client SHALL update the address bar
with `history.replaceState` when a room is created or left, and SHALL use the browser clipboard
when available.

#### Scenario: Shared link joins the room

- **WHEN** the page is opened with `?room=<uuid>`
- **THEN** the client connects to that room without further interaction

#### Scenario: Creating a room updates the address

- **WHEN** room creation succeeds
- **THEN** the address bar carries `?room=<uuid>` and the room row shows the link

#### Scenario: Copy link reports the outcome

- **WHEN** the copy button is pressed
- **THEN** the status line reports success or failure of the clipboard write

### Requirement: Lobby actions

The lobby SHALL create rooms with `POST ROOM_ENDPOINT` under a 10 second timeout and SHALL report
transport failures, non-200 responses, and empty bodies. Leaving a room SHALL close the socket
with a normal closure and reset the lobby.

#### Scenario: Create room succeeds

- **WHEN** room creation returns a UUID
- **THEN** the client stores the room, shows its link, and connects

#### Scenario: Create room fails

- **WHEN** the request fails, returns a non-200 status, or returns an empty body
- **THEN** the create button re-enables and the status line shows the failure

#### Scenario: Leave closes the session

- **WHEN** the leave button is pressed
- **THEN** the socket closes with code 1000 and the lobby is restored

### Requirement: Room session

The client SHALL connect to `WS_ENDPOINT?room=<uuid>`, parse the JSON envelope, and emit join
data for type 4, update data for type 100, failure messages for type 11, and close events with
code and reason. Update data SHALL drive the field and statistics. A close with 1001 SHALL reset
the room and report that the room closed; other codes SHALL show a disconnected status.

#### Scenario: Session starts

- **WHEN** the socket opens and JOIN_SUCCESS arrives
- **THEN** the client records its player identifier and the world size

#### Scenario: Updates drive the render

- **WHEN** an UPDATE arrives
- **THEN** the field renders the reported players, world size, and world state

#### Scenario: Room timeout closes the session

- **WHEN** the socket closes with 1001
- **THEN** the client clears state, resets to the lobby, and shows that the room closed

#### Scenario: Admission failure is reported

- **WHEN** a FAILURE frame or any other close code arrives
- **THEN** the status line reports the message or the code and reason

#### Scenario: Unknown frames are ignored

- **WHEN** a text frame has an unhandled type or payload shape
- **THEN** the client ignores it and keeps the session open

### Requirement: Field rendering

The field SHALL fit the `1200 x 800` world into the stage preserving aspect ratio, centered with
the remainder as background, draw an 80-unit grid and a border, and draw one circle per player
with its radius scaled by the fit factor. Player color SHALL come from the update payload with a
fallback when absent or invalid. The local player SHALL have a light outline and a `you` label.
An empty field SHALL show a waiting message while a room is active and a join prompt otherwise.

#### Scenario: Field fits the stage

- **WHEN** the stage is resized
- **THEN** the world is letterboxed with the same aspect ratio and redrawn

#### Scenario: Players render from the snapshot

- **WHEN** an UPDATE lists players
- **THEN** each player is drawn at its reported position, radius, and color

#### Scenario: Local player is identified

- **WHEN** the update contains the client's player identifier
- **THEN** that circle is outlined and labelled `you`

#### Scenario: Empty state is shown

- **WHEN** no update has arrived
- **THEN** the field shows the waiting or join prompt text

### Requirement: Movement input

The client SHALL poll `W`, `S`, `A`, `D` and the arrow keys each frame and send a key event only
when a direction's state changes. Losing focus or page visibility SHALL release every direction,
and the client SHALL suppress re-sending those releases until the physical key is released.

#### Scenario: Direction transition is sent once

- **WHEN** a key is held across frames
- **THEN** only the initial press is sent

#### Scenario: Overlapping keys keep the direction

- **WHEN** an arrow key and its letter counterpart are pressed together
- **THEN** the direction stays pressed until both are released

#### Scenario: Focus loss releases input

- **WHEN** the window loses focus or the page becomes hidden
- **THEN** releases are sent for all pressed directions and no key remains held

### Requirement: Status and statistics

The status line SHALL report connection, room, error, and close states. Statistics SHALL show
the room, player count, local player, local position, and the update tick, and SHALL clear when
no game state is available. Because UPDATE carries no tick field, the tick statistic SHALL
display zero.

#### Scenario: Statistics follow the snapshot

- **WHEN** updates arrive
- **THEN** the player count, local position, and tick lines reflect the latest update

#### Scenario: Statistics clear on close

- **WHEN** the session ends or the state is cleared
- **THEN** the statistics show that no game state is available

#### Scenario: Tick displays zero without a tick field

- **WHEN** an UPDATE omits the tick member
- **THEN** the tick statistic shows zero

### Requirement: Responsive layout

The layout SHALL place the stage beside a 320-unit side panel when the window is at least 900
units wide, and SHALL stack the stage above the full-width panel below that width, keeping the
stage at a 3:2 aspect ratio in the narrow layout.

#### Scenario: Wide window shows the side panel

- **WHEN** the window is at least 900 units wide
- **THEN** the panel sits to the right of the stage

#### Scenario: Narrow window stacks the layout

- **WHEN** the window is narrower than 900 units
- **THEN** the stage spans the width above the stacked panel
