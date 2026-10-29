# Stack Context

Generated: 2026-10-16

- Go 1.27.2 on Windows; Wails v2.15.0, one standard window.
- React 19 + TypeScript + Vite; generated Wails bindings.
- No telemetry database, daemon, tray, updater, or custom PHP package.
- Go packages under internal; console helper under cmd/runner.
- Test: go test ./...; Windows runner integration tests require a built helper.
- Frontend check/build: npm run build (TypeScript + Vite).
- Formatting: gofmt; focused Go integration tests exercise real processes and HTTP.
- Source: outputs/herd-workbench. Disposable fixtures and browser evidence: work/.
- Project commands require explicit session trust; discovery never returns .env values.
- Only the app's own child processes are controlled. Forced stops are explicit.
- UI uses real backend results; empty/unavailable states never contain fake traces.
- CI not configured yet; build.ps1 runs the local checks and builds release binaries.
