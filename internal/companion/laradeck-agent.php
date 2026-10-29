<?php
/**
 * LaraDeck Zero-Package PHP Debugger Agent
 * Automatically loaded via auto_prepend_file.
 * Does not modify any project files or require Composer packages.
 */

if (PHP_SAPI === 'cli' && !isset($_SERVER['LARADECK_AGENT_CLI'])) {
    // Only instrument web requests by default, or CLI if explicitly flagged
    return;
}

(function () {
    $port = 9912;
    $dumpsInHtml = false;
    $agentStartTime = microtime(true);
    $agentStartMem = memory_get_usage();

    $recordedQueries = [];
    $recordedLogs = [];
    $recordedEvents = [];
    $recordedDumps = [];
    $requestUser = null;

    // 1. Intercept Symfony / Laravel dump() and dd()
    $registerVarDumper = function () use (&$recordedDumps, $port, $dumpsInHtml) {
        if (!class_exists(\Symfony\Component\VarDumper\VarDumper::class, false)) {
            return;
        }

        \Symfony\Component\VarDumper\VarDumper::setHandler(function ($var) use (&$recordedDumps, $port, $dumpsInHtml) {
            $trace = debug_backtrace(DEBUG_BACKTRACE_IGNORE_ARGS, 8);
            $caller = ['file' => 'unknown', 'line' => 0];
            foreach ($trace as $frame) {
                $file = $frame['file'] ?? '';
                if ($file && !str_contains($file, 'VarDumper') && !str_contains($file, 'laradeck-agent.php')) {
                    $caller = $frame;
                    break;
                }
            }

            // Stringify / serialize payload safely
            $payload = '';
            if (is_scalar($var) || is_null($var)) {
                $payload = var_export($var, true);
            } else {
                $payload = json_encode($var, JSON_PRETTY_PRINT | JSON_PARTIAL_OUTPUT_ON_ERROR | JSON_INVALID_UTF8_SUBSTITUTE);
                if (!$payload) {
                    $payload = print_r($var, true);
                }
            }

            $dumpItem = [
                'id' => 'dump_' . hrtime(true),
                'file' => $caller['file'] ?? 'unknown',
                'line' => (int)($caller['line'] ?? 0),
                'payload' => $payload,
                'createdAt' => gmdate('Y-m-d\TH:i:s\Z')
            ];

            $recordedDumps[] = $dumpItem;

            // Stream immediately to LaraDeck dump listener
            $ctx = stream_context_create([
                'http' => [
                    'method' => 'POST',
                    'header' => "Content-Type: application/json\r\n",
                    'content' => json_encode($dumpItem),
                    'timeout' => 0.15,
                    'ignore_errors' => true
                ]
            ]);
            @file_get_contents("http://127.0.0.1:{$port}/dumps", false, $ctx);

            // If HTML dumps are enabled, render using standard dumper; otherwise suppress output to keep JSON/redirects clean
            if ($dumpsInHtml) {
                $cloner = new \Symfony\Component\VarDumper\Cloner\VarCloner();
                $dumper = 'cli' === PHP_SAPI ? new \Symfony\Component\VarDumper\Dumper\CliDumper() : new \Symfony\Component\VarDumper\Dumper\HtmlDumper();
                $dumper->dump($cloner->cloneVar($var));
            }
        });
    };

    // 2. Hook into Laravel event dispatcher on application boot
    $hookedLaravel = false;
    $hookLaravel = function () use (&$hookedLaravel, &$recordedQueries, &$recordedLogs, &$recordedEvents, &$requestUser, &$registerVarDumper) {
        if ($hookedLaravel) return;
        if (!class_exists(\Illuminate\Container\Container::class, false)) return;

        $app = \Illuminate\Container\Container::getInstance();
        if (!$app || !method_exists($app, 'bound') || !$app->bound('events')) return;

        $hookedLaravel = true;
        $registerVarDumper();

        $events = $app->make('events');
        if (!$events) return;

        // Query listener
        $events->listen(\Illuminate\Database\Events\QueryExecuted::class, function ($query) use (&$recordedQueries) {
            $sql = $query->sql;
            // Interpolate bindings safely for plain reading
            if (!empty($query->bindings)) {
                foreach ($query->bindings as $binding) {
                    $val = is_numeric($binding) ? $binding : "'" . addslashes((string)$binding) . "'";
                    $sql = preg_replace('/\?/', (string)$val, $sql, 1);
                }
            }

            $recordedQueries[] = [
                'sql' => $sql,
                'rawSql' => $query->sql,
                'timeMs' => (float)$query->time,
                'connection' => $query->connectionName ?? 'default',
            ];
        });

        // Log message listener
        $events->listen(\Illuminate\Log\Events\MessageLogged::class, function ($log) use (&$recordedLogs) {
            $recordedLogs[] = [
                'level' => $log->level ?? 'info',
                'message' => $log->message ?? '',
                'context' => is_array($log->context ?? null) ? json_encode($log->context) : '',
                'time' => gmdate('Y-m-d\TH:i:s\Z')
            ];
        });

        // Request handled listener (capture user and response stats)
        $events->listen(\Illuminate\Foundation\Http\Events\RequestHandled::class, function ($handled) use (&$requestUser) {
            $req = $handled->request ?? null;
            if ($req && method_exists($req, 'user') && ($user = $req->user())) {
                $requestUser = [
                    'id' => $user->id ?? $user->getKey() ?? null,
                    'email' => $user->email ?? null,
                    'name' => $user->name ?? null,
                ];
            }
        });
    };

    // Try hooking via autoloader or early ticks
    spl_autoload_register(function ($class) use ($hookLaravel, $registerVarDumper) {
        if ($class === 'Symfony\\Component\\VarDumper\\VarDumper') {
            $registerVarDumper();
        } elseif ($class === 'Illuminate\\Container\\Container' || $class === 'Illuminate\\Events\\Dispatcher') {
            $hookLaravel();
        }
    }, true, true);

    // 3. Register shutdown function to capture unhandled exceptions and flush full telemetry frame
    register_shutdown_function(function () use (
        &$recordedQueries,
        &$recordedLogs,
        &$recordedEvents,
        &$recordedDumps,
        &$requestUser,
        $port,
        $agentStartTime,
        $agentStartMem
    ) {
        $durationMs = round((microtime(true) - $agentStartTime) * 1000, 2);
        $memPeakBytes = memory_get_peak_usage() - $agentStartMem;

        // Check for fatal errors
        $lastError = error_get_last();
        $exception = null;
        if ($lastError && in_array($lastError['type'], [E_ERROR, E_PARSE, E_CORE_ERROR, E_COMPILE_ERROR, E_USER_ERROR])) {
            $exception = [
                'message' => $lastError['message'],
                'file' => $lastError['file'],
                'line' => $lastError['line'],
                'class' => 'FatalError',
                'stack' => []
            ];
        }

        $statusCode = http_response_code() ?: 200;
        $method = $_SERVER['REQUEST_METHOD'] ?? 'GET';
        $uri = $_SERVER['REQUEST_URI'] ?? '/';

        $frame = [
            'id' => 'req_' . hrtime(true),
            'method' => $method,
            'url' => $uri,
            'status' => $statusCode,
            'durationMs' => $durationMs,
            'memoryPeakBytes' => $memPeakBytes,
            'user' => $requestUser,
            'queries' => $recordedQueries,
            'logs' => $recordedLogs,
            'dumps' => $recordedDumps,
            'exception' => $exception,
            'createdAt' => gmdate('Y-m-d\TH:i:s\Z')
        ];

        // Non-blocking flush to LaraDeck telemetry endpoint
        $ctx = stream_context_create([
            'http' => [
                'method' => 'POST',
                'header' => "Content-Type: application/json\r\n",
                'content' => json_encode($frame),
                'timeout' => 0.25,
                'ignore_errors' => true
            ]
        ]);
        @file_get_contents("http://127.0.0.1:{$port}/telemetry", false, $ctx);
    });
})();
