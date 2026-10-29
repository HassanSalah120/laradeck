# LaraDeck compared with LaraMonster

Comparison date: 20 October 2026. LaraDeck means the current working tree in `outputs/herd-workbench`, including uncommitted work. LaraMonster evidence is its official documentation, not a hands-on installation. Code presence, passing checks, and a finished user workflow are different levels of evidence.

## Assessment

LaraMonster is a standalone local PHP environment; LaraDeck is currently a Windows companion to Herd. LaraMonster documents broader environment ownership and deeper automatic instrumentation. LaraDeck already overlaps substantially on development workflows and has additional inspection, audit, and database tooling that is not documented in LaraMonster. Counting tabs would overstate parity: several newer LaraDeck features are partial or simulated.

The strongest current direction for LaraDeck is a trustworthy Laravel workbench on top of Herd. Becoming a standalone LaraMonster alternative would require a separate environment-management effort, not just additional panels.

## Capability comparison

Here, **implemented** means a UI/backend path was found in source; it does not claim fresh end-to-end verification. **Partial** means a material part of the documented use case is missing. LaraMonster's tier and capabilities below are vendor claims; direct citations are in the research inventory later in this file.

| Capability | LaraMonster documentation | LaraDeck current source | Assessment |
|---|---|---|---|
| Platforms and delivery | macOS, Windows, Linux; CLI and browser dashboard | Windows/Wails; diagnostic CLI and MCP, no browser dashboard | LaraMonster has wider documented host coverage |
| Site serving, domains, HTTPS | Bundled Caddy; parking/linking, DNS and trusted HTTPS | Discovers Herd sites and delegates HTTPS operations to Herd | No standalone environment parity |
| PHP versions and Composer | Installs/manages versions, per-site PHP, shims and settings | Resolves Herd PHP, inspects settings/extensions, edits selected INI directives; Composer tooling | Partial; no independent PHP runtime manager |
| Development services | Managed MySQL/MariaDB, PostgreSQL, Redis/Valkey, Mailpit, Meilisearch | Native service profiles, binary detection/install flows, Mailpit and Meilisearch; system installers for some dependencies | Overlap; installer launch is not fully provisioned service setup |
| Project creation | Multiple starters, DB creation/migrations, optional assets/Git/packages | Composer-based wizard and recipes | Partial; narrower setup workflow |
| Requests and SQL | Automatic per-site debugger agent | Clockwork reader, file watching, bounded request history | Implemented through an external project dependency |
| Dumps and step debugging | Automatic dumps plus Xdebug step debugging | Dump storage API/view and PHP setting controls; automatic dump collector and step-debugger workflow not found | Material gap |
| Logs and exceptions | Agent-backed logs/exceptions and monitor | Log reading, redacted history, grouped exception incidents | Meaningful overlap; different capture mechanism |
| Request/command/job activity | Pro nested activity traces, events/listeners and PDF export | Clockwork queries and optional Telescope batch associations; simplified waterfall | Partial; not equivalent cross-context tracing |
| N+1, slow SQL and load | Pro query analysis, Explain, performance checks and load tests | Query fingerprint heuristics, EXPLAIN UI/backend, API assertions/boundary cases, native/k6 load paths | Substantial overlap; accuracy and live compatibility still need acceptance tests |
| Events, queues and schedules | Pro event dispatch, job inspection/execution/retry and workers | Event/listener listing, failed-job actions, owned queue workers, scheduler and saved commands | Partial; generic event/job dispatch UI not found |
| Webhooks | Pro recording, signatures, route review, replay and test events | Signed dispatch backend; no corresponding dispatch UI call or inbound recording/replay workflow found | Partial backend, not webhook-workbench parity |
| Mail | Free integrated inbox | Mailpit capture/inbox via service adapter | Implemented, dependent on Mailpit |
| Public tunnels | Free Cloudflare quick tunnels with password/expiry options | Cloudflared, ngrok and Expose process adapters | Provider breadth; equivalent password/expiry controls not established |
| Model editing, ERD, schema inspection | No ERD/model editor documented | Model records CRUD, ERD/advisories and EXPLAIN screens | Additional LaraDeck tooling; ERD introspection has discovery/fallback limits |
| Migrations and snapshots | Wizard migrations; ordinary CLI workflows for existing apps | Migration list/preview/apply/rollback, schema baseline/diff, DB snapshot/restore | Additional structured LaraDeck workflows |
| Cache and sessions | Redis/Valkey service setup, config cache and performance actions | Bounded inspection/reveal/delete for supported file/database/PhpRedis stores | Additional LaraDeck tooling; not all drivers supported |
| Quality and audit | Pro performance/project checkup | PHP/TS scanning, PHPStan/Larastan, Pint, ESLint, Gitleaks/Semgrep and browser/axe adapters, findings/history | Broader visible audit scope; external analyzers require setup |
| HTTP client and test drafting | General API client not documented | Request/assertion/boundary/load tools and route-based Pest/PHPUnit draft generation | Additional LaraDeck tooling |
| Environment drift and integration | .env editing, packages and CLI | .env editing/drift, Telescope/Pulse setup, support bundles, reports and MCP diagnostic tools | LaraDeck has additional developer-tool integrations |
| Remote profiles | SSH project/database profiles not documented | Profile persistence, SSH version probe and command safety assessment | Partial scaffolding; not a full remote administration workflow |
| Reverb and feature flags | Dedicated tools not documented | Reverb configuration probe and simulated frames; Pennant source-name/usage scanner | Not live WebSocket inspection or runtime feature-flag management |

