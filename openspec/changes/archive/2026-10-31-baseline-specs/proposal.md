# Proposal

## Why

Behavior is described across `README.md` and the code, but there are no OpenSpec
capability specs: `openspec list --specs` is empty, so there is no validated,
scenario-level contract for the server and its shared libraries. The hand-written docs
also drift from the code (for example, `README.md` still describes pre-refactor outbox
behavior), which makes them unreliable as a change baseline.

## What Changes

- Add baseline capability specs that describe the current behavior of the Go room game
  as requirements with WHEN/THEN scenarios.
- No runtime, API, or configuration changes; this is a documentation-only baseline.
- Capture the shared `Heap` and `Scheduler` contracts as their own capabilities so future
  changes can target them explicitly.
- State the current non-goals (auth, persistence, reconnect, collision, scoring, TLS,
  horizontal scaling) so later changes do not silently widen scope.

## Capabilities

### New Capabilities

- `room-lifecycle`: room creation, admission and capacity, spawn assignment, join
  success, disconnect, room timeout, close ordering, and server shutdown.
- `room-gameplay`: authoritative world bounds, latest-state input, server-side movement,
  and the 30 Hz tick that produces snapshots.
- `room-messaging`: per-player outbox channels, broadcast snapshots, targeted private
  messages, latest-per-type coalescing, and the extension channel.
- `wire-protocol`: HTTP and WebSocket endpoints, JSON envelope and message types,
  handshake defaults and limits, close-code mapping, and the client-facing frame shapes.
- `server-configuration`: mandatory `.env` loading, validation of the six variables, and
  compiled-in world and gameplay constants.
- `shared-scheduler`: one-shot, absolute-time and periodic scheduling, callback execution
  model, and `Shutdown` semantics, including how the room schedules `ROOM_TIMEOUT`.
- `shared-heap`: the generic priority-heap contract used by the scheduler (ordering,
  construction, and empty-safe operations).

### Modified Capabilities

- None; the project has no existing capability specs.

## Impact

- Documentation only: adds `openspec/changes/baseline-specs/specs/<capability>/spec.md`
  and this proposal. No Go, JavaScript, or deployment changes.
- Source of truth: the current code at HEAD plus `README.md`; where they disagree, the
  code wins and the intent will be called out for a follow-up docs fix.
- The browser client in `assets/` remains a reference implementation; its UI behavior is
  intentionally not specified in this baseline.
- After review, archiving the change materializes `openspec/specs/`.
