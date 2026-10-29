<?php
declare(strict_types=1);

/**
 * Herd Workbench - Full-Stack PHP Batch Analyzer
 * Evaluates PHP files using native PhpToken::tokenize() (PHP 8.0+) in a single batch pass.
 */

ini_set('display_errors', '0');
error_reporting(E_ALL & ~E_NOTICE & ~E_DEPRECATED);

$rawInput = file_get_contents($argv[1] ?? 'php://stdin');
if (!$rawInput) {
    fwrite(STDERR, "Invalid analyzer input\n");
    exit(1);
}

$payload = json_decode($rawInput, true);
if (!is_array($payload) || !isset($payload['files']) || !is_array($payload['files'])) {
    fwrite(STDERR, "Invalid analyzer input\n");
    exit(1);
}

$files = $payload['files'];
$contextFiles = $payload['contextFiles'] ?? $files;
$context = $payload['context'] ?? [];
$hasInertiaV2 = !empty($context['hasInertiaV2']);
$usesOctane = !empty($context['usesOctaneOrWorkerman']);
$hasEcho = !empty($context['hasEchoOrReverb']);
$projectPath = rtrim($context['projectPath'] ?? '', '/\\');

$findings = [];

// Incidental words such as "TODO: add transaction" are not suppressions.
function hasExplicitRuleSuppression(string $comment, string $rule): bool {
    if (!preg_match('/@?laradeck-ignore(?:-next-line|-line)?(?:\s*:\s*([a-zA-Z0-9_, -]+))?/i', $comment, $matches)) {
        return false;
    }
    if (empty($matches[1])) return true;
    $rules = preg_split('/[\s,]+/', trim($matches[1]));
    return in_array($rule, $rules, true) || in_array('all', $rules, true);
}

function getIdentifier(PhpToken $token): string {
    $text = $token->text;
    if ($token->id === T_STRING || 
        (defined('T_NAME_QUALIFIED') && $token->id === T_NAME_QUALIFIED) || 
        (defined('T_NAME_FULLY_QUALIFIED') && $token->id === T_NAME_FULLY_QUALIFIED)) {
        $parts = explode('\\', $text);
        return end($parts);
    }
    return $text;
}

function getLineSnippet(string $content, int $lineNumber): string {
    $lines = explode("\n", $content);
    if ($lineNumber >= 1 && $lineNumber <= count($lines)) {
        return trim($lines[$lineNumber - 1]);
    }
    return '';
}

function normalizePath(string $path): string {
    return str_replace('\\', '/', $path);
}

function getBlockType(array $tokens, int $openBraceIdx): string {
    for ($p = $openBraceIdx - 1; $p >= 0; $p--) {
        if ($tokens[$p]->isIgnorable()) continue;
        if ($tokens[$p]->text === ')') {
            $pDepth = 1;
            for ($k = $p - 1; $k >= 0; $k--) {
                if ($tokens[$k]->text === ')') $pDepth++;
                elseif ($tokens[$k]->text === '(') {
                    $pDepth--;
                    if ($pDepth === 0) {
                        for ($kw = $k - 1; $kw >= 0; $kw--) {
                            if ($tokens[$kw]->isIgnorable()) continue;
                            $kwText = strtolower($tokens[$kw]->text);
                            if (in_array($kwText, ['if', 'elseif', 'while', 'for', 'foreach', 'switch', 'catch'], true)) {
                                return $kwText;
                            }
                            break;
                        }
                        break;
                    }
                }
            }
        }
        $tText = strtolower($tokens[$p]->text);
        if (in_array($tText, ['else', 'try', 'do', 'finally'], true)) {
            return $tText;
        }
        break;
    }
    return 'other';
}

function getNextKeywordAfterBrace(array $tokens, int $closeBraceIdx, int $tokenCount): string {
    for ($n = $closeBraceIdx + 1; $n < min($closeBraceIdx + 20, $tokenCount); $n++) {
        if ($tokens[$n]->isIgnorable()) continue;
        $txt = strtolower($tokens[$n]->text);
        if (in_array($txt, ['elseif', 'else'], true)) {
            return $txt;
        }
        break;
    }
    return '';
}

function parseConstructorParam(array $tokens, int $from, int $to, array &$unprotectedModelParams): void {
    $hasWithoutRelations = false;
    $typeHint = '';
    $varName = '';
    $varLine = 0;

    $builtinScalars = [
        'int', 'integer', 'string', 'bool', 'boolean', 'float', 'double',
        'array', 'callable', 'iterable', 'mixed', 'object', 'void', 'null',
        'false', 'true', 'carbon', 'carbonimmutable', 'collection', 'supportcollection',
        'lazycollection', 'request', 'formrequest', 'closure', 'datetime', 'datetimeimmutable',
        'throwable', 'exception', 'stdclass', 'dto', 'data', 'enum', 'status', 'type',
        'uploadedfile', 'file', 'message', 'view', 'htmlable', 'arrayable', 'jsonable',
        'uuid', 'ulid', 'stringable', 'channel', 'presencechannel', 'privatechannel',
        'authenticatable', 'mailable', 'notification', 'builder', 'queryexception',
        'pipeline', 'loggerinterface', 'dispatcher', 'container', 'application',
    ];

    for ($i = $from; $i <= $to; $i++) {
        if (!isset($tokens[$i])) continue;
        $tok = $tokens[$i];
        if ($tok->id === (defined('T_ATTRIBUTE') ? T_ATTRIBUTE : 0) || str_contains($tok->text, 'WithoutRelations')) {
            $hasWithoutRelations = true;
        }
        if ($tok->id === T_VARIABLE) {
            $varName = ltrim($tok->text, '$');
            $varLine = $tok->line;
        } elseif (($tok->id === T_STRING || 
                   (defined('T_NAME_QUALIFIED') && $tok->id === T_NAME_QUALIFIED) || 
                   (defined('T_NAME_FULLY_QUALIFIED') && $tok->id === T_NAME_FULLY_QUALIFIED)) && $varName === '') {
            $ident = getIdentifier($tok);
            if (!in_array($ident, ['public', 'protected', 'private', 'readonly'], true)) {
                $typeHint = $ident;
            }
        }
    }

    if ($varName !== '' && $typeHint !== '') {
        $cleanType = strtolower(ltrim($typeHint, '?'));
        if (!in_array($cleanType, $builtinScalars, true) && !$hasWithoutRelations) {
            // Check non-model suffixes
            $isNonModelSuffix = str_ends_with($cleanType, 'dto') || 
                                str_ends_with($cleanType, 'data') || 
                                str_ends_with($cleanType, 'enum') || 
                                str_ends_with($cleanType, 'formrequest') ||
                                str_ends_with($cleanType, 'service') ||
                                str_ends_with($cleanType, 'action') ||
                                str_ends_with($cleanType, 'helper') ||
                                str_ends_with($cleanType, 'event') ||
                                str_ends_with($cleanType, 'listener') ||
                                str_ends_with($cleanType, 'channel') ||
                                str_ends_with($cleanType, 'payload') ||
                                str_ends_with($cleanType, 'client') ||
                                str_ends_with($cleanType, 'config');
            if (!$isNonModelSuffix) {
                $unprotectedModelParams[] = [
                    'name' => $varName,
                    'line' => $varLine,
                    'type' => $typeHint,
                ];
            }
        }
    }
}

$transactionalMiddleware = []; // [className => citation string "file:line"]
$routesUnderTransactionalMiddleware = []; // [ControllerShortName => [methodName => citation]]
$controllersUnderTransactionalMiddleware = []; // [ControllerShortName => citation]

// Pre-pass 1: Discover Transactional Middleware in app/Http/Middleware or classes implementing handle() with DB::transaction wrapping $next
foreach ($contextFiles as $filePath) {
    $norm = normalizePath($filePath);
    if (str_contains($norm, '/Middleware/') || str_ends_with($norm, 'Middleware.php')) {
        $c = @file_get_contents($filePath);
        if ($c !== false && str_contains($c, 'class ') && (str_contains($c, 'DB::transaction') || str_contains($c, 'DB::beginTransaction')) && str_contains($c, '$next')) {
            if (preg_match('/class\s+([A-Za-z0-9_]+)/', $c, $m)) {
                $mwClass = $m[1];
                $lines = explode("\n", $c);
                $txLine = 1;
                foreach ($lines as $lIdx => $lText) {
                    if (str_contains($lText, 'DB::transaction') || str_contains($lText, 'DB::beginTransaction')) {
                        $txLine = $lIdx + 1;
                        break;
                    }
                }
                $rel = $norm;
                if ($projectPath !== '' && str_starts_with($rel, normalizePath($projectPath))) {
                    $rel = ltrim(substr($rel, strlen(normalizePath($projectPath))), '/');
                }
                $transactionalMiddleware[$mwClass] = "{$rel}:{$txLine}";
            }
        }
    }
}