Local source evidence: [application bindings](app.go), [main UI](frontend/src/Workbench.tsx), [companion workflows](internal/workbench/companion.go), [PHP bridge](internal/companion/bridge.php), [Herd dependency](internal/workbench/project.go), [service manager](internal/services/services.go), [dependency catalog](internal/dependencies/catalog.go), [database bridge](internal/database/scripts/model_explorer.php), [audits](internal/quality/audit.go), [API/load tests](internal/testing/testing.go), [MCP](internal/mcp/mcp.go), [tunnel/webhook backend](internal/sharing/sharing.go).

## Features that currently look more complete than they are

1. **Reverb:** `SimulateEvent` only appends an in-memory frame. Its success message says the event was broadcast, but it sends nothing to Reverb. `RecordFrame` has no production capture caller other than that simulation. Subscriber counts are initialized locally, not measured. [Source](internal/realtime/reverb.go)
2. **Waterfall:** the backend passes no timeline events and the builder places SQL queries consecutively from time zero. These are reconstructed query bars, not measured request chronology or a nested flamegraph. [Backend](internal/workbench/backend.go) · [builder](internal/telemetry/timeline.go)
3. **Health:** storage writeability and a PHP/database probe exist, but the queue component is always returned as healthy without probing a queue. Its combined score must not be treated as complete operational health. [Source](internal/health/heartbeat.go)
4. **Architecture map:** components are grouped from directory conventions and a few regex rules. `Calls` stays empty; exported Mermaid connects selected first nodes across layers without discovering those actual dependencies. This is a component inventory with illustrative edges, not a verified call graph. [Source](internal/architecture/canvas.go)
5. **Remote and Pennant:** remote UI exposes connection testing and safety assessment, not connected remote versions of the workbench tools. Pennant scans literal source definitions/usages rather than evaluating scoped runtime state or changing it. [Remote manager](internal/remote/profile_manager.go) · [remote UI](frontend/src/RemoteProfilesPanel.tsx) · [flags scanner](internal/flags/pennant.go)

## Suggested priorities

1. Correct the above success messages and unsupported inferences; use unavailable/unknown where no measurement exists.
2. Complete a real instrumentation path for requests, commands, jobs, dumps and live events, then test representative workflows through the packaged desktop app. Build on Clockwork/Telescope deliberately or choose an owned agent; avoid presenting reconstructed data as captured data.
3. Complete the already-started event/job and webhook workflows before adding more panels. Preserve LaraDeck's stronger audit/database workflows and document/test their supported boundaries.
4. Decide whether Herd remains a required dependency. Only if standalone environment replacement is the goal should Caddy/DNS/HTTPS/PHP installation and cross-platform distribution become part of the comparison target.

