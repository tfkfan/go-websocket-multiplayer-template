# Design

## Context

See `proposal.md` for motivation. Current state that shapes the approach:

- `internal/server/game_server.go` serves the client with one line:
  `http.FileServer(http.Dir(cfg.AssetsDir))` behind the `/` pattern. `ASSETS_DIR` is mandatory and
  a missing directory yields 404s for static paths while `/room` and `/ws` keep working.
- The client is `assets/index.html`, `assets/app.js`, `assets/websocket.js`, and
  `assets/styles.css`. It mirrors the server's message numbers in `websocket.js`, owns room flow
  and rendering in `app.js`, and the layout in `styles.css` (900 px stacking breakpoint,
  320 px side panel, 1200 × 800 canvas).
- The wire protocol, room behavior, and gameplay constants are fixed by existing specs and do not
  change.
- Project convention: no comments in source or configuration files (GDScript, scene text,
  `project.godot`, `export_presets.cfg`, Go, YAML).
- The environment has no Godot binary or Web export templates, so scene/preset text cannot be
  validated by the editor here.

## Goals / Non-Goals

**Goals:**

- A Godot 4.7 project whose scene text is small and conventional, with behavior and styling in
  GDScript so correctness is reviewable without an editor session.
- Preserve every observable client behavior listed in `specs/godot-client/spec.md` and the visual
  proportions of the Canvas layout.
- A deterministic static-delivery contract for the exported build (root document, `.wasm`, `.pck`).
- A build-then-run path: one export command, then the existing `go run .`.

**Non-Goals:**

- No custom HTML shell, PWA, thread support, GDExtension, native desktop/mobile preset, or audio.
- No pixel-perfect reproduction of CSS gradients, box shadows, or the responsive page scroll.
- No automated browser/Godot test harness; no changes to server protocol, gameplay, or spatial
  behavior.
- No server-side build orchestration (the Go process does not invoke Godot).

## Decisions

### D1. Source and build separation under `client/`

`client/` is the Godot project; `client/build/web` is the export output and is ignored via
`client/.gitignore` (`.godot/`, `build/`). `ASSETS_DIR` becomes `./client/build/web` in `.env`.
Generated `wasm`/`pck` binaries stay out of version control, matching the chosen build-then-run
model.

Alternatives considered: committing the export (rejected: stale binary blobs, review noise);
serving `client/` directly (rejected: exposes project sources and has no `index.html` until an
export exists).

### D2. Godot 4.7, Compatibility renderer, single-threaded Web export

The Web platform only supports the Compatibility (WebGL 2) renderer. The export preset disables
thread support and extensions, so the page needs no `Cross-Origin-Opener-Policy` or
`Cross-Origin-Embedder-Policy` headers and the Go server stays a plain static server. A 30 Hz
snapshot client does not benefit from threads.

