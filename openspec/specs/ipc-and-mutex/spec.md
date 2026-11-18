# ipc-and-mutex Specification

## Purpose

Provides cross-process singleton and CS2 render-exclusivity locking alongside a bidirectional JSON-RPC 2.0 and event-streaming IPC channel between `strikeye.exe` and `strikeye-agent.exe`.

## Requirements

### Requirement: Cross-process singleton and CS2 render mutexes
The IPC and lock subsystem SHALL expose distinct cross-process mutual-exclusion locks for the agent singleton (`Local\StrikeyeAgentSingletonMutex`), UI singleton (`Local\StrikeyeUISingletonMutex`), and active CS2 rendering (`Local\StrikeyeCS2RenderActiveMutex`), supporting non-blocking ownership probing and clean release.

#### Scenario: Concurrent mutex acquisition blocked
- **WHEN** process A holds the active CS2 render mutex and process B attempts a non-blocking acquire
- **THEN** process B receives an already-locked result without blocking or displacing process A

#### Scenario: Mutex released on explicit release or owner termination
- **WHEN** the lock holder releases the mutex
- **THEN** subsequent acquisition by another caller succeeds immediately

### Requirement: Bidirectional JSON-RPC 2.0 IPC transport
The IPC subsystem SHALL run a JSON-RPC 2.0 server and client over Windows Named Pipe (`\\.\pipe\strikeye-ipc`) on Windows targets (and an abstracted local stream transport for cross-platform tests), handling concurrent client requests, malformed frame errors, and clean shutdown.

#### Scenario: Request-response round trip
- **WHEN** an IPC client sends a valid JSON-RPC 2.0 request such as `agent.ping` or `render.status`
- **THEN** the server invokes the registered handler and returns a matching JSON-RPC 2.0 response with the same request ID

#### Scenario: Malformed JSON-RPC payload
- **WHEN** an IPC client sends invalid JSON or an unknown method name
- **THEN** the server returns a standard JSON-RPC error response without closing or crashing the server loop

### Requirement: Server-to-client event subscription stream
The IPC subsystem SHALL allow connected clients to subscribe to asynchronous agent events (such as demo ingestion, parse progress, and render task state updates) delivered concurrently without blocking RPC request/response handling.

#### Scenario: Event broadcast to subscribed client
- **WHEN** the server emits a status event while a client subscription is active
- **THEN** the client receives the notification frame in order and continues processing independent RPC calls
