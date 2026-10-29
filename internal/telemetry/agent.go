package telemetry

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Embedded PHP agent source
var AgentPHPContent = `<?php
/**
 * LaraDeck Zero-Package PHP Debugger Agent
 * Automatically loaded via auto_prepend_file.
 * Does not modify any project files or require Composer packages.
 */

if (PHP_SAPI === 'cli' && !isset($_SERVER['LARADECK_AGENT_CLI'])) {
    return;
}

(function () {
    $port = 9912;
    $dumpsInHtml = false;
    $agentStartTime = microtime(true);
    $agentStartMem = memory_get_usage();

    $recordedQueries = [];
    $recordedLogs = [];
    $recordedDumps = [];
    $requestUser = null;

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

            if ($dumpsInHtml) {
                $cloner = new \Symfony\Component\VarDumper\Cloner\VarCloner();
                $dumper = 'cli' === PHP_SAPI ? new \Symfony\Component\VarDumper\Dumper\CliDumper() : new \Symfony\Component\VarDumper\Dumper\HtmlDumper();
                $dumper->dump($cloner->cloneVar($var));
            }
        });
    };

    $hookedLaravel = false;
    $hookLaravel = function () use (&$hookedLaravel, &$recordedQueries, &$recordedLogs, &$requestUser, &$registerVarDumper) {
        if ($hookedLaravel) return;
        if (!class_exists(\Illuminate\Container\Container::class, false)) return;

        $app = \Illuminate\Container\Container::getInstance();
        if (!$app || !method_exists($app, 'bound') || !$app->bound('events')) return;

        $hookedLaravel = true;
        $registerVarDumper();

        $events = $app->make('events');
        if (!$events) return;

        $events->listen(\Illuminate\Database\Events\QueryExecuted::class, function ($query) use (&$recordedQueries) {
            $sql = $query->sql;
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

        $events->listen(\Illuminate\Log\Events\MessageLogged::class, function ($log) use (&$recordedLogs) {
            $recordedLogs[] = [
                'level' => $log->level ?? 'info',
                'message' => $log->message ?? '',
                'context' => is_array($log->context ?? null) ? json_encode($log->context) : '',
                'time' => gmdate('Y-m-d\TH:i:s\Z')
            ];
        });

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

    spl_autoload_register(function ($class) use ($hookLaravel, $registerVarDumper) {
        if ($class === 'Symfony\\Component\\VarDumper\\VarDumper') {
            $registerVarDumper();
        } elseif ($class === 'Illuminate\\Container\\Container' || $class === 'Illuminate\\Events\\Dispatcher') {
            $hookLaravel();
        }
    }, true, true);

    register_shutdown_function(function () use (
        &$recordedQueries,
        &$recordedLogs,
        &$recordedDumps,
        &$requestUser,
        $port,
        $agentStartTime,
        $agentStartMem
    ) {
        $durationMs = round((microtime(true) - $agentStartTime) * 1000, 2);
        $memPeakBytes = memory_get_peak_usage() - $agentStartMem;

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
`

// EnsureAgentScript writes laradeck-agent.php into the user's .laradeck/agent directory
func EnsureAgentScript() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home: %w", err)
	}

	agentDir := filepath.Join(home, ".laradeck", "agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create agent directory: %w", err)
	}

	agentPath := filepath.Join(agentDir, "laradeck-agent.php")
	// Normalize path with forward slashes for PHP INI compatibility
	normalizedPath := filepath.ToSlash(agentPath)

	if err := os.WriteFile(agentPath, []byte(AgentPHPContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write agent script: %w", err)
	}

	return normalizedPath, nil
}

// AttachDebugger injects auto_prepend_file into the project's .user.ini
func AttachDebugger(projectPath string) error {
	agentPath, err := EnsureAgentScript()
	if err != nil {
		return err
	}

	targetDir := filepath.Join(projectPath, "public")
	if fi, err := os.Stat(targetDir); err != nil || !fi.IsDir() {
		targetDir = projectPath
	}

	userIniPath := filepath.Join(targetDir, ".user.ini")
	prependLine := fmt.Sprintf(`auto_prepend_file = "%s"`, agentPath)

	var lines []string
	if data, err := os.ReadFile(userIniPath); err == nil {
		rawLines := strings.Split(string(data), "\n")
		for _, line := range rawLines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "auto_prepend_file") {
				continue // replace existing
			}
			if trimmed != "" {
				lines = append(lines, line)
			}
		}
	}

	lines = append(lines, prependLine)
	return os.WriteFile(userIniPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// DetachDebugger removes auto_prepend_file from the project's .user.ini
func DetachDebugger(projectPath string) error {
	targetDir := filepath.Join(projectPath, "public")
	userIniPath := filepath.Join(targetDir, ".user.ini")

	data, err := os.ReadFile(userIniPath)
	if err != nil {
		// Also check project root
		userIniPath = filepath.Join(projectPath, ".user.ini")
		data, err = os.ReadFile(userIniPath)
		if err != nil {
			return nil // already clean
		}
	}

	var remaining []string
	rawLines := strings.Split(string(data), "\n")
	for _, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "auto_prepend_file") && strings.Contains(line, "laradeck-agent.php") {
			continue
		}
		if trimmed != "" {
			remaining = append(remaining, line)
		}
	}

	if len(remaining) == 0 {
		_ = os.Remove(userIniPath)
		return nil
	}

	return os.WriteFile(userIniPath, []byte(strings.Join(remaining, "\n")+"\n"), 0644)
}

// IsDebuggerAttached checks if the project has auto_prepend_file configured for laradeck-agent.php
func IsDebuggerAttached(projectPath string) bool {
	checkPaths := []string{
		filepath.Join(projectPath, "public", ".user.ini"),
		filepath.Join(projectPath, ".user.ini"),
	}

	for _, p := range checkPaths {
		if data, err := os.ReadFile(p); err == nil {
			if strings.Contains(string(data), "laradeck-agent.php") {
				return true
			}
		}
	}
	return false
}
