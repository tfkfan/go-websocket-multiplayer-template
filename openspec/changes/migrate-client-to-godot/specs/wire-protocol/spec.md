# Spec Delta

## ADDED Requirements

### Requirement: Web export asset delivery

The server SHALL deliver the Godot Web export as static files: `/` SHALL return the exported
`index.html`, `.wasm` files SHALL be served as `application/wasm`, and `.pck` files SHALL be
served as `application/octet-stream`.

#### Scenario: Root serves the export entry document

- **WHEN** a client requests `/` and the asset root contains `index.html`
- **THEN** the server returns that document as HTML

#### Scenario: WebAssembly module has its content type

- **WHEN** a client requests a `.wasm` file from the export
- **THEN** the response content type is `application/wasm`

#### Scenario: Game pack has its content type

- **WHEN** a client requests a `.pck` file from the export
- **THEN** the response content type is `application/octet-stream`
