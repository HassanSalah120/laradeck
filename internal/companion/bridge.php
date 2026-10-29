<?php
// Only invoked by the trusted-project coordinator. JSON input never becomes PHP.
ob_start();
try {
    [$script, $root, $operation, $request] = $argv;
    $input = json_decode(file_get_contents($request), true, 32, JSON_THROW_ON_ERROR);
    require $root.'/vendor/autoload.php';
    $app = require $root.'/bootstrap/app.php';
    $app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
    if (!$app->environment(['local', 'testing'])) {
        throw new RuntimeException('Companion runtime tools require APP_ENV=local or testing. No action was performed.');
    }
    function localConnection($db): void {
        $c = $db->getConfig();
        if (($c['driver'] ?? '') === 'sqlite') { if(str_starts_with((string)($c['database']??''),'\\\\')||str_starts_with((string)($c['database']??''),'//'))throw new RuntimeException('Network SQLite paths are unsupported'); return; }
        $hosts = (array) ($c['host'] ?? '');
        foreach (['read', 'write'] as $side) {
            if (isset($c[$side]['host'])) $hosts = array_merge($hosts, (array) $c[$side]['host']);
        }
        foreach ($hosts as $host) {
            if (!in_array(strtolower((string)$host), ['127.0.0.1', 'localhost', '::1'], true)) {
                throw new RuntimeException('Runtime database tools only support loopback database hosts.');
            }
        }
    }
    function identity($db): array {
        localConnection($db);
        $c = $db->getConfig();
        return ['connection'=>$db->getName(), 'driver'=>$db->getDriverName(), 'host'=>$c['host'] ?? 'file', 'port'=>$c['port'] ?? null, 'database'=>$db->getDatabaseName(), 'prefix'=>$db->getTablePrefix()];
    }
    function requireConfirmation(array $input, string $target): void {
        if (($input['confirmation'] ?? '') !== $target) throw new RuntimeException('Type the displayed target exactly to confirm this operation.');
    }
    function boundedValue($value): string {
        if (is_array($value)) return substr(json_encode($value, JSON_INVALID_UTF8_SUBSTITUTE | JSON_PARTIAL_OUTPUT_ON_ERROR), 0, 4000);
        if (is_object($value)) return '[object '.get_class($value).']';
        return substr((string)$value, 0, 4000);
    }
    function localRedis($connection): void {
        $config = config('database.redis.'.($connection->getName() ?: 'default'), []);
        if (!empty($config['url'])) $config['host']=parse_url($config['url'],PHP_URL_HOST);
        $host = strtolower((string)($config['host'] ?? ''));
        $isLocal = in_array($host, ['127.0.0.1','localhost','::1','host.docker.internal'], true) || preg_match('/^172\.(1[6-9]|2[0-9]|3[0-1])\./', $host);
        if (!$isLocal) throw new RuntimeException('Redis inspection requires a loopback or local Docker bridge host.');
    }
    function localRedisName(string $name): void {
        $config=config('database.redis.'.$name,[]);$host=strtolower((string)($config['host']??''));
        if(!empty($config['url']))$host=strtolower((string)parse_url($config['url'],PHP_URL_HOST));
        $isLocal = in_array($host,['127.0.0.1','localhost','::1','host.docker.internal'],true) || preg_match('/^172\.(1[6-9]|2[0-9]|3[0-1])\./', $host);
        if(!$isLocal)throw new RuntimeException('Redis requires a named loopback or local Docker bridge connection; cluster/remote stores are unsupported.');
    }
    function redisEntries($store,$connection,array $input): array {
        $isCluster = $connection instanceof Illuminate\Redis\Connections\PhpRedisClusterConnection;
        if(!($connection instanceof Illuminate\Redis\Connections\PhpRedisConnection) && !$isCluster) {
            throw new RuntimeException('Bounded Redis browsing requires PhpRedis or PhpRedisCluster');
        }
        $prefix=$store->getPrefix();$global=$connection->client()->getOption(Redis::OPT_PREFIX)?:'';$physical=$global.$prefix;
        if($physical==='')throw new RuntimeException('Redis inspection requires a project-specific prefix');
        $cursor=$input['cursor']??0;
        if((string)$cursor==='0')$cursor=version_compare(phpversion('redis'),'6.1.0','>=')?null:'0';
        $match=str_replace(['\\','*','?','[',']'],['\\\\','\\*','\\?','\\[','\\]'],$physical).'*';
        $scan=$connection->scan($cursor,['match'=>$match,'count'=>50]);$entries=[];
        foreach(array_slice($scan[1]??[],0,200) as $raw){
            if(!str_starts_with($raw,$physical))continue;$key=substr($raw,strlen($physical));
            if(($input['search']??'')!==''&&stripos($key,$input['search'])===false)continue;
            $ttl=$connection->ttl($prefix.$key);$value='[masked]';
            if(($input['reveal']??false)&&($input['key']??'')===$key){
                // Read raw bytes; do not let Laravel unserialize arbitrary objects.
                $value=boundedValue($connection->get($prefix.$key));
            }
            $entries[]=['key'=>$key,'ttl'=>$ttl,'value'=>$value];
        }
        return ['entries'=>$entries,'next'=>(string)($scan[0]??'0'),'note'=>'SCAN pages can be empty while a nonzero cursor remains. Redis expiry is reported by TTL.'];
    }
    function fileEntries(string $base, array $input, bool $cache): array {
        if (!is_dir($base)) return ['entries'=>[], 'next'=>0, 'note'=>'Store directory does not exist yet'];
        $base = realpath($base); $cursor=max(0,(int)($input['cursor'] ?? 0)); $position=0; $entries=[]; $visited=0; $truncated=false;
        $iterator=new RecursiveIteratorIterator(new RecursiveDirectoryIterator($base, FilesystemIterator::SKIP_DOTS));
        foreach ($iterator as $file) {
            if (++$visited > 10000) { $truncated = true; break; }
            if (!$file->isFile() || $file->isLink() || $file->getFilename()==='.gitignore') continue;
            $path=$file->getRealPath(); if (!$path || !str_starts_with($path,$base.DIRECTORY_SEPARATOR)) continue;
            $key=str_replace(DIRECTORY_SEPARATOR,'/',substr($path,strlen($base)+1));
            if (($input['search'] ?? '') !== '' && stripos($key,$input['search'])===false) continue;
            if ($position++ < $cursor) continue;
            $ttl=$cache ? max(0,(int)file_get_contents($path,false,null,0,10)-time()) : max(0,$file->getMTime()+config('session.lifetime',120)*60-time());
            $value='[masked]';
            if (($input['reveal'] ?? false) && $key===($input['key'] ?? '')) {
                // Never unserialize application objects while inspecting stores.
                $value=substr(file_get_contents($path,false,null,$cache?10:0,4000),0,4000);
            }
            $entries[]=['key'=>$key,'ttl'=>$ttl,'bytes'=>$file->getSize(),'value'=>$value];
            if (count($entries)>=50) break;
        }
        $note = $cache ? 'File cache keys are one-way hashed; displayed identifiers are relative file paths.' : 'Session contents are masked. File entries use modification-time expiry.';
        if ($truncated) {
            $note .= ' Reached 10,000 files scan limit; directory traversal truncated to prevent memory exhaustion.';
        }
        return ['entries'=>$entries,'next'=>count($entries)===50?$cursor+50:0,'note'=>$note,'truncated'=>$truncated,'scanned'=>$visited];
    }
    function deleteFileEntry(string $base, string $key): void {
        $base=realpath($base); $path=realpath($base.DIRECTORY_SEPARATOR.$key);
        if (!$base || !$path || !str_starts_with($path,$base.DIRECTORY_SEPARATOR) || !is_file($path) || is_link($path)) throw new RuntimeException('Invalid store entry');
        if (!unlink($path)) throw new RuntimeException('Unable to delete entry');
    }
    $data=[];
    switch ($operation) {
    case 'runtime':
        $db=$app['db']->connection();
        $data=['environment'=>$app->environment(),'php'=>PHP_VERSION,'missingExtensions'=>[], 'configCached'=>$app->configurationIsCached(), 'database'=>identity($db),'cache'=>config('cache.default'),'session'=>config('session.driver'),'queue'=>config('queue.default'),'extensions'=>get_loaded_extensions()];
        $requirements=json_decode(file_get_contents($root.'/composer.json'),true)['require'] ?? [];
        foreach($requirements as $name=>$constraint) if(str_starts_with($name,'ext-')&&!extension_loaded(substr($name,4))) $data['missingExtensions'][]=substr($name,4);
        break;
    case 'integration-url':
        $kind=$input['kind']??'';if(!in_array($kind,['telescope','pulse'],true))throw new RuntimeException('Unsupported integration');
        $path=trim((string)config($kind.'.path',$kind),'/');
        if(str_contains($path,':')||str_contains($path,'?')||str_contains($path,'#'))throw new RuntimeException('Dashboard path must be local');
        $registered=false;foreach($app['router']->getRoutes() as $route){if($route->uri()===$path){$registered=true;break;}}
        if(!$registered)throw new RuntimeException('Dashboard route is not registered. Complete provider setup and review cached routes.');
        $data=['path'=>'/'.$path];break;
    case 'routes':
        $routes=[];
        foreach($app['router']->getRoutes() as $route) $routes[]=['uri'=>$route->uri(),'methods'=>$route->methods(),'name'=>$route->getName(),'action'=>$route->getActionName(),'middleware'=>$route->gatherMiddleware(),'parameters'=>$route->parameterNames()];
        $data=['routes'=>$routes]; break;
    case 'migrations':
        $db=$app['db']->connection(); $target=identity($db); $migrator=$app['migrator'];
        $repo=$migrator->getRepository(); $ran=$repo->repositoryExists()?$repo->getRan():[];
        $files=$migrator->getMigrationFiles(array_merge([$app->databasePath('migrations')],$migrator->paths()));
        $rows=[]; foreach($files as $name=>$file) $rows[]=['name'=>$name,'status'=>in_array($name,$ran,true)?'applied':'pending'];
        $schema=$db->getSchemaBuilder(); $tables=[];
        $allTables=$schema->getTables();
        $tableFilter = trim((string)($input['tableFilter'] ?? ''));
        if ($tableFilter !== '') {
            $allTables = array_values(array_filter($allTables, function($t) use ($tableFilter) {
                return stripos($t['name'] ?? '', $tableFilter) !== false;
            }));
        }
        $limit = max(50, min((int)($input['limit'] ?? 1000), 5000));
        $offset = max(0, (int)($input['offset'] ?? 0));
        $pagedTables = array_slice($allTables, $offset, $limit);
        $schemaTruncated = ($offset + count($pagedTables)) < count($allTables);
        foreach($pagedTables as $table) {
            $name=$table['name'];if($db->getDriverName()==='pgsql'&&!empty($table['schema']))$name=$table['schema'].'.'.$name; $tables[]=['name'=>$name,'columns'=>$schema->getColumns($name),'indexes'=>$schema->getIndexes($name),'foreignKeys'=>$schema->getForeignKeys($name)];
        }
        $data=['target'=>$target,'migrations'=>$rows,'schema'=>$tables,'totalTables'=>count($allTables),'schemaLimit'=>$limit,'schemaOffset'=>$offset,'schemaTruncated'=>$schemaTruncated]; break;
    case 'migration-preview':
    case 'migrate':
    case 'rollback':
        $db=$app['db']->connection();$target=identity($db);$label=$target['connection'].' / '.$target['database'];
        if(!class_exists(Illuminate\Database\Events\ConnectionEstablished::class)||!method_exists($db,'beforeExecuting'))throw new RuntimeException('Migration execution needs Laravel connection events and beforeExecuting support.');
        $guard=function($connection)use($target){$connection->beforeExecuting(function($query,$bindings,$actual)use($target){if(identity($actual)!==$target)throw new RuntimeException('Migration attempted a different database connection than the confirmed target. Use a separately reviewed multi-database migration workflow.');});};
        foreach($app['db']->getConnections() as $connection)$guard($connection);
        $app['events']->listen(Illuminate\Database\Events\ConnectionEstablished::class,function($event)use($guard){$guard($event->connection);});

        if($operation!=='migration-preview') requireConfirmation($input,$label);
        $output=new Symfony\Component\Console\Output\BufferedOutput;
        $args=['--no-interaction'=>true];
        if($operation==='migration-preview') $args['--pretend']=true;
        if($operation==='rollback')$args['--step']=1;
        if($operation==='migration-preview'){ $code=0; $db->pretend(function()use($args,$output,&$code){$code=Illuminate\Support\Facades\Artisan::call('migrate',$args,$output);}); }else{ $code=Illuminate\Support\Facades\Artisan::call($operation==='rollback'?'migrate:rollback':'migrate',$args,$output); }
        $data=['target'=>$target,'exitCode'=>$code,'output'=>$output->fetch(),'note'=>'Migration PHP is application code; even --pretend may execute non-SQL side effects.'];
        if($code!==0)throw new RuntimeException('Migration command failed: '.$data['output']);break;
    case 'cache':
    case 'cache-delete':
        $driver=config('cache.default');$cfg=config('cache.stores.'.$driver,[]);$store=$app['cache']->store($driver)->getStore();$key=(string)($input['key'] ?? '');
        if($operation==='cache-delete'){if($key==='')throw new RuntimeException('Select an entry');requireConfirmation($input,$key);}
        if(($cfg['driver'] ?? $driver)==='file'){
            $base=$cfg['path'] ?? storage_path('framework/cache/data');
            if($operation==='cache-delete'){deleteFileEntry($base,$key);$data=['deleted'=>true];}else{$data=fileEntries($base,$input,true);}
        }elseif(($cfg['driver'] ?? $driver)==='database'){
            $db=$store->getConnection(); localConnection($db);$prefix=$store->getPrefix();$table=$cfg['table'] ?? 'cache';
            if($operation==='cache-delete'){$store->forget($key);$data=['deleted'=>true];}else{
                $q=$db->table($table)->where('expiration','>',time())->orderBy('key');
                $q->whereRaw($db->getQueryGrammar()->wrap('key')." LIKE ? ESCAPE '!'",[str_replace(['!','%','_'],['!!','!%','!_'],$prefix).'%']);
                if(($input['search'] ?? '')!=='')$q->whereRaw($db->getQueryGrammar()->wrap('key')." LIKE ? ESCAPE '!'",['%'.str_replace(['!','%','_'],['!!','!%','!_'],$input['search']).'%']);
                $cursor=max(0,(int)($input['cursor']??0));$entries=[];
                foreach($q->offset($cursor)->limit(50)->get(['key','value','expiration']) as $row){$short=substr($row->key,strlen($prefix));$entries[]=['key'=>$short,'ttl'=>max(0,$row->expiration-time()),'bytes'=>strlen($row->value),'value'=>($input['reveal']??false)&&$key===$short?boundedValue($row->value):'[masked]'];}
                $data=['entries'=>$entries,'next'=>count($entries)===50?$cursor+50:0];
            }
        }elseif(($cfg['driver'] ?? $driver)==='redis'){
            localRedisName($cfg['connection'] ?? 'cache');$connection=$store->connection();localRedis($connection);$prefix=$store->getPrefix();
            if($operation==='cache-delete'){$store->forget($key);$data=['deleted'=>true];}else{
                $data=redisEntries($store,$connection,$input);
            }
        }else{throw new RuntimeException('Cache inspection supports file, database and PhpRedis stores; current driver: '.$driver);}
        $data['driver']=$driver;break;
    case 'sessions':
    case 'session-delete':
        $driver=config('session.driver');$key=(string)($input['key']??'');if($operation==='session-delete'){if($key==='')throw new RuntimeException('Select a session');requireConfirmation($input,$key);}
        if($driver==='file'){
            $base=config('session.files');if($operation==='session-delete'){deleteFileEntry($base,$key);$data=['deleted'=>true];}else{$data=fileEntries($base,$input,false);}
        }elseif($driver==='database'){
            $db=$app['db']->connection(config('session.connection'));localConnection($db);$q=$db->table(config('session.table','sessions'));
            if($operation==='session-delete'){if(!preg_match('/^[a-zA-Z0-9_-]{1,200}$/',$key))throw new RuntimeException('Invalid session identifier');$data=['deleted'=>$q->where('id',$key)->delete()>0];}else{
                $cursor=max(0,(int)($input['cursor']??0));if(($input['search']??'')!=='')$q->whereRaw($db->getQueryGrammar()->wrap('id')." LIKE ? ESCAPE '!'",['%'.str_replace(['!','%','_'],['!!','!%','!_'],$input['search']).'%']);$entries=[];
                foreach($q->orderBy('id')->offset($cursor)->limit(50)->get(['id','last_activity','payload']) as $row)$entries[]=['key'=>$row->id,'ttl'=>max(0,$row->last_activity+config('session.lifetime',120)*60-time()),'value'=>($input['reveal']??false)&&$key===$row->id?substr(base64_decode($row->payload),0,4000):'[masked]'];
                $data=['entries'=>$entries,'next'=>count($entries)===50?$cursor+50:0];
            }
        }elseif($driver==='redis'){
            // The session Redis handler uses a separate cache store in Laravel.
            $handler=$app['session']->driver()->getHandler();
            if(!method_exists($handler,'getCache'))throw new RuntimeException('The configured Redis session handler does not expose a cache store');
            $store=$handler->getCache()->getStore();localRedisName(config('session.connection') ?: 'default');$connection=$store->connection();localRedis($connection);$prefix=$store->getPrefix();
            if($operation==='session-delete'){$store->forget($key);$data=['deleted'=>true];}else{
                $data=redisEntries($store,$connection,$input);
            }
        }else throw new RuntimeException('Session inspection supports file, database and PhpRedis stores; current driver: '.$driver);
        $data['driver']=$driver;break;
    case 'telescope':
        if(!class_exists(Laravel\Telescope\Telescope::class))throw new RuntimeException('Telescope is not installed; use Integrations');
        $connection=config('telescope.storage.database.connection') ?: config('database.default');$db=$app['db']->connection($connection);localConnection($db);
        $batch=(string)($input['batch']??'');$q=$db->table('telescope_entries');
        if($batch!==''){$q->where('batch_id',$batch)->orderBy('sequence');}else{$q->where('type','request')->orderByDesc('sequence');}
        $data=['entries'=>[],'note'=>'Only identical Telescope batch IDs establish association; cross-process jobs may belong to another batch. Payloads and bindings are not exposed.'];
        foreach($q->limit(100)->get(['uuid','batch_id','type','created_at','content']) as $entry){
            $content=json_decode($entry->content,true)??[];
            $summary=$entry->type;
            foreach(['uri','name','class','view'] as $field){if(isset($content[$field])&&is_scalar($content[$field])){$summary=substr((string)$content[$field],0,300);if($field==='uri')$summary=parse_url($summary,PHP_URL_PATH)?:'[request]';break;}}
            $data['entries'][]=['id'=>$entry->uuid,'batch'=>$entry->batch_id,'type'=>$entry->type,'time'=>$entry->created_at,'summary'=>$summary,'duration'=>$content['duration']??$content['time']??null];
        }break;
    case 'snapshot-create':
    case 'snapshot-restore':
        $db=$app['db']->connection();$target=identity($db);$targetHash=hash('sha256',json_encode($target));$path=$input['path'];
        if($operation==='snapshot-restore'){
            requireConfirmation($input,$target['connection'].' / '.$target['database']);
            if(($input['targetHash']??'')!==$targetHash)throw new RuntimeException('Snapshot belongs to a different connection or database');
            if(!is_file($path))throw new RuntimeException('Snapshot data is missing');
        }
        $driver=$db->getDriverName();$creating=$operation==='snapshot-create';
        if($driver==='sqlite'){
            if($db->getDatabaseName()===':memory:')throw new RuntimeException('Snapshots require an on-disk SQLite database');
            if(!class_exists(SQLite3::class))throw new RuntimeException('Enable the sqlite3 PHP extension for consistent SQLite backup');
            $source=new SQLite3($creating?$db->getDatabaseName():$path,SQLITE3_OPEN_READONLY);
            $destination=new SQLite3($creating?$path:$db->getDatabaseName());$destination->busyTimeout(5000);
            if(!$source->backup($destination))throw new RuntimeException('SQLite backup failed; another connection may hold a lock');$source->close();$destination->close();
        }elseif(in_array($driver,['mysql','mariadb','pgsql'],true)){
            $tool=$input['tool']??'';$allowed=$driver==='pgsql'?($creating?'pg_dump.exe':'pg_restore.exe'):($creating?'mysqldump.exe':'mysql.exe');
            if(strtolower(basename(str_replace('\\','/',$tool)))!==$allowed || !is_file($tool))throw new RuntimeException('Select the native '.$allowed.' client');
            $cfg=$db->getConfig();$host=(string)(is_array($cfg['host']??null)?$cfg['host'][0]:($cfg['host']??'127.0.0.1'));$user=(string)($cfg['username']??'');$port=(string)($cfg['port']??($driver==='pgsql'?5432:3306));$name=$db->getDatabaseName();if($name===''||($driver==='pgsql'&&(str_contains($name,'=')||str_contains($name,'://'))))throw new RuntimeException('Native snapshots require a plain database name');
            $env=getenv();$args=[$tool];
            if($driver==='pgsql'){
                $env['PGPASSWORD']=$cfg['password']??'';$env['PGCONNECT_TIMEOUT']='5';$args=array_merge($args,['--host='.$host,'--port='.$port,'--username='.$user]);
                $args=array_merge($args,$creating?['--format=custom','--file='.$path,$name]:['--clean','--if-exists','--no-owner','--exit-on-error','--dbname='.$name,$path]);
            }else{
                $env['MYSQL_PWD']=$cfg['password']??'';$args=array_merge($args,['--host='.$host,'--port='.$port,'--user='.$user,'--connect-timeout=5']);
                $args=array_merge($args,$creating?['--single-transaction','--routines','--triggers','--events','--hex-blob','--result-file='.$path,'--',$name]:['--database='.$name]);
            }
            $out=tempnam(sys_get_temp_dir(),'ld-native-');$err=tempnam(sys_get_temp_dir(),'ld-native-');
            try{$descriptors=[0=>$creating?['pipe','r']:['file',$path,'r'],1=>['file',$out,'w'],2=>['file',$err,'w']];$process=proc_open($args,$descriptors,$pipes,$root,$env,['bypass_shell'=>true]);if(!is_resource($process))throw new RuntimeException('Unable to launch native database client');if($creating)fclose($pipes[0]);$code=proc_close($process);if($code!==0)throw new RuntimeException('Native database operation failed (exit '.$code.'). Check client compatibility, permissions and active connections.');}finally{@unlink($out);@unlink($err);}
        }else throw new RuntimeException('Snapshots support SQLite, MySQL/MariaDB and PostgreSQL');
        $data=['target'=>$target,'targetHash'=>$targetHash,'driver'=>$driver,'bytes'=>filesize($path),'completed'=>true];break;
    default: throw new RuntimeException('Unsupported companion operation');
    }
    ob_end_clean();echo json_encode(['data'=>$data],JSON_INVALID_UTF8_SUBSTITUTE|JSON_PARTIAL_OUTPUT_ON_ERROR);
} catch (Throwable $e) {
    while(ob_get_level())ob_end_clean();
    // Do not return SQL bindings, connection strings, stack frames or credentials.
    $message=$e instanceof Illuminate\Database\QueryException ? 'Database inspection failed. Check the selected connection, schema and permissions.' : $e->getMessage();
    echo json_encode(['error'=>substr($message,0,1500)],JSON_INVALID_UTF8_SUBSTITUTE);
}
