<?php

if ($argc < 3) {
    fwrite(STDERR, "Usage: php tinker_runner.php <project_path> <code_file>\n");
    exit(1);
}

$projectPath = rtrim(str_replace('\\', '/', $argv[1]), '/');
$codeFile = $argv[2];

if (!file_exists($codeFile)) {
    echo json_encode([
        'output' => '',
        'durationMs' => 0,
        'queries' => [],
        'error' => 'Code file not found: ' . $codeFile
    ]);
    exit(0);
}

$code = file_get_contents($codeFile);

$autoload = $projectPath . '/vendor/autoload.php';
$bootstrap = $projectPath . '/bootstrap/app.php';

if (!file_exists($autoload) || !file_exists($bootstrap)) {
    echo json_encode([
        'output' => '',
        'durationMs' => 0,
        'queries' => [],
        'error' => 'Not a valid Laravel project (missing vendor/autoload.php or bootstrap/app.php)'
    ]);
    exit(0);
}

// 1. Boot Laravel
require_once $autoload;

$queries = [];
$error = null;
$output = '';
$start = microtime(true);

try {
    $app = require_once $bootstrap;
    if (is_object($app) && method_exists($app, 'make')) {
        if ($app->bound(\Illuminate\Contracts\Console\Kernel::class)) {
            $kernel = $app->make(\Illuminate\Contracts\Console\Kernel::class);
            $kernel->bootstrap();
        }
    }

    // 2. Attach DB listener
    if (class_exists(\Illuminate\Support\Facades\DB::class)) {
        try {
            \Illuminate\Support\Facades\DB::listen(function ($query) use (&$queries) {
                $sql = $query->sql;
                $bindings = $query->bindings ?? [];
                $time = $query->time; // ms
                $connection = $query->connectionName ?? 'default';

                $formattedBindings = [];
                if (is_array($bindings)) {
                    foreach ($bindings as $b) {
                        if ($b instanceof \DateTimeInterface) {
                            $formattedBindings[] = $b->format('Y-m-d H:i:s');
                        } elseif (is_object($b)) {
                            $formattedBindings[] = get_class($b);
                        } else {
                            $formattedBindings[] = $b;
                        }
                    }
                }

                $queries[] = [
                    'sql' => $sql,
                    'timeMs' => round((float)$time, 2),
                    'bindings' => $formattedBindings,
                    'connection' => (string)$connection,
                ];
            });
        } catch (\Throwable $t) {
            // DB might not be configured or booted
        }
    }

    // 3. Evaluate snippet
    ob_start();

    $trimmedCode = trim($code);
    // Remove starting <?php if present
    if (str_starts_with($trimmedCode, '<?php')) {
        $trimmedCode = trim(substr($trimmedCode, 5));
    }

    // Split top-level statements using PHP's lexer so semicolons inside
    // strings, closures and for-loops remain untouched. Return the final
    // expression when it parses as one, without executing the snippet twice.
    $statements = [];
    $buffer = '';
    $depth = 0;
    $hasCode = false;
    foreach (PhpToken::tokenize('<?php ' . $trimmedCode) as $token) {
        if ($token->id === T_OPEN_TAG) continue;
        if ($token->text === ';' && $depth === 0) {
            if ($hasCode) $statements[] = $buffer;
            $buffer = '';
            $hasCode = false;
            continue;
        }
        $buffer .= $token->text;
        if (!$token->isIgnorable()) $hasCode = true;
        if (in_array($token->text, ['(', '[', '{'], true)) $depth++;
        if (in_array($token->text, [')', ']', '}'], true)) $depth--;
    }
    if ($hasCode) $statements[] = $buffer;
    $evalCode = $trimmedCode;
    if ($statements) {
        $expression = array_pop($statements);
        try {
            token_get_all('<?php return ' . $expression . "\n;", TOKEN_PARSE);
            $prefix = $statements ? implode(';', $statements) . ';' : '';
            $evalCode = $prefix . 'return ' . $expression . "\n;";
        } catch (\ParseError $parseError) {
            // Echo, declarations, explicit return and control-flow stay intact.
        }
    }
    $retVal = eval($evalCode);

    $buffered = ob_get_clean();

    // Format return value
    $dumpStr = '';
    if ($retVal !== null) {
        if (class_exists(\Symfony\Component\VarDumper\Dumper\CliDumper::class) && class_exists(\Symfony\Component\VarDumper\Cloner\VarCloner::class)) {
            $cloner = new \Symfony\Component\VarDumper\Cloner\VarCloner();
            $dumper = new \Symfony\Component\VarDumper\Dumper\CliDumper();
            $dumper->setColors(false);
            $dumpStr = $dumper->dump($cloner->cloneVar($retVal), true);
        } elseif (is_scalar($retVal) || is_null($retVal)) {
            $dumpStr = var_export($retVal, true);
        } else {
            $dumpStr = print_r($retVal, true);
        }
    }

    if ($buffered !== '' && $dumpStr !== '') {
        $output = rtrim($buffered) . "\n\n=> " . trim($dumpStr);
    } elseif ($dumpStr !== '') {
        $output = "=> " . trim($dumpStr);
    } else {
        $output = $buffered !== '' ? $buffered : '=> null';
    }

} catch (\Throwable $e) {
    if (ob_get_level() > 0) {
        $buffered = ob_get_clean();
    }
    $error = get_class($e) . ': ' . $e->getMessage() . "\n" . $e->getFile() . ':' . $e->getLine();
    if (!empty($buffered)) {
        $output = $buffered;
    }
}

$duration = round((microtime(true) - $start) * 1000, 2);

echo json_encode([
    'output' => $output,
    'durationMs' => $duration,
    'queries' => $queries,
    'error' => $error,
], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