Alternatives considered: threaded export (rejected: cross-origin isolation headers and no
performance need); targeting 4.4 (rejected: Godot 4.7 is the current stable and the user's choice).

### D3. Minimal scenes, behavior and theme in GDScript

Project tree:

```text
client/
  project.godot
  export_presets.cfg
  .gitignore
  icon.svg
  scenes/main.tscn
  scenes/game_field.tscn
  scripts/protocol.gd
  scripts/platform.gd
  scripts/network.gd
  scripts/main.gd
  scripts/game_field.gd
  scripts/ui_theme.gd
```

`scenes/main.tscn` holds only the structural nodes and named children; `scenes/game_field.tscn` is
a single full-rect `Control`. No theme resources are hand-authored in scene text. Styling is
produced by `ui_theme.gd` (a `StyleBoxFlat` factory applying the existing palette) from
`main.gd`, and `protocol.gd` exposes `class_name Protocol` with the message numbers and endpoint
paths.

Node tree of `main.tscn`:

```text
Main (Control, main.gd)
  Background (ColorRect)
  Layout (Control)
    Stage (Control)
      GameField (instance of game_field.tscn)
    Panel (PanelContainer)
      PanelMargin (MarginContainer)
        PanelScroll (ScrollContainer)
          Body (VBoxContainer)
            Title (Label), Subtitle (Label)
            CreateButton (Button)
            RoomRow (HBoxContainer): RoomLabel, RoomLink (LinkButton), CopyButton, LeaveButton
            Status (Label)
            StateHeader (Label), Stats (Label)
            ControlsHeader (Label), ControlsHelp (Label)
```

`network.gd` is an autoload singleton (`Network`) owning the `WebSocketPeer` and one
`HTTPRequest`, exposing signals: `room_created(room_id)`, `room_create_failed(message)`,
`opened`, `join_received(data)`, `update_received(data)`, `closed(code, reason)`, and
`transport_error(message)`.

Alternatives considered: a single script doing everything (rejected: transport, drawing, and UI
would mix); authoring all theme resources in `.tscn`/`.tres` (rejected: verbose, unvalidated
sub-resource text without an editor); building the whole UI programmatically with no scene nodes
(rejected: loses the editor's scene view and the project's "scene" vocabulary).

### D4. Endpoint and room derivation

`platform.gd` centralizes browser access behind `is_web()` and `eval_js(code)`:

- On web, HTTP origin is `window.location.origin`, the WebSocket scheme follows the page scheme
  (`ws:`/`wss:`), and the initial room is
  `new URLSearchParams(window.location.search).get('room')`.
- The client hard-codes `Protocol.RoomEndpoint` (`/room`) and `Protocol.WebSocketEndpoint`
  (`/ws`), matching the server's mandatory variables and the existing spec expectation that
  endpoint changes require a client change.
- On desktop/editor, the same endpoints are used against `http://127.0.0.1:8080`; the room is
  absent because there is no address bar. This keeps the project runnable from the editor.
- Creating a room replaces the address on web with `history.replaceState` so a reload rejoins;
  leaving resets the address to the bare path.

Alternatives considered: export-time feature tags for the origin (rejected: extra build
configuration); fetching runtime config from the server (rejected: static hosting, no template
injection).

### D5. Transport over `WebSocketPeer` and `HTTPRequest`

`network.gd` connects to `<ws-origin><WebSocketEndpoint>?room=<uuid>`, polls the peer every frame,
drains packets as UTF-8 JSON envelopes, and dispatches by numeric `type`. Sends use
`send_text(JSON.stringify({"type": type, "data": data}))` only while the peer is open. Close codes
and reasons are captured from the peer before the state resets. Room creation uses
`HTTPRequest.request(origin + RoomEndpoint, [], HTTPClient.METHOD_POST)` with a bounded timeout and
a single in-flight request; success is `RESULT_SUCCESS`, HTTP 200, and a non-empty trimmed body.

The client mirrors `Protocol` numbers: `JOIN_SUCCESS` 4, `FAILURE` 11, `UPDATE` 100, `PRIVATE`
110, `PLAYER_KEY_DOWN` 200.

Alternatives considered: driving the browser `WebSocket` through `JavaScriptBridge` (rejected:
reimplements transport in JavaScript, which the change removes); multiplayer API
(`WebSocketMultiplayerPeer`, rejected: wrong abstraction for a custom envelope).

### D6. Rendering with a drawing `Control`

`game_field.gd` keeps the latest snapshot (`players`, `width`, `height`, `tick`), the joined
player id, and whether a room is active. `_draw` scales the world into the control:

- `scale = min(size.x / width, size.y / height)`, centered, so the 3:2 field never distorts.
- Background `#0c1522`, 80-unit grid at `rgba(90, 135, 175, 0.13)`, 4-unit border `#37516d`,
  matching the Canvas palette.
- Each player is one circle at `position * scale` with `radius * scale` and its server color.
- The local player gets a widened white outline (`#f8fafc`) and a `you` label above the circle.
- Without a snapshot, the field shows the queued room state or the create/open prompt.

`queue_redraw()` runs on snapshot and resize changes. At 30 Hz and a handful of players, redrawing
the whole field is cheaper and simpler than pooling node-per-player sprites.

Alternatives considered: `Node2D` + `Camera2D` (rejected: fitting into a UI stage is harder);
`Sprite2D` per player (rejected: no textures, procedural circles are the model).

### D7. Responsive layout and input

Layout is computed in `main.gd` on resize rather than delegated to containers:

- Wide (window width ≥ 900): margins 18, gap 18, panel 320 wide on the right, stage fills the
  rest.
- Narrow (< 900): margins 12, stage spans the width with height `width × 2/3`, panel below with the
  remaining space. Panel content lives in a `ScrollContainer` so it stays reachable.

This reproduces the CSS breakpoint and field aspect without fighting container minimum-size
behavior in a hand-authored scene.

Input is polled each frame with physical keys, matching the old `event.code` mapping: `W`/`Up` →
UP, `S`/`Down` → DOWN, `A`/`Left` → LEFT, `D`/`Right` → RIGHT. A direction counts as pressed while
any bound key is held; the client sends one press on the false→true edge and one release on the
true→false edge, so key repeat and overlapping keys cannot duplicate events. Releases are forced
on `NOTIFICATION_APPLICATION_FOCUS_OUT`, on session close, and, on web, through a
`visibilitychange` callback registered via `JavaScriptBridge.create_callback`.

Alternatives considered: a `project.godot` input map (rejected: hand-authored `InputEventKey`
`Object(...)` literals are brittle without the editor); `_input` event bookkeeping (rejected:
more state for the same result).

### D8. Styling with a scripted theme

`ui_theme.gd` builds `StyleBoxFlat` variants from the existing palette (`#101a27` panel,
`#263a50` border, `#38bdf8` accent, `#f87171` danger, `#0c1522` field) with 14 px panel radius,
9 px control radius, and a soft shadow for the panel; buttons get primary/ghost variants and the
status label switches its border/background to the error palette. Gradients are flattened to
solid colors and the mono font is replaced by the default font.

Alternatives considered: a `.tres` theme (rejected: hand-authored style resources are harder to
review); embedding fonts (rejected: extra binary assets for a demo).

### D9. Static delivery contract

`internal/server/assets.go` gains `assetHandler(root string) http.Handler`: it wraps
`http.FileServer(http.Dir(root))` and sets `Content-Type` for the extensions that Godot's loader
depends on — `.wasm` → `application/wasm`, `.pck` → `application/octet-stream` — before delegating.
Missing paths, `/`, and `index.html` fallbacks keep the standard `FileServer` behavior, and
`http.Error` still overwrites the type on 404 responses.

`internal/server/game_server.go` registers `assetHandler(cfg.AssetsDir)` instead of constructing
the file server inline. `internal/server/assets_test.go` uses `httptest` with a temporary root to
assert the root document, the two content types, and 404 for a missing path.

Alternatives considered: relying on Go's built-in `.wasm` mapping and content sniffing (works
today but depends on the platform MIME database for some extensions); replacing `FileServer`
(rejected: unnecessary risk around ranges and caching).

