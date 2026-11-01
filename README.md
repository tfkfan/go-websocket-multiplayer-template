# Circles — realtime room game

A room-based multiplayer game in Go with a Godot 4.7 web client. Each room owns a shared
**1200 × 800** field. Players either open a shared room link, create a room themselves, or press
**Play** and get matched with the next waiting players; everyone moves with WASD or the arrow keys
and renders as a colored circle. The server is authoritative and broadcasts full state 30 times per
second.

## Screenshots

**Lobby** — create a linkable room, or press **Play** to enter matchmaking:

![Lobby with Create room and Play](docs/lobby.png)

**Gameplay** — two matched players in one room. Your circle has a white outline and a `you` label;
the side panel shows the room, player count, position, and update tick:

![Two matched players in a room](docs/game.png)

## What it does

- `POST /room` creates a room and returns its UUID; the Godot client joins it immediately.
- A room link (`/?room=<uuid>`) admits up to `ROOM_CAPACITY` players.
- A WebSocket connection **without** a room enters matchmaking: the Matchmaker queues the player
  and blocks until `ROOM_CAPACITY` players are waiting, then creates a room and places the whole
  batch before any queued caller returns. Disconnecting removes the player from the queue, wherever
  it sits.
- Movement is server-side: 260 world units per second, normalized diagonals, circles clamped inside
  the rectangle. A player that contacts another slides out of overlap; the other player is never
  moved by someone else's collision.
- Every player gets a spawn slot around the field center and a palette color.
- Every 1/30 s the room actor serializes one full `UPDATE` snapshot and publishes it to every
  connected player; clients redraw from it rather than predicting state.
- Rooms expire after `ROOM_TIMEOUT`; clients receive a `1001 Going Away` close. Invalid, unknown,
  or already-closed rooms fail the upgrade with `1014`. A full room or duplicate player currently
  receives neither join data nor a close frame (recorded in the specs).

