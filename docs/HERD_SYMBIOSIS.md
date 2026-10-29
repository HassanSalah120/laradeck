# LaraDeck & Laravel Herd Symbiosis

LaraDeck is engineered to work in seamless symbiosis with **Laravel Herd for Windows**.

---

## 1. Division of Responsibilities

Rather than duplicating or fighting Herd's core strengths, LaraDeck splits responsibilities intelligently:

| Feature Area | Managed by Laravel Herd | Managed by LaraDeck |
| :--- | :---: | :---: |
| **Site Serving & Reverse Proxy** | ✅ Built-in Caddy Server | 🔄 Proxies to local runtimes if needed |
| **Domain Resolution (`.test`)** | ✅ Loopback DNS & Hosts | 👁 Reads discovered sites & URLs |
| **Local SSL / HTTPS Certificates** | ✅ Herd Root CA Manager | 👁 Inspects certificate status |
| **Herd PHP Version Switching** | ✅ `herd isolate <ver>` | ✅ 1-Click GUI isolation trigger |
| **Standalone Windows PHP Runtimes** | ❌ Bundled only | ✅ Downloads official 8.1–8.4 from windows.php.net |
| **Composer Management & PATH** | ❌ Bundled only | ✅ User PATH shims & updates |
| **Zero-Package Debugger Agent** | ❌ Not available | ✅ `.user.ini` auto-prepend agent |
| **Telemetry & Microsecond Flamegraphs** | ❌ Not available | ✅ Native microsecond activity tracer |
| **Events & Queues Pro Workbench** | ❌ Not available | ✅ Constructor reflection & "Run Now" |
| **Webhook Pre-flight Diagnostics** | ❌ Not available | ✅ CSRF & Auth trap checks, HMAC signer |
| **Performance Radar & 1-Click Fixes** | ❌ Not available | ✅ 0–100 scored audit & automated fixes |
| **Authenticated Load Testing** | ❌ Not available | ✅ Native concurrent engine with user sessions |
| **Database Studio & ERD Canvas** | ❌ Not available | ✅ Model schema & relationship visualizer |
| **Tinker REPL & Query Inspector** | ❌ Not available | ✅ Real-time PHP sandbox |

---

## 2. Dual PHP Runtime Strategy

LaraDeck provides developers with two flexible options for PHP runtimes:

1. **Herd's Bundled Runtimes**: LaraDeck detects existing PHP versions installed by Laravel Herd under `%APPDATA%\Herd\bin`. You can switch your site's PHP version directly from LaraDeck using the `Use for Active Project` button, which executes `herd isolate <version>` behind the scenes.
2. **Standalone Official Windows Runtimes**: For projects that require specific extension modules or custom configurations without altering Herd's global environment, LaraDeck can download, configure, and isolate official Windows PHP runtimes (x64 NTS) into `%USERPROFILE%/.laradeck/runtimes/php`.

---

## 3. Site Discovery Workflow

1. LaraDeck checks Herd's configuration paths:
   - `%USERPROFILE%/.config/herd/config/valet/config.json`
   - `%USERPROFILE%/Herd`
2. All linked directories and parked sites are instantly populated in LaraDeck's project sidebar.
3. Trust policies protect sensitive actions: LaraDeck requires explicitly granting "Trust" to a project folder before executing arbitrary artisan or tinker commands.