### D10. Startup and documentation

`.env` sets `ASSETS_DIR=./client/build/web`. The server does not check for the export at startup:
the existing spec says a missing asset root yields 404s and leaves the API working, and the
integration tests start the server without an export. Instead the README documents the export
prerequisite and command, the Godot version, the ignored output, and that generated assets are not
committed. The server never launches Godot.

Export commands (run from the repository root):

```bash
godot --headless --path client --import
godot --headless --path client --export-release "Web"
go run .
```

## Risks / Trade-offs

- Hand-authored `.tscn`, `project.godot`, and `export_presets.cfg` cannot be editor-validated here
  → keep scene text to standard nodes and simple properties, put behavior/theme in GDScript, and
  run `godot --headless --path client --import` as the first verification step where Godot exists.
- The web build is larger and slower to start than the Canvas client (wasm engine payload)
  → accepted for a demo; served locally, and the room/link flow is unchanged; no optimization work
  in scope.
- Godot is not installed in this environment, so no export or browser verification can happen
  during implementation → tasks record the exact manual verification commands and the pending
  status; Go tests and OpenSpec validation still run.
- Clipboard and `history.replaceState` are browser-only and can be denied → copy reports failure
  in the status line, and the URL update is best effort; the room still works from the parameter.
- Releasing keys on tab hide depends on a browser event reaching the callback → both the engine
  focus-out notification and the JS `visibilitychange` listener release held directions; worst
  case a stale press persists until the next focus change.
- Visual parity is approximate: gradients, shadows, and the monospace font are reduced to flat
  styles → palette, spacing, proportions, and control placement match; screenshots should be
  regenerated from the Godot build once available.
- The static content-type wrapper runs before `FileServer` and could tag an error response
  → `http.Error` overwrites the type on 404/405; the asset test covers this path.

## Migration Plan

1. Add the `client/` project, preset, and ignore rules; remove `assets/`.
2. Repoint `ASSETS_DIR`, update the README (build step, layout, protocol pointer, Godot
   requirement), and add the asset handler with its test.
3. Run `go test ./...`, `go test -race ./...`, and `openspec validate migrate-client-to-godot
   --strict`.
4. When Godot 4.7 with Web export templates is available: import, export, start `go run .`, and
   verify lobby, create, copy, leave, two-player updates, movement, self label, and close codes in
   a browser.
5. Archive the change to merge the spec deltas.

Rollback is a revert to `assets/` and `ASSETS_DIR=./assets` from git history; there is no persisted
state or schema, and the removed `/app.js`, `/styles.css`, and `/websocket.js` paths have no
external consumers.