The game is intentionally a demo/template: there is no authentication, persistence, reconnect,
scoring, TLS, or multi-process state. The behavioral contract lives in OpenSpec capability specs
under [`openspec/specs/`](openspec/specs/) — see
[AI-assisted development](#ai-assisted-development-with-openspec).

## Architecture

```text
                    client (Godot Web export)
          POST /room        WS ?room=<uuid>        WS (no room)
              |                   |                     |
              +---------- Matchmaker (server object) --+
                          rooms map  +  pending queue
                          store/load   FIFO, capacity-based
                              |              |
                              |        match formed, batch placed
                              |              |
                          +-----------------------+
                          | room actor            |
                          | membership, commands, |
                          | ticker, timeout       |
                          +-----------------------+
                              |
                    game room policy (Circles)
```

- The **Matchmaker** (`internal/core/server`) owns the listener, HTTP limits, static assets, room
  storage keyed by UUID, and the waiting queue. It is game-agnostic: a `Definition` supplies the
  room factory, pending-player construction, blocking placement, and post-match read loop.
- The queue (`pendingQueue`) is a mutex-guarded FIFO with one wake signal. A joining caller appends
  itself and waits; the caller that completes a batch takes the oldest capacity-sized subslice under
  a formation lock, creates and places the room, and only then returns the room to every member.
  A failed placement requeues survivors at the front and pauses formation until a new arrival.
- **Direct rooms** and **matchmaking** share storage but not queue slots: `POST /room` plus
  `?room=` keep working exactly as before.
- One **room actor** (`core/room.Room.Run`) owns membership, the command queue, the ticker, the
  timeout, delivery, and close ordering. The game layer owns simulation, spawn slots, contacts, and
  UPDATE construction.
- A **member** pairs an entity with an optional session. A player is the current entity kind and the
  only kind that contributes to UPDATE; an entity without a session is still simulated, indexed, and
  collided, but cannot be addressed.
- Each connection has exactly **one reader** (the HTTP handler or the queue read pump) and **one
  writer** goroutine. Movement keys are an atomic bitmask updated by the reader; the room tick reads
  the latest state, with no key-event queue.
- Each tick moves every member in stable key order, refreshes the room quadtree once, and relocates
  exactly one body per contact (the only mover, or the later mover when both moved). Corrected
  positions are published immediately; the index catches up on the next tick's refresh.
- Each **session** owns an outbound mailbox: a map from `SnapshotType` (`SystemType`, `PrivateType`,
  `BroadcastType`, `CustomType`) to the latest payload of that type; its writer drains one payload
  per type in numeric order. `SendTo`/`PublishTo` stage bytes directly into the target session's
  outbox — no actor command.

## Requirements

| Tool | Version | Needed for |
| --- | --- | --- |
| [Go](https://go.dev/dl/) | `1.27.3` or newer | building, running, and testing the server; `go.mod` declares `go 1.27.3` |
| [Godot](https://godotengine.org/download) | `4.7` with the Web export templates | rebuilding and re-exporting the client; the project targets `4.7` in `client/project.godot` |
| [OpenSpec CLI](https://openspec.dev/) | `1.16.0` or newer | the spec workflow, validation, and archive (`openspec --version`) |
| Browser | WebGL 2 with WebSocket support | playing the Godot Web export |

Also required:

- A `.env` file in the working directory (checked in; all six values are required)
- The configured port free (sample: `8080`)
- An agent with the OpenSpec commands is optional and only needed for the assistant-driven spec
  workflow; the CLI alone can list, validate, and archive

This checkout is set up for Go `1.27.3`, Godot `4.7`, and OpenSpec CLI `1.16.0`:

```bash
go version            # go1.27.3
godot --version       # 4.7 (Web export templates installed)
openspec --version    # 1.16.0
```

Godot is only needed when rebuilding the client; the exported files under `client/build/web` are
enough to serve the game.

## Run

Build the Godot client first (once, and after any client change), then start the server. The export
lands in the ignored `client/build/web` directory, which `ASSETS_DIR` points at. Godot's CLI does
not create the output folder, so create it before the first export.

```bash
mkdir -p client/build/web
godot --headless --path client --import
godot --headless --path client --export-release "Web"
go run .
```

`build_godot_web.sh` is a local convenience script for the same export; it hard-codes the Godot path
and XDG cache used by this checkout, so adjust those paths before reusing it elsewhere.

Without an export, the room and WebSocket endpoints still work, but `/` returns 404 because there is
no `index.html` to serve. Expected startup output:

```text
listening on http://[::]:8080
listening websocket on ws://[::]:8080/ws
```

Open <http://localhost:8080> and either:

- press **Play** to enter matchmaking (a room is created once `ROOM_CAPACITY` players are waiting),
  or
- press **Create room** and share the displayed link; opening the link joins that room.

## Controls

| Input | Action |
| --- | --- |
| `W` / `↑` | Move up |
| `S` / `↓` | Move down |
| `A` / `←` | Move left |
| `D` / `→` | Move right |

Your circle is outlined in white and labelled **you**. Releasing the window or switching tabs
releases all movement keys.

## Configuration

`.env` is loaded at startup. All values are mandatory and process environment variables take
precedence over file values.

| Variable | Sample | Meaning |
| --- | --- | --- |
| `PORT` | `:8080` | `net.Listen` address |
| `WS_ENDPOINT` | `/ws` | WebSocket upgrade path |
| `ROOM_ENDPOINT` | `/room` | Room-creation path |
| `ASSETS_DIR` | `./client/build/web` | Godot Web export root |
| `ROOM_TIMEOUT` | `30m` | Fixed room lifetime; positive duration |
| `ROOM_CAPACITY` | `2` | Match size, maximum players per room, and spawn slots; positive integer |

The checked-in `.env` uses `ROOM_CAPACITY=2`, so **Play** starts a game as soon as two players are
waiting. The client hard-codes `/room` and `/ws`; changing the endpoint variables also requires
editing `client/scripts/protocol.gd`.

World and gameplay constants are compiled in:

| Constant | Value |
| --- | --- |
| World size | `1200 × 800` |
| Player radius | `25` |
| Tick rate | `30 Hz` |
| Movement speed | `260 units/s` |
| Outbound state | Latest snapshot per player (coalesced) |

## Matchmaking rules

- Queue order is FIFO; a batch is always the oldest `ROOM_CAPACITY` waiting players.
- Waiting players block until their room exists and they are placed; a new caller can complete a
  batch that includes earlier waiters.
- Placement is atomic: every member is admitted before any caller returns. If placement fails
  (for example a player disconnects mid-placement), the room is discarded and surviving players
  stay queued for the next match.
- A disconnect removes the pending player from anywhere in the queue and does not consume a capacity
  slot.
- Direct rooms are unaffected: they are created on demand, joined by link, and never enter the
  matchmaking queue.

## Wire protocol

Messages are JSON text frames with a numeric type and payload: `{"type": <number>, "data": ...}`.
The server numbers form the `event.MessageType` enum (`internal/game/event/protocol.go`); the Godot
client mirrors them in `client/scripts/protocol.gd`.

| Direction | Type | Payload |
| --- | --- | --- |
| Client → server | `200` | `{key: <direction>, state: bool}` where direction is `UP`, `DOWN`, `LEFT`, or `RIGHT` |
| Server → client | `4` | Join success: `{playerId, roomId, width, height, radius, color, tickRate}` |
| Server → client | `100` | Update: `{players:[{id, position:{x,y}, radius, color}], width, height}` |
| Server → one client | `110` | Private event produced by `GameRoom.SendTo` |

`playerId` from JOIN_SUCCESS identifies the client's circle inside the `players` array; the public
UPDATE is identical for every recipient and lists only players, ordered by player key. Private `110`
frames are sent only to their target and are never included in the shared UPDATE. Updates are full
snapshots, not deltas. Unknown or malformed client messages are logged and ignored.

The room comes from the WebSocket query string: a valid `room` joins that room directly, no `room`
parameter enters matchmaking, and a malformed `room` closes with `1014`. No client `JOIN` message is
required. `X-Player-Id` is optional for non-browser clients; duplicates are rejected. UPDATE carries
no `tick` field, so the client's tick statistic reads zero.

## Targeted messages

The shared UPDATE is public and must not carry per-player secrets. Use `SendTo` to set the target
session's private data slot:

```go
err := gr.SendTo(playerID, event.MsgPrivate, map[string]string{
    "message": "only this player receives this",
})
```

`SendTo` marshals the payload and stages it in the target’s `PrivateType` outbox slot — no actor
command. The session writer drains slots in numeric order (`SystemType`, `PrivateType`,
`BroadcastType`, `CustomType`), so private data is never embedded in UPDATE.

The outbox has one latest slot per type, so multiple `SendTo` calls coalesce until the writer drains
that slot: the last private payload wins. Broadcast data is the same shared `UPDATE` for every
player.

Extension types use `PublishTo`:

```go
err := gr.PublishTo(playerID, snapshot.CustomType, myMessageType, payload)
```

Add a `SnapshotType` constant before `TypeCount` in
[snapshot.go](internal/core/snapshot/snapshot.go) and use it through `PublishTo`; the outbox and
writer pick it up in numeric order.

`SendTo` returns `room.ErrPlayerNotFound` for missing/stopped players and `room.ErrRoomClosed` while
closing. A nil return means staged for the writer, not delivered; a stop racing with `SendTo` drops
the frame. It does not need to run on the actor goroutine — actor code can publish through the
member's session directly.

Private frames use type `110` and are normal top-level events. The Godot client does not handle
`PRIVATE` yet; the message dispatch in `client/scripts/network.gd` is the extension point.

Close codes: `1014` for an invalid or malformed room ID, unknown room, or already-closed room;
`1001` for room expiry or a server-initiated room close; `1009` for messages over 32 KiB; `1011`
when a writer fails. A join rejected by the room's own admission checks (full room or duplicate
player) currently receives neither join data nor a close frame — the specs record this gap
explicitly. Server shutdown may produce `1006` because the server context is canceled before room
close handshakes.

## Project layout

```text
main.go                         server bootstrap and shutdown
build_godot_web.sh              local Web export helper (hard-coded paths)
internal/config/config.go       .env loading and validation
internal/core/math/             clamped scalar helpers
internal/core/shared/           generic heap, concurrent set, reusable-timer scheduler
internal/core/spatial/          world-bounded quadtree + circle/AABB collision detection
internal/core/entity/           entity contract, base identity/position, world size
internal/core/snapshot/         snapshot types and latest-per-type outbox
internal/core/session/          per-connection outbox, writer, close behavior
internal/core/room/             generic room: membership, commands, ticker, timeout, delivery
internal/core/server/           Matchmaker: listener, routes, assets, room storage, queue, shutdown
internal/game/server/           Circles definition: endpoints, room factory, pending players, placement
internal/game/room/             game policy: admission, spawns, simulation, contacts, UPDATE
internal/game/player/           player entity, controller, profile, movement
internal/game/event/            message numbers, payload shapes, JSON envelope
client/project.godot            Godot 4.7 project settings and Network autoload
client/export_presets.cfg       single-threaded Web export preset
client/scenes/main.tscn         lobby and side-panel scene
client/scenes/game_field.tscn   field rendering scene
client/scripts/main.gd          room/queue flow, layout, input, status
client/scripts/network.gd       HTTP room creation and WebSocket transport
client/scripts/platform.gd      browser bridge and desktop fallback
client/scripts/game_field.gd    field and player drawing
client/scripts/protocol.gd      message numbers and endpoint paths
client/scripts/ui_theme.gd      palette and control styles
client/build/web/               generated Web export (ignored)
docs/                           README screenshots
openspec/config.yaml            OpenSpec workflow schema (spec-driven)
openspec/specs/                 capability specs (behavioral contract)
openspec/changes/               in-flight changes and archive
main_test.go                    integration and leak tests
```

## AI-assisted development with OpenSpec

The project uses [OpenSpec](https://openspec.dev/) (CLI `1.16.0`) for spec-driven development. The
behavioral contract is a set of validated capabilities under
[`openspec/specs/`](openspec/specs/), not a free-form document. The current set passes
`openspec validate --specs --strict`: fourteen capabilities, 104 requirements, and 251 scenarios.
The specs describe current behavior; known gaps (full or duplicate joins receiving no close frame,
and UPDATE carrying no `tick`) are recorded as scenarios rather than quietly omitted.

| Capability | Covers | Requirements |
| --- | --- | --- |
| [`room-lifecycle`](openspec/specs/room-lifecycle/spec.md) | creation, registration, admission, spawn slots, join, rejected joins, disconnect, timeout, close, shutdown | 10 |
| [`room-gameplay`](openspec/specs/room-gameplay/spec.md) | world bounds, input state, movement, tick cadence, collision ordering, one-sided correction, determinism | 8 |
| [`room-messaging`](openspec/specs/room-messaging/spec.md) | outbox channels, coalescing, broadcast UPDATE, writer, private messages, error semantics, extension | 7 |
| [`matchmaking`](openspec/specs/matchmaking/spec.md) | queue admission, blocking capacity formation, atomic placement, disconnect removal, room storage, shutdown, direct-room coexistence | 8 |
| [`entity-model`](openspec/specs/entity-model/spec.md) | heterogeneous membership, connection-less members, per-instance behavior, deterministic ticks, snapshot contribution, game-rule-free core | 7 |
| [`wire-protocol`](openspec/specs/wire-protocol/spec.md) | routing, asset delivery, room creation, direct vs matchmaking room selection, envelope, message numbers, payload shapes, handshake limits, identity, close codes | 12 |
| [`server-configuration`](openspec/specs/server-configuration/spec.md) | `.env`, required variables, validation, endpoint wiring, compiled constants | 5 |
| [`server-runtime`](openspec/specs/server-runtime/spec.md) | startup and listener, HTTP limits, room registry, shutdown triggers and deadline, exit status | 6 |
| [`spatial-index`](openspec/specs/spatial-index/spec.md) | world-bounded quadtree, insertion, bulk replacement, shape/radius/nearest queries, room integration | 8 |
| [`spatial-collision`](openspec/specs/spatial-collision/spec.md) | geometry, penetration, exact contact detection, ordering, read-only reporting | 6 |
| [`shared-scheduler`](openspec/specs/shared-scheduler/spec.md) | timer API, callbacks, shutdown, room timeout integration | 7 |
| [`shared-heap`](openspec/specs/shared-heap/spec.md) | generic heap contract used by the scheduler | 4 |
| [`shared-set`](openspec/specs/shared-set/spec.md) | concurrent keyed set used for room membership | 8 |
| [`godot-client`](openspec/specs/godot-client/spec.md) | project/export, browser bridge, lobby with Play queue, session, rendering, input, statistics, layout | 8 |

### Workflow

Behavior changes are proposed as a change, implemented, and then archived; archiving merges the
delta specs into `openspec/specs/`.

| Step | Purpose | Agent command | CLI |
| --- | --- | --- | --- |
| Explore | think through a problem before proposing | `/opsx:explore` | `openspec list --specs` |
| Propose | create a change with proposal, specs, design, tasks | `/opsx:propose` | `openspec new change <name>` |
| Apply | implement the change and check off tasks | `/opsx:apply` | `openspec status --change <name>` |
| Update | revise a change's planning artifacts | `/opsx:update` | - |
| Sync | merge delta specs without archiving | `/opsx:sync` | - |
| Archive | merge specs and archive the change | `/opsx:archive` | `openspec archive <name>` |

The `/opsx:*` commands and matching skills are installed locally under `.claude/` and `.dsh/`
(both gitignored). The portable part of the workflow is the `openspec/` tree plus the OpenSpec CLI.

Useful commands:

```bash
openspec list --specs                  # capabilities and requirement counts
openspec show room-lifecycle --type spec
openspec list                          # active changes
openspec validate --specs --strict     # validate the main specs
openspec validate <change> --strict    # validate a change's deltas
```

Rules of thumb:

- Do not edit `openspec/specs/` directly for a behavior change; add a change under
  `openspec/changes/` and let archive merge it.
- Keep scenarios observable and testable; they double as acceptance criteria.
- Validate strictly before review or archive.

## Test

```bash
go test ./...
go test -race ./...
```

The integration test starts the real server on port **8080**, creates a direct room, connects two
players, validates JOIN_SUCCESS and the UPDATE shape, sends a `RIGHT` key event, and asserts that
the player's `position.x` increases. The collision test drives two players together and asserts they
never overlap in UPDATE. Unit tests cover the queue struct and matchmaker (capacity, FIFO, removal,
failed-placement requeue, shutdown, concurrency), the room actor's batch admission, sessions and
writers, spatial structures, and the scheduler/set/heap helpers. The suite takes a few seconds and is
not parallel-safe because the integration test uses a fixed port; on a machine where `:8080` is
already taken, run it in an isolated network namespace (`unshare -rn`) or stop the other server.

The Godot client is not exercised by the Go suite. Rebuild it with the export commands above and
verify in a browser; `godot --headless --path client --import` catches script and scene parse errors.
Specs are validated separately with `openspec validate --specs --strict`.

## Limitations

- Queue state and rooms are in memory. There is no matchmaking rating, team/party grouping, queue
  timeout, maximum queue length, or fairness policy; waiting players stay queued until capacity is
  reached or they leave.
- Room timeout is fixed from creation; player activity does not extend it.
- Room closing waits for WebSocket close handshakes (5 s to write the close frame plus 5 s for the
  peer to answer, so up to ~10 s), and the room stays in storage until that finishes; joins during
  that window are rejected as closed.
- Collisions are discrete tick detection, not continuous collision detection: fast objects can
  tunnel. Correction is one-sided and processed in stable player order, so a head-on pair is
  resolved in favor of the earlier mover, and dense clusters or wall corners may retain a small
  residual overlap in a single pass. Correction reaches the spatial index on the next tick's refresh.
- No rounds, scoring, win conditions, spectators, or reconnect/session resumption.
- No obstacles, collectable items, shots, or area skills are spawned yet. The spatial engine
  supports them, but there is no wire format or rendering, and spatial queries must run on the room
  actor goroutine.
- Room UUID is the only access capability; there is no auth, rate limiting, TLS, health endpoint,
  metrics, or multi-instance support.
- The Go server never builds or checks the client: `/` returns 404 until the Web export exists under
  `client/build/web`, and the Godot Web payload (`index.wasm`) is tens of megabytes.
- Slow clients skip intermediate snapshots; each outbox keeps only the latest payload per type, and
  `SendTo` calls coalesce to the last private payload until the writer drains the slot.
- UPDATE carries no tick counter, so the client panel's `tick` line stays at `0`.
