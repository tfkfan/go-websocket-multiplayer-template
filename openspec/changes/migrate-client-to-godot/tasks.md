# Tasks

## 1. Godot project scaffold

- [ ] 1.1 Create `client/project.godot` with the project name, `run/main_scene="res://scenes/main.tscn"`, a 1280 × 800 window, the Compatibility renderer, and the `Network` autoload; verify the file declares the main scene and autoload entries and that `client/scenes/main.tscn` and `client/scripts/network.gd` exist.
- [ ] 1.2 Add `client/.gitignore` covering `.godot/` and `build/`, plus a small `client/icon.svg`; verify `git check-ignore client/.godot/imported client/build/web/index.html` reports both paths as ignored.
- [ ] 1.3 Add `client/export_presets.cfg` with a `Web` preset exporting to `build/web/index.html`, thread support and extension support disabled, and PWA disabled; verify the preset names `platform="Web"`, sets `variant/thread_support=false`, `variant/extensions_support=false`, and the export path.

## 2. Client scripts

- [ ] 2.1 Add `client/scripts/protocol.gd` with the message numbers and `/room` and `/ws` endpoints; verify every number matches `internal/game/room/protocol.go` (`MsgJoinSuccess` 4, `MsgFailure` 11, `MsgUpdate` 100, `MsgPrivate` 110, `MsgPlayerKeyDown` 200) by comparing the two files.
- [ ] 2.2 Add `client/scripts/platform.gd` with web detection, JavaScript evaluation, room query lookup, address update, and visibility-callback registration, plus the `http://127.0.0.1:8080` desktop fallback; verify no browser call runs when `OS.has_feature("web")` is false.
- [ ] 2.3 Add `client/scripts/network.gd` as the autoload transport: HTTPRequest room creation, WebSocketPeer connect/poll/send/close, close-code capture, and the signals `room_created`, `room_create_failed`, `opened`, `join_received`, `update_received`, `closed`, `transport_error`; verify the signal list matches design.md D3 and that the script references no scene nodes.
- [ ] 2.4 Add `client/scripts/game_field.gd` with snapshot state and `_draw`; verify the draw path covers the background, 80-unit grid, 4-unit border, every player circle, the self outline and `you` label, both empty-state prompts, and calls `queue_redraw` on state and resize changes.
- [ ] 2.5 Add `client/scripts/ui_theme.gd` producing the panel, button, and status styles from the existing palette; verify each color matches its `assets/styles.css` counterpart (`#070c14`, `#101a27`, `#0c1522`, `#263a50`, `#38bdf8`, `#f87171`, `#e8eef7`, `#8aa0b8`) before `assets/` is removed.
- [ ] 2.6 Add `client/scripts/main.gd` for room lifecycle, status and statistics text, responsive layout, input polling, and key release on focus loss; verify the 900 px breakpoint, the 320 px panel, the close-code branches for 1001 and other codes, and the status strings match `specs/godot-client/spec.md` and the old `assets/app.js` behavior.

## 3. Scenes

- [ ] 3.1 Add `client/scenes/game_field.tscn` as a full-rect `Control` with `game_field.gd`; verify the script path resolves to an existing file and the node name matches what `main.gd` instantiates.
- [ ] 3.2 Add `client/scenes/main.tscn` with the tree from design.md D3 (background, stage with the GameField instance, panel with title, subtitle, create, room row, status, state, statistics, and controls); verify every node name that `main.gd` accesses appears in the scene and every `res://` script path in the scene exists on disk.
- [ ] 3.3 Verify the client sources contain no comments and no HTML/JS/CSS application files, and that every scene reference uses `res://` paths: `grep -rn '^\s*[#;]' client` returns no source comments and `find client -name '*.html' -o -name '*.js' -o -name '*.css'` returns nothing outside the ignored build output.

## 4. Static delivery

- [ ] 4.1 Add `internal/server/assets.go` with `assetHandler(root)` wrapping `http.FileServer` and forcing `.wasm` to `application/wasm` and `.pck` to `application/octet-stream`; verify the package builds with `go build ./...`.
- [ ] 4.2 Register `assetHandler(cfg.AssetsDir)` for `/` in `internal/server/game_server.go`; verify `go vet ./internal/server/` passes and the old inline file-server construction is gone.
- [ ] 4.3 Add `internal/server/assets_test.go` covering the root document, both content types, and 404 for a missing path using `httptest` and a temporary root; verify `go test ./internal/server/` passes.
- [ ] 4.4 Verify the existing suite still passes with no export directory present: `go test ./...`.

## 5. Configuration and documentation

- [ ] 5.1 Set `ASSETS_DIR=./client/build/web` in `.env`; verify `go run .` starts and the room endpoint still works while `/` returns 404 until the export exists (stop the process afterwards).
- [ ] 5.2 Update `README.md`: the Godot 4.7 and Web export template requirement, a WebGL 2 browser requirement, the import/export/run commands, the generated `client/build/web` output, the new project layout, the environment table, and the protocol section pointing at `client/scripts/protocol.gd`; verify no `assets/`, `app.js`, `styles.css`, or `websocket.js` references remain and each documented command matches design.md D10.
- [ ] 5.3 Note in the README's screenshots section that the images show the Canvas client and should be regenerated from the Godot build; verify the note is present and links still resolve.

## 6. Cutover and integration

- [ ] 6.1 Remove `assets/` (`git rm -r assets`) and verify `grep -rn "assets/" --exclude-dir=.cache --exclude-dir=.git .` returns no code, config, or documentation references.
- [ ] 6.2 Run `go vet ./...`, `go test ./...`, `go test -race ./...`, and `openspec validate migrate-client-to-godot --strict`; verify all pass and the integration suite still starts the server on port 8080.
- [ ] 6.3 With Godot 4.7 and the Web export templates installed, run `godot --headless --path client --import`, `godot --headless --path client --export-release "Web"`, then `go run .`; verify `client/build/web` contains `index.html`, `.js`, `.wasm`, and `.pck`, the page loads at `/`, create/copy/leave work, a `?room=` link joins in a second browser tab, movement updates at 30 Hz, the self circle is outlined and labelled, and closing codes surface as specified. This environment has no Godot installation, so record the verification as pending rather than completing it here.

## Workflow follow-up

- Archive the change after review and a green Godot export run.
- Regenerate `docs/lobby.png` and `docs/game.png` from the Godot build once Godot 4.7 is available and drop the README note from task 5.3.
