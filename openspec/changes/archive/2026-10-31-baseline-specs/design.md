# Design

## Context

The repository is a single Go binary. `internal/config` loads `.env`; `internal/server`
owns HTTP/WS routing and the room registry; `internal/game/room` runs one actor per room
(commands, tick, snapshot publication, per-connection writer, close); `internal/game/player`
holds movement, the snapshot enum, and the latest-per-type outbox; `internal/shared` holds
the generic heap and the timer scheduler used for `ROOM_TIMEOUT`. The browser client in
`assets/` is a reference consumer of the same WebSocket protocol.

Motivation is in [proposal.md](proposal.md). The relevant constraint: the behavior is
already implemented and running; this change must describe it, not alter it.

## Goals / Non-Goals

**Goals:**

- Capture the current externally observable behavior in seven cohesive capabilities with
  testable WHEN/THEN scenarios.
- Make room lifecycle, gameplay, messaging, protocol, configuration, scheduler, and heap
  contracts independently ownable for future changes.
- Keep the artifacts verifiable: `openspec validate --strict` clean, scenarios traceable to
  code paths or tests.

**Non-Goals:**

- Any runtime, API, database, dependency, or configuration change.
- Specifying browser UI details (DOM, styling, text); only client-facing protocol behavior
  is captured, under `wire-protocol`.
- Fixing documentation drift in this change (for example README wording or the removed
  `SPEC.md`); the baseline describes code behavior and leaves doc cleanup to a follow-up.
- Adding product requirements that do not exist yet (auth, persistence, scoring, collision,
  reconnect, TLS, scaling).
- Introducing a design document as an implementation plan; this file records the decisions
  behind the spec baseline only.

## Decisions

1. **Seven capabilities rather than one `game-room` capability.**
   `room-lifecycle`, `room-gameplay`, `room-messaging`, `wire-protocol`,
   `server-configuration`, `shared-scheduler`, and `shared-heap` each own a distinct system
   behavior. A single catch-all capability would force unrelated future deltas to collide in
   one file; splitting also lets the scheduler and heap evolve without touching room specs.
   Alternative: one capability per source package. Rejected because packages are
   implementation boundaries, not behavior boundaries (e.g. messaging spans player and room
   code).

2. **Specs describe observable behavior, not internals.**
   Requirements use SHALL/MUST plus WHEN/THEN scenarios that a test or protocol client can
   exercise. Internal names, goroutine counts, and data structures stay out of requirement
   text except where they are part of the contract (the scheduler's callback execution model
   and the heap's typed API are contracts for their consumers). Alternative: mirror the
   previous hand-written spec, including layout and editing guidance. Rejected because that
   content is not behavior and would rot faster.

3. **Capture the baseline as a change with ADDED deltas.**
   The project has no main specs, so every capability is new and uses `## ADDED
   Requirements`. The deltas live in `openspec/changes/baseline-specs/specs/` and become
   `openspec/specs/<capability>/spec.md` when the change is archived. Alternative: write
   main specs directly. Rejected because it bypasses validation, the proposal/spec contract,
   and the archive-based history that OpenSpec relies on.

4. **The code at HEAD is the source of truth.**
   Where README or the removed `SPEC.md` disagreed with the implementation, the
   implementation wins. This keeps the baseline honest and makes any follow-up doc fix
   explicit instead of silently baking old prose into specs.

5. **English, spec-native wording.**
   The project's docs, identifiers, and tests are English, so the specs are English and use
   the OpenSpec delta format (`## Purpose`, `## ADDED Requirements`, `### Requirement:`,
   `#### Scenario:`).

6. **The web client is out of scope.**
   `assets/` remains a reference implementation. Its hard-coded endpoints and the frames it
   consumes are covered by `wire-protocol`; its rendering and UI behavior are not
   specified.

## Risks / Trade-offs

- [Specs drift from code again] -> Keep `openspec validate --strict` in review; add a
  follow-up to run it in CI once the baseline is archived.
- [Over-specification of volatile details] -> Prefer observable outcomes over internal
  mechanics; exact close reasons are limited to values the server actually sends
  (`room closed`, `player already connected`).
- [Baseline codifies demo limitations as requirements] -> Limitations such as “no collision”
  and “no reconnect” are stated explicitly so future changes must modify the spec instead of
  silently widening scope.
- [Archive rewrites the layout of specs] -> Verify `openspec/specs/` with
  `openspec validate --specs --strict` immediately after archiving.
- [Concurrent worktree changes mix with the baseline] -> This change touches only
  `openspec/`; the apply step checks that no Go or JS files were modified by it.

## Migration Plan

1. Review the proposal and the seven delta specs.
2. Run `openspec validate baseline-specs --strict` and fix any findings.
3. Confirm the capability list in the proposal matches the spec directories and that no
   runtime files changed.
4. Archive `baseline-specs` to create `openspec/specs/<capability>/spec.md`, then validate
   the main specs.

Rollback: before archiving, delete or edit the change directory (no runtime impact). After
archiving, revert the commit that contains the archived change and the new main specs.

## Open Questions

- Should the browser client later become its own capability (`web-client`) covering create,
  join, render, input, and close UX? Deferrable: it does not change these specs or the room
  protocol.
