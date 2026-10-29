# Laravel companion implementation

One integrated addition to the existing Windows Go/Wails v2 app. Preserve current UI, project trust, owned-process lifecycle, audit behavior and unrelated working-tree changes.

| Capability | Working outcome and boundary |
| --- | --- |
| Workspace health | Inspect every discovered project without booting untrusted PHP. Show dependency/PHP/HTTPS/Clockwork readiness, owned processes and audit coverage; unknown stays unknown. |
| Startup profiles | Persist project selections for Vite, queue worker, scheduler and shared local services. Start/stop owned components, report conflicts and partial starts without killing existing services. |
| Guided repairs | Static diagnoses plus trusted runtime checks. Explicit previews and confirmations for dependency install, config clear, asset build and Clockwork setup. No automatic migrations or credential generation. |
| Performance comparison | Pin a captured request, compare same method/URI; show timing, query and memory availability, normalized query differences and uncontrolled-comparison caveat. |
| Migrations | Runtime-selected connection identity, pending/applied migrations, SQL preview, schema snapshots/diff and confirmed migrate/rollback. Never infer pending migrations from filenames alone. |
| Audit history | Retain completed scans, compare rule/source fingerprints; line movement preserves identity, relevant source changes invalidate it. Preserve existing false-positive citations. |
| Timeline | Actual Clockwork request queries/exceptions plus optional Telescope batch correlation. No invented causal links. |
| Cache/session inspection | Bounded inspection of supported file/database/Redis stores, masked values with explicit reveal, individual deletion only. Explicit unsupported-store responses. |
| Route tests | Select route/method, inspect middleware/parameters, send local requests, generate editable Pest/PHPUnit test drafts and save only to a new test file. |
| Database snapshots | Local-only SQLite/MySQL/PostgreSQL snapshots using database-native tools. Scope snapshots to project and connection identity; confirm restore, reject target mismatch, keep a recovery snapshot. |
| Telescope/Pulse | Detect installation and provide reviewed package/setup operations and dashboard links. Telescope timeline reads bounded batches. Never silently install providers or migrate databases. |

Architecture: a companion capability package owns the bounded Laravel PHP bridge and persisted companion artifacts. Workbench remains the coordinator and trust/lifecycle owner. Existing process runner and API client remain authoritative. UI exposes functional tools, loading/errors and confirmations through one workspace entry rather than multiplying top-level navigation.

Verification: owner tests for fingerprints, comparison, path/target confinement and action allowlists; real disposable Laravel/SQLite bridge checks for migrations, cache, sessions and snapshot/restore; owned-process and workspace integration tests; frontend build and browser acceptance; production Wails build and packaged smoke. Do not run mutating checks against user projects. Document native-engine/tool availability limits honestly.
