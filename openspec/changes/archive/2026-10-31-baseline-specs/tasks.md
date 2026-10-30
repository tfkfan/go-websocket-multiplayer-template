# Tasks

## 1. Verify room capabilities against the implementation

- [x] 1.1 Confirm `room-lifecycle` requirements (admission, duplicate/full rejection, spawn slots, timeout, close ordering, shutdown) against `internal/game/room/game_room.go`, `commands.go`, and `players.go`; adjust the delta if any scenario does not match the code
- [x] 1.2 Confirm `room-gameplay` requirements (world bounds, input bits, movement, diagonal normalization, dt cap, no collision) against `internal/game/player/player.go`, `snapshot.go`, and the room tick loop; adjust the delta if needed
- [x] 1.3 Confirm `room-messaging` requirements (outbox channels and drain order, coalescing, shared UPDATE, private errors, extension type) against `internal/game/player/outbox.go`, `PublishTo`, and `publishSnapshot`; adjust the delta if needed

## 2. Verify protocol, configuration, and shared capabilities

- [x] 2.1 Confirm `wire-protocol` requirements (routes, POST response, envelope, message numbers, handshake defaults, close codes) against `internal/game/room/protocol.go`, `internal/server/game_server.go`, `main.go`, and the `coder/websocket` defaults; adjust the delta if needed
- [x] 2.2 Confirm `server-configuration` requirements (mandatory `.env`, six variables, validation, endpoint wiring, fixed constants) against `internal/config/config.go`, `.env`, and the room constants; adjust the delta if needed
- [x] 2.3 Confirm `shared-scheduler` and `shared-heap` requirements (timer semantics, Shutdown, room timeout integration, typed heap API) against `internal/shared/scheduler.go`, `heap.go`, and the room timeout test; adjust the deltas if needed

## 3. Validate and freeze the baseline

- [x] 3.1 Run `openspec validate baseline-specs --strict` and fix every finding
- [x] 3.2 Confirm the proposal's capability list matches the spec directories and that every delta spec validates
- [x] 3.3 Confirm the change introduces only `openspec/` artifacts and no Go, JavaScript, or configuration edits from this change

## Workflow follow-up

- Archive the change to materialize `openspec/specs/` after review.
- Run `openspec validate --specs --strict` on the archived main specs.
