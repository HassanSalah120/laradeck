export namespace actions {
	
	export class ActionRunResult {
	    success: boolean;
	    exitCode: number;
	    output: string[];
	    durationMs: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ActionRunResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.exitCode = source["exitCode"];
	        this.output = source["output"];
	        this.durationMs = source["durationMs"];
	        this.error = source["error"];
	    }
	}
	export class ParameterInfo {
	    name: string;
	    type: string;
	    hasDefault: boolean;
	    defaultValue?: any;
	
	    static createFrom(source: any = {}) {
	        return new ParameterInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.hasDefault = source["hasDefault"];
	        this.defaultValue = source["defaultValue"];
	    }
	}
	export class EventIntrospection {
	    eventClass: string;
	    parameters: ParameterInfo[];
	    listeners: string[];
	    dispatchedFrom: string[];
	    hasListeners: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EventIntrospection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.eventClass = source["eventClass"];
	        this.parameters = this.convertValues(source["parameters"], ParameterInfo);
	        this.listeners = source["listeners"];
	        this.dispatchedFrom = source["dispatchedFrom"];
	        this.hasListeners = source["hasListeners"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EventListenerItem {
	    event: string;
	    listeners: string[];
	
	    static createFrom(source: any = {}) {
	        return new EventListenerItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.event = source["event"];
	        this.listeners = source["listeners"];
	    }
	}
	export class FailedJobItem {
	    id: string;
	    uuid: string;
	    connection: string;
	    queue: string;
	    class: string;
	    failedAt: string;
	    exception: string;
	
	    static createFrom(source: any = {}) {
	        return new FailedJobItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.uuid = source["uuid"];
	        this.connection = source["connection"];
	        this.queue = source["queue"];
	        this.class = source["class"];
	        this.failedAt = source["failedAt"];
	        this.exception = source["exception"];
	    }
	}
	export class JobIntrospection {
	    jobClass: string;
	    parameters: ParameterInfo[];
	    summary: string;
	    schedule?: string;
	    queue: string;
	    connection: string;
	    tries: number;
	    timeout: number;
	    dispatchedFrom: string[];
	
	    static createFrom(source: any = {}) {
	        return new JobIntrospection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jobClass = source["jobClass"];
	        this.parameters = this.convertValues(source["parameters"], ParameterInfo);
	        this.summary = source["summary"];
	        this.schedule = source["schedule"];
	        this.queue = source["queue"];
	        this.connection = source["connection"];
	        this.tries = source["tries"];
	        this.timeout = source["timeout"];
	        this.dispatchedFrom = source["dispatchedFrom"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class JobRunResult {
	    success: boolean;
	    output: string[];
	    durationMs: number;
	    queries: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new JobRunResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.output = source["output"];
	        this.durationMs = source["durationMs"];
	        this.queries = source["queries"];
	        this.error = source["error"];
	    }
	}
	
	export class QueueWorkerStatus {
	    running: boolean;
	    pid: number;
	    queue: string;
	    output: string[];
	    startedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new QueueWorkerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.pid = source["pid"];
	        this.queue = source["queue"];
	        this.output = source["output"];
	        this.startedAt = source["startedAt"];
	    }
	}
	export class ScheduledTaskItem {
	    expression: string;
	    command: string;
	    interval: string;
	    description: string;
	    nextDue: string;
	    hasMutex: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScheduledTaskItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.expression = source["expression"];
	        this.command = source["command"];
	        this.interval = source["interval"];
	        this.description = source["description"];
	        this.nextDue = source["nextDue"];
	        this.hasMutex = source["hasMutex"];
	    }
	}

}

export namespace activity {
	
	export class TraceNode {
	    id: string;
	    type: string;
	    label: string;
	    detail?: string;
	    startMs: number;
	    durationMs: number;
	    isFramework: boolean;
	    children?: TraceNode[];
	
	    static createFrom(source: any = {}) {
	        return new TraceNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.label = source["label"];
	        this.detail = source["detail"];
	        this.startMs = source["startMs"];
	        this.durationMs = source["durationMs"];
	        this.isFramework = source["isFramework"];
	        this.children = this.convertValues(source["children"], TraceNode);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ActivityTrace {
	    id: string;
	    projectId: string;
	    type: string;
	    summary: string;
	    status: number;
	    totalTimeMs: number;
	    memoryMb: number;
	    timestamp: string;
	    rootNode: TraceNode;
	
	    static createFrom(source: any = {}) {
	        return new ActivityTrace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.type = source["type"];
	        this.summary = source["summary"];
	        this.status = source["status"];
	        this.totalTimeMs = source["totalTimeMs"];
	        this.memoryMb = source["memoryMb"];
	        this.timestamp = source["timestamp"];
	        this.rootNode = this.convertValues(source["rootNode"], TraceNode);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace architecture {
	
	export class BoundaryViolation {
	    ruleId: string;
	    severity: string;
	    sourceFile: string;
	    sourceKind: string;
	    targetFile?: string;
	    message: string;
	    explanation: string;
	
	    static createFrom(source: any = {}) {
	        return new BoundaryViolation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ruleId = source["ruleId"];
	        this.severity = source["severity"];
	        this.sourceFile = source["sourceFile"];
	        this.sourceKind = source["sourceKind"];
	        this.targetFile = source["targetFile"];
	        this.message = source["message"];
	        this.explanation = source["explanation"];
	    }
	}
	export class ArchitectureCanvasReport {
	    projectId: string;
	    layers: Record<string, Array<ComponentNode>>;
	    violations: BoundaryViolation[];
	    mermaidCode: string;
	    totalComponents: number;
	
	    static createFrom(source: any = {}) {
	        return new ArchitectureCanvasReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectId = source["projectId"];
	        this.layers = this.convertValues(source["layers"], Array<ComponentNode>, true);
	        this.violations = this.convertValues(source["violations"], BoundaryViolation);
	        this.mermaidCode = source["mermaidCode"];
	        this.totalComponents = source["totalComponents"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ComponentNode {
	    id: string;
	    name: string;
	    layer: number;
	    layerName: string;
	    kind: string;
	    filePath: string;
	    calls: string[];
	
	    static createFrom(source: any = {}) {
	        return new ComponentNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.layer = source["layer"];
	        this.layerName = source["layerName"];
	        this.kind = source["kind"];
	        this.filePath = source["filePath"];
	        this.calls = source["calls"];
	    }
	}

}

export namespace database {
	
	export class ERDAdvisorItem {
	    table: string;
	    column: string;
	    issue: string;
	    severity: string;
	    suggestion: string;
	    migrationSql?: string;
	
	    static createFrom(source: any = {}) {
	        return new ERDAdvisorItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.table = source["table"];
	        this.column = source["column"];
	        this.issue = source["issue"];
	        this.severity = source["severity"];
	        this.suggestion = source["suggestion"];
	        this.migrationSql = source["migrationSql"];
	    }
	}
	export class ERDColumn {
	    name: string;
	    type: string;
	    nullable: boolean;
	    default?: string;
	    isPrimary: boolean;
	    isForeign: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ERDColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.nullable = source["nullable"];
	        this.default = source["default"];
	        this.isPrimary = source["isPrimary"];
	        this.isForeign = source["isForeign"];
	    }
	}
	export class ERDIndex {
	    name: string;
	    columns: string[];
	    unique: boolean;
	    primary: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ERDIndex(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = source["columns"];
	        this.unique = source["unique"];
	        this.primary = source["primary"];
	    }
	}
	export class ERDRelation {
	    name: string;
	    type: string;
	    sourceModel: string;
	    sourceTable: string;
	    targetModel: string;
	    targetTable: string;
	    foreignKey: string;
	    localKey: string;
	    pivotTable?: string;
	
	    static createFrom(source: any = {}) {
	        return new ERDRelation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.sourceModel = source["sourceModel"];
	        this.sourceTable = source["sourceTable"];
	        this.targetModel = source["targetModel"];
	        this.targetTable = source["targetTable"];
	        this.foreignKey = source["foreignKey"];
	        this.localKey = source["localKey"];
	        this.pivotTable = source["pivotTable"];
	    }
	}
	export class ERDTable {
	    modelClass: string;
	    modelName: string;
	    table: string;
	    primaryKey: string;
	    columns: ERDColumn[];
	    indexes: ERDIndex[];
	    rowCount: number;
	    relations: ERDRelation[];
	
	    static createFrom(source: any = {}) {
	        return new ERDTable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modelClass = source["modelClass"];
	        this.modelName = source["modelName"];
	        this.table = source["table"];
	        this.primaryKey = source["primaryKey"];
	        this.columns = this.convertValues(source["columns"], ERDColumn);
	        this.indexes = this.convertValues(source["indexes"], ERDIndex);
	        this.rowCount = source["rowCount"];
	        this.relations = this.convertValues(source["relations"], ERDRelation);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ERDSchemaResult {
	    tables: ERDTable[];
	    relations: ERDRelation[];
	    advisories: ERDAdvisorItem[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ERDSchemaResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tables = this.convertValues(source["tables"], ERDTable);
	        this.relations = this.convertValues(source["relations"], ERDRelation);
	        this.advisories = this.convertValues(source["advisories"], ERDAdvisorItem);
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ExplainBottleneck {
	    type: string;
	    table: string;
	    details: string;
	    severity: string;
	    suggestion: string;
	
	    static createFrom(source: any = {}) {
	        return new ExplainBottleneck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.table = source["table"];
	        this.details = source["details"];
	        this.severity = source["severity"];
	        this.suggestion = source["suggestion"];
	    }
	}
	export class ExplainResult {
	    query: string;
	    raw: any[];
	    bottlenecks: ExplainBottleneck[];
	    score: number;
	    summary: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ExplainResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.raw = source["raw"];
	        this.bottlenecks = this.convertValues(source["bottlenecks"], ExplainBottleneck);
	        this.score = source["score"];
	        this.summary = source["summary"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModelInfo {
	    class: string;
	    name: string;
	    table: string;
	    primaryKey: string;
	    fillable: string[];
	    hidden: string[];
	    casts: Record<string, string>;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.class = source["class"];
	        this.name = source["name"];
	        this.table = source["table"];
	        this.primaryKey = source["primaryKey"];
	        this.fillable = source["fillable"];
	        this.hidden = source["hidden"];
	        this.casts = source["casts"];
	        this.count = source["count"];
	    }
	}
	export class ModelRecordUpdateResult {
	    success: boolean;
	    record?: Record<string, any>;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelRecordUpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.record = source["record"];
	        this.error = source["error"];
	    }
	}
	export class RecordQueryResult {
	    records: any[];
	    total: number;
	    page: number;
	    perPage: number;
	    columns: string[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new RecordQueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.records = source["records"];
	        this.total = source["total"];
	        this.page = source["page"];
	        this.perPage = source["perPage"];
	        this.columns = source["columns"];
	        this.error = source["error"];
	    }
	}

}

export namespace dependencies {
	
	export class Status {
	    id: string;
	    name: string;
	    scope: string;
	    executable: string;
	    repository?: string;
	    wingetId?: string;
	    package?: string;
	    website: string;
	    note: string;
	    state: string;
	    version: string;
	    path: string;
	    message: string;
	    canInstall: boolean;
	    progress: number;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.scope = source["scope"];
	        this.executable = source["executable"];
	        this.repository = source["repository"];
	        this.wingetId = source["wingetId"];
	        this.package = source["package"];
	        this.website = source["website"];
	        this.note = source["note"];
	        this.state = source["state"];
	        this.version = source["version"];
	        this.path = source["path"];
	        this.message = source["message"];
	        this.canInstall = source["canInstall"];
	        this.progress = source["progress"];
	    }
	}

}

export namespace environment {
	
	export class DriftReport {
	    missingKeys: string[];
	    extraKeys: string[];
	    matching: number;
	
	    static createFrom(source: any = {}) {
	        return new DriftReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.missingKeys = source["missingKeys"];
	        this.extraKeys = source["extraKeys"];
	        this.matching = source["matching"];
	    }
	}
	export class EnvEntry {
	    lineNumber: number;
	    type: string;
	    raw: string;
	    key: string;
	    value: string;
	    maskedValue: string;
	    quote: string;
	    comment: string;
	    isSensitive: boolean;
	    initialKey?: string;
	    initialValue?: string;
	    initialQuote?: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lineNumber = source["lineNumber"];
	        this.type = source["type"];
	        this.raw = source["raw"];
	        this.key = source["key"];
	        this.value = source["value"];
	        this.maskedValue = source["maskedValue"];
	        this.quote = source["quote"];
	        this.comment = source["comment"];
	        this.isSensitive = source["isSensitive"];
	        this.initialKey = source["initialKey"];
	        this.initialValue = source["initialValue"];
	        this.initialQuote = source["initialQuote"];
	    }
	}
	export class EnvFile {
	    path: string;
	    fingerprint: string;
	    lineEnding: string;
	    hasTrailingNewline: boolean;
	    entries: EnvEntry[];
	
	    static createFrom(source: any = {}) {
	        return new EnvFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fingerprint = source["fingerprint"];
	        this.lineEnding = source["lineEnding"];
	        this.hasTrailingNewline = source["hasTrailingNewline"];
	        this.entries = this.convertValues(source["entries"], EnvEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class IniChangePreview {
	    iniPath: string;
	    fingerprint: string;
	    key: string;
	    oldValue: string;
	    newValue: string;
	    line: number;
	
	    static createFrom(source: any = {}) {
	        return new IniChangePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.iniPath = source["iniPath"];
	        this.fingerprint = source["fingerprint"];
	        this.key = source["key"];
	        this.oldValue = source["oldValue"];
	        this.newValue = source["newValue"];
	        this.line = source["line"];
	    }
	}
	export class PHPInfo {
	    version: string;
	    loadedIni: string;
	    additionalIniFiles: string[];
	    extensions: string[];
	    keySettings: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new PHPInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.loadedIni = source["loadedIni"];
	        this.additionalIniFiles = source["additionalIniFiles"];
	        this.extensions = source["extensions"];
	        this.keySettings = source["keySettings"];
	    }
	}

}

export namespace flags {
	
	export class FeatureFlag {
	    name: string;
	    definedIn: string;
	    locations: string[];
	    defaultState: string;
	    provider: string;
	
	    static createFrom(source: any = {}) {
	        return new FeatureFlag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.definedIn = source["definedIn"];
	        this.locations = source["locations"];
	        this.defaultState = source["defaultState"];
	        this.provider = source["provider"];
	    }
	}
	export class FlagsReport {
	    hasPennant: boolean;
	    flags: FeatureFlag[];
	    totalFlags: number;
	
	    static createFrom(source: any = {}) {
	        return new FlagsReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasPennant = source["hasPennant"];
	        this.flags = this.convertValues(source["flags"], FeatureFlag);
	        this.totalFlags = source["totalFlags"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace health {
	
	export class ComponentHealth {
	    component: string;
	    status: string;
	    latencyMs?: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ComponentHealth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.component = source["component"];
	        this.status = source["status"];
	        this.latencyMs = source["latencyMs"];
	        this.message = source["message"];
	    }
	}
	export class HeartbeatReport {
	    projectId: string;
	    // Go type: time
	    timestamp: any;
	    healthScore: number;
	    overall: string;
	    components: ComponentHealth[];
	
	    static createFrom(source: any = {}) {
	        return new HeartbeatReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectId = source["projectId"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.healthScore = source["healthScore"];
	        this.overall = source["overall"];
	        this.components = this.convertValues(source["components"], ComponentHealth);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace mail {
	
	export class MailAttachment {
	    partId: string;
	    fileName: string;
	    contentType: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new MailAttachment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.partId = source["partId"];
	        this.fileName = source["fileName"];
	        this.contentType = source["contentType"];
	        this.size = source["size"];
	    }
	}
	export class MailMessageDetail {
	    id: string;
	    from: string;
	    to: string[];
	    cc: string[];
	    bcc: string[];
	    subject: string;
	    createdAt: string;
	    text: string;
	    html: string;
	    safeHtml: string;
	    hasRemoteImages: boolean;
	    headers: Record<string, string>;
	    attachments: MailAttachment[];
	
	    static createFrom(source: any = {}) {
	        return new MailMessageDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.cc = source["cc"];
	        this.bcc = source["bcc"];
	        this.subject = source["subject"];
	        this.createdAt = source["createdAt"];
	        this.text = source["text"];
	        this.html = source["html"];
	        this.safeHtml = source["safeHtml"];
	        this.hasRemoteImages = source["hasRemoteImages"];
	        this.headers = source["headers"];
	        this.attachments = this.convertValues(source["attachments"], MailAttachment);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MailMessageSummary {
	    id: string;
	    from: string;
	    to: string[];
	    subject: string;
	    snippet: string;
	    createdAt: string;
	    size: number;
	    attachments: number;
	    read: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MailMessageSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.subject = source["subject"];
	        this.snippet = source["snippet"];
	        this.createdAt = source["createdAt"];
	        this.size = source["size"];
	        this.attachments = source["attachments"];
	        this.read = source["read"];
	    }
	}
	export class MailboxResult {
	    total: number;
	    unread: number;
	    messages: MailMessageSummary[];
	
	    static createFrom(source: any = {}) {
	        return new MailboxResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.unread = source["unread"];
	        this.messages = this.convertValues(source["messages"], MailMessageSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace performance {
	
	export class AuthCredentials {
	    type: string;
	    token?: string;
	    name?: string;
	    value?: string;
	    user?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new AuthCredentials(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.token = source["token"];
	        this.name = source["name"];
	        this.value = source["value"];
	        this.user = source["user"];
	    }
	}
	export class CheckupItem {
	    id: string;
	    title: string;
	    category: string;
	    status: string;
	    message: string;
	    fixAction?: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckupItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.category = source["category"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.fixAction = source["fixAction"];
	    }
	}
	export class CheckupReport {
	    items: CheckupItem[];
	    score: number;
	    scannedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckupReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], CheckupItem);
	        this.score = source["score"];
	        this.scannedAt = source["scannedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LoadTestPoint {
	    elapsedSec: number;
	    requestsCount: number;
	    rps: number;
	    avgMs: number;
	
	    static createFrom(source: any = {}) {
	        return new LoadTestPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.elapsedSec = source["elapsedSec"];
	        this.requestsCount = source["requestsCount"];
	        this.rps = source["rps"];
	        this.avgMs = source["avgMs"];
	    }
	}
	export class LoadTestResult {
	    totalRequests: number;
	    successful: number;
	    failed: number;
	    durationSec: number;
	    rps: number;
	    minMs: number;
	    maxMs: number;
	    avgMs: number;
	    p50Ms: number;
	    p90Ms: number;
	    p99Ms: number;
	    statusCodes: Record<number, number>;
	    timeSeries: LoadTestPoint[];
	    redirectWarning: boolean;
	    errors?: string[];
	
	    static createFrom(source: any = {}) {
	        return new LoadTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalRequests = source["totalRequests"];
	        this.successful = source["successful"];
	        this.failed = source["failed"];
	        this.durationSec = source["durationSec"];
	        this.rps = source["rps"];
	        this.minMs = source["minMs"];
	        this.maxMs = source["maxMs"];
	        this.avgMs = source["avgMs"];
	        this.p50Ms = source["p50Ms"];
	        this.p90Ms = source["p90Ms"];
	        this.p99Ms = source["p99Ms"];
	        this.statusCodes = source["statusCodes"];
	        this.timeSeries = this.convertValues(source["timeSeries"], LoadTestPoint);
	        this.redirectWarning = source["redirectWarning"];
	        this.errors = source["errors"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LoadTestSpec {
	    url: string;
	    method: string;
	    concurrency: number;
	    durationSec: number;
	    maxRequests: number;
	    headers: Record<string, string>;
	    cookies: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new LoadTestSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.method = source["method"];
	        this.concurrency = source["concurrency"];
	        this.durationSec = source["durationSec"];
	        this.maxRequests = source["maxRequests"];
	        this.headers = source["headers"];
	        this.cookies = source["cookies"];
	        this.body = source["body"];
	    }
	}

}

export namespace quality {
	
	export class AnalyzerResult {
	    id: string;
	    name: string;
	    version: string;
	    configuration: string;
	    status: string;
	    coverage: string;
	    message: string;
	    durationSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzerResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.configuration = source["configuration"];
	        this.status = source["status"];
	        this.coverage = source["coverage"];
	        this.message = source["message"];
	        this.durationSeconds = source["durationSeconds"];
	    }
	}
	export class AuditOptions {
	    includeConventions: boolean;
	    includePerformanceAdvice: boolean;
	    gitHistory: boolean;
	    gitleaksBaseline: string;
	    semgrepConfig: string;
	    semgrepWsl: string;
	
	    static createFrom(source: any = {}) {
	        return new AuditOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.includeConventions = source["includeConventions"];
	        this.includePerformanceAdvice = source["includePerformanceAdvice"];
	        this.gitHistory = source["gitHistory"];
	        this.gitleaksBaseline = source["gitleaksBaseline"];
	        this.semgrepConfig = source["semgrepConfig"];
	        this.semgrepWsl = source["semgrepWsl"];
	    }
	}
	export class CodeBestPracticeFinding {
	    fingerprint?: string;
	    tool: string;
	    tools: string[];
	    kind: string;
	    certainty: string;
	    category: string;
	    rule: string;
	    severity: string;
	    file: string;
	    line: number;
	    message: string;
	    snippet?: string;
	    fixGuidance?: string;
	    githubRef?: string;
	
	    static createFrom(source: any = {}) {
	        return new CodeBestPracticeFinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fingerprint = source["fingerprint"];
	        this.tool = source["tool"];
	        this.tools = source["tools"];
	        this.kind = source["kind"];
	        this.certainty = source["certainty"];
	        this.category = source["category"];
	        this.rule = source["rule"];
	        this.severity = source["severity"];
	        this.file = source["file"];
	        this.line = source["line"];
	        this.message = source["message"];
	        this.snippet = source["snippet"];
	        this.fixGuidance = source["fixGuidance"];
	        this.githubRef = source["githubRef"];
	    }
	}
	export class AuditReport {
	    comparisonKey?: string;
	    projectId: string;
	    running: boolean;
	    startedAt: string;
	    finishedAt: string;
	    findings: CodeBestPracticeFinding[];
	    analyzers: AnalyzerResult[];
	
	    static createFrom(source: any = {}) {
	        return new AuditReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.comparisonKey = source["comparisonKey"];
	        this.projectId = source["projectId"];
	        this.running = source["running"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.findings = this.convertValues(source["findings"], CodeBestPracticeFinding);
	        this.analyzers = this.convertValues(source["analyzers"], AnalyzerResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SetupFile {
	    path: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new SetupFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.content = source["content"];
	    }
	}
	export class AuditSetupPlan {
	    id: string;
	    projectId: string;
	    kind: string;
	    manager: string;
	    message: string;
	    commands: runner.Spec[];
	    files: SetupFile[];
	    fingerprints: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new AuditSetupPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.kind = source["kind"];
	        this.manager = source["manager"];
	        this.message = source["message"];
	        this.commands = this.convertValues(source["commands"], runner.Spec);
	        this.files = this.convertValues(source["files"], SetupFile);
	        this.fingerprints = source["fingerprints"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BrowserSessionStatus {
	    running: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserSessionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.message = source["message"];
	    }
	}
	
	export class ConfigAuditItem {
	    category: string;
	    key: string;
	    value: string;
	    status: string;
	    message: string;
	    fixGuidance: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigAuditItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category = source["category"];
	        this.key = source["key"];
	        this.value = source["value"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.fixGuidance = source["fixGuidance"];
	    }
	}
	export class QualityFinding {
	    tool: string;
	    rule: string;
	    severity: string;
	    file: string;
	    line: number;
	    message: string;
	    suggestion?: string;
	
	    static createFrom(source: any = {}) {
	        return new QualityFinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.rule = source["rule"];
	        this.severity = source["severity"];
	        this.file = source["file"];
	        this.line = source["line"];
	        this.message = source["message"];
	        this.suggestion = source["suggestion"];
	    }
	}
	export class QualityReport {
	    tool: string;
	    findings: QualityFinding[];
	    totalErrors: number;
	    totalWarns: number;
	    rawOutput?: string;
	    durationSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new QualityReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.findings = this.convertValues(source["findings"], QualityFinding);
	        this.totalErrors = source["totalErrors"];
	        this.totalWarns = source["totalWarns"];
	        this.rawOutput = source["rawOutput"];
	        this.durationSeconds = source["durationSeconds"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RulePolicy {
	    id: string;
	    kind: string;
	    owner: string;
	    optional: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RulePolicy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.owner = source["owner"];
	        this.optional = source["optional"];
	    }
	}

}

export namespace realtime {
	
	export class ChannelInfo {
	    name: string;
	    type: string;
	    subscribers: number;
	    lastEvent?: string;
	    lastSeen?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChannelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.subscribers = source["subscribers"];
	        this.lastEvent = source["lastEvent"];
	        this.lastSeen = source["lastSeen"];
	    }
	}
	export class EventSimulationSpec {
	    channel: string;
	    event: string;
	    payload: string;
	
	    static createFrom(source: any = {}) {
	        return new EventSimulationSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channel = source["channel"];
	        this.event = source["event"];
	        this.payload = source["payload"];
	    }
	}
	export class ReverbConfig {
	    appKey: string;
	    host: string;
	    port: number;
	    scheme: string;
	    driver: string;
	    isRunning: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ReverbConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appKey = source["appKey"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.scheme = source["scheme"];
	        this.driver = source["driver"];
	        this.isRunning = source["isRunning"];
	    }
	}
	export class WebSocketFrame {
	    id: string;
	    // Go type: time
	    timestamp: any;
	    direction: string;
	    event: string;
	    channel?: string;
	    data: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new WebSocketFrame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.direction = source["direction"];
	        this.event = source["event"];
	        this.channel = source["channel"];
	        this.data = source["data"];
	        this.size = source["size"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SimulationResult {
	    success: boolean;
	    message: string;
	    frame: WebSocketFrame;
	
	    static createFrom(source: any = {}) {
	        return new SimulationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.frame = this.convertValues(source["frame"], WebSocketFrame);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace recipes {
	
	export class ProjectWizardSpec {
	    directory: string;
	    name: string;
	    starter: string;
	    database: string;
	    phpBinary: string;
	    gitInit: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProjectWizardSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.directory = source["directory"];
	        this.name = source["name"];
	        this.starter = source["starter"];
	        this.database = source["database"];
	        this.phpBinary = source["phpBinary"];
	        this.gitInit = source["gitInit"];
	    }
	}
	export class RecipeStep {
	    type: string;
	    title: string;
	    command?: string;
	    args?: string[];
	    targetFile?: string;
	    content?: string;
	    envKey?: string;
	    envValue?: string;
	    verifyCheck?: string;
	
	    static createFrom(source: any = {}) {
	        return new RecipeStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.title = source["title"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.targetFile = source["targetFile"];
	        this.content = source["content"];
	        this.envKey = source["envKey"];
	        this.envValue = source["envValue"];
	        this.verifyCheck = source["verifyCheck"];
	    }
	}
	export class Recipe {
	    id: string;
	    title: string;
	    description: string;
	    category: string;
	    prerequisites: string[];
	    steps: RecipeStep[];
	    rollbackInstructions: string;
	
	    static createFrom(source: any = {}) {
	        return new Recipe(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.category = source["category"];
	        this.prerequisites = source["prerequisites"];
	        this.steps = this.convertValues(source["steps"], RecipeStep);
	        this.rollbackInstructions = source["rollbackInstructions"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StepExecutionResult {
	    stepIndex: number;
	    title: string;
	    success: boolean;
	    output: string[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new StepExecutionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stepIndex = source["stepIndex"];
	        this.title = source["title"];
	        this.success = source["success"];
	        this.output = source["output"];
	        this.error = source["error"];
	    }
	}
	export class RecipeExecutionResult {
	    recipeId: string;
	    success: boolean;
	    stepResults: StepExecutionResult[];
	    durationMs: number;
	    recoveryGuidance?: string;
	
	    static createFrom(source: any = {}) {
	        return new RecipeExecutionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recipeId = source["recipeId"];
	        this.success = source["success"];
	        this.stepResults = this.convertValues(source["stepResults"], StepExecutionResult);
	        this.durationMs = source["durationMs"];
	        this.recoveryGuidance = source["recoveryGuidance"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RecipePreview {
	    recipeId: string;
	    title: string;
	    commandsToRun: string[];
	    filesModified: string[];
	    envKeysSet: string[];
	
	    static createFrom(source: any = {}) {
	        return new RecipePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recipeId = source["recipeId"];
	        this.title = source["title"];
	        this.commandsToRun = source["commandsToRun"];
	        this.filesModified = source["filesModified"];
	        this.envKeysSet = source["envKeysSet"];
	    }
	}
	

}

export namespace remote {
	
	export class RemoteProfile {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	    user: string;
	    authType: string;
	    keyPath?: string;
	    remotePath: string;
	    environment: string;
	    readOnly: boolean;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    lastTested?: any;
	    status?: string;
	    latencyMs?: number;
	    remotePhp?: string;
	    remoteVer?: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.authType = source["authType"];
	        this.keyPath = source["keyPath"];
	        this.remotePath = source["remotePath"];
	        this.environment = source["environment"];
	        this.readOnly = source["readOnly"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.lastTested = this.convertValues(source["lastTested"], null);
	        this.status = source["status"];
	        this.latencyMs = source["latencyMs"];
	        this.remotePhp = source["remotePhp"];
	        this.remoteVer = source["remoteVer"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RemoteTestResult {
	    reachable: boolean;
	    latencyMs: number;
	    php?: string;
	    laravel?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reachable = source["reachable"];
	        this.latencyMs = source["latencyMs"];
	        this.php = source["php"];
	        this.laravel = source["laravel"];
	        this.error = source["error"];
	    }
	}
	export class SafetyAssessment {
	    allowed: boolean;
	    reason: string;
	    requiresToken: boolean;
	    expectedToken?: string;
	
	    static createFrom(source: any = {}) {
	        return new SafetyAssessment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allowed = source["allowed"];
	        this.reason = source["reason"];
	        this.requiresToken = source["requiresToken"];
	        this.expectedToken = source["expectedToken"];
	    }
	}

}

export namespace reports {
	
	export class ArchitectureEdge {
	    source: string;
	    target: string;
	    label: string;
	    provenance: string;
	
	    static createFrom(source: any = {}) {
	        return new ArchitectureEdge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.target = source["target"];
	        this.label = source["label"];
	        this.provenance = source["provenance"];
	    }
	}
	export class ArchitectureNode {
	    id: string;
	    label: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new ArchitectureNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.type = source["type"];
	    }
	}
	export class ArchitectureGraph {
	    nodes: ArchitectureNode[];
	    edges: ArchitectureEdge[];
	
	    static createFrom(source: any = {}) {
	        return new ArchitectureGraph(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], ArchitectureNode);
	        this.edges = this.convertValues(source["edges"], ArchitectureEdge);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class HostingAdvice {
	    recommendedPlan: string;
	    minimumRam: string;
	    cpuRequirement: string;
	    phpFpmWorkers: number;
	    opcacheMemory: string;
	    recommendations: string[];
	
	    static createFrom(source: any = {}) {
	        return new HostingAdvice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.recommendedPlan = source["recommendedPlan"];
	        this.minimumRam = source["minimumRam"];
	        this.cpuRequirement = source["cpuRequirement"];
	        this.phpFpmWorkers = source["phpFpmWorkers"];
	        this.opcacheMemory = source["opcacheMemory"];
	        this.recommendations = source["recommendations"];
	    }
	}
	export class ProjectSnapshot {
	    generatedAt: string;
	    projectName: string;
	    projectPath: string;
	    projectUrl: string;
	    phpVersion: string;
	    framework: string;
	    clockwork: string;
	    routesCount: number;
	    routesSample: string[];
	    services: string[];
	    eventsCount: number;
	    databaseDriver: string;
	    cacheDriver: string;
	    queueDriver: string;
	    envKeys: string[];
	    auditGood: number;
	    auditWarn: number;
	    auditInfo: number;
	    hostingAdvice: HostingAdvice;
	    graph: ArchitectureGraph;
	
	    static createFrom(source: any = {}) {
	        return new ProjectSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.generatedAt = source["generatedAt"];
	        this.projectName = source["projectName"];
	        this.projectPath = source["projectPath"];
	        this.projectUrl = source["projectUrl"];
	        this.phpVersion = source["phpVersion"];
	        this.framework = source["framework"];
	        this.clockwork = source["clockwork"];
	        this.routesCount = source["routesCount"];
	        this.routesSample = source["routesSample"];
	        this.services = source["services"];
	        this.eventsCount = source["eventsCount"];
	        this.databaseDriver = source["databaseDriver"];
	        this.cacheDriver = source["cacheDriver"];
	        this.queueDriver = source["queueDriver"];
	        this.envKeys = source["envKeys"];
	        this.auditGood = source["auditGood"];
	        this.auditWarn = source["auditWarn"];
	        this.auditInfo = source["auditInfo"];
	        this.hostingAdvice = this.convertValues(source["hostingAdvice"], HostingAdvice);
	        this.graph = this.convertValues(source["graph"], ArchitectureGraph);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace runner {
	
	export class Spec {
	    env?: string[];
	    executable: string;
	    args: string[];
	    directory: string;
	
	    static createFrom(source: any = {}) {
	        return new Spec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.env = source["env"];
	        this.executable = source["executable"];
	        this.args = source["args"];
	        this.directory = source["directory"];
	    }
	}

}

export namespace runtimes {
	
	export class ComposerStatus {
	    installed: boolean;
	    version: string;
	    path: string;
	    inPath: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ComposerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.version = source["version"];
	        this.path = source["path"];
	        this.inPath = source["inPath"];
	    }
	}
	export class ExtensionStatus {
	    name: string;
	    enabled: boolean;
	    available: boolean;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new ExtensionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.available = source["available"];
	        this.description = source["description"];
	    }
	}
	export class RuntimeInfo {
	    id: string;
	    name: string;
	    version: string;
	    type: string;
	    binaryPath: string;
	    iniPath: string;
	    installed: boolean;
	    isDefault: boolean;
	    downloadUrl?: string;
	    extensions?: ExtensionStatus[];
	
	    static createFrom(source: any = {}) {
	        return new RuntimeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.type = source["type"];
	        this.binaryPath = source["binaryPath"];
	        this.iniPath = source["iniPath"];
	        this.installed = source["installed"];
	        this.isDefault = source["isDefault"];
	        this.downloadUrl = source["downloadUrl"];
	        this.extensions = this.convertValues(source["extensions"], ExtensionStatus);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace services {
	
	export class DockerStatus {
	    dockerInstalled: boolean;
	    dockerRunning: boolean;
	    wslInstalled: boolean;
	    version: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DockerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dockerInstalled = source["dockerInstalled"];
	        this.dockerRunning = source["dockerRunning"];
	        this.wslInstalled = source["wslInstalled"];
	        this.version = source["version"];
	        this.message = source["message"];
	    }
	}
	export class EnvSyncOptions {
	    databaseEngine: string;
	    databaseName: string;
	    configureRedis: boolean;
	    configureMailpit: boolean;
	    configureMeilisearch: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EnvSyncOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.databaseEngine = source["databaseEngine"];
	        this.databaseName = source["databaseName"];
	        this.configureRedis = source["configureRedis"];
	        this.configureMailpit = source["configureMailpit"];
	        this.configureMeilisearch = source["configureMeilisearch"];
	    }
	}
	export class EnvSyncResult {
	    backupPath: string;
	    updated: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new EnvSyncResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backupPath = source["backupPath"];
	        this.updated = source["updated"];
	    }
	}
	export class ServiceInstance {
	    id: string;
	    engine: string;
	    name: string;
	    port: number;
	    adminPort?: number;
	    dataDir: string;
	    status: string;
	    isOwned: boolean;
	    pid: number;
	    memoryMb: number;
	    cpuUsage: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServiceInstance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.engine = source["engine"];
	        this.name = source["name"];
	        this.port = source["port"];
	        this.adminPort = source["adminPort"];
	        this.dataDir = source["dataDir"];
	        this.status = source["status"];
	        this.isOwned = source["isOwned"];
	        this.pid = source["pid"];
	        this.memoryMb = source["memoryMb"];
	        this.cpuUsage = source["cpuUsage"];
	        this.error = source["error"];
	    }
	}

}

export namespace sharing {
	
	export class IntegrationItem {
	    name: string;
	    type: string;
	    detectedIn: string;
	    status: string;
	    environment: string;
	
	    static createFrom(source: any = {}) {
	        return new IntegrationItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.detectedIn = source["detectedIn"];
	        this.status = source["status"];
	        this.environment = source["environment"];
	    }
	}
	export class TunnelStatus {
	    running: boolean;
	    provider: string;
	    localTarget: string;
	    publicUrl: string;
	    startedAt: string;
	    pid: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TunnelStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.provider = source["provider"];
	        this.localTarget = source["localTarget"];
	        this.publicUrl = source["publicUrl"];
	        this.startedAt = source["startedAt"];
	        this.pid = source["pid"];
	        this.error = source["error"];
	    }
	}
	export class WebhookDispatchResult {
	    statusCode: number;
	    durationMs: number;
	    responseHeaders: Record<string, string>;
	    responseBody: string;
	    computedSignature: string;
	    signatureValid: boolean;
	    success: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new WebhookDispatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.statusCode = source["statusCode"];
	        this.durationMs = source["durationMs"];
	        this.responseHeaders = source["responseHeaders"];
	        this.responseBody = source["responseBody"];
	        this.computedSignature = source["computedSignature"];
	        this.signatureValid = source["signatureValid"];
	        this.success = source["success"];
	        this.error = source["error"];
	    }
	}
	export class WebhookScenario {
	    id: string;
	    name: string;
	    provider: string;
	    targetUrl: string;
	    secret: string;
	    headers: Record<string, string>;
	    payload: string;
	
	    static createFrom(source: any = {}) {
	        return new WebhookScenario(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.targetUrl = source["targetUrl"];
	        this.secret = source["secret"];
	        this.headers = source["headers"];
	        this.payload = source["payload"];
	    }
	}

}

export namespace storage {
	
	export class AIConfig {
	    enabled: boolean;
	    provider: string;
	    apiKeyRef: string;
	    model: string;
	    endpoint: string;
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.provider = source["provider"];
	        this.apiKeyRef = source["apiKeyRef"];
	        this.model = source["model"];
	        this.endpoint = source["endpoint"];
	    }
	}
	export class DiagnosticRecord {
	    id: string;
	    projectId: string;
	    source: string;
	    type: string;
	    timestamp: string;
	    status: number;
	    uri: string;
	    duration: number;
	    summary: string;
	    details: string;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.source = source["source"];
	        this.type = source["type"];
	        this.timestamp = source["timestamp"];
	        this.status = source["status"];
	        this.uri = source["uri"];
	        this.duration = source["duration"];
	        this.summary = source["summary"];
	        this.details = source["details"];
	    }
	}
	export class DumpRecord {
	    id: string;
	    projectId: string;
	    file: string;
	    line: number;
	    payload: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new DumpRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.file = source["file"];
	        this.line = source["line"];
	        this.payload = source["payload"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class ErrorLogRecord {
	    id: string;
	    projectId: string;
	    fingerprint: string;
	    level: string;
	    message: string;
	    file: string;
	    line: number;
	    stack: string;
	    occurrences: number;
	    firstSeen: string;
	    lastSeen: string;
	
	    static createFrom(source: any = {}) {
	        return new ErrorLogRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.fingerprint = source["fingerprint"];
	        this.level = source["level"];
	        this.message = source["message"];
	        this.file = source["file"];
	        this.line = source["line"];
	        this.stack = source["stack"];
	        this.occurrences = source["occurrences"];
	        this.firstSeen = source["firstSeen"];
	        this.lastSeen = source["lastSeen"];
	    }
	}
	export class LogPailEntry {
	    id: string;
	    // Go type: time
	    timestamp: any;
	    dateStr: string;
	    env: string;
	    level: string;
	    message: string;
	    context?: string;
	    trace?: string[];
	    appTrace?: string[];
	    isError: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LogPailEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.dateStr = source["dateStr"];
	        this.env = source["env"];
	        this.level = source["level"];
	        this.message = source["message"];
	        this.context = source["context"];
	        this.trace = source["trace"];
	        this.appTrace = source["appTrace"];
	        this.isError = source["isError"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ExceptionIncident {
	    fingerprint: string;
	    exception: string;
	    message: string;
	    location: string;
	    count: number;
	    // Go type: time
	    firstSeen: any;
	    // Go type: time
	    lastSeen: any;
	    sampleEntry: LogPailEntry;
	    aiPrompt: string;
	
	    static createFrom(source: any = {}) {
	        return new ExceptionIncident(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fingerprint = source["fingerprint"];
	        this.exception = source["exception"];
	        this.message = source["message"];
	        this.location = source["location"];
	        this.count = source["count"];
	        this.firstSeen = this.convertValues(source["firstSeen"], null);
	        this.lastSeen = this.convertValues(source["lastSeen"], null);
	        this.sampleEntry = this.convertValues(source["sampleEntry"], LogPailEntry);
	        this.aiPrompt = source["aiPrompt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class LogPailReport {
	    totalEntries: number;
	    errorCount: number;
	    warnCount: number;
	    entries: LogPailEntry[];
	    incidents: ExceptionIncident[];
	
	    static createFrom(source: any = {}) {
	        return new LogPailReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalEntries = source["totalEntries"];
	        this.errorCount = source["errorCount"];
	        this.warnCount = source["warnCount"];
	        this.entries = this.convertValues(source["entries"], LogPailEntry);
	        this.incidents = this.convertValues(source["incidents"], ExceptionIncident);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SavedCmd {
	    id: string;
	    name: string;
	    executable: string;
	    args: string[];
	    directory: string;
	    destructive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SavedCmd(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.executable = source["executable"];
	        this.args = source["args"];
	        this.directory = source["directory"];
	        this.destructive = source["destructive"];
	    }
	}
	export class ServiceConf {
	    engine: string;
	    port: number;
	    dataDir: string;
	    autoStart: boolean;
	    customArgs: string;
	
	    static createFrom(source: any = {}) {
	        return new ServiceConf(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.port = source["port"];
	        this.dataDir = source["dataDir"];
	        this.autoStart = source["autoStart"];
	        this.customArgs = source["customArgs"];
	    }
	}
	export class Settings {
	    version: number;
	    recentProject: string;
	    trusted: Record<string, boolean>;
	    customPaths: string[];
	    theme: string;
	    maxRetention: number;
	    savedCommands: Record<string, Array<SavedCmd>>;
	    serviceConfigs: Record<string, ServiceConf>;
	    aiConfig: AIConfig;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.recentProject = source["recentProject"];
	        this.trusted = source["trusted"];
	        this.customPaths = source["customPaths"];
	        this.theme = source["theme"];
	        this.maxRetention = source["maxRetention"];
	        this.savedCommands = this.convertValues(source["savedCommands"], Array<SavedCmd>, true);
	        this.serviceConfigs = this.convertValues(source["serviceConfigs"], ServiceConf, true);
	        this.aiConfig = this.convertValues(source["aiConfig"], AIConfig);
	        this.updatedAt = source["updatedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace telemetry {
	
	export class DumpPayload {
	    id: string;
	    projectId: string;
	    file: string;
	    line: number;
	    payload: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new DumpPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.file = source["file"];
	        this.line = source["line"];
	        this.payload = source["payload"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class NPlusOneIssue {
	    fingerprint: string;
	    normalizedQuery: string;
	    count: number;
	    totalDuration: number;
	    table: string;
	    column: string;
	    eagerLoadFix: string;
	    severity: string;
	    explanation: string;
	
	    static createFrom(source: any = {}) {
	        return new NPlusOneIssue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fingerprint = source["fingerprint"];
	        this.normalizedQuery = source["normalizedQuery"];
	        this.count = source["count"];
	        this.totalDuration = source["totalDuration"];
	        this.table = source["table"];
	        this.column = source["column"];
	        this.eagerLoadFix = source["eagerLoadFix"];
	        this.severity = source["severity"];
	        this.explanation = source["explanation"];
	    }
	}
	export class NPlusOneReport {
	    requestId: string;
	    uri: string;
	    totalLoops: number;
	    wastedTime: number;
	    issues: NPlusOneIssue[];
	
	    static createFrom(source: any = {}) {
	        return new NPlusOneReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requestId = source["requestId"];
	        this.uri = source["uri"];
	        this.totalLoops = source["totalLoops"];
	        this.wastedTime = source["wastedTime"];
	        this.issues = this.convertValues(source["issues"], NPlusOneIssue);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TelemetryException {
	    message: string;
	    file: string;
	    line: number;
	    class: string;
	    stack?: string[];
	
	    static createFrom(source: any = {}) {
	        return new TelemetryException(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.file = source["file"];
	        this.line = source["line"];
	        this.class = source["class"];
	        this.stack = source["stack"];
	    }
	}
	export class TelemetryLog {
	    level: string;
	    message: string;
	    context?: string;
	    time: string;
	
	    static createFrom(source: any = {}) {
	        return new TelemetryLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.message = source["message"];
	        this.context = source["context"];
	        this.time = source["time"];
	    }
	}
	export class TelemetryQuery {
	    sql: string;
	    rawSql: string;
	    timeMs: number;
	    connection: string;
	
	    static createFrom(source: any = {}) {
	        return new TelemetryQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sql = source["sql"];
	        this.rawSql = source["rawSql"];
	        this.timeMs = source["timeMs"];
	        this.connection = source["connection"];
	    }
	}
	export class RequestTelemetryFrame {
	    id: string;
	    projectId?: string;
	    method: string;
	    url: string;
	    status: number;
	    durationMs: number;
	    memoryPeakBytes: number;
	    user?: Record<string, any>;
	    queries: TelemetryQuery[];
	    logs: TelemetryLog[];
	    dumps: DumpPayload[];
	    exception?: TelemetryException;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new RequestTelemetryFrame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.method = source["method"];
	        this.url = source["url"];
	        this.status = source["status"];
	        this.durationMs = source["durationMs"];
	        this.memoryPeakBytes = source["memoryPeakBytes"];
	        this.user = source["user"];
	        this.queries = this.convertValues(source["queries"], TelemetryQuery);
	        this.logs = this.convertValues(source["logs"], TelemetryLog);
	        this.dumps = this.convertValues(source["dumps"], DumpPayload);
	        this.exception = this.convertValues(source["exception"], TelemetryException);
	        this.createdAt = source["createdAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class WaterfallSegment {
	    id: string;
	    category: string;
	    label: string;
	    startMs: number;
	    durationMs: number;
	    percentage: number;
	    details?: string;
	    status?: string;
	
	    static createFrom(source: any = {}) {
	        return new WaterfallSegment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.label = source["label"];
	        this.startMs = source["startMs"];
	        this.durationMs = source["durationMs"];
	        this.percentage = source["percentage"];
	        this.details = source["details"];
	        this.status = source["status"];
	    }
	}
	export class WaterfallFlamegraph {
	    requestId: string;
	    totalDuration: number;
	    segments: WaterfallSegment[];
	    breakdown: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new WaterfallFlamegraph(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requestId = source["requestId"];
	        this.totalDuration = source["totalDuration"];
	        this.segments = this.convertValues(source["segments"], WaterfallSegment);
	        this.breakdown = source["breakdown"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class XdebugStatus {
	    installed: boolean;
	    version?: string;
	    mode?: string;
	    port: number;
	    ideKey: string;
	    triggerUrl?: string;
	    configStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new XdebugStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.version = source["version"];
	        this.mode = source["mode"];
	        this.port = source["port"];
	        this.ideKey = source["ideKey"];
	        this.triggerUrl = source["triggerUrl"];
	        this.configStatus = source["configStatus"];
	    }
	}

}

export namespace testing {
	
	export class APIAssertion {
	    type: string;
	    target: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new APIAssertion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.target = source["target"];
	        this.value = source["value"];
	    }
	}
	export class APIRequestSpec {
	    id: string;
	    name: string;
	    method: string;
	    url: string;
	    headers: Record<string, string>;
	    body: string;
	    assertions: APIAssertion[];
	
	    static createFrom(source: any = {}) {
	        return new APIRequestSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.method = source["method"];
	        this.url = source["url"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	        this.assertions = this.convertValues(source["assertions"], APIAssertion);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AssertionResult {
	    type: string;
	    passed: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new AssertionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.passed = source["passed"];
	        this.message = source["message"];
	    }
	}
	export class APIResponseResult {
	    statusCode: number;
	    durationMs: number;
	    headers: Record<string, string>;
	    body: string;
	    size: number;
	    assertionResults: AssertionResult[];
	    passed: boolean;
	    error?: string;
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new APIResponseResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.statusCode = source["statusCode"];
	        this.durationMs = source["durationMs"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	        this.size = source["size"];
	        this.assertionResults = this.convertValues(source["assertionResults"], AssertionResult);
	        this.passed = source["passed"];
	        this.error = source["error"];
	        this.truncated = source["truncated"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class LoadTestResult {
	    engine: string;
	    totalRequests: number;
	    successful: number;
	    failed: number;
	    requestsPerSecond: number;
	    minLatencyMs: number;
	    maxLatencyMs: number;
	    avgLatencyMs: number;
	    p50LatencyMs: number;
	    p90LatencyMs: number;
	    p99LatencyMs: number;
	    statusDistribution: Record<number, number>;
	    errorSample: string[];
	    completedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new LoadTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.totalRequests = source["totalRequests"];
	        this.successful = source["successful"];
	        this.failed = source["failed"];
	        this.requestsPerSecond = source["requestsPerSecond"];
	        this.minLatencyMs = source["minLatencyMs"];
	        this.maxLatencyMs = source["maxLatencyMs"];
	        this.avgLatencyMs = source["avgLatencyMs"];
	        this.p50LatencyMs = source["p50LatencyMs"];
	        this.p90LatencyMs = source["p90LatencyMs"];
	        this.p99LatencyMs = source["p99LatencyMs"];
	        this.statusDistribution = source["statusDistribution"];
	        this.errorSample = source["errorSample"];
	        this.completedAt = source["completedAt"];
	    }
	}
	export class LoadTestSpec {
	    targetUrl: string;
	    method: string;
	    virtualUsers: number;
	    durationSeconds: number;
	    headers: Record<string, string>;
	    body: string;
	    allowMutation: boolean;
	    engine?: string;
	
	    static createFrom(source: any = {}) {
	        return new LoadTestSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.targetUrl = source["targetUrl"];
	        this.method = source["method"];
	        this.virtualUsers = source["virtualUsers"];
	        this.durationSeconds = source["durationSeconds"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	        this.allowMutation = source["allowMutation"];
	        this.engine = source["engine"];
	    }
	}
	export class ProjectTestResult {
	    suite: string;
	    total: number;
	    passed: number;
	    failed: number;
	    skipped: number;
	    durationSeconds: number;
	    output: string[];
	    success: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProjectTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.suite = source["suite"];
	        this.total = source["total"];
	        this.passed = source["passed"];
	        this.failed = source["failed"];
	        this.skipped = source["skipped"];
	        this.durationSeconds = source["durationSeconds"];
	        this.output = source["output"];
	        this.success = source["success"];
	    }
	}

}

export namespace tinker {
	
	export class QueryLog {
	    sql: string;
	    timeMs: number;
	    bindings?: any[];
	    connection?: string;
	
	    static createFrom(source: any = {}) {
	        return new QueryLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sql = source["sql"];
	        this.timeMs = source["timeMs"];
	        this.bindings = source["bindings"];
	        this.connection = source["connection"];
	    }
	}
	export class EvaluationResult {
	    output: string;
	    durationMs: number;
	    queries: QueryLog[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new EvaluationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.output = source["output"];
	        this.durationMs = source["durationMs"];
	        this.queries = this.convertValues(source["queries"], QueryLog);
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace webhooks {
	
	export class WebhookDelivery {
	    id: string;
	    endpointId: string;
	    method: string;
	    clientIp: string;
	    headers: Record<string, Array<string>>;
	    body: string;
	    receivedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new WebhookDelivery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.endpointId = source["endpointId"];
	        this.method = source["method"];
	        this.clientIp = source["clientIp"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	        this.receivedAt = source["receivedAt"];
	    }
	}
	export class WebhookReplayResult {
	    deliveryId: string;
	    targetUrl: string;
	    statusCode: number;
	    durationMs: number;
	    response: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new WebhookReplayResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deliveryId = source["deliveryId"];
	        this.targetUrl = source["targetUrl"];
	        this.statusCode = source["statusCode"];
	        this.durationMs = source["durationMs"];
	        this.response = source["response"];
	        this.error = source["error"];
	    }
	}
	export class WebhookRouteInspection {
	    uri: string;
	    name: string;
	    action: string;
	    provider: string;
	    csrfBlocked: boolean;
	    csrfFix: string;
	    authBlocked: boolean;
	    secretConfigured: boolean;
	    secretKeyName: string;
	
	    static createFrom(source: any = {}) {
	        return new WebhookRouteInspection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uri = source["uri"];
	        this.name = source["name"];
	        this.action = source["action"];
	        this.provider = source["provider"];
	        this.csrfBlocked = source["csrfBlocked"];
	        this.csrfFix = source["csrfFix"];
	        this.authBlocked = source["authBlocked"];
	        this.secretConfigured = source["secretConfigured"];
	        this.secretKeyName = source["secretKeyName"];
	    }
	}

}

export namespace wizard {
	
	export class WizardStepResult {
	    step: string;
	    success: boolean;
	    output?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new WizardStepResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.step = source["step"];
	        this.success = source["success"];
	        this.output = source["output"];
	        this.error = source["error"];
	    }
	}
	export class WizardResult {
	    projectPath: string;
	    success: boolean;
	    steps: WizardStepResult[];
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new WizardResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectPath = source["projectPath"];
	        this.success = source["success"];
	        this.steps = this.convertValues(source["steps"], WizardStepResult);
	        this.durationMs = source["durationMs"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WizardSpec {
	    directory: string;
	    name: string;
	    starter: string;
	    databaseEngine: string;
	    databaseName: string;
	    phpBinary: string;
	    selectedPackages: string[];
	    gitInit: boolean;
	    autoSyncEnvServices: boolean;
	    linkWithHerd: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WizardSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.directory = source["directory"];
	        this.name = source["name"];
	        this.starter = source["starter"];
	        this.databaseEngine = source["databaseEngine"];
	        this.databaseName = source["databaseName"];
	        this.phpBinary = source["phpBinary"];
	        this.selectedPackages = source["selectedPackages"];
	        this.gitInit = source["gitInit"];
	        this.autoSyncEnvServices = source["autoSyncEnvServices"];
	        this.linkWithHerd = source["linkWithHerd"];
	    }
	}

}

export namespace workbench {
	
	export class Capability {
	    command: string;
	    json: boolean;
	    status: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new Capability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	        this.json = source["json"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	    }
	}
	export class TimelineEvent {
	    description: string;
	    start: number;
	    end: number;
	    duration: number;
	
	    static createFrom(source: any = {}) {
	        return new TimelineEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.description = source["description"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.duration = source["duration"];
	    }
	}
	export class Query {
	    sql: string;
	    duration: number;
	    connection: string;
	    repeated: boolean;
	    time?: number;
	
	    static createFrom(source: any = {}) {
	        return new Query(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sql = source["sql"];
	        this.duration = source["duration"];
	        this.connection = source["connection"];
	        this.repeated = source["repeated"];
	        this.time = source["time"];
	    }
	}
	export class Request {
	    memory?: number;
	    id: string;
	    method: string;
	    uri: string;
	    status: number;
	    time: number;
	    duration: number;
	    databaseDuration: number;
	    queries: Query[];
	    queryCount: number;
	    timeline?: TimelineEvent[];
	    truncated: boolean;
	    exceptions: number;
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.memory = source["memory"];
	        this.id = source["id"];
	        this.method = source["method"];
	        this.uri = source["uri"];
	        this.status = source["status"];
	        this.time = source["time"];
	        this.duration = source["duration"];
	        this.databaseDuration = source["databaseDuration"];
	        this.queries = this.convertValues(source["queries"], Query);
	        this.queryCount = source["queryCount"];
	        this.timeline = this.convertValues(source["timeline"], TimelineEvent);
	        this.truncated = source["truncated"];
	        this.exceptions = source["exceptions"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Capture {
	    projectId: string;
	    status: string;
	    setupCommand?: string;
	    requests: Request[];
	    watching: boolean;
	    message: string;
	    updated: string;
	
	    static createFrom(source: any = {}) {
	        return new Capture(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectId = source["projectId"];
	        this.status = source["status"];
	        this.setupCommand = source["setupCommand"];
	        this.requests = this.convertValues(source["requests"], Request);
	        this.watching = source["watching"];
	        this.message = source["message"];
	        this.updated = source["updated"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DependencyInstallation {
	    id: string;
	    projectId: string;
	    name: string;
	    running: boolean;
	    state: string;
	    startedAt: string;
	    message: string;
	    output: string[];
	
	    static createFrom(source: any = {}) {
	        return new DependencyInstallation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.name = source["name"];
	        this.running = source["running"];
	        this.state = source["state"];
	        this.startedAt = source["startedAt"];
	        this.message = source["message"];
	        this.output = source["output"];
	    }
	}
	export class HealthCheck {
	    name: string;
	    status: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	    }
	}
	export class LogEntry {
	    id: string;
	    timestamp: string;
	    environment: string;
	    level: string;
	    message: string;
	    file: string;
	    line: number;
	    stack: string;
	    fingerprint: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.environment = source["environment"];
	        this.level = source["level"];
	        this.message = source["message"];
	        this.file = source["file"];
	        this.line = source["line"];
	        this.stack = source["stack"];
	        this.fingerprint = source["fingerprint"];
	    }
	}
	export class PackageSummary {
	    status: string;
	    label: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new PackageSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.label = source["label"];
	        this.detail = source["detail"];
	    }
	}
	export class Project {
	    id: string;
	    name: string;
	    path: string;
	    url: string;
	    php: string;
	    framework: string;
	    clockwork: string;
	    vendorPresent: boolean;
	    dependencyState: string;
	    secured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.url = source["url"];
	        this.php = source["php"];
	        this.framework = source["framework"];
	        this.clockwork = source["clockwork"];
	        this.vendorPresent = source["vendorPresent"];
	        this.dependencyState = source["dependencyState"];
	        this.secured = source["secured"];
	    }
	}
	export class ProjectHealth {
	    project: Project;
	    trusted: boolean;
	    checks: HealthCheck[];
	    running: number;
	    lastAudit: string;
	    findings: number;
	    coverage: string;
	    composer: PackageSummary;
	    npm: PackageSummary;
	
	    static createFrom(source: any = {}) {
	        return new ProjectHealth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.project = this.convertValues(source["project"], Project);
	        this.trusted = source["trusted"];
	        this.checks = this.convertValues(source["checks"], HealthCheck);
	        this.running = source["running"];
	        this.lastAudit = source["lastAudit"];
	        this.findings = source["findings"];
	        this.coverage = source["coverage"];
	        this.composer = this.convertValues(source["composer"], PackageSummary);
	        this.npm = this.convertValues(source["npm"], PackageSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class Route {
	    method: string;
	    uri: string;
	    name: string;
	    action: string;
	
	    static createFrom(source: any = {}) {
	        return new Route(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.method = source["method"];
	        this.uri = source["uri"];
	        this.name = source["name"];
	        this.action = source["action"];
	    }
	}
	export class Run {
	    id: string;
	    projectId: string;
	    name: string;
	    running: boolean;
	    pid: number;
	    exitCode: number;
	    output: string[];
	
	    static createFrom(source: any = {}) {
	        return new Run(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.name = source["name"];
	        this.running = source["running"];
	        this.pid = source["pid"];
	        this.exitCode = source["exitCode"];
	        this.output = source["output"];
	    }
	}
	export class State {
	    auditReports: Record<string, quality.AuditReport>;
	    auditBrowsers: Record<string, quality.BrowserSessionStatus>;
	    projects: Project[];
	    selected: string;
	    trusted: boolean;
	    capture: Capture;
	    runs: Run[];
	    services: services.ServiceInstance[];
	    docker: services.DockerStatus;
	    settings: storage.Settings;
	    tunnel: sharing.TunnelStatus;
	    error: string;
	    loading: boolean;
	    dependencies: dependencies.Status[];
	    installation: DependencyInstallation;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.auditReports = this.convertValues(source["auditReports"], quality.AuditReport, true);
	        this.auditBrowsers = this.convertValues(source["auditBrowsers"], quality.BrowserSessionStatus, true);
	        this.projects = this.convertValues(source["projects"], Project);
	        this.selected = source["selected"];
	        this.trusted = source["trusted"];
	        this.capture = this.convertValues(source["capture"], Capture);
	        this.runs = this.convertValues(source["runs"], Run);
	        this.services = this.convertValues(source["services"], services.ServiceInstance);
	        this.docker = this.convertValues(source["docker"], services.DockerStatus);
	        this.settings = this.convertValues(source["settings"], storage.Settings);
	        this.tunnel = this.convertValues(source["tunnel"], sharing.TunnelStatus);
	        this.error = source["error"];
	        this.loading = source["loading"];
	        this.dependencies = this.convertValues(source["dependencies"], dependencies.Status);
	        this.installation = this.convertValues(source["installation"], DependencyInstallation);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

