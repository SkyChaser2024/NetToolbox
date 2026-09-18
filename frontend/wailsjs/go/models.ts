export namespace adapters {
	
	export class Adapter {
	    deviceName: string;
	    name: string;
	    description: string;
	    mac: string;
	    ipv4: string[];
	    ipv6: string[];
	    up: boolean;
	    recommended: boolean;
	    interfaceIndex: number;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new Adapter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceName = source["deviceName"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.mac = source["mac"];
	        this.ipv4 = source["ipv4"];
	        this.ipv6 = source["ipv6"];
	        this.up = source["up"];
	        this.recommended = source["recommended"];
	        this.interfaceIndex = source["interfaceIndex"];
	        this.kind = source["kind"];
	    }
	}
	export class Result {
	    adapters: Adapter[];
	    npcapAvailable: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.adapters = this.convertValues(source["adapters"], Adapter);
	        this.npcapAvailable = source["npcapAvailable"];
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

export namespace main {
	
	export class AuthRequest {
	    deviceName: string;
	    adapterLabel: string;
	    localMac: string;
	    username: string;
	    password: string;
	    identity: string;
	    identitySuffix: string;
	    startDelayMs: number;
	    retryDelayMs: number;
	    debug: boolean;
	    rememberPassword: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AuthRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceName = source["deviceName"];
	        this.adapterLabel = source["adapterLabel"];
	        this.localMac = source["localMac"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.identity = source["identity"];
	        this.identitySuffix = source["identitySuffix"];
	        this.startDelayMs = source["startDelayMs"];
	        this.retryDelayMs = source["retryDelayMs"];
	        this.debug = source["debug"];
	        this.rememberPassword = source["rememberPassword"];
	    }
	}
	export class BootstrapData {
	    profile: settings.Profile;
	    diagnostics: settings.DiagnosticSettings;
	    system: settings.SystemPreferences;
	    adapters: adapters.Adapter[];
	    networkInterfaces: systemnet.NetworkInterface[];
	    npcapAvailable: boolean;
	    adapterError?: string;
	    configurationError?: string;
	    cachedOverview?: networkdiag.OverviewResult;
	    authState: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new BootstrapData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], settings.Profile);
	        this.diagnostics = this.convertValues(source["diagnostics"], settings.DiagnosticSettings);
	        this.system = this.convertValues(source["system"], settings.SystemPreferences);
	        this.adapters = this.convertValues(source["adapters"], adapters.Adapter);
	        this.networkInterfaces = this.convertValues(source["networkInterfaces"], systemnet.NetworkInterface);
	        this.npcapAvailable = source["npcapAvailable"];
	        this.adapterError = source["adapterError"];
	        this.configurationError = source["configurationError"];
	        this.cachedOverview = this.convertValues(source["cachedOverview"], networkdiag.OverviewResult);
	        this.authState = source["authState"];
	        this.version = source["version"];
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
	export class PingRequest {
	    sessionId: string;
	    target: string;
	    protocol: string;
	    count: number;
	    timeoutMs: number;
	    intervalMs: number;

	    static createFrom(source: any = {}) {
	        return new PingRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.target = source["target"];
	        this.protocol = source["protocol"];
	        this.count = source["count"];
	        this.timeoutMs = source["timeoutMs"];
	        this.intervalMs = source["intervalMs"];
	    }
	}
	export class SettingsRequest {
	    profile: settings.Profile;
	    diagnostics: settings.DiagnosticSettings;
	    system: settings.SystemPreferences;
	
	    static createFrom(source: any = {}) {
	        return new SettingsRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], settings.Profile);
	        this.diagnostics = this.convertValues(source["diagnostics"], settings.DiagnosticSettings);
	        this.system = this.convertValues(source["system"], settings.SystemPreferences);
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
	export class SettingsResult {
	    profile: settings.Profile;
	    diagnostics: settings.DiagnosticSettings;
	    system: settings.SystemPreferences;
	    networkInterfaces: systemnet.NetworkInterface[];
	
	    static createFrom(source: any = {}) {
	        return new SettingsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], settings.Profile);
	        this.diagnostics = this.convertValues(source["diagnostics"], settings.DiagnosticSettings);
	        this.system = this.convertValues(source["system"], settings.SystemPreferences);
	        this.networkInterfaces = this.convertValues(source["networkInterfaces"], systemnet.NetworkInterface);
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
	export class TraceRequest {
	    sessionId: string;
	    target: string;
	    protocol: string;
	    maxHops: number;
	    timeoutMs: number;
	    resolveHostnames: boolean;

	    static createFrom(source: any = {}) {
	        return new TraceRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.target = source["target"];
	        this.protocol = source["protocol"];
	        this.maxHops = source["maxHops"];
	        this.timeoutMs = source["timeoutMs"];
	        this.resolveHostnames = source["resolveHostnames"];
	    }
	}

}

export namespace networkdiag {
	
	export class IPProbeResult {
	    available: boolean;
	    address?: string;
	    latencyMs?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new IPProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.address = source["address"];
	        this.latencyMs = source["latencyMs"];
	        this.error = source["error"];
	    }
	}
	export class WebsiteProbeResult {
	    url: string;
	    host: string;
	    available: boolean;
	    address?: string;
	    statusCode?: number;
	    latencyMs?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new WebsiteProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.host = source["host"];
	        this.available = source["available"];
	        this.address = source["address"];
	        this.statusCode = source["statusCode"];
	        this.latencyMs = source["latencyMs"];
	        this.error = source["error"];
	    }
	}
	export class URLProbeResult {
	    available: boolean;
	    latencyMs?: number;
	    bytesRead?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new URLProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.latencyMs = source["latencyMs"];
	        this.bytesRead = source["bytesRead"];
	        this.error = source["error"];
	    }
	}
	export class IPv6Result {
	    connectionType: string;
	    summary: string;
	    ipv4: IPProbeResult;
	    ipv6: IPProbeResult;
	    localGlobal: string[];
	    localLink: string[];
	    dnsAAAA: boolean;
	    largePacket?: URLProbeResult;
	    sites: WebsiteProbeResult[];
	
	    static createFrom(source: any = {}) {
	        return new IPv6Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connectionType = source["connectionType"];
	        this.summary = source["summary"];
	        this.ipv4 = this.convertValues(source["ipv4"], IPProbeResult);
	        this.ipv6 = this.convertValues(source["ipv6"], IPProbeResult);
	        this.localGlobal = source["localGlobal"];
	        this.localLink = source["localLink"];
	        this.dnsAAAA = source["dnsAAAA"];
	        this.largePacket = this.convertValues(source["largePacket"], URLProbeResult);
	        this.sites = this.convertValues(source["sites"], WebsiteProbeResult);
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
	export class LatencyProbe {
	    id: string;
	    name: string;
	    host: string;
	    url: string;
	    region: string;
	    status: string;
	    address?: string;
	    statusCode?: number;
	    latencyMs?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new LatencyProbe(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.url = source["url"];
	        this.region = source["region"];
	        this.status = source["status"];
	        this.address = source["address"];
	        this.statusCode = source["statusCode"];
	        this.latencyMs = source["latencyMs"];
	        this.error = source["error"];
	    }
	}
	export class NATProbe {
	    server: string;
	    serverIp?: string;
	    endpoint?: string;
	    latencyMs?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new NATProbe(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server = source["server"];
	        this.serverIp = source["serverIp"];
	        this.endpoint = source["endpoint"];
	        this.latencyMs = source["latencyMs"];
	        this.error = source["error"];
	    }
	}
	export class NATResult {
	    status: string;
	    type: string;
	    summary: string;
	    localIp: string;
	    publicIp: string;
	    publicPort: number;
	    mappingBehavior: string;
	    filteringBehavior: string;
	    latencyMs: number;
	    serverCount: number;
	    rfc5780: boolean;
	    probes: NATProbe[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new NATResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.type = source["type"];
	        this.summary = source["summary"];
	        this.localIp = source["localIp"];
	        this.publicIp = source["publicIp"];
	        this.publicPort = source["publicPort"];
	        this.mappingBehavior = source["mappingBehavior"];
	        this.filteringBehavior = source["filteringBehavior"];
	        this.latencyMs = source["latencyMs"];
	        this.serverCount = source["serverCount"];
	        this.rfc5780 = source["rfc5780"];
	        this.probes = this.convertValues(source["probes"], NATProbe);
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
	export class PublicNetworkInfo {
	    available: boolean;
	    address?: string;
	    source?: string;
	    isp?: string;
	    asn?: number;
	    asnOrganization?: string;
	    country?: string;
	    region?: string;
	    city?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PublicNetworkInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.address = source["address"];
	        this.source = source["source"];
	        this.isp = source["isp"];
	        this.asn = source["asn"];
	        this.asnOrganization = source["asnOrganization"];
	        this.country = source["country"];
	        this.region = source["region"];
	        this.city = source["city"];
	        this.error = source["error"];
	    }
	}
	export class OverviewResult {
	    ipv4: PublicNetworkInfo;
	    ipv6: PublicNetworkInfo;
	    probes: LatencyProbe[];
	    checkedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new OverviewResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ipv4 = this.convertValues(source["ipv4"], PublicNetworkInfo);
	        this.ipv6 = this.convertValues(source["ipv6"], PublicNetworkInfo);
	        this.probes = this.convertValues(source["probes"], LatencyProbe);
	        this.checkedAt = source["checkedAt"];
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

export namespace settings {

	export class LatencyTarget {
	    id: string;
	    name: string;
	    url: string;
	    region: string;

	    static createFrom(source: any = {}) {
	        return new LatencyTarget(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.region = source["region"];
	    }
	}
	export class DiagnosticSettings {
	    latencyTargets: LatencyTarget[];
	    natServers: string[];
	    ipv4Endpoints: string[];
	    ipv6Endpoints: string[];
	    ipv6Sites: string[];
	    aaaaDomain: string;
	    ipv6LargeUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.latencyTargets = this.convertValues(source["latencyTargets"], LatencyTarget);
	        this.natServers = source["natServers"];
	        this.ipv4Endpoints = source["ipv4Endpoints"];
	        this.ipv6Endpoints = source["ipv6Endpoints"];
	        this.ipv6Sites = source["ipv6Sites"];
	        this.aaaaDomain = source["aaaaDomain"];
	        this.ipv6LargeUrl = source["ipv6LargeUrl"];
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

	export class Profile {
	    deviceName: string;
	    adapterLabel: string;
	    localMac: string;
	    username: string;
	    identity: string;
	    identitySuffix: string;
	    startDelayMs: number;
	    retryDelayMs: number;
	    debug: boolean;
	    rememberPassword: boolean;
	    passwordSet: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deviceName = source["deviceName"];
	        this.adapterLabel = source["adapterLabel"];
	        this.localMac = source["localMac"];
	        this.username = source["username"];
	        this.identity = source["identity"];
	        this.identitySuffix = source["identitySuffix"];
	        this.startDelayMs = source["startDelayMs"];
	        this.retryDelayMs = source["retryDelayMs"];
	        this.debug = source["debug"];
	        this.rememberPassword = source["rememberPassword"];
	        this.passwordSet = source["passwordSet"];
	    }
	}
	export class SystemPreferences {
	    priorityMode: string;
	    autoAuthenticate: boolean;
	    closeToTray: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SystemPreferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.priorityMode = source["priorityMode"];
	        this.autoAuthenticate = source["autoAuthenticate"];
	        this.closeToTray = source["closeToTray"];
	    }
	}

}

export namespace systemnet {
	
	export class NetworkInterface {
	    index: number;
	    name: string;
	    description: string;
	    kind: string;
	    physical: boolean;
	    up: boolean;
	    mac: string;
	    ipv4: string[];
	    ipv6: string[];
	    linkSpeedMbps: number;
	    ipv4Metric: number;
	    ipv6Metric: number;
	
	    static createFrom(source: any = {}) {
	        return new NetworkInterface(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.kind = source["kind"];
	        this.physical = source["physical"];
	        this.up = source["up"];
	        this.mac = source["mac"];
	        this.ipv4 = source["ipv4"];
	        this.ipv6 = source["ipv6"];
	        this.linkSpeedMbps = source["linkSpeedMbps"];
	        this.ipv4Metric = source["ipv4Metric"];
	        this.ipv6Metric = source["ipv6Metric"];
	    }
	}

}
