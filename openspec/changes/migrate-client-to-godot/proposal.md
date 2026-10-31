# Proposal

## Why

The client is a hand-written HTML/CSS/JavaScript canvas app in `assets/`. Its rendering, input
handling, and protocol constants live in three loosely coupled files, and the client cannot grow
beyond the single canvas without re-implementing engine concerns (scene composition, input maps,
UI layout, asset pipeline). Migrating the client to a Godot 4.7 project gives it a real scene/UI
model, a typed protocol layer, and an exportable web build that the existing Go server can serve
unchanged.

The `assets/` name also no longer describes its role: it holds the whole client application, not
static assets. Renaming it to `client/` makes the repository layout reflect the actual boundary
between the Go server and the Godot client.

## What Changes

- **BREAKING** Replace `assets/index.html`, `assets/app.js`, `assets/websocket.js`, and
  `assets/styles.css` with a Godot 4.7 project under `client/`: `project.godot`, scenes, GDScript,
  an input map, and a committed Web export preset. No hand-written HTML, JavaScript, or CSS
  sources remain in the repository.
- Keep the served web page working at `/` and the room link at `/?room=<uuid>`. The page is now
  the single-threaded Godot Web export (`index.html`, `.js`, `.wasm`, `.pck`), which must be built
  with `godot --headless --export-release "Web" --path client` before `go run .`. Export output
  stays out of version control under `client/build/web`.
- Keep `ASSETS_DIR` as the configuration variable and repoint its sample value from `./assets` to
  `./client/build/web`. A missing or unbuilt export directory still yields 404 for static paths
  while the room and WebSocket endpoints keep working.
- Preserve client behavior from the Canvas app: Create room, room link display/copy, leave,
  `?room=` auto-join, 30 Hz full-snapshot rendering of the 1200 × 800 field, WASD and arrow
  movement with release on focus loss, self outline and `you` label, status/statistics panel,
  close-code handling (`1001` room closed, `1014` admission failures), and the responsive
  field-plus-panel layout.
- Update the README, project layout, environment table, and protocol documentation for the Godot
  client and the new build step.
- No wire-format, gameplay, room, or spatial behavior changes. Message numbers, payload shapes,
  endpoints, and close codes stay as they are.

## Capabilities

### New Capabilities

- `godot-client`: the Godot 4.7 client project, its export/build contract, lobby and room-join
  flow, WebSocket session and protocol mirror, field rendering, input handling, and status
  reporting.

### Modified Capabilities

- `wire-protocol`: static delivery now serves a Godot Web export rather than hand-written HTML,
  JS, and CSS; the root document and its `.js`, `.wasm`, and `.pck` companions require the
  documented content types and 404 behavior.
- `server-configuration`: `ASSETS_DIR` now denotes the Godot Web export root, and the hard-coded
  endpoint paths live in the Godot client instead of the browser client.

## Impact

- Removed: `assets/index.html`, `assets/app.js`, `assets/websocket.js`, `assets/styles.css`.
- Added: `client/project.godot`, `client/export_presets.cfg`, `client/scenes/`, `client/scripts/`,
  `client/.gitignore`, and the documented `client/build/web` export output (ignored).
- Changed: `.env` (`ASSETS_DIR=./client/build/web`), `README.md`, the spec deltas in this change,
  and any test that asserts static-file serving.
- Unchanged: Go packages, HTTP/WebSocket endpoints, wire protocol, room lifecycle and gameplay,
  spatial engine, and the compiled gameplay constants.
- New local tooling requirement: Godot 4.7 with the Web export templates installed, one export
  step before starting the server. This environment does not currently have Godot installed, so
  the export itself cannot be produced or verified here; the repository gains the sources, the
  preset, and the documented command, and export verification is a manual step.
- The screenshots in `docs/` depict the Canvas client. The Godot layout intentionally matches
  them; they should be regenerated from the Godot build once one is available.
