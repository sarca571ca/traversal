export namespace app {
	
	export class Character {
	    id: number;
	    name: string;
	    currentSystem: string;
	
	    static createFrom(source: any = {}) {
	        return new Character(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.currentSystem = source["currentSystem"];
	    }
	}
	export class MapConnection {
	    id: number;
	    systemA: number;
	    systemB: number;
	    whTypeA: string;
	    whTypeB: string;
	    sigIdA: string;
	    sigIdB: string;
	    shipSize: string;
	    massStatus: string;
	    eol: boolean;
	    expiresAt: number;
	
	    static createFrom(source: any = {}) {
	        return new MapConnection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.systemA = source["systemA"];
	        this.systemB = source["systemB"];
	        this.whTypeA = source["whTypeA"];
	        this.whTypeB = source["whTypeB"];
	        this.sigIdA = source["sigIdA"];
	        this.sigIdB = source["sigIdB"];
	        this.shipSize = source["shipSize"];
	        this.massStatus = source["massStatus"];
	        this.eol = source["eol"];
	        this.expiresAt = source["expiresAt"];
	    }
	}
	export class SystemNode {
	    id: number;
	    name: string;
	    security: number;
	    x: number;
	    y: number;
	    chars: number[];
	
	    static createFrom(source: any = {}) {
	        return new SystemNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.security = source["security"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.chars = source["chars"];
	    }
	}
	export class MapData {
	    systems: SystemNode[];
	    connections: MapConnection[];
	
	    static createFrom(source: any = {}) {
	        return new MapData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.systems = this.convertValues(source["systems"], SystemNode);
	        this.connections = this.convertValues(source["connections"], MapConnection);
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
	export class MapViewport {
	    tx: number;
	    ty: number;
	    scale: number;
	
	    static createFrom(source: any = {}) {
	        return new MapViewport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tx = source["tx"];
	        this.ty = source["ty"];
	        this.scale = source["scale"];
	    }
	}
	export class RouteJump {
	    systemId: number;
	    systemName: string;
	    jumpType: string;
	
	    static createFrom(source: any = {}) {
	        return new RouteJump(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.systemId = source["systemId"];
	        this.systemName = source["systemName"];
	        this.jumpType = source["jumpType"];
	    }
	}
	export class RouteResult {
	    jumps: RouteJump[];
	    totalJumps: number;
	
	    static createFrom(source: any = {}) {
	        return new RouteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jumps = this.convertValues(source["jumps"], RouteJump);
	        this.totalJumps = source["totalJumps"];
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
	export class SigEntry {
	    id: number;
	    sigId: string;
	    sigType: string;
	    notes: string;
	
	    static createFrom(source: any = {}) {
	        return new SigEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sigId = source["sigId"];
	        this.sigType = source["sigType"];
	        this.notes = source["notes"];
	    }
	}

}

