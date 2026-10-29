<?php

if ($argc < 3) {
    echo json_encode(['error' => 'Usage: php model_explorer.php <project_path> <operation> [payload_file]']);
    exit(1);
}

$projectPath = rtrim(str_replace('\\', '/', $argv[1]), '/');
$operation = $argv[2];
$payloadFile = $argv[3] ?? null;
$payload = [];

if ($payloadFile && file_exists($payloadFile)) {
    $content = file_get_contents($payloadFile);
    // Only the command envelope is an array; nested JSON objects must retain
    // their shape when Eloquent serializes updated attributes.
    $decoded = json_decode($content);
    $payload = is_object($decoded) ? (array) $decoded : [];
}

$autoload = $projectPath . '/vendor/autoload.php';
$bootstrap = $projectPath . '/bootstrap/app.php';

if (!file_exists($autoload) || !file_exists($bootstrap)) {
    echo json_encode(['error' => 'Not a valid Laravel project (missing vendor/autoload.php or bootstrap/app.php)']);
    exit(0);
}

require_once $autoload;

try {
    $app = require_once $bootstrap;
    if (is_object($app) && method_exists($app, 'make') && $app->bound(\Illuminate\Contracts\Console\Kernel::class)) {
        $kernel = $app->make(\Illuminate\Contracts\Console\Kernel::class);
        $kernel->bootstrap();
    }

    switch ($operation) {
        case 'list_models':
            handleListModels($projectPath);
            break;

        case 'get_records':
            handleGetRecords($payload);
            break;

        case 'update_record':
            handleUpdateRecord($payload);
            break;

        case 'delete_record':
            handleDeleteRecord($payload);
            break;

        case 'get_erd_schema':
            handleGetErdSchema($projectPath);
            break;

        case 'explain_query':
            handleExplainQuery($payload);
            break;

        default:
            echo json_encode(['error' => 'Unknown operation: ' . $operation]);
            break;
    }
} catch (\Throwable $e) {
    echo json_encode(['error' => get_class($e) . ': ' . $e->getMessage() . "\n" . $e->getFile() . ':' . $e->getLine()]);
}

