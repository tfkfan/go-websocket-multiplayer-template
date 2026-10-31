# Spec Delta

## MODIFIED Requirements

### Requirement: Endpoint and asset wiring

`PORT` SHALL be passed to `net.Listen`; `WS_ENDPOINT` and `ROOM_ENDPOINT` SHALL be registered on
the HTTP mux; `ASSETS_DIR` SHALL be the root of the built Godot Web export, and a missing or
unbuilt directory SHALL yield 404 responses. Endpoint patterns SHALL not be validated beyond the
mux behavior, and the Godot client SHALL keep its own hard-coded endpoint paths.

#### Scenario: Missing assets directory yields 404s

- **WHEN** `ASSETS_DIR` points to a directory that does not exist or has no `index.html`
- **THEN** asset requests return 404 and the WebSocket and room endpoints still work

#### Scenario: Changing endpoints also requires a client change

- **WHEN** an endpoint variable differs from the value hard-coded in the Godot client
- **THEN** the Godot client cannot reach that endpoint until it is updated
