# LaraDeck Architecture & Technical Design

This document details the internal architecture, design principles, and technical implementation of **LaraDeck**.

---

## 1. High-Level Architecture

LaraDeck is built with **Go** (for native system calls, process containment, and high-performance HTTP engines) and **React 18 + Vite 7 + TypeScript** (for responsive desktop UI). It packages into a single native Windows x64 executable via **Wails v2**, while simultaneously supporting a headless/loopback **Browser Dashboard** mode.

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                                     LARADECK SUITE                                      │
├────────────────────────────────────────────┬────────────────────────────────────────────┤
│           DESKTOP HOST (Wails v2)          │         BROWSER DASHBOARD (/api/call)      │
│     Native Windows Edge WebView2 Window    │     Loopback HTTP (127.0.0.1:<random_port>)   │
└────────────────────────────────────────────┴────────────────────────────────────────────┘
                                      │
                        ┌─────────────▼─────────────┐
                        │   Wails IPC / HTTP Proxy  │
                        │    browserPlatform.ts     │
                        └─────────────┬─────────────┘
                                      │
                        ┌─────────────▼─────────────┐
                        │    Go Core (app.go)       │
                        │    internal/workbench/    │
                        └─────────────┬─────────────┘
                                      │
     ┌──────────────────┬─────────────┼──────────────────┬──────────────────┐
     ▼                  ▼             ▼                  ▼                  ▼
┌──────────────┐ ┌──────────────┐ ┌───────────────┐ ┌──────────────┐ ┌──────────────┐
│  Runtimes &  │ │ Zero-Package │ │   Events &    │ │ Performance  │ │   Database   │
│   Services   │ │   Telemetry  │ │   Queues      │ │    Radar     │ │   & Tinker   │
│ (PHP/Herd/   │ │ (AutoPrepend │ │ (Supervisor/  │ │ (Checkup/    │ │ (ERD/Safety/ │
│  Composer)   │ │  Agent/Logs) │ │  Reflection)  │ │  Load Tester)│ │  Explains)   │
└──────────────┘ └──────────────┘ └───────────────┘ └──────────────┘ └──────────────┘
```

---

## 2. Core Subsystems

### A. Dual Delivery Engine (`internal/apphost/`)
- **Native Desktop**: Utilizes Microsoft Edge WebView2 on Windows for hardware-accelerated rendering and native desktop window controls.
- **Loopback Browser Host**: When started in headless or browser mode, LaraDeck spins up an ephemeral HTTP server on loopback `127.0.0.1`.
- **Session Security**: Browser sessions authenticate via high-entropy cryptographically generated bearer tokens. Cross-origin requests are rejected by strict origin verification.
- **Transparent IPC Bridge (`frontend/src/browserPlatform.ts`)**: An ES6 `Proxy` wraps `window.go.main.App`. When running inside a standard browser, calls are seamlessly converted to JSON-RPC over `/api/call` without changing any component logic. Server-Sent Events (SSE) pipe real-time notifications to the frontend.

### B. Zero-Package Debugger Agent (`internal/companion/`)
- **Attachment Technique**: Uses PHP's `.user.ini` mechanism placed in `public/.user.ini` (or the project root) with:
  ```ini
  auto_prepend_file = "C:/Users/<Username>/.laradeck/agent/laradeck-agent.php"
  ```
- **Non-Intrusive Lifecycle**:
  1. Bootstrapped before any project code executes.
  2. Intercepts Symfony's `VarDumper::setHandler()`.
  3. Registers listeners for `Illuminate\Database\Events\QueryExecuted`, `Illuminate\Log\Events\MessageLogged`, and `Illuminate\Foundation\Http\Events\RequestHandled`.
  4. Collects microsecond timestamps, memory usage, queries, and logs into a single `RequestTelemetryFrame`.
  5. Transmits telemetry via non-blocking HTTP socket back to LaraDeck's ingest listener (`127.0.0.1:49152/telemetry`).
- **Dumps In HTML Toggle**: Developers can toggle between in-app stream capture or injecting HTML dump blocks directly into the browser's HTTP response.

### C. Process Containment & Runner (`cmd/runner/` & `internal/runner/`)
- **Windows Job Objects**: On Windows, child processes (Vite dev server, queue workers, artisan commands) are assigned to dedicated Windows Job Objects.
- **Leak-Proof Termination**: When LaraDeck exits or a task is stopped, the entire process tree is terminated atomically, preventing orphaned Node or PHP background processes from consuming ports or CPU.
- **Runner Helper**: `laradeck-runner.exe` acts as an unprivileged child worker helper that handles stdin/stdout redirection, process elevation, and WSL bridge execution.

### D. Activity Tracing Engine (`internal/activity/`)
- Records structured request spans:
  - Middleware execution
  - Controller actions
  - Eloquent database queries
  - Domain events & listeners
  - Blade / Inertia view rendering
- **App vs Vendor Segregation**: Labels spans with `isFramework` boolean flags so developers can filter out repetitive Laravel framework layers and focus purely on application business logic.
- **Offline HTML Export**: Generates zero-dependency standalone HTML files featuring interactive CSS flamegraph styling for sharing with teammates or attaching to pull requests.

### E. Events & Queues Pro Workbench (`internal/actions/`)
- **Reflection Parser**: Analyzes event and job classes directly using local PHP runtime reflection.
- **Dynamic Parameter Forms**: Extracts parameter types, default values, and nullable flags to dynamically construct input forms.
- **Source Code Dispatch Scanner**: Scans project PHP files for AST / regex references where `event(new ...)` or `Job::dispatch(...)` is invoked, providing instant clickable links to code occurrences.
- **Synchronous Execution ("Run Now")**: Executes the job's `handle()` method synchronously inside a captured artisan runner environment to inspect return values, query count, and duration without waiting for a queue worker.

### F. Webhook Route Diagnostics (`internal/webhooks/`)
- **Route Scanner**: Inspects `routes/api.php`, `routes/web.php`, and controller annotations for webhook patterns (`stripe`, `github`, `shopify`, `paddle`, generic).
- **Pre-flight Checks**:
  1. **CSRF Risk**: Flags routes running through `web` middleware groups without an exclusion in `bootstrap/app.php`.
  2. **Auth Traps**: Identifies `auth` or `auth:sanctum` middleware that would reject incoming third-party webhooks with HTTP 401 or HTTP 302 redirects.
  3. **Secret Verification**: Verifies presence of webhook secret keys in `.env`.
- **HMAC Signer**: Calculates provider-compliant HMAC-SHA256 signatures for testing and replaying requests.

### G. Performance Radar & Authenticated Load Tester (`internal/performance/`)
- **Checkup Ruleset**: Audits OPcache, route caching, config caching, event caching, view caching, composer autoloader optimization, profiler overhead, and database/queue drivers.
- **1-Click Remediation**: Directly executes artisan optimize commands and updates configurations.
- **Concurrent Load Engine**: High-concurrency native Go HTTP client pool benchmarking throughput (RPS), min/max/avg latency, and P50/P90/P99 percentiles.
- **Auth Credentials Provider**: Leverages the project's Laravel session driver to generate authentic session cookies or Sanctum tokens for real user IDs, enabling realistic authenticated route benchmarks.

---

## 3. Truthful Telemetry Principles

LaraDeck adheres strictly to **zero simulated telemetry**:
- No synthetic timeline placement.
- No hardcoded "healthy" flags.
- Real microsecond relative offsets (`StartMs`, `DurationMs`).
- Real socket probes for Reverb/WebSocket connections.
- Real database and queue connection checks.