These are recommendations, not implementation changes made by this comparison.

## Validation during this comparison

- `npm run build`: passed TypeScript and Vite production build. Vite warned that the main JS chunk exceeds 500 kB (about 558 kB minified / 150 kB gzip).
- `go test ./internal/telemetry ./internal/realtime ./internal/remote ./internal/flags ./internal/health ./internal/architecture ./internal/database ./internal/runner ./internal/services`: passed (some results cached).
- No live services, SSH connections, public tunnels, production resources, or LaraMonster installation were exercised. No application source was changed. These checks do not establish functional parity.

## LaraMonster primary-source research

Research snapshot: 20 October 2026. This note summarizes LaraMonster's official documentation and product page. These are vendor-documented capabilities, not independently tested behavior. The docs site's sitemap listed 23 documentation pages; Defuddle was used to extract them. The browser reader could not open the docs host directly.

## Product scope and supported platforms

LaraMonster is presented as a local PHP/Laravel development environment with a desktop app, `laramonster` CLI, and optional browser dashboard. It serves parked or linked projects, installs/manages PHP versions and common development services, and provides Laravel debugging tools. The documented platforms are macOS 11+ (Apple Silicon and Intel), Windows 10/11 x64, and Linux x86-64; the Linux desktop needs GTK 3 and WebKitGTK 4.1, while its CLI can run without a desktop. The docs say PHP, Composer, Node.js, and a database need not be preinstalled. [Introduction](https://monstersgate.com/laramonster/docs) · [Installation](https://monstersgate.com/laramonster/docs/installation)

Sites use `.localhost` by default and can use `.test` with LaraMonster's DNS setup. The bundled Caddy server serves PHP projects, while the site panel can proxy to an already-running Node/Vite app, `php artisan serve`, `php -S`, Docker container, or Apache virtual host, including WebSockets for hot reload. The browser dashboard is documented for a server, WSL, or a phone on the network. [Sites and domains](https://monstersgate.com/laramonster/docs/sites) · [DNS](https://monstersgate.com/laramonster/docs/dns)

## Documented feature set

| Area | Officially documented capabilities | Availability |
|---|---|---|
| PHP and Laravel sites | Multiple PHP versions, default and per-site PHP selection, per-site `php.ini` settings/extensions, Composer and shell shims, `.env` editor, Composer package installation, local HTTPS certificates, project parking/linking | Free ([PHP](https://monstersgate.com/laramonster/docs/php), [PHP settings](https://monstersgate.com/laramonster/docs/php-settings), [HTTPS](https://monstersgate.com/laramonster/docs/https), [sites](https://monstersgate.com/laramonster/docs/sites)) |
| Local services | MySQL/MariaDB, PostgreSQL, Redis/Valkey, Mailpit, Meilisearch; service controls and ready-made `.env` values. Laravel Scout is named for Meilisearch; Redis is described for cache, queues, sessions, and Horizon. | Free ([services](https://monstersgate.com/laramonster/docs/services)) |
| Mail | Local SMTP capture and an in-app inbox | Free ([mail](https://monstersgate.com/laramonster/docs/mail)) |
| New project setup | Laravel, React, Vue, Livewire, and blank PHP starters; SQLite/MySQL/PostgreSQL; curated and Packagist packages including Filament, Pest, Boost, Laravel AI, MCP, and Prism; writes `.env`, creates DB, runs migrations, optionally builds assets and makes a first Git commit | Free ([first project](https://monstersgate.com/laramonster/docs/first-project)) |
| Debugging | Per-site PHP agent records Laravel request dumps, queries, logs and exceptions; failing queued jobs/commands; Xdebug step debugging; no package installation or project-file modification is claimed | Free ([debugger](https://monstersgate.com/laramonster/docs/debugger)) |
| Public links | Cloudflare quick tunnels with optional password and expiry; intended for demos, mobile testing, and receiving webhooks | Free ([public links](https://monstersgate.com/laramonster/docs/public-links)) |
| Activity | Traces requests, Artisan commands and jobs with nested events, listeners, payloads, query timing and errors; PDF export | Pro ([activity](https://monstersgate.com/laramonster/docs/activity)) |
| Events and jobs | Inspect event/listener relationships, dispatch events using forms or PHP, inspect/run/queue jobs, retry failed jobs, and supervise workers; PDF guides | Pro ([events](https://monstersgate.com/laramonster/docs/events), [jobs](https://monstersgate.com/laramonster/docs/jobs)) |
| Webhooks | Find likely webhook routes, flag CSRF/auth/secret issues, record deliveries, inspect signatures/responses, replay deliveries, and send signed test events | Pro ([webhooks](https://monstersgate.com/laramonster/docs/webhooks)) |
| Performance | Project checkup with cache/autoloader/OPcache/Xdebug checks and one-click local fixes; N+1 and slow-query analysis, Explain, plus short load tests with guest or signed-in users | Pro ([performance](https://monstersgate.com/laramonster/docs/performance)) |
| Monitoring | Live LaraMonster/system CPU and memory, processes, resource recommendations, busiest sites, and service logs | Free ([monitor](https://monstersgate.com/laramonster/docs/monitor)) |

## Specific comparison questions

- **Remote hosts:** The docs describe a local development environment. They document a browser dashboard for server/WSL/network access and a reverse proxy to a server the user runs, but no SSH remote project or remote database profile workflow appears in the documentation reviewed. Treat those features as *not documented*, rather than asserting they are unsupported. [Sites](https://monstersgate.com/laramonster/docs/sites) · [CLI](https://monstersgate.com/laramonster/docs/cli)
- **AI:** AI-related packages can be selected/installed in the project wizard. The docs do not describe a built-in AI coding, query, or diagnostics assistant. [First project](https://monstersgate.com/laramonster/docs/first-project)
- **API client:** No general HTTP/API request client is documented in the feature index or reviewed docs pages. Webhook test-event sending is a narrower Pro feature. [Docs index](https://monstersgate.com/laramonster/docs) · [Webhooks](https://monstersgate.com/laramonster/docs/webhooks)
- **ERD/schema visualization:** No database ERD or schema diagram is documented. The services page instead points users to external clients such as TablePlus, DBeaver, phpMyAdmin, or command-line tools. [Services](https://monstersgate.com/laramonster/docs/services)
- **Migrations and cache:** The new-project wizard runs `php artisan migrate`; existing projects use normal CLI migration commands. Cache-related features are Redis/Valkey service setup, a `.env` warning and clear-config-cache action, and Pro performance checks/fixes. The reviewed docs do not describe a general migration browser/editor or visual cache manager. [First project](https://monstersgate.com/laramonster/docs/first-project) · [Sites](https://monstersgate.com/laramonster/docs/sites) · [Performance](https://monstersgate.com/laramonster/docs/performance)

## Pricing and caveats

The free tier includes the local environment, services, debugger, monitor, and public links. The product page currently lists LaraMonster Pro for **1,000 LE / 6 months** or **1,900 LE / 1 year**. A license is for one computer at a time; Pro features pause when a time-limited license ends, while free features continue. Pro works offline for three days after its last successful license check. [Product and pricing](https://monstersgate.com/laramonster#licenses) · [Pro licensing](https://monstersgate.com/laramonster/docs/pro)

Important documented limits: LaraMonster's database/service examples bind to `127.0.0.1` and use default users with no password; the docs explicitly say these local credentials should not be copied to a server. Public-link URLs change when restarted, depend on Cloudflare quick tunnels, and expose the local site and its data; the docs recommend passwords and short durations and warn against sharing apps with `APP_DEBUG=true` and real secrets. The docs position quick tunnels for development/demos, not hosting. [Services](https://monstersgate.com/laramonster/docs/services) · [Public links](https://monstersgate.com/laramonster/docs/public-links)

The installation guide says the current app is not Apple-signed and Windows may show a first-launch warning. Linux desktop prerequisites are GTK/WebKitGTK; supported Linux download is x86-64. [Installation](https://monstersgate.com/laramonster/docs/installation)