function handleListModels(string $projectPath): void {
    $modelsDir = $projectPath . '/app/Models';
    $searchDirs = [];
    if (is_dir($modelsDir)) {
        $searchDirs['App\\Models\\'] = $modelsDir;
    } else {
        $appDir = $projectPath . '/app';
        if (is_dir($appDir)) {
            $searchDirs['App\\'] = $appDir;
        }
    }

    $models = [];
    foreach ($searchDirs as $namespace => $dir) {
        $files = scandir($dir);
        foreach ($files as $file) {
            if (!str_ends_with($file, '.php')) {
                continue;
            }
            $className = $namespace . substr($file, 0, -4);
            if (!class_exists($className)) {
                continue;
            }

            try {
                $ref = new \ReflectionClass($className);
                if ($ref->isAbstract() || !$ref->isSubclassOf(\Illuminate\Database\Eloquent\Model::class)) {
                    continue;
                }

                /** @var \Illuminate\Database\Eloquent\Model $instance */
                $instance = new $className();
                $table = $instance->getTable();
                $pk = $instance->getKeyName();
                $fillable = $instance->getFillable();
                $hidden = $instance->getHidden();
                $casts = $instance->getCasts();

                $count = 0;
                try {
                    $count = \Illuminate\Support\Facades\DB::table($table)->count();
                } catch (\Throwable $t) {
                    // Table might not exist yet
                }

                $models[] = [
                    'class' => $className,
                    'name' => substr($file, 0, -4),
                    'table' => $table,
                    'primaryKey' => $pk,
                    'fillable' => array_values($fillable),
                    'hidden' => array_values($hidden),
                    'casts' => $casts,
                    'count' => $count,
                ];
            } catch (\Throwable $t) {
                // Ignore model instantiation error
            }
        }
    }

    // Sort models by name
    usort($models, fn($a, $b) => strcmp($a['name'], $b['name']));

    echo json_encode(['models' => $models], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
}

function handleGetRecords(array $payload): void {
    $modelClass = $payload['class'] ?? '';
    if (!$modelClass || !class_exists($modelClass)) {
        echo json_encode(['error' => 'Model class not found: ' . $modelClass]);
        return;
    }

    $page = max(1, (int)($payload['page'] ?? 1));
    $perPage = max(5, min(100, (int)($payload['perPage'] ?? 25)));
    $search = trim($payload['search'] ?? '');
    $sortCol = trim($payload['sortColumn'] ?? '');
    $sortDir = strtolower(trim($payload['sortDirection'] ?? 'desc')) === 'asc' ? 'asc' : 'desc';

    /** @var \Illuminate\Database\Eloquent\Model $instance */
    $instance = new $modelClass();
    $pk = $instance->getKeyName();
    $fillable = $instance->getFillable();

    $query = $modelClass::query();

    if ($search !== '' && !empty($fillable)) {
        $query->where(function ($q) use ($fillable, $search) {
            foreach ($fillable as $col) {
                $q->orWhere($col, 'LIKE', '%' . $search . '%');
            }
        });
    }

    $total = $query->count();

    if ($sortCol !== '') {
        $query->orderBy($sortCol, $sortDir);
    } else {
        $query->orderBy($pk, $sortDir);
    }

    $records = $query->forPage($page, $perPage)->get();
    $recordsArray = $records->toArray();

    // Determine all visible column keys
    $columns = [];
    if (!empty($recordsArray)) {
        $columns = array_keys($recordsArray[0]);
    } else {
        $columns = array_merge([$pk], $fillable, ['created_at', 'updated_at']);
        $columns = array_values(array_unique($columns));
    }

    echo json_encode([
        'records' => $recordsArray,
        'total' => $total,
        'page' => $page,
        'perPage' => $perPage,
        'columns' => $columns,
    ], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
}

function handleUpdateRecord(array $payload): void {
    $modelClass = $payload['class'] ?? '';
    $id = $payload['id'] ?? null;
    $attributes = $payload['attributes'] ?? [];

    if (!$modelClass || !class_exists($modelClass)) {
        echo json_encode(['error' => 'Model class not found: ' . $modelClass]);
        return;
    }
    if ($id === null) {
        echo json_encode(['error' => 'Record ID is required']);
        return;
    }

    /** @var \Illuminate\Database\Eloquent\Model $model */
    $model = $modelClass::findOrFail($id);
    $casts = $model->getCasts();
    foreach ($attributes as $key => $value) {
        $cast = explode(':', strtolower($casts[$key] ?? ''));
        $type = $cast[0] === 'encrypted' ? ($cast[1] ?? '') : $cast[0];
        if ($value !== null && in_array($type, ['bool', 'boolean'], true) && !is_bool($value)) {
            throw new InvalidArgumentException("{$key}: expected a boolean, not a string");
        }
        if ($value !== null && in_array($type, ['array', 'json', 'object', 'collection'], true) && !is_array($value) && !is_object($value)) {
            throw new InvalidArgumentException("{$key}: expected a JSON object or array");
        }
        $model->setAttribute($key, $value);
    }
    $model->save();

    echo json_encode([
        'success' => true,
        'record' => $model->fresh()->toArray(),
    ], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
}

function handleDeleteRecord(array $payload): void {
    $modelClass = $payload['class'] ?? '';
    $id = $payload['id'] ?? null;

    if (!$modelClass || !class_exists($modelClass)) {
        echo json_encode(['error' => 'Model class not found: ' . $modelClass]);
        return;
    }
    if ($id === null) {
        echo json_encode(['error' => 'Record ID is required']);
        return;
    }

    /** @var \Illuminate\Database\Eloquent\Model $model */
    $model = $modelClass::findOrFail($id);
    $model->delete();

    echo json_encode(['success' => true], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
}

function handleGetErdSchema(string $projectPath): void {
    $modelsDir = $projectPath . '/app/Models';
    $searchDirs = [];
    if (is_dir($modelsDir)) {
        $searchDirs['App\\Models\\'] = $modelsDir;
    } else {
        $appDir = $projectPath . '/app';
        if (is_dir($appDir)) {
            $searchDirs['App\\'] = $appDir;
        }
    }

    $tables = [];
    $allRelations = [];
    $advisories = [];
    $baseModelMethods = get_class_methods(\Illuminate\Database\Eloquent\Model::class);
    $baseModelLookup = array_flip($baseModelMethods ?: []);

    foreach ($searchDirs as $namespace => $dir) {
        $files = scandir($dir);
        foreach ($files as $file) {
            if (!str_ends_with($file, '.php')) {
                continue;
            }
            $className = $namespace . substr($file, 0, -4);
            if (!class_exists($className)) {
                continue;
            }

            try {
                $ref = new \ReflectionClass($className);
                if ($ref->isAbstract() || !$ref->isSubclassOf(\Illuminate\Database\Eloquent\Model::class)) {
                    continue;
                }

                /** @var \Illuminate\Database\Eloquent\Model $instance */
                $instance = new $className();
                $table = $instance->getTable();
                $pk = $instance->getKeyName();

                // Get row count safely
                $rowCount = 0;
                try {
                    $rowCount = \Illuminate\Support\Facades\DB::table($table)->count();
                } catch (\Throwable $t) {}

                // Get columns
                $columns = [];
                $knownIndexes = [];
                $hasSchemaColumns = false;

                try {
                    if (method_exists(\Illuminate\Support\Facades\Schema::class, 'getColumns')) {
                        $rawCols = \Illuminate\Support\Facades\Schema::getColumns($table);
                        foreach ($rawCols as $col) {
                            $colName = $col['name'] ?? '';
                            if (!$colName) continue;
                            $columns[] = [
                                'name' => $colName,
                                'type' => $col['type_name'] ?? $col['type'] ?? 'string',
                                'nullable' => (bool)($col['nullable'] ?? false),
                                'default' => isset($col['default']) ? (string)$col['default'] : '',
                                'isPrimary' => $colName === $pk,
                                'isForeign' => str_ends_with($colName, '_id') && $colName !== $pk,
                            ];
                        }
                        $hasSchemaColumns = count($columns) > 0;
                    }
                } catch (\Throwable $t) {}

                if (!$hasSchemaColumns) {
                    try {
                        $listing = \Illuminate\Support\Facades\Schema::getColumnListing($table);
                        $casts = $instance->getCasts();
                        foreach ($listing as $colName) {
                            $columns[] = [
                                'name' => $colName,
                                'type' => $casts[$colName] ?? ($colName === $pk ? 'bigint' : (str_ends_with($colName, '_id') ? 'bigint' : (str_ends_with($colName, '_at') ? 'timestamp' : 'string'))),
                                'nullable' => $colName !== $pk,
                                'default' => '',
                                'isPrimary' => $colName === $pk,
                                'isForeign' => str_ends_with($colName, '_id') && $colName !== $pk,
                            ];
                        }
                    } catch (\Throwable $t) {}
                }

                // Get indexes
                try {
                    if (method_exists(\Illuminate\Support\Facades\Schema::class, 'getIndexes')) {
                        $rawIndexes = \Illuminate\Support\Facades\Schema::getIndexes($table);
                        foreach ($rawIndexes as $idx) {
                            $idxName = $idx['name'] ?? '';
                            $idxCols = $idx['columns'] ?? [];
                            $isUnique = (bool)($idx['unique'] ?? false);
                            $isPrimary = (bool)($idx['primary'] ?? false);
                            $knownIndexes[] = [
                                'name' => $idxName,
                                'columns' => $idxCols,
                                'unique' => $isUnique,
                                'primary' => $isPrimary,
                            ];
                        }
                    }
                } catch (\Throwable $t) {}

                // Inspect relations
                $modelRelations = [];
                $methods = $ref->getMethods(\ReflectionMethod::IS_PUBLIC);
                foreach ($methods as $method) {
                    $mName = $method->getName();
                    if (isset($baseModelLookup[$mName]) || $method->getNumberOfRequiredParameters() > 0) {
                        continue;
                    }
                    if (str_starts_with($mName, 'get') || str_starts_with($mName, 'set') || str_starts_with($mName, 'scope') || str_starts_with($mName, 'is')) {
                        continue;
                    }

                    try {
                        $relObj = $method->invoke($instance);
                        if ($relObj instanceof \Illuminate\Database\Eloquent\Relations\Relation) {
                            $relShortName = (new \ReflectionClass($relObj))->getShortName();
                            $targetModel = get_class($relObj->getRelated());
                            $targetTable = $relObj->getRelated()->getTable();
                            $foreignKey = '';
                            $localKey = '';
                            $pivotTable = '';

                            if (method_exists($relObj, 'getForeignKeyName')) {
                                $foreignKey = (string)$relObj->getForeignKeyName();
                            }
                            if (method_exists($relObj, 'getOwnerKeyName')) {
                                $localKey = (string)$relObj->getOwnerKeyName();
                            } elseif (method_exists($relObj, 'getLocalKeyName')) {
                                $localKey = (string)$relObj->getLocalKeyName();
                            }
                            if ($relShortName === 'BelongsToMany') {
                                if (method_exists($relObj, 'getTable')) {
                                    $pivotTable = (string)$relObj->getTable();
                                }
                                if (method_exists($relObj, 'getForeignPivotKeyName')) {
                                    $foreignKey = (string)$relObj->getForeignPivotKeyName();
                                }
                                if (method_exists($relObj, 'getRelatedPivotKeyName')) {
                                    $localKey = (string)$relObj->getRelatedPivotKeyName();
                                }
                            }

                            $relData = [
                                'name' => $mName,
                                'type' => $relShortName,
                                'sourceModel' => $className,
                                'sourceTable' => $table,
                                'targetModel' => $targetModel,
                                'targetTable' => $targetTable,
                                'foreignKey' => $foreignKey,
                                'localKey' => $localKey,
                                'pivotTable' => $pivotTable,
                            ];
                            $modelRelations[] = $relData;
                            $allRelations[] = $relData;
                        }
                    } catch (\Throwable $t) {
                        // ignore relation invocation failures
                    }
                }

                // Check for missing index advisories on columns ending in _id
                $indexedColumns = [];
                foreach ($knownIndexes as $idx) {
                    foreach ($idx['columns'] as $c) {
                        $indexedColumns[$c] = true;
                    }
                }

                foreach ($columns as $c) {
                    $cName = $c['name'];
                    if (str_ends_with($cName, '_id') && $cName !== $pk && !isset($indexedColumns[$cName])) {
                        $advisories[] = [
                            'table' => $table,
                            'column' => $cName,
                            'issue' => "Unindexed Foreign Key Column",
                            'severity' => "warning",
                            'suggestion' => "Column '{$cName}' in '{$table}' represents a relationship key but lacks an index, leading to slow JOIN queries.",
                            'migrationSql' => "\$table->index('{$cName}');",
                        ];
                    }
                }

                $tables[] = [
                    'modelClass' => $className,
                    'modelName' => substr($file, 0, -4),
                    'table' => $table,
                    'primaryKey' => $pk,
                    'columns' => $columns,
                    'indexes' => $knownIndexes,
                    'rowCount' => $rowCount,
                    'relations' => $modelRelations,
                ];
            } catch (\Throwable $t) {
                // Ignore model analysis errors
            }
        }
    }

    echo json_encode([
        'tables' => $tables,
        'relations' => $allRelations,
        'advisories' => $advisories,
    ], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
}

function handleExplainQuery(array $payload): void {
    $sql = trim($payload['query'] ?? '');
    if ($sql === '') {
        echo json_encode(['error' => 'Query string is required for explain']);
        return;
    }

    $bottlenecks = [];
    $score = 100;
    $rawRows = [];

    try {
        $driver = \Illuminate\Support\Facades\DB::connection()->getDriverName();
        $explainCmd = ($driver === 'sqlite') ? ('EXPLAIN QUERY PLAN ' . $sql) : ('EXPLAIN ' . $sql);
        
        $results = \Illuminate\Support\Facades\DB::select($explainCmd);
        foreach ($results as $r) {
            $rawRows[] = (array)$r;
        }

        foreach ($rawRows as $row) {
            // MySQL analysis
            $type = strtoupper(trim((string)($row['type'] ?? '')));
            $table = (string)($row['table'] ?? '');
            $extra = (string)($row['Extra'] ?? $row['extra'] ?? '');
            $key = $row['key'] ?? null;
            $detail = (string)($row['detail'] ?? '');

            if ($type === 'ALL') {
                $score -= 30;
                $bottlenecks[] = [
                    'type' => 'full_table_scan',
                    'table' => $table,
                    'details' => "Full table scan on '{$table}' (type: ALL). Every row is inspected.",
                    'severity' => 'critical',
                    'suggestion' => "Add an index covering the WHERE or JOIN conditions for table '{$table}'.",
                ];
            }
            if (stripos($extra, 'Using filesort') !== false) {
                $score -= 15;
                $bottlenecks[] = [
                    'type' => 'filesort',
                    'table' => $table,
                    'details' => "Query performs a filesort on disk or memory to sort records.",
                    'severity' => 'warning',
                    'suggestion' => "Create a composite index matching the ORDER BY and WHERE clauses.",
                ];
            }
            if (stripos($extra, 'Using temporary') !== false) {
                $score -= 15;
                $bottlenecks[] = [
                    'type' => 'temporary_table',
                    'table' => $table,
                    'details' => "Database created an internal temporary table to execute the query.",
                    'severity' => 'warning',
                    'suggestion' => "Optimize GROUP BY and ORDER BY columns to share an existing index prefix.",
                ];
            }
            if ($key === null && $type !== '' && $type !== 'CONST') {
                $score -= 15;
                $bottlenecks[] = [
                    'type' => 'no_index_used',
                    'table' => $table,
                    'details' => "No index was selected by query optimizer for table '{$table}'.",
                    'severity' => 'warning',
                    'suggestion' => "Check foreign key or search filters.",
                ];
            }

            // SQLite analysis
            if ($driver === 'sqlite' && stripos($detail, 'SCAN TABLE') !== false) {
                $score -= 25;
                $bottlenecks[] = [
                    'type' => 'full_table_scan',
                    'table' => $table ?: 'sqlite_table',
                    'details' => $detail,
                    'severity' => 'critical',
                    'suggestion' => "Add an index to avoid scanning the entire SQLite table.",
                ];
            }
        }

        if ($score < 10) $score = 10;

        $summary = "Optimal query plan with index access.";
        if (count($bottlenecks) > 0) {
            $summary = count($bottlenecks) . " performance bottlenecks detected.";
        }

        echo json_encode([
            'query' => $sql,
            'raw' => $rawRows,
            'bottlenecks' => $bottlenecks,
            'score' => $score,
            'summary' => $summary,
        ], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
    } catch (\Throwable $e) {
        echo json_encode(['error' => 'Explain failed: ' . $e->getMessage()]);
    }
}

