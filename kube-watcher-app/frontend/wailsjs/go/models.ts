export namespace data {
	
	export class AIPlaybook {
	    title: string;
	    prompt: string;
	    target: string;
	    description: string;
	    commands: string[];
	
	    static createFrom(source: any = {}) {
	        return new AIPlaybook(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.prompt = source["prompt"];
	        this.target = source["target"];
	        this.description = source["description"];
	        this.commands = source["commands"];
	    }
	}
	export class AIPriority {
	    title: string;
	    severity: string;
	    detail: string;
	    actionLabel: string;
	
	    static createFrom(source: any = {}) {
	        return new AIPriority(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.severity = source["severity"];
	        this.detail = source["detail"];
	        this.actionLabel = source["actionLabel"];
	    }
	}
	export class AnomalyDeploymentPlan {
	    profile: string;
	    title: string;
	    summary: string;
	    mode: string;
	    namespace: string;
	    services: string[];
	    commands: string[];
	    validation: string[];
	    artifacts: string[];
	
	    static createFrom(source: any = {}) {
	        return new AnomalyDeploymentPlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = source["profile"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.mode = source["mode"];
	        this.namespace = source["namespace"];
	        this.services = source["services"];
	        this.commands = source["commands"];
	        this.validation = source["validation"];
	        this.artifacts = source["artifacts"];
	    }
	}
	export class AIWorkspaceSummary {
	    headline: string;
	    subheadline: string;
	    openAlerts: number;
	    criticalAlerts: number;
	    automationReady: number;
	    deployTargets: number;
	
	    static createFrom(source: any = {}) {
	        return new AIWorkspaceSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.headline = source["headline"];
	        this.subheadline = source["subheadline"];
	        this.openAlerts = source["openAlerts"];
	        this.criticalAlerts = source["criticalAlerts"];
	        this.automationReady = source["automationReady"];
	        this.deployTargets = source["deployTargets"];
	    }
	}
	export class AIWorkspace {
	    summary: AIWorkspaceSummary;
	    priorities: AIPriority[];
	    playbooks: AIPlaybook[];
	    deploymentPlans: AnomalyDeploymentPlan[];
	
	    static createFrom(source: any = {}) {
	        return new AIWorkspace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summary = this.convertValues(source["summary"], AIWorkspaceSummary);
	        this.priorities = this.convertValues(source["priorities"], AIPriority);
	        this.playbooks = this.convertValues(source["playbooks"], AIPlaybook);
	        this.deploymentPlans = this.convertValues(source["deploymentPlans"], AnomalyDeploymentPlan);
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
	
	export class AlertRecord {
	    id: string;
	    kind: string;
	    namespace: string;
	    name: string;
	    cluster: string;
	    severity: string;
	    reason: string;
	    message: string;
	    status: string;
	    // Go type: time
	    receivedAt: any;
	    rootCause?: string;
	    summary?: string;
	    actions?: string[];
	    confidence?: number;
	
	    static createFrom(source: any = {}) {
	        return new AlertRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.namespace = source["namespace"];
	        this.name = source["name"];
	        this.cluster = source["cluster"];
	        this.severity = source["severity"];
	        this.reason = source["reason"];
	        this.message = source["message"];
	        this.status = source["status"];
	        this.receivedAt = this.convertValues(source["receivedAt"], null);
	        this.rootCause = source["rootCause"];
	        this.summary = source["summary"];
	        this.actions = source["actions"];
	        this.confidence = source["confidence"];
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
	
	export class Incident {
	    id: string;
	    // Go type: time
	    timestamp: any;
	    kind: string;
	    severity: string;
	    namespace: string;
	    name: string;
	    reason: string;
	    message: string;
	    occurrences: number;
	    history?: number[];
	    metadata?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Incident(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.kind = source["kind"];
	        this.severity = source["severity"];
	        this.namespace = source["namespace"];
	        this.name = source["name"];
	        this.reason = source["reason"];
	        this.message = source["message"];
	        this.occurrences = source["occurrences"];
	        this.history = source["history"];
	        this.metadata = source["metadata"];
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
	export class LogLine {
	    // Go type: time
	    timestamp: any;
	    level: string;
	    source: string;
	    message: string;
	    raw: string;
	
	    static createFrom(source: any = {}) {
	        return new LogLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.level = source["level"];
	        this.source = source["source"];
	        this.message = source["message"];
	        this.raw = source["raw"];
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
	export class Recommendation {
	    id: string;
	    title: string;
	    severity: string;
	    summary: string;
	    steps: string[];
	    relatedKind: string;
	    frequencyDelta: number;
	    alertRef?: string;
	
	    static createFrom(source: any = {}) {
	        return new Recommendation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.severity = source["severity"];
	        this.summary = source["summary"];
	        this.steps = source["steps"];
	        this.relatedKind = source["relatedKind"];
	        this.frequencyDelta = source["frequencyDelta"];
	        this.alertRef = source["alertRef"];
	    }
	}
	export class ServiceStatus {
	    name: string;
	    image: string;
	    status: string;
	    startedAt?: string;
	    id?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServiceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.image = source["image"];
	        this.status = source["status"];
	        this.startedAt = source["startedAt"];
	        this.id = source["id"];
	    }
	}
	export class StatusResponse {
	    status: string;
	    version: string;
	    cluster: string;
	    Latency: number;
	
	    static createFrom(source: any = {}) {
	        return new StatusResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.version = source["version"];
	        this.cluster = source["cluster"];
	        this.Latency = source["Latency"];
	    }
	}
	export class ToolResult {
	    tool: string;
	    output?: Record<string, any>;
	    raw?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.output = source["output"];
	        this.raw = source["raw"];
	        this.error = source["error"];
	    }
	}

}

