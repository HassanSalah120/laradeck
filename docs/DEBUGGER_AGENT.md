# Zero-Package Debugger Agent Guide

LaraDeck includes a zero-package, non-intrusive PHP debugger agent located at:
`internal/companion/laradeck-agent.php`

---

## 1. How It Works

Traditional PHP debuggers and profilers require installing vendor packages into `composer.json` or configuring complex native Xdebug extensions. While LaraDeck fully supports Xdebug, it also provides a lightweight, zero-package agent that attaches without modifying your project's codebase.

### Attachment via `.user.ini`

When you click **Attach Agent** in LaraDeck:
1. LaraDeck ensures the agent script is copied to `%USERPROFILE%/.laradeck/agent/laradeck-agent.php`.
2. LaraDeck creates or updates `public/.user.ini` (or the project root `.user.ini`) with:
   ```ini
   auto_prepend_file = "C:/Users/<YourUser>/.laradeck/agent/laradeck-agent.php"
   ```
3. When PHP-FPM, PHP FastCGI, or Laravel Herd serves a request, PHP automatically loads `laradeck-agent.php` before executing `public/index.php`.

---

## 2. Capabilities

### In-App Dumps Stream (`dump()` & `dd()`)
- Intercepts Symfony's `VarDumper::setHandler()`.
- Captures variable dumps without halting the HTTP request.
- Streams formatted dumps with file name, line number, timestamp, and syntax highlighting directly to LaraDeck's **In-App Dumps** panel.

### Dumps In HTML Response Toggle
- By default, dumps are captured into LaraDeck's desktop interface so your API responses and JSON payloads stay clean.
- If you prefer seeing dumps rendered directly inside your browser response (standard Laravel behavior), toggle **Render Dumps in HTML Response** in the LaraDeck toolbar.

### Full Request-Lifecycle Telemetry
The agent automatically hooks into Laravel's core event dispatcher:
- `Illuminate\Database\Events\QueryExecuted`: Captures executed SQL queries, bindings, execution duration in milliseconds, and database connection name.
- `Illuminate\Log\Events\MessageLogged`: Captures log messages and severity levels (`emergency`, `alert`, `critical`, `error`, `warning`, `notice`, `info`, `debug`).
- `Illuminate\Foundation\Http\Events\RequestHandled`: Computes request execution duration and peak memory usage (`memory_get_peak_usage()`).

When the request completes, the telemetry payload is dispatched asynchronously over loopback HTTP to LaraDeck's telemetry ingest server:
```json
{
  "id": "req_01hxyz...",
  "method": "GET",
  "url": "http://myapp.test/api/orders",
  "status": 200,
  "durationMs": 42.5,
  "memoryPeakBytes": 18454912,
  "queryCount": 3,
  "queries": [...],
  "logs": [...],
  "dumps": [...]
}
```

---

## 3. Detaching the Agent

Click **Detach Agent** in the LaraDeck toolbar at any time. LaraDeck cleanly removes the `auto_prepend_file` directive from `.user.ini`, leaving your project in its original pristine state.