// Pre-pass 2: Correlate Transactional Middleware with Routes and Route Groups
if (!empty($transactionalMiddleware)) {
    $mwNames = array_keys($transactionalMiddleware);
    $mwPattern = implode('|', array_map('preg_quote', $mwNames));

    foreach ($contextFiles as $filePath) {
        $norm = normalizePath($filePath);
        if (str_contains($norm, '/routes/') || str_ends_with($norm, 'routes.php')) {
            $c = @file_get_contents($filePath);
            if ($c === false || !preg_match("/(?:$mwPattern)/i", $c)) {
                continue;
            }

            try {
                $rTokens = PhpToken::tokenize($c);
            } catch (\Throwable $e) {
                continue;
            }

            $groupStack = [];
            $currentGroupMw = [];
            $braceDepth = 0;
            $rCount = count($rTokens);

            for ($i = 0; $i < $rCount; $i++) {
                $tok = $rTokens[$i];

                if ($tok->text === '{') {
                    $braceDepth++;
                    $groupStack[$braceDepth] = $currentGroupMw;
                    continue;
                }
                if ($tok->text === '}') {
                    unset($groupStack[$braceDepth]);
                    $braceDepth--;
                    $currentGroupMw = $groupStack[$braceDepth] ?? [];
                    continue;
                }

                if ($tok->id === T_STRING && strtolower($tok->text) === 'middleware') {
                    $mwArgs = [];
                    $depth = 0;
                    for ($j = $i + 1; $j < min($i + 30, $rCount); $j++) {
                        if ($rTokens[$j]->text === '(') { $depth++; continue; }
                        if ($rTokens[$j]->text === ')') { $depth--; if ($depth <= 0) break; }
                        if ($depth > 0 && ($rTokens[$j]->id === T_STRING || $rTokens[$j]->id === T_CONSTANT_ENCAPSED_STRING)) {
                            $argVal = trim($rTokens[$j]->text, "'\"");
                            if (isset($transactionalMiddleware[$argVal])) {
                                $mwArgs[] = $argVal;
                            }
                        }
                    }
                    if (!empty($mwArgs)) {
                        $currentGroupMw = array_merge($currentGroupMw, $mwArgs);
                    }
                }

                $allActiveMw = $currentGroupMw;
                foreach ($groupStack as $gMw) {
                    $allActiveMw = array_merge($allActiveMw, $gMw);
                }
                $allActiveMw = array_unique($allActiveMw);

                if (!empty($allActiveMw)) {
                    // Array syntax: [SomeController::class, 'method']
                    if ($tok->text === '[') {
                        $controller = '';
                        $method = '';
                        for ($j = $i + 1; $j < min($i + 15, $rCount); $j++) {
                            if ($rTokens[$j]->text === ']') break;
                            if ($controller === '' && ($rTokens[$j]->id === T_STRING || (defined('T_NAME_QUALIFIED') && $rTokens[$j]->id === T_NAME_QUALIFIED))) {
                                $text = $rTokens[$j]->text;
                                $parts = explode('\\', $text);
                                $ident = end($parts);
                                if (str_ends_with($ident, 'Controller')) {
                                    $controller = $ident;
                                }
                            }
                            if ($controller !== '' && $rTokens[$j]->id === T_CONSTANT_ENCAPSED_STRING) {
                                $method = trim($rTokens[$j]->text, "'\"");
                                break;
                            }
                        }
                        if ($controller !== '' && $method !== '') {
                            $routesUnderTransactionalMiddleware[$controller][$method] = $transactionalMiddleware[$allActiveMw[0]];
                        }
                    }

                    // String syntax: 'SomeController@method'
                    if ($tok->id === T_CONSTANT_ENCAPSED_STRING && str_contains($tok->text, '@')) {
                        $str = trim($tok->text, "'\"");
                        $parts = explode('@', $str);
                        if (count($parts) === 2) {
                            $cParts = explode('\\', $parts[0]);
                            $controller = end($cParts);
                            $method = $parts[1];
                            $routesUnderTransactionalMiddleware[$controller][$method] = $transactionalMiddleware[$allActiveMw[0]];
                        }
                    }

                    // Resource syntax: Route::apiResource('foo', SomeController::class)
                    if ($tok->id === T_STRING && in_array(strtolower($tok->text), ['resource', 'apiresource'], true)) {
                        for ($j = $i + 1; $j < min($i + 15, $rCount); $j++) {
                            if ($rTokens[$j]->text === ';') break;
                            if ($rTokens[$j]->id === T_STRING || (defined('T_NAME_QUALIFIED') && $rTokens[$j]->id === T_NAME_QUALIFIED)) {
                                $text = $rTokens[$j]->text;
                                $parts = explode('\\', $text);
                                $ident = end($parts);
                                if (str_ends_with($ident, 'Controller')) {
                                    $controllersUnderTransactionalMiddleware[$ident] = $transactionalMiddleware[$allActiveMw[0]];
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

foreach ($files as $filePath) {
    if (!file_exists($filePath) || !is_readable($filePath)) {
        fwrite(STDERR, "A selected PHP source file could not be read\n"); exit(1);
    }

    $content = file_get_contents($filePath);
    if ($content === false) { fwrite(STDERR, "PHP source read failed\n"); exit(1); }
    if (strlen($content) === 0) {
        continue;
    }

    $relPath = normalizePath($filePath);
    if ($projectPath !== '' && str_starts_with($relPath, normalizePath($projectPath))) {
        $relPath = ltrim(substr($relPath, strlen(normalizePath($projectPath))), '/');
    }

    try {
        $tokens = PhpToken::tokenize($content);
    } catch (\Throwable $e) {
        fwrite(STDERR, "PHP tokenization failed\n"); exit(1);
    }

    $tokenCount = count($tokens);
    $inClass = false;
    $classLine = 1;
    $className = '';
    $implementsShouldQueue = false;
    $implementsShouldBroadcast = false;
    $hasWithoutRelations = false;
    $hasBroadcastWith = false;
    $hasTries = false;
    $hasTimeout = false;
    $hasBackoff = false;
    $hasRetryUntil = false;
    $classMethods = [];
    $constructLine = 0;
    $unprotectedModelParams = [];
    $currentMethod = '';
    $currentMethodLine = 1;
    $methodBraceDepth = 0;
    $methodWriteCount = 0;
    $methodCurrentPathWrites = 0;
    $methodMaxPathWrites = 0;
    $methodBranchStack = [];
    $methodHasTransaction = false;
    $methodHasDelegation = false;
    $methodFirstWriteLine = 0;
    $methodHasValidation = false;
    $methodHasFileUploadStore = false;
    $methodFileUploadLine = 0;
    $methodCreatesUser = false;
    $methodDispatchesRegistered = false;
    $methodHasTransactionalComment = false;

    // Multi-write transaction analysis tracking
    $methodsWithTransaction = [];
    $methodsCalledInTransaction = [];
    $methodCalledMethods = [];
    $multiWriteCandidates = [];

    $hiddenAttributes = [];
    $exposedAttributes = [];

    $braceDepth = 0;
    $closureBraceDepths = [];
    $pendingClosure = false;
    $inArrowFn = false;
    $isRouteFile = str_contains($relPath, 'routes/');
    $isController = str_contains($relPath, 'Controllers/');
    $isModel = str_contains($relPath, 'Models/');
    $isJob = str_contains($relPath, 'Jobs/');
    $isMail = str_contains($relPath, 'Mail/');
    $isNotification = str_contains($relPath, 'Notifications/');
    $isAbstract = false;
    $isEvent = str_contains($relPath, 'Events/');
    $isMiddleware = str_contains($relPath, 'Middleware/');
    $isService = str_contains($relPath, 'Services/') || str_contains($relPath, 'Actions/');
    $isConfig = str_starts_with($relPath, 'config/');
    $isTest = str_starts_with($relPath, 'tests/');

    // Pass 1: Parse tokens and check rules
    for ($i = 0; $i < $tokenCount; $i++) {
        $token = $tokens[$i];
        $id = $token->id;
        $name = getIdentifier($token);

        if ($currentMethod !== '' && ($id === T_COMMENT || $id === T_DOC_COMMENT)) {
            if (hasExplicitRuleSuppression($token->text, 'MultiWriteTransactionRequired')) {
                $methodHasTransactionalComment = true;
            }
        }

        // Closure tracking
        if ($id === T_FUNCTION) {
            $isAnon = false;
            for ($n = $i + 1; $n < min($i + 5, $tokenCount); $n++) {
                if (!$tokens[$n]->isIgnorable()) {
                    if ($tokens[$n]->text === '(') {
                        $isAnon = true;
                    }
                    break;
                }
            }
            if ($isAnon) {
                $pendingClosure = true;
            }
        }
        if ($id === (defined('T_FN') ? T_FN : 0) || $token->text === 'fn') {
            $inArrowFn = true;
        }

        if ($token->text === '{') {
            $braceDepth++;
            if ($pendingClosure) {
                $closureBraceDepths[] = $braceDepth;
                $pendingClosure = false;
            }
            if ($currentMethod !== '') {
                $methodBraceDepth++;
                if ($methodBraceDepth >= 2) {
                    $bType = getBlockType($tokens, $i);
                    if ($bType === 'if') {
                        $methodBranchStack[] = [
                            'type' => 'if_chain',
                            'depth' => $methodBraceDepth,
                            'branches' => [],
                            'currentWrites' => 0,
                            'currentHasReturn' => false,
                            'hasElse' => false,
                        ];
                    } elseif ($bType === 'elseif' || $bType === 'else') {
                        if (!empty($methodBranchStack)) {
                            $lastIdx = count($methodBranchStack) - 1;
                            if ($methodBranchStack[$lastIdx]['type'] === 'if_chain') {
                                $methodBranchStack[$lastIdx]['depth'] = $methodBraceDepth;
                                if ($bType === 'else') {
                                    $methodBranchStack[$lastIdx]['hasElse'] = true;
                                }
                            }
                        }
                    }
                }
            }
        } elseif ($token->text === '}') {
            if (!empty($closureBraceDepths) && end($closureBraceDepths) === $braceDepth) {
                array_pop($closureBraceDepths);
            }
            $braceDepth--;
            if ($currentMethod !== '') {
                if (!empty($methodBranchStack)) {
                    $lastIdx = count($methodBranchStack) - 1;
                    if ($methodBranchStack[$lastIdx]['type'] === 'if_chain' && $methodBranchStack[$lastIdx]['depth'] === $methodBraceDepth) {
                        $top = &$methodBranchStack[$lastIdx];
                        $top['branches'][] = [
                            'writes' => $top['currentWrites'],
                            'hasReturn' => $top['currentHasReturn'],
                        ];
                        $top['currentWrites'] = 0;
                        $top['currentHasReturn'] = false;

                        $nextKw = getNextKeywordAfterBrace($tokens, $i, $tokenCount);
                        if ($nextKw === 'else') {
                            $top['hasElse'] = true;
                        } elseif ($nextKw !== 'elseif') {
                            $chain = array_pop($methodBranchStack);
                            $hasElse = $chain['hasElse'];
                            $branches = $chain['branches'];

                            $worstNonReturning = 0;
                            $allReturned = !empty($branches);
                            foreach ($branches as $br) {
                                if ($br['hasReturn']) {
                                    $parentCurrent = !empty($methodBranchStack) ? $methodBranchStack[count($methodBranchStack) - 1]['currentWrites'] : $methodCurrentPathWrites;
                                    $methodMaxPathWrites = max($methodMaxPathWrites, $parentCurrent + $br['writes']);
                                } else {
                                    $allReturned = false;
                                    if ($br['writes'] > $worstNonReturning) {
                                        $worstNonReturning = $br['writes'];
                                    }
                                }
                            }

                            if ($hasElse) {
                                if (!empty($methodBranchStack)) {
                                    $methodBranchStack[count($methodBranchStack) - 1]['currentWrites'] += $worstNonReturning;
                                } else {
                                    if (!$allReturned) {
                                        $methodCurrentPathWrites += $worstNonReturning;
                                        $methodMaxPathWrites = max($methodMaxPathWrites, $methodCurrentPathWrites);
                                    }
                                }
                            } else {
                                $chainContribution = max(0, $worstNonReturning);
                                if (!empty($methodBranchStack)) {
                                    $methodBranchStack[count($methodBranchStack) - 1]['currentWrites'] += $chainContribution;
                                } else {
                                    $methodCurrentPathWrites += $chainContribution;
                                    $methodMaxPathWrites = max($methodMaxPathWrites, $methodCurrentPathWrites);
                                }
                            }
                        }
                        unset($top);
                    }
                }

                $methodBraceDepth--;
                if ($methodBraceDepth <= 0) {
                    // Method exit checks
                    $effectiveWrites = max($methodMaxPathWrites, $methodCurrentPathWrites);
                    // Record MultiWriteTransactionRequired candidate for post-file caller analysis
                    if ($effectiveWrites >= 2 && !$methodHasTransaction && !$methodHasDelegation && !$isTest && !str_contains($relPath, 'database/') && !str_contains($relPath, 'tests/')) {
                        $multiWriteCandidates[] = [
                            'method' => $currentMethod,
                            'writeCount' => $effectiveWrites,
                            'hasTransaction' => $methodHasTransaction,
                            'hasDelegation' => $methodHasDelegation,
                            'firstWriteLine' => $methodFirstWriteLine ?: $currentMethodLine,
                            'hasTransactionalComment' => $methodHasTransactionalComment,
                        ];
                    }

                    // Rule 37: UnsafeFileUploadValidation
                    if ($isController && $methodHasFileUploadStore && !$methodHasValidation) {
                        $findings[] = [
                            'category' => 'Security',
                            'rule' => 'UnsafeFileUploadValidation',
                            'severity' => 'warning',
                            'file' => $relPath,
                            'line' => $methodFileUploadLine ?: $currentMethodLine,
                            'message' => "File upload stored via store()/move() without detected validation or Form Request in method '{$currentMethod}()'.",
                            'snippet' => getLineSnippet($content, $methodFileUploadLine ?: $currentMethodLine),
                            'fixGuidance' => "Validate uploaded files (e.g. 'file' => 'required|file|mimes:pdf,jpg|max:2048') or use a dedicated Form Request to prevent webshell uploads.",
                            'githubRef' => 'https://laravel.com/docs/validation#rule-mimes',
                        ];
                    }

                    // Rule 38: RegistrationRegisteredEvent (Optional Advice)
                    $isRegistrationMethod = (str_contains($relPath, 'Register') || $currentMethod === 'register' || ($currentMethod === 'store' && str_contains($relPath, 'RegisteredUser')));
                    if ($isController && $isRegistrationMethod && $methodCreatesUser && !$methodDispatchesRegistered) {
                        $findings[] = [
                            'category' => 'Convention',
                            'rule' => 'RegistrationRegisteredEvent',
                            'severity' => 'info',
                            'file' => $relPath,
                            'line' => $currentMethodLine ?: 1,
                            'message' => "User registration flow creates User without dispatching the Registered event.",
                            'snippet' => getLineSnippet($content, $currentMethodLine ?: 1),
                            'fixGuidance' => "Dispatch event(new Registered(\$user)) to trigger Laravel email verification and welcome notifications.",
                            'githubRef' => 'https://laravel.com/docs/verification',
                        ];
                    }

                    $currentMethod = '';
                    $methodBraceDepth = 0;
                    $methodWriteCount = 0;
                    $methodCurrentPathWrites = 0;
                    $methodMaxPathWrites = 0;
                    $methodBranchStack = [];
                    $methodHasTransaction = false;
                    $methodHasDelegation = false;
                    $methodFirstWriteLine = 0;
                    $methodHasValidation = false;
                    $methodHasFileUploadStore = false;
                    $methodFileUploadLine = 0;
                    $methodCreatesUser = false;
                    $methodDispatchesRegistered = false;
                    $methodHasTransactionalComment = false;
                }
            }
        } elseif ($inArrowFn && ($token->text === ';' || $token->text === ')')) {
            $inArrowFn = false;
        }

        if ($currentMethod !== '' && ($id === (defined('T_RETURN') ? T_RETURN : 0) || $token->text === 'return' || $id === (defined('T_THROW') ? T_THROW : 0) || $token->text === 'throw')) {
            if (!empty($methodBranchStack)) {
                $lastIdx = count($methodBranchStack) - 1;
                $methodBranchStack[$lastIdx]['currentHasReturn'] = true;
            } else {
                $methodMaxPathWrites = max($methodMaxPathWrites, $methodCurrentPathWrites);
                $methodCurrentPathWrites = 0;
            }
        }

        $inRouteClosure = ($isRouteFile && (!empty($closureBraceDepths) || $inArrowFn));

        // Class tracking
        if ($id === T_CLASS) {
            $prevTok = null;
            for ($p = $i - 1; $p >= 0; $p--) {
                if (!$tokens[$p]->isIgnorable()) {
                    $prevTok = $tokens[$p];
                    break;
                }
            }

            $isClassConstant = ($prevTok && ($prevTok->text === '::' || (defined('T_DOUBLE_COLON') && $prevTok->id === T_DOUBLE_COLON)));
            $isAnonymousClass = ($prevTok && (strtolower($prevTok->text) === 'new' || (defined('T_NEW') && $prevTok->id === T_NEW)));

            if (!$isClassConstant && !$isAnonymousClass) {
                // Check if class declaration is abstract
                for ($p = $i - 1; $p >= max(0, $i - 10); $p--) {
                    if ($tokens[$p]->text === ';' || $tokens[$p]->text === '{' || $tokens[$p]->text === '}') break;
                    if (strtolower($tokens[$p]->text) === 'abstract') {
                        $isAbstract = true;
                        break;
                    }
                }

                $inClass = true;
                $classLine = $token->line;
                for ($c = $i + 1; $c < min($i + 120, $tokenCount); $c++) {
                    if ($tokens[$c]->text === '{') break;
                    if ($className === '' && $tokens[$c]->id === T_STRING) {
                        $className = $tokens[$c]->text;
                    }
                    $cIdent = getIdentifier($tokens[$c]);
                    if ($cIdent === 'ShouldQueue') $implementsShouldQueue = true;
                    if ($cIdent === 'ShouldBroadcast' || $cIdent === 'ShouldBroadcastNow') $implementsShouldBroadcast = true;
                }
            }
        }

        // Method tracking
        if ($id === T_FUNCTION) {
            $funcToken = null;
            for ($f = $i + 1; $f < min($i + 5, $tokenCount); $f++) {
                if (!$tokens[$f]->isIgnorable()) {
                    $funcToken = $tokens[$f];
                    break;
                }
            }
            if ($funcToken && $funcToken->id === T_STRING) {
                $currentMethod = $funcToken->text;
                $currentMethodLine = $funcToken->line;
                $methodBraceDepth = 0;
                $classMethods[] = $currentMethod;
                $methodWriteCount = 0;
                $methodCurrentPathWrites = 0;
                $methodMaxPathWrites = 0;
                $methodBranchStack = [];
                $methodHasTransaction = false;
                $methodHasDelegation = false;
                $methodFirstWriteLine = 0;
                $methodHasValidation = false;
                $methodHasFileUploadStore = false;
                $methodCreatesUser = false;
                $methodDispatchesRegistered = false;
                $methodHasTransactionalComment = false;

                if (in_array($currentMethod, ['retryUntil', 'backoff'], true)) {
                    $hasRetryUntil = true;
                }

                // Check docblock or attributes directly preceding the function declaration
                for ($p = $i - 1; $p >= max(0, $i - 15); $p--) {
                    if ($tokens[$p]->text === '}' || $tokens[$p]->text === ';') break;
                    if ($tokens[$p]->id === T_COMMENT || $tokens[$p]->id === T_DOC_COMMENT) {
                        if (hasExplicitRuleSuppression($tokens[$p]->text, 'MultiWriteTransactionRequired')) {
                            $methodHasTransactionalComment = true;
                        }
                    }
                }

                // Inspect parameters until { for FormRequest
                for ($p = $f + 1; $p < min($f + 60, $tokenCount); $p++) {
                    if ($tokens[$p]->text === '{') break;
                    if ($tokens[$p]->id !== T_VARIABLE) {
                        $pIdent = getIdentifier($tokens[$p]);
                        if (str_ends_with(strtolower($pIdent), 'request') && strtolower($pIdent) !== 'request') {
                            $methodHasValidation = true;
                        }
                    }
                }

                if ($currentMethod === '__construct') {
                    $constructLine = $funcToken->line;
                    $parenStart = -1;
                    for ($s = $f + 1; $s < min($f + 10, $tokenCount); $s++) {
                        if ($tokens[$s]->text === '(') { $parenStart = $s; break; }
                    }
                    if ($parenStart >= 0) {
                        $pDepth = 1;
                        $paramStart = $parenStart + 1;
                        for ($p = $parenStart + 1; $p < min($parenStart + 250, $tokenCount); $p++) {
                            if ($tokens[$p]->text === '(') $pDepth++;
                            elseif ($tokens[$p]->text === ')') {
                                $pDepth--;
                                if ($pDepth === 0) {
                                    parseConstructorParam($tokens, $paramStart, $p - 1, $unprotectedModelParams);
                                    break;
                                }
                            } elseif ($tokens[$p]->text === ',' && $pDepth === 1) {
                                parseConstructorParam($tokens, $paramStart, $p - 1, $unprotectedModelParams);
                                $paramStart = $p + 1;
                            }
                        }
                    }
                }
            }
        }

        // Attribute tracking #[WithoutRelations]
        if ($token->id === (defined('T_ATTRIBUTE') ? T_ATTRIBUTE : 0) || str_contains($token->text, '#[')) {
            if (str_contains($token->text, 'WithoutRelations')) {
                $hasWithoutRelations = true;
            }
        }

        // Job properties: public $tries / public $timeout / public $backoff
        if ($token->id === T_VARIABLE && in_array($token->text, ['$tries', '$timeout', '$backoff', '$maxExceptions'], true)) {
            if ($token->text === '$tries') $hasTries = true;
            if ($token->text === '$timeout') $hasTimeout = true;
            if ($token->text === '$backoff' || $token->text === '$maxExceptions') $hasBackoff = true;
        }

        if ($currentMethod === 'broadcastWith') {
            $hasBroadcastWith = true;
        }

        // Model properties: $hidden, $casts, $appends
        if ($isModel && $token->id === T_VARIABLE) {
            if ($token->text === '$hidden') {
                for ($k = $i + 1; $k < min($i + 60, $tokenCount); $k++) {
                    if ($tokens[$k]->text === ';') break;
                    if ($tokens[$k]->id === T_CONSTANT_ENCAPSED_STRING) {
                        $hiddenAttributes[] = trim($tokens[$k]->text, "'\"");
                    }
                }
            } elseif ($token->text === '$casts' || $token->text === '$appends') {
                for ($k = $i + 1; $k < min($i + 100, $tokenCount); $k++) {
                    if ($tokens[$k]->text === ';') break;
                    if ($tokens[$k]->id === T_CONSTANT_ENCAPSED_STRING) {
                        $cVal = trim($tokens[$k]->text, "'\"");
                        if (preg_match('/^(password|remember_token|api_token|secret)$/i', $cVal)) {
                            $exposedAttributes[] = ['name' => $cVal, 'line' => $tokens[$k]->line];
                        }
                    }
                }
            }
        }

        // Method body context tracking
        if ($currentMethod !== '') {
            // Track method calls on $this (e.g. $this->executeCall(...))
            if ($token->id === T_VARIABLE && $token->text === '$this') {
                $arrow = null;
                $targetFunc = null;
                for ($k = $i + 1; $k < min($i + 6, $tokenCount); $k++) {
                    if (!$tokens[$k]->isIgnorable()) {
                        if ($arrow === null && $tokens[$k]->text === '->') {
                            $arrow = $tokens[$k];
                        } elseif ($arrow !== null && $targetFunc === null && $tokens[$k]->id === T_STRING) {
                            $targetFunc = $tokens[$k]->text;
                            for ($lp = $k + 1; $lp < min($k + 5, $tokenCount); $lp++) {
                                if (!$tokens[$lp]->isIgnorable()) {
                                    if ($tokens[$lp]->text === '(') {
                                        $methodCalledMethods[$currentMethod][] = $targetFunc;
                                    }
                                    break;
                                }
                            }
                            break;
                        } else {
                            break;
                        }
                    }
                }
            }
            if (in_array($name, ['validate', 'validator', 'validateWithBag'], true)) {
                $methodHasValidation = true;
            }
            if (in_array($name, ['transaction', 'beginTransaction', 'commit'], true)) {
                $methodHasTransaction = true;
                $methodsWithTransaction[$currentMethod] = true;
            }
            if (in_array($name, ['execute', 'handle', 'run', 'dispatch', 'dispatchSync'], true)) {
                $methodHasDelegation = true;
            }
            if ($name === 'Registered') {
                $methodDispatchesRegistered = true;
            }
            if ($name === 'User') {
                for ($k = $i + 1; $k < min($i + 6, $tokenCount); $k++) {
                    if ($tokens[$k]->text === 'create' || $tokens[$k]->text === 'save') {
                        $methodCreatesUser = true;
                        break;
                    }
                }
            }
            if (in_array($name, ['store', 'storeAs', 'storePublicly', 'move'], true)) {
                $prev = null;
                for ($p = $i - 1; $p >= 0; $p--) {
                    if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
                }
                if ($prev && $prev->text === '->') {
                    $methodHasFileUploadStore = true;
                    if ($methodFileUploadLine === 0) $methodFileUploadLine = $token->line;
                }
            }
            if (in_array($name, ['save', 'update', 'delete', 'create', 'insert', 'upsert', 'sync', 'attach', 'detach'], true)) {
                $prev = null;
                for ($p = $i - 1; $p >= 0; $p--) {
                    if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
                }
                if ($prev && in_array($prev->text, ['->', '::'], true)) {
                    $caller = null;
                    for ($cp = $p - 1; $cp >= 0; $cp--) {
                        if (!$tokens[$cp]->isIgnorable()) { $caller = $tokens[$cp]; break; }
                    }
                    $callerName = $caller ? strtolower(getIdentifier($caller)) : '';
                    $nonDbCallers = ['request', '$request', 'session', '$session', 'cache', '$cache', 'log', 'route', 'validator', '$validator', 'file', 'storage', 'cookie', 'schema', 'blueprint', 'form', '$form', 'response', 'view', 'event', 'bus', 'queue', 'str', 'arr', 'date', 'carbon', 'app', 'config', 'lang'];
                    $isServiceDelegation = (
                        str_ends_with($callerName, 'service') ||
                        str_ends_with($callerName, 'action') ||
                        str_ends_with($callerName, 'manager') ||
                        str_ends_with($callerName, 'repository') ||
                        str_ends_with($callerName, 'client') ||
                        str_ends_with($callerName, 'gateway')
                    );
                    if ($isServiceDelegation) {
                        $methodHasDelegation = true;
                    } elseif (!in_array($callerName, $nonDbCallers, true)) {
                        $methodWriteCount++;
                        if ($methodFirstWriteLine === 0) $methodFirstWriteLine = $token->line;
                        if (!empty($methodBranchStack)) {
                            $lastIdx = count($methodBranchStack) - 1;
                            $methodBranchStack[$lastIdx]['currentWrites']++;
                        } else {
                            $methodCurrentPathWrites++;
                            $methodMaxPathWrites = max($methodMaxPathWrites, $methodCurrentPathWrites);
                        }
                    }
                }
            }

            // Rule 39: UnsafeMassAssignmentRequestAll
            if (!$isTest && in_array($name, ['create', 'update', 'fill', 'firstOrCreate', 'updateOrCreate'], true)) {
                $hasRequestAll = false;
                for ($k = $i + 1; $k < min($i + 20, $tokenCount); $k++) {
                    if ($tokens[$k]->text === ';') break;
                    if ($tokens[$k]->text === '$request' || $tokens[$k]->text === 'request') {
                        for ($ak = $k + 1; $ak < min($k + 6, $tokenCount); $ak++) {
                            if ($tokens[$ak]->text === 'all') {
                                $hasRequestAll = true;
                                break 2;
                            }
                        }
                    } elseif ($tokens[$k]->text === '$_POST' || $tokens[$k]->text === '$_REQUEST') {
                        $hasRequestAll = true;
                        break;
                    }
                }
                if ($hasRequestAll) {
                    $findings[] = [
                        'category' => 'Security',
                        'rule' => 'UnsafeMassAssignmentRequestAll',
                        'severity' => 'warning',
                        'file' => $relPath,
                        'line' => $token->line,
                        'message' => "Unsafe mass-assignment: passing raw \$request->all() directly into {$name}().",
                        'snippet' => getLineSnippet($content, $token->line),
                        'fixGuidance' => "Pass only explicitly validated attributes via \$request->validated() or \$request->safe() to prevent unauthorized privilege escalation.",
                        'githubRef' => 'https://laravel.com/docs/validation#form-request-validation',
                    ];
                }
            }
        }

        // Rule 1: NoEnvOutsideConfig
        if (!$isConfig && !$isTest && $name === 'env') {
            // Check previous non-whitespace token isn't -> or :: or function
            $prev = null;
            for ($p = $i - 1; $p >= 0; $p--) {
                if (!$tokens[$p]->isIgnorable()) {
                    $prev = $tokens[$p];
                    break;
                }
            }
            if ($prev && !in_array($prev->text, ['->', '::', '?', 'function', 'fn'], true)) {
                // Check next token is '('
                $next = null;
                for ($n = $i + 1; $n < $tokenCount; $n++) {
                    if (!$tokens[$n]->isIgnorable()) {
                        $next = $tokens[$n];
                        break;
                    }
                }
                if ($next && $next->text === '(') {
                    $findings[] = [
                        'category' => 'Architecture',
                        'rule' => 'NoEnvOutsideConfig',
                        'severity' => 'error',
                        'file' => $relPath,
                        'line' => $token->line,
                        'message' => 'Direct env() call outside config/ returns null when configuration is cached for production (config:cache).',
                        'snippet' => getLineSnippet($content, $token->line),
                        'fixGuidance' => "Move the environment variable into a config file (e.g. config/services.php) and call config('key') instead.",
                        'githubRef' => 'https://github.com/alexeymezenin/laravel-best-practices#do-not-get-data-from-the-env-file-directly',
                    ];
                }
            }
        }

        // Rule 6: NoDebugCallsInApp
        if (!$isTest && in_array($name, ['dd', 'dump', 'var_dump', 'ray'], true)) {
            $prev = null;
            for ($p = $i - 1; $p >= 0; $p--) {
                if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
            }
            if ($prev && !in_array($prev->text, ['->', '::', 'function', 'fn'], true)) {
                $next = null;
                for ($n = $i + 1; $n < $tokenCount; $n++) {
                    if (!$tokens[$n]->isIgnorable()) { $next = $tokens[$n]; break; }
                }
                if ($next && $next->text === '(') {
                    $findings[] = [
                        'category' => 'Security',
                        'rule' => 'NoDebugCallsInApp',
                        'severity' => 'error',
                        'file' => $relPath,
                        'line' => $token->line,
                        'message' => "Debug statement '{$name}()' left in committed application code.",
                        'snippet' => getLineSnippet($content, $token->line),
                        'fixGuidance' => "Remove {$name}() or replace with Log::info() before deploying.",
                        'githubRef' => 'https://github.com/alexeymezenin/laravel-best-practices',
                    ];
                }
            }
        }

        // Rule 7: EmptyGuardedRisk on Models
        if ($isModel && $token->text === '$guarded') {
            // Look forward for = []
            $matchedEmpty = false;
            for ($k = $i + 1; $k < min($i + 15, $tokenCount); $k++) {
                if ($tokens[$k]->text === ';') break;
                if ($tokens[$k]->text === '[' && isset($tokens[$k+1]) && $tokens[$k+1]->text === ']') {
                    $matchedEmpty = true;
                    break;
                }
            }
            if ($matchedEmpty) {
                $findings[] = [
                    'category' => 'Convention',
                    'rule' => 'EmptyGuardedRisk',
                    'severity' => 'info',
                    'file' => $relPath,
                    'line' => $token->line,
                    'message' => 'Model defines protected $guarded = [], completely disabling mass-assignment protection.',
                    'snippet' => getLineSnippet($content, $token->line),
                    'fixGuidance' => 'Define explicit protected $fillable = [...] attributes to prevent unexpected mass-assignment vulnerabilities.',
                    'githubRef' => 'https://github.com/alexeymezenin/laravel-best-practices#mass-assignment',
                ];
            }
        }

        // Rule 5: NoMutationsInRoutes
        if ($isRouteFile && in_array($name, ['create', 'update', 'delete', 'save', 'destroy', 'insert', 'upsert'], true)) {
            $prev = null;
            for ($p = $i - 1; $p >= 0; $p--) {
                if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
            }
            if ($prev && in_array($prev->text, ['->', '::'], true)) {
                $isRouteRegistration = false;
                if ($prev->text === '::') {
                    $classPrev = null;
                    for ($cp = $p - 1; $cp >= 0; $cp--) {
                        if (!$tokens[$cp]->isIgnorable()) { $classPrev = $tokens[$cp]; break; }
                    }
                    if ($classPrev && in_array(getIdentifier($classPrev), ['Route', 'Router'], true)) {
                        $isRouteRegistration = true;
                    }
                } elseif ($prev->text === '->') {
                    $callerPrev = null;
                    for ($cp = $p - 1; $cp >= 0; $cp--) {
                        if (!$tokens[$cp]->isIgnorable()) { $callerPrev = $tokens[$cp]; break; }
                    }
                    if ($callerPrev && in_array(getIdentifier($callerPrev), ['router', '$router', '$route', 'Route'], true)) {
                        $isRouteRegistration = true;
                    }
                }

                if (!$isRouteRegistration && $inRouteClosure) {
                    $findings[] = [
                        'category' => 'Architecture',
                        'rule' => 'NoMutationsInRoutes',
                        'severity' => 'warning',
                        'file' => $relPath,
                        'line' => $token->line,
                        'message' => "Database mutation '{$name}()' found inside route closure. Prevents route caching (route:cache).",
                        'snippet' => getLineSnippet($content, $token->line),
                        'fixGuidance' => 'Delegate persistence and mutation logic to a Controller or Invokable Action class.',
                        'githubRef' => 'https://github.com/alexeymezenin/laravel-best-practices#never-put-any-logic-in-routes-files',
                    ];
                }
            }
        }

        // Rule 4: FormRequestValidation
        if ($isController && $name === 'validate') {
            // Count array keys inside validate([ ... ])
            $ruleCount = 0;
            $inArray = false;
            for ($k = $i + 1; $k < min($i + 120, $tokenCount); $k++) {
                if ($tokens[$k]->text === '[') $inArray = true;
                if ($tokens[$k]->text === ']') break;
                if ($inArray && $tokens[$k]->text === '=>') {
                    $ruleCount++;
                }
            }
            if ($ruleCount >= 5) {
                $suggestedReq = str_replace('Controller', 'Request', basename($relPath, '.php'));
                $findings[] = [
                    'category' => 'Architecture',
                    'rule' => 'FormRequestValidation',
                    'severity' => 'info',
                    'file' => $relPath,
                    'line' => $token->line,
                    'message' => "Substantial inline validation ($ruleCount rules) in controller method.",
                    'snippet' => getLineSnippet($content, $token->line),
                    'fixGuidance' => 'Extract into a dedicated Form Request class (php artisan make:request) to keep controllers thin.',
                    'githubRef' => 'https://github.com/alexeymezenin/laravel-best-practices#validation',
                    'refactoring' => [
                        'type' => 'extract_form_request',
                        'suggestedName' => $suggestedReq,
                        'targetPath' => 'app/Http/Requests/' . $suggestedReq . '.php',
                    ],
                ];
            }
        }

        // Rule 6: AvoidRawSqlInjection (whereRaw, DB::raw with concatenation / taint input)
        if (in_array($name, ['raw', 'whereRaw', 'selectRaw', 'havingRaw', 'orderByRaw'], true)) {
            $prev = null;
            for ($p = $i - 1; $p >= 0; $p--) {
                if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
            }
            if ($prev && in_array($prev->text, ['->', '::'], true)) {
                // Check if string concatenation with variables or request input occurs inside first argument
                $hasConcat = false;
                $hasTaintedRequest = false;
                for ($k = $i + 1; $k < min($i + 35, $tokenCount); $k++) {
                    if ($tokens[$k]->text === ',') break; // second param (bindings) started
                    if ($tokens[$k]->text === ')') break;
                    if ($tokens[$k]->text === '.' && isset($tokens[$k+1]) && ($tokens[$k+1]->id === T_VARIABLE || $tokens[$k+1]->text === '$request')) {
                        $hasConcat = true;
                        if ($tokens[$k+1]->text === '$request') $hasTaintedRequest = true;
                        break;
                    }
                    if ($tokens[$k]->id === T_CONSTANT_ENCAPSED_STRING && str_contains($tokens[$k]->text, '$')) {
                        $hasConcat = true;
                        break;
                    }
                    if ($tokens[$k]->text === '$request') {
                        $hasConcat = true;
                        $hasTaintedRequest = true;
                        break;
                    }
                }
                if ($hasConcat) {
                    $severity = $hasTaintedRequest ? 'danger' : 'warning';
                    $msg = $hasTaintedRequest 
                        ? 'High-risk SQL injection taint: user request input interpolated directly into raw SQL expression.'
                        : 'Potential SQL injection risk: raw SQL uses variable concatenation instead of parameterized bindings.';
                    $findings[] = [
                        'category' => 'Security',
                        'rule' => 'AvoidRawSqlInjection',
                        'severity' => $severity,
                        'file' => $relPath,
                        'line' => $token->line,
                        'message' => $msg,
                        'snippet' => getLineSnippet($content, $token->line),
                        'fixGuidance' => "Pass variables as query parameter bindings (e.g. whereRaw('status = ?', [\$status])) instead of string concatenation.",
                        'githubRef' => 'https://github.com/alexeymezenin/laravel-best-practices#prefer-to-use-eloquent-over-using-query-builder-and-raw-sql-queries-prefer-collections-over-arrays',
                    ];
                }
            }
        }

        // Rule 15: LongLivedProcessStateLeak (Octane / Reverb / Workerman)
        if (($isController || $isService || $isMiddleware) && $token->id === T_STATIC) {
            // Check if next token is variable or array property
            $next = null;
            for ($n = $i + 1; $n < min($i + 5, $tokenCount); $n++) {
                if (!$tokens[$n]->isIgnorable()) { $next = $tokens[$n]; break; }
            }
            if ($next && $next->id === T_VARIABLE) {
                $severity = $usesOctane ? 'error' : 'warning';
                $findings[] = [
                    'category' => 'Architecture',
                    'rule' => 'LongLivedProcessStateLeak',
                    'severity' => $severity,
                    'file' => $relPath,
                    'line' => $token->line,
                    'message' => "Static mutable property '{$next->text}' leaks state across requests in persistent process environments (Octane, Reverb, Workerman).",
                    'snippet' => getLineSnippet($content, $token->line),
                    'fixGuidance' => 'Avoid storing request-scoped state in static properties. Use instance dependencies or request-bound services.',
                    'githubRef' => 'https://laravel.com/docs/octane#managing-memory-leaks',
                ];
            }
        }

        // Rule 16: VersionAwareLazyProps (Inertia v1 vs v2)
        if ($isController && $name === 'lazy') {
            $prev = null;
            for ($p = $i - 1; $p >= 0; $p--) {
                if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
            }
            if ($prev && $prev->text === '::') {
                $classPrev = null;
                for ($cp = $i - 2; $cp >= 0; $cp--) {
                    if (!$tokens[$cp]->isIgnorable()) { $classPrev = $tokens[$cp]; break; }
                }
                if ($classPrev && getIdentifier($classPrev) === 'Inertia') {
                    if ($hasInertiaV2) {
                        $findings[] = [
                            'category' => 'Inertia.js',
                            'rule' => 'VersionAwareLazyProps',
                            'severity' => 'info',
                            'file' => $relPath,
                            'line' => $token->line,
                            'message' => 'Inertia::lazy() is deprecated in Inertia v2.',
                            'snippet' => getLineSnippet($content, $token->line),
                            'fixGuidance' => 'Use Inertia::defer(fn() => ...) for automatic background loading or Inertia::optional(fn() => ...) for on-demand partial reloads.',
                            'githubRef' => 'https://inertiajs.com/deferred-props',
                        ];
                    }
                }
            }
        }

        // Rule 17: WrapInertiaPropsInClosures
        if ($isController && $name === 'render') {
            $prev = null;
            for ($p = $i - 1; $p >= 0; $p--) {
                if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
            }
            if ($prev && $prev->text === '::') {
                $classPrev = null;
                for ($cp = $i - 2; $cp >= 0; $cp--) {
                    if (!$tokens[$cp]->isIgnorable()) { $classPrev = $tokens[$cp]; break; }
                }
                if ($classPrev && getIdentifier($classPrev) === 'Inertia') {
                    // Inspect props array for direct model calls without fn() =>
                    $propCount = 0;
                    $directQueryLine = 0;
                    $inPropsArray = false;
                    for ($k = $i + 1; $k < min($i + 180, $tokenCount); $k++) {
                        if ($tokens[$k]->text === '[') $inPropsArray = true;
                        if ($tokens[$k]->text === '=>') {
                            $propCount++;
                            // Check next non-whitespace tokens
                            $valToken = null;
                            for ($v = $k + 1; $v < min($k + 10, $tokenCount); $v++) {
                                if (!$tokens[$v]->isIgnorable()) { $valToken = $tokens[$v]; break; }
                            }
                            if ($valToken && in_array(getIdentifier($valToken), ['User', 'Order', 'Product', 'DB', 'Model'], true)) {
                                $directQueryLine = $valToken->line;
                            }
                        }
                        if ($inPropsArray && $tokens[$k]->text === ']') break;
                    }
                    if ($propCount >= 2 && $directQueryLine > 0) {
                        $findings[] = [
                            'category' => 'Inertia.js',
                            'rule' => 'WrapInertiaPropsInClosures',
                            'severity' => 'warning',
                            'file' => $relPath,
                            'line' => $directQueryLine,
                            'message' => 'Direct Eloquent query inside Inertia::render() array executes on every partial reload even when not requested.',
                            'snippet' => getLineSnippet($content, $directQueryLine),
                            'fixGuidance' => "Wrap query in a closure: 'data' => fn () => Model::paginate() so partial reloads can skip unused props.",
                            'githubRef' => 'https://inertiajs.com/partial-reloads#lazy-data-evaluation',
                        ];
                    }
                }
            }
        }

        // Rule 18: LightSharedProps in HandleInertiaRequests
        if ($isMiddleware && str_contains($relPath, 'HandleInertiaRequests') && $currentMethod === 'share') {
            if (in_array($name, ['all', 'get', 'paginate', 'count', 'pluck', 'first'], true)) {
                $prev = null;
                for ($p = $i - 1; $p >= 0; $p--) {
                    if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
                }
                if ($prev && in_array($prev->text, ['->', '::'], true)) {
                    $isDbQuery = false;

                    if ($prev->text === '::') {
                        $classPrev = null;
                        for ($cp = $p - 1; $cp >= 0; $cp--) {
                            if (!$tokens[$cp]->isIgnorable()) { $classPrev = $tokens[$cp]; break; }
                        }
                        if ($classPrev) {
                            $callerName = getIdentifier($classPrev);
                            $nonModelCallers = ['Route', 'Auth', 'Session', 'Config', 'Cache', 'Arr', 'Str', 'URL', 'Vite', 'App', 'Cookie', 'Gate', 'Log', 'Event'];
                            if (!in_array($callerName, $nonModelCallers, true)) {
                                $isDbQuery = true;
                            }
                        }
                    } elseif ($prev->text === '->') {
                        $chainTokens = [];
                        for ($cp = $p - 1; $cp >= max(0, $p - 25); $cp--) {
                            if ($tokens[$cp]->text === ';' || $tokens[$cp]->text === '[' || $tokens[$cp]->text === '=>') break;
                            if (!$tokens[$cp]->isIgnorable()) {
                                $chainTokens[] = $tokens[$cp]->text;
                            }
                        }
                        $chainStr = implode(' ', array_reverse($chainTokens));

                        // Check if chain starts from request/session/cache/config/headers/cookies/Arr
                        $isNonDbChain = (bool)preg_match('/(\$request|session|cache|config|headers|cookies|Arr)\b/i', $chainStr);

                        if (!$isNonDbChain) {
                            if (preg_match('/(DB|where|with|select|query|table|latest|oldest|join)\b/i', $chainStr)) {
                                $isDbQuery = true;
                            } elseif (in_array($name, ['all', 'paginate'], true)) {
                                $isDbQuery = true;
                            }
                        }
                    }

                    if ($isDbQuery) {
                        $findings[] = [
                            'category' => 'Inertia.js',
                            'rule' => 'LightSharedProps',
                            'severity' => 'warning',
                            'file' => $relPath,
                            'line' => $token->line,
                            'message' => 'Database query inside HandleInertiaRequests::share() executes on every single request across the entire app.',
                            'snippet' => getLineSnippet($content, $token->line),
                            'fixGuidance' => 'Keep shared props lightweight (auth.user, flash) or wrap expensive properties in closures.',
                            'githubRef' => 'https://inertiajs.com/shared-data',
                        ];
                    }
                }
            }
        }

        // Rule 33: QueuedBroadcasts (ShouldBroadcastNow vs ShouldBroadcast)
        if ($isEvent && $name === 'ShouldBroadcastNow') {
            $findings[] = [
                'category' => 'Real-Time & Sockets',
                'rule' => 'QueuedBroadcasts',
                'severity' => 'warning',
                'file' => $relPath,
                'line' => $token->line,
                'message' => 'Event implements ShouldBroadcastNow, synchronously broadcasting over WebSockets during HTTP request.',
                'snippet' => getLineSnippet($content, $token->line),
                'fixGuidance' => 'Implement ShouldBroadcast to offload socket transmission to background queue workers, preventing HTTP latency spikes.',
                'githubRef' => 'https://laravel.com/docs/broadcasting#broadcast-queue',
            ];
        }

        // Rule 34: ChannelAuthGuards (public Channel for sensitive models)
        if ($isEvent && $name === 'Channel') {
            $prev = null;
            for ($p = $i - 1; $p >= 0; $p--) {
                if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
            }
            if ($prev && $prev->text === 'new') {
                // Check if channel name string contains order, user, payment, account
                $nextString = null;
                for ($k = $i + 1; $k < min($i + 10, $tokenCount); $k++) {
                    if ($tokens[$k]->id === T_CONSTANT_ENCAPSED_STRING) {
                        $nextString = $tokens[$k]->text;
                        break;
                    }
                }
                if ($nextString && preg_match('/(user|order|payment|invoice|account)/i', $nextString)) {
                    $findings[] = [
                        'category' => 'Real-Time & Sockets',
                        'rule' => 'ChannelAuthGuards',
                        'severity' => 'error',
                        'file' => $relPath,
                        'line' => $token->line,
                        'message' => "Sensitive entity broadcast on a public Channel ({$nextString}).",
                        'snippet' => getLineSnippet($content, $token->line),
                        'fixGuidance' => 'Use PrivateChannel or PresenceChannel and authorize subscriber access in routes/channels.php.',
                        'githubRef' => 'https://laravel.com/docs/broadcasting#authorizing-channels',
                    ];
                }
            }
        }

        // Rule: SensitiveDataLogging
        if (!$isTest && (in_array($name, ['info', 'debug', 'warning', 'error', 'critical', 'notice', 'alert', 'emergency'], true) || $name === 'logger')) {
            $isLogCall = false;
            if ($name === 'logger') {
                $isLogCall = true;
            } else {
                $prev = null;
                for ($p = $i - 1; $p >= 0; $p--) {
                    if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
                }
                if ($prev && $prev->text === '::') {
                    $classPrev = null;
                    for ($cp = $p - 1; $cp >= 0; $cp--) {
                        if (!$tokens[$cp]->isIgnorable()) { $classPrev = $tokens[$cp]; break; }
                    }
                    if ($classPrev && getIdentifier($classPrev) === 'Log') {
                        $isLogCall = true;
                    }
                }
            }

            if ($isLogCall) {
                $pDepth = 0;
                $foundSensitive = false;
                $sensitiveMatch = '';
                for ($k = $i + 1; $k < min($i + 80, $tokenCount); $k++) {
                    if ($tokens[$k]->text === '(') $pDepth++;
                    elseif ($tokens[$k]->text === ')') {
                        $pDepth--;
                        if ($pDepth <= 0) break;
                    }
                    if ($pDepth >= 1 && $tokens[$k]->id === T_CONSTANT_ENCAPSED_STRING) {
                        $cleanStr = trim($tokens[$k]->text, "'\"");
                        if (preg_match('/^(password|secret|bearer_token|bearer|api_key|apikey|card_number|credit_card|cvv|ssn)$/i', $cleanStr)) {
                            for ($nx = $k + 1; $nx < min($k + 5, $tokenCount); $nx++) {
                                if (!$tokens[$nx]->isIgnorable()) {
                                    if ($tokens[$nx]->text === '=>') {
                                        $foundSensitive = true;
                                        $sensitiveMatch = $cleanStr;
                                    }
                                    break;
                                }
                            }
                        }
                    }
                }
                if ($foundSensitive) {
                    $findings[] = [
                        'category' => 'Security',
                        'rule' => 'SensitiveDataLogging',
                        'severity' => 'warning',
                        'file' => $relPath,
                        'line' => $token->line,
                        'message' => "Potential credential or sensitive data ('{$sensitiveMatch}') logged directly via {$name}().",
                        'snippet' => getLineSnippet($content, $token->line),
                        'fixGuidance' => "Redact sensitive attributes before logging (e.g. ['{$sensitiveMatch}' => '[REDACTED]']) to prevent writing credentials to disk.",
                        'githubRef' => 'https://owasp.org/www-community/attacks/Log_Injection',
                    ];
                }
            }
        }

        // Rule: UnsafeDynamicExecution
        if (!$isTest && in_array($name, ['eval', 'unserialize', 'shell_exec', 'passthru', 'system'], true)) {
            $prev = null;
            for ($p = $i - 1; $p >= 0; $p--) {
                if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
            }
            if (!$prev || (!in_array($prev->text, ['->', '::', 'function', 'fn'], true) && $prev->id !== (defined('T_OBJECT_OPERATOR') ? T_OBJECT_OPERATOR : 0))) {
                $isVulnerable = false;
                if ($name === 'eval') {
                    $isVulnerable = true;
                } elseif ($name === 'unserialize') {
                    $hasAllowedClasses = false;
                    for ($k = $i + 1; $k < min($i + 40, $tokenCount); $k++) {
                        if ($tokens[$k]->text === ';') break;
                        if (str_contains($tokens[$k]->text, 'allowed_classes')) {
                            $hasAllowedClasses = true;
                            break;
                        }
                    }
                    if (!$hasAllowedClasses) {
                        $isVulnerable = true;
                    }
                } elseif (in_array($name, ['shell_exec', 'passthru', 'system'], true)) {
                    for ($k = $i + 1; $k < min($i + 30, $tokenCount); $k++) {
                        if ($tokens[$k]->text === ';') break;
                        if ($tokens[$k]->id === T_VARIABLE) {
                            $isVulnerable = true;
                            break;
                        }
                    }
                }

                if ($isVulnerable) {
                    $findings[] = [
                        'category' => 'Security',
                        'rule' => 'UnsafeDynamicExecution',
                        'severity' => 'error',
                        'file' => $relPath,
                        'line' => $token->line,
                        'message' => "Unsafe dynamic execution '{$name}()' detected. Risks Remote Code Execution (RCE).",
                        'snippet' => getLineSnippet($content, $token->line),
                        'fixGuidance' => "Avoid eval() and raw unserialize(). Use json_decode() or parameterized Symfony Process runners instead of raw shell functions.",
                        'githubRef' => 'https://owasp.org/www-community/attacks/Code_Injection',
                    ];
                }
            }
        }

        // Rule: UnsafeExternalRedirect
        if ($name === 'away') {
            $prev = null;
            for ($p = $i - 1; $p >= 0; $p--) {
                if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
            }
            if ($prev && in_array($prev->text, ['->', '::'], true)) {
                $caller = null;
                if ($prev->text === '::') {
                    for ($cp = $p - 1; $cp >= 0; $cp--) {
                        if (!$tokens[$cp]->isIgnorable()) { $caller = $tokens[$cp]; break; }
                    }
                } elseif ($prev->text === '->') {
                    for ($cp = $p - 1; $cp >= 0; $cp--) {
                        if ($tokens[$cp]->text === '(') {
                            for ($kp = $cp - 1; $kp >= 0; $kp--) {
                                if (!$tokens[$kp]->isIgnorable()) { $caller = $tokens[$kp]; break 2; }
                            }
                        } elseif (!$tokens[$cp]->isIgnorable() && $tokens[$cp]->text !== ')') {
                            $caller = $tokens[$cp];
                            break;
                        }
                    }
                }
                if ($caller && in_array(getIdentifier($caller), ['redirect', 'Redirect'], true)) {
                    // Extract all argument tokens inside away(...)
                    $argTokens = [];
                    $depth = 0;
                    for ($k = $i + 1; $k < min($i + 50, $tokenCount); $k++) {
                        if ($tokens[$k]->text === '(') { $depth++; if ($depth === 1) continue; }
                        if ($tokens[$k]->text === ')') { $depth--; if ($depth <= 0) break; }
                        if ($depth >= 1 && !$tokens[$k]->isIgnorable()) {
                            $argTokens[] = $tokens[$k];
                        }
                    }

                    if (!empty($argTokens)) {
                        $isTainted = false;
                        $firstArg = $argTokens[0];

                        // 1. Check if destination begins with a hardcoded trusted string URL (e.g. 'https://trusted.com/...')
                        if ($firstArg->id === T_CONSTANT_ENCAPSED_STRING) {
                            $unquoted = trim($firstArg->text, "'\"");
                            if (preg_match('#^https?://[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}#i', $unquoted) || str_starts_with($unquoted, '/')) {
                                $isTainted = false;
                            }
                        } else {
                            // 2. Direct user input check inside argument expression:
                            // e.g. away($request->input('url')), away(request('target')), away($_GET['url'])
                            $argStr = implode('', array_map(fn($t) => $t->text, $argTokens));
                            if (preg_match('/(\$request->|request\s*\(|\$_GET|\$_POST|\$_REQUEST|Input::)/i', $argStr)) {
                                $isTainted = true;
                            }

                            // 3. Variable taint tracking within the current method
                            // If away($destination) is called with a variable, trace its assignment in the method
                            if (!$isTainted && count($argTokens) === 1 && $firstArg->id === T_VARIABLE) {
                                $varName = $firstArg->text;
                                for ($p = $i - 1; $p >= max(0, $i - 200); $p--) {
                                    if ($tokens[$p]->text === 'function' && $tokens[$p]->id === (defined('T_FUNCTION') ? T_FUNCTION : 0)) break;
                                    if ($tokens[$p]->id === T_VARIABLE && $tokens[$p]->text === $varName) {
                                        $nextAssign = null;
                                        for ($nx = $p + 1; $nx < min($p + 5, $tokenCount); $nx++) {
                                            if (!$tokens[$nx]->isIgnorable()) { $nextAssign = $tokens[$nx]; break; }
                                        }
                                        if ($nextAssign && $nextAssign->text === '=') {
                                            $rhsTokens = [];
                                            for ($rk = $nx + 1; $rk < min($nx + 30, $tokenCount); $rk++) {
                                                if ($tokens[$rk]->text === ';') break;
                                                if (!$tokens[$rk]->isIgnorable()) {
                                                    $rhsTokens[] = $tokens[$rk];
                                                }
                                            }
                                            $rhsStr = implode('', array_map(fn($t) => $t->text, $rhsTokens));
                                            if (preg_match('/(\$request->|request\s*\(|\$_GET|\$_POST|\$_REQUEST|Input::)/i', $rhsStr)) {
                                                $isTainted = true;
                                            }
                                            break;
                                        }
                                    }
                                }
                            }

                            // 4. Check for URL/host validation in the method
                            // If user input was validated or sanitized against trusted hosts, it is not tainted
                            if ($isTainted) {
                                for ($p = $i - 1; $p >= max(0, $i - 200); $p--) {
                                    if ($tokens[$p]->text === 'function' && $tokens[$p]->id === (defined('T_FUNCTION') ? T_FUNCTION : 0)) break;
                                    $tIdent = getIdentifier($tokens[$p]);
                                    if (preg_match('/^(?:validate.*|sanitize.*|isallowed.*|assert.*redirect.*|filter_var|parse_url|TrustedRedirector|ExternalRedirectValidator)$/i', $tIdent)) {
                                        $isTainted = false;
                                        break;
                                    }
                                }
                            }
                        }

                        if ($isTainted) {
                            $findings[] = [
                                'category' => 'Security',
                                'rule' => 'UnsafeExternalRedirect',
                                'severity' => 'warning',
                                'file' => $relPath,
                                'line' => $token->line,
                                'message' => 'Potential open redirect: redirect()->away() called with unvalidated user request input.',
                                'snippet' => getLineSnippet($content, $token->line),
                                'fixGuidance' => 'Validate the target URL against an allowlist of trusted domains or use route() / url() to prevent phishing redirects.',
                                'githubRef' => 'https://laravel.com/docs/routing#redirects',
                            ];
                        }
                    }
                }
            }
        }

        // Rule: UnboundedQueryInJobs
        if (($isJob || str_contains($relPath, 'Commands/') || str_contains($relPath, 'Console/')) && in_array($name, ['all', 'get'], true)) {
            $prev = null;
            for ($p = $i - 1; $p >= 0; $p--) {
                if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
            }
            if ($prev && in_array($prev->text, ['::', '->'], true)) {
                $isDirectModel = false;
                if ($prev->text === '::') {
                    $classPrev = null;
                    for ($cp = $p - 1; $cp >= 0; $cp--) {
                        if (!$tokens[$cp]->isIgnorable()) { $classPrev = $tokens[$cp]; break; }
                    }
                    if ($classPrev && !in_array(getIdentifier($classPrev), ['Route', 'Auth', 'Session', 'Config', 'Cache', 'Arr', 'Str', 'URL', 'Vite', 'App', 'Cookie', 'Gate', 'Log', 'Event'], true)) {
                        $isDirectModel = true;
                    }
                }
                if ($isDirectModel) {
                    $lineSnippet = getLineSnippet($content, $token->line);
                    if (str_contains($lineSnippet, 'foreach') || str_contains($lineSnippet, '::all()')) {
                        $findings[] = [
                            'category' => 'Performance',
                            'rule' => 'UnboundedQueryInJobs',
                            'severity' => 'warning',
                            'file' => $relPath,
                            'line' => $token->line,
                            'message' => "Unbounded query Model::{$name}() inside background job or command. May exhaust worker memory (OOM).",
                            'snippet' => $lineSnippet,
                            'fixGuidance' => "Use Model::chunk(100, function (\$records) { ... }) or Model::cursor() to stream large datasets without memory exhaustion.",
                            'githubRef' => 'https://laravel.com/docs/queries#chunking-results',
                        ];
                    }
                }
            }
        }

        // Rule: AuthRouteRateLimiting
        // Rate limiting is strictly required on mutating credential submission endpoints (POST/PUT/PATCH).
        // GET routes only display login/register forms and must not be flagged.
        if ($isRouteFile && in_array($name, ['post', 'put', 'patch'], true)) {
            $prev = null;
            for ($p = $i - 1; $p >= 0; $p--) {
                if (!$tokens[$p]->isIgnorable()) { $prev = $tokens[$p]; break; }
            }
            if ($prev && in_array($prev->text, ['::', '->'], true)) {
                $routeStr = '';
                for ($k = $i + 1; $k < min($i + 10, $tokenCount); $k++) {
                    if ($tokens[$k]->id === T_CONSTANT_ENCAPSED_STRING) {
                        $routeStr = trim($tokens[$k]->text, "'\"");
                        break;
                    }
                    if ($tokens[$k]->text === ';') break;
                }
                if ($routeStr !== '' && preg_match('/^(login|register|password\/reset|password\/email|forgot-password|otp|two-factor-challenge)$/i', ltrim($routeStr, '/'))) {
                    $hasThrottle = false;
                    for ($k = $i + 1; $k < min($i + 50, $tokenCount); $k++) {
                        if ($tokens[$k]->text === ';') break;
                        if (str_contains(strtolower($tokens[$k]->text), 'throttle')) {
                            $hasThrottle = true;
                            break;
                        }
                    }
                    if (!$hasThrottle) {
                        $findings[] = [
                            'category' => 'Security',
                            'rule' => 'AuthRouteRateLimiting',
                            'severity' => 'warning',
                            'file' => $relPath,
                            'line' => $token->line,
                            'message' => "Authentication route '{$routeStr}' does not define throttle rate limiting middleware.",
                            'snippet' => getLineSnippet($content, $token->line),
                            'fixGuidance' => "Apply throttle middleware (e.g. ->middleware('throttle:6,1')) to prevent credential stuffing and brute force attacks.",
                            'githubRef' => 'https://laravel.com/docs/routing#rate-limiting',
                        ];
                    }
                }
            }
        }
    }

    // Post-file checks
    // Rule 13: JobRetriesTimeout (scoped to app/Jobs/ only, skipping Mail, Notifications, and abstract base classes)
    if ($isJob && !$isAbstract && !$isMail && !$isNotification && $inClass && (!$hasTries && !$hasTimeout && !$hasBackoff && !$hasRetryUntil)) {
        $targetLine = $classLine ?: 1;
        $findings[] = [
            'category' => 'Architecture',
            'rule' => 'JobRetriesTimeout',
            'severity' => 'info',
            'file' => $relPath,
            'line' => $targetLine,
            'message' => "Queued job '{$className}' does not define public \$tries or public \$timeout.",
            'snippet' => getLineSnippet($content, $targetLine),
            'fixGuidance' => 'Declare public int $tries = 3; or public int $timeout = 120; to avoid stalled queue workers.',
            'githubRef' => 'https://laravel.com/docs/queues#max-job-attempts-and-timeout',
        ];
    }

    // Rule 14: QueueJobWithoutRelations
    // Scoped strictly to queued contexts: must implement ShouldQueue or ShouldBroadcast,
    // or be a Job implementing/using queue traits. Suppress for synchronous Mail/Notifications/Events.
    $isQueuedTarget = $implementsShouldQueue || $implementsShouldBroadcast;
    if ($isJob && ($implementsShouldQueue || str_contains($content, 'ShouldQueue') || str_contains($content, 'InteractsWithQueue'))) {
        $isQueuedTarget = true;
    }
    if (($isMail || $isNotification) && !$implementsShouldQueue) {
        $isQueuedTarget = false;
    }
    if ($isEvent && !$implementsShouldBroadcast) {
        $isQueuedTarget = false;
    }

    if ($isQueuedTarget && $inClass && $constructLine > 0 && !$hasWithoutRelations) {
        if (!empty($unprotectedModelParams)) {
            $firstParam = $unprotectedModelParams[0];
            $targetLine = $firstParam['line'] ?: $constructLine;
            $findings[] = [
                'category' => 'Performance',
                'rule' => 'QueueJobWithoutRelations',
                'severity' => 'info',
                'file' => $relPath,
                'line' => $targetLine,
                'message' => "Queued job or event '{$className}' accepts model \${$firstParam['name']} without #[WithoutRelations] attribute.",
                'snippet' => getLineSnippet($content, $targetLine),
                'fixGuidance' => 'Add the #[WithoutRelations] attribute to constructor model arguments to avoid serializing heavy relationship trees.',
                'githubRef' => 'https://laravel.com/docs/queues#ignoring-relations',
            ];
        }
    }

    // Rule 35: BroadcastPayloadSanitization
    if (($implementsShouldBroadcast || $isEvent) && $inClass && str_contains($content, 'ShouldBroadcast') && !$hasBroadcastWith) {
        $targetLine = $classLine ?: 1;
        $findings[] = [
            'category' => 'Real-Time & Sockets',
            'rule' => 'BroadcastPayloadSanitization',
            'severity' => 'warning',
            'file' => $relPath,
            'line' => $targetLine,
            'message' => "Broadcast event '{$className}' does not define a broadcastWith() method.",
            'snippet' => getLineSnippet($content, $targetLine),
            'fixGuidance' => 'Implement broadcastWith(): array to broadcast only explicit attributes rather than the full model with relations.',
            'githubRef' => 'https://laravel.com/docs/broadcasting#broadcast-data',
        ];
    }

    // Rule: ModelExposedSensitiveAttributes
    if ($isModel && !empty($exposedAttributes)) {
        $hiddenLower = array_map('strtolower', $hiddenAttributes);
        foreach ($exposedAttributes as $exposed) {
            if (!in_array(strtolower($exposed['name']), $hiddenLower, true)) {
                $targetLine = $exposed['line'] ?: ($classLine ?: 1);
                $findings[] = [
                    'category' => 'Security',
                    'rule' => 'ModelExposedSensitiveAttributes',
                    'severity' => 'warning',
                    'file' => $relPath,
                    'line' => $targetLine,
                    'message' => "Model '{$className}' has sensitive attribute '{$exposed['name']}' not hidden in \$hidden.",
                    'snippet' => getLineSnippet($content, $targetLine),
                    'fixGuidance' => "Add '{$exposed['name']}' to protected \$hidden = [...] to prevent accidental leakage in API responses.",
                    'githubRef' => 'https://laravel.com/docs/eloquent-serialization#hiding-attributes-from-json',
                ];
            }
        }
    }

    // Rule 36: MultiWriteTransactionRequired (post-file evaluation with caller transaction resolution)
    if (!empty($multiWriteCandidates)) {
        // Resolve methods invoked by transaction-owning methods (including 2 levels deep)
        foreach ($methodsWithTransaction as $txMethod => $_) {
            if (isset($methodCalledMethods[$txMethod])) {
                foreach ($methodCalledMethods[$txMethod] as $callee) {
                    $methodsCalledInTransaction[$callee] = true;
                    if (isset($methodCalledMethods[$callee])) {
                        foreach ($methodCalledMethods[$callee] as $subCallee) {
                            $methodsCalledInTransaction[$subCallee] = true;
                        }
                    }
                }
            }
        }

        $isSupportClass = (str_ends_with($relPath, 'Support.php') || str_ends_with($relPath, 'Helper.php') || str_ends_with($relPath, 'Utils.php') || str_ends_with($className, 'Support') || str_ends_with($className, 'Helper') || str_ends_with($className, 'Utils'));

        foreach ($multiWriteCandidates as $candidate) {
            $cMethod = $candidate['method'];
            if ($isSupportClass || isset($methodsCalledInTransaction[$cMethod])) {
                continue;
            }

            if (!empty($candidate['hasDelegation'])) {
                continue;
            }

            // In Action / Service classes, if the class has transaction entrypoints, internal execution step helpers are suppressed
            if (!$isController && !empty($methodsWithTransaction) && (str_starts_with($cMethod, 'execute') || str_starts_with($cMethod, 'perform') || str_starts_with($cMethod, 'checkAndHandle') || str_starts_with($cMethod, 'sync') || str_starts_with($cMethod, 'save') || str_starts_with($cMethod, 'apply') || str_starts_with($cMethod, 'handle'))) {
                continue;
            }

            // Universal suppression if the method or docblock has a transactional, caller-owned, or false-positive comment
            if (!empty($candidate['hasTransactionalComment'])) {
                continue;
            }

            // In Controller classes:
            // Suppress if the action is routed through transactional middleware (e.g. RecordAdminActivity),
            // or if the controller uses transactional middleware.
            if ($isController) {
                if (isset($routesUnderTransactionalMiddleware[$className][$cMethod]) ||
                    isset($controllersUnderTransactionalMiddleware[$className])) {
                    continue;
                }
            }

            $findings[] = [
                'category' => 'Architecture',
                'rule' => 'MultiWriteTransactionRequired',
                'severity' => 'warning',
                'file' => $relPath,
                'line' => $candidate['firstWriteLine'],
                'message' => "Method '{$cMethod}()' performs multiple ({$candidate['writeCount']}) database writes without DB::transaction().",
                'snippet' => getLineSnippet($content, $candidate['firstWriteLine']),
                'fixGuidance' => "Wrap multi-write workflows in DB::transaction(function () { ... }) to guarantee atomic data consistency.",
                'githubRef' => 'https://laravel.com/docs/database#database-transactions',
            ];
        }
    }
}

// Post-scan: Filter findings that have inline suppression comments in the source
if (!empty($findings)) {
    $filteredFindings = [];
    $fileLinesCache = [];
    foreach ($findings as $f) {
        $fFile = $f['file'];
        $fLine = $f['line'];
        $fRule = $f['rule'];

        $fullPath = $projectPath !== '' ? $projectPath . '/' . $fFile : $fFile;
        if (!isset($fileLinesCache[$fFile])) {
            if (file_exists($fullPath)) {
                $fileLinesCache[$fFile] = file($fullPath, FILE_IGNORE_NEW_LINES) ?: [];
            } else {
                $fileLinesCache[$fFile] = [];
            }
        }
        $lines = $fileLinesCache[$fFile];

        $isSuppressed = false;
        // Check current line and previous lines (up to 5 lines back) for suppression comments
        for ($chk = max(0, $fLine - 5); $chk <= min(count($lines) - 1, $fLine - 1); $chk++) {
            $lineContent = $lines[$chk] ?? '';
            if (preg_match('/(?:@laradeck-ignore|@phpstan-ignore|laradeck-ignore|false[\s_-]?positive)(?:-line|-next-line)?(?::\s*([a-zA-Z0-9_, -]+))?/i', $lineContent, $m)) {
                if (empty($m[1]) || str_contains($m[1], $fRule) || str_contains($m[1], 'all')) {
                    $isSuppressed = true;
                    break;
                }
            }
        }

        if (!$isSuppressed) {
            $filteredFindings[] = $f;
        }
    }
    $findings = $filteredFindings;
}

header('Content-Type: application/json');
echo json_encode($findings, JSON_UNESCAPED_SLASHES);
