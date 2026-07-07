export namespace main {
	
	export class BranchInfo {
	    name: string;
	    current: boolean;
	    remote: boolean;
	    default: boolean;
	    upstream: string;
	    commit: string;
	    relativeTime: string;
	    subject: string;
	
	    static createFrom(source: any = {}) {
	        return new BranchInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.current = source["current"];
	        this.remote = source["remote"];
	        this.default = source["default"];
	        this.upstream = source["upstream"];
	        this.commit = source["commit"];
	        this.relativeTime = source["relativeTime"];
	        this.subject = source["subject"];
	    }
	}
	export class BranchResponse {
	    path: string;
	    current: string;
	    branches: BranchInfo[];
	    generated: string;
	
	    static createFrom(source: any = {}) {
	        return new BranchResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.current = source["current"];
	        this.branches = this.convertValues(source["branches"], BranchInfo);
	        this.generated = source["generated"];
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
	export class ChangedFile {
	    path: string;
	    oldPath?: string;
	    status: string;
	    staged: boolean;
	    unstaged: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChangedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.oldPath = source["oldPath"];
	        this.status = source["status"];
	        this.staged = source["staged"];
	        this.unstaged = source["unstaged"];
	    }
	}
	export class CommandResult {
	    path: string;
	    command: string;
	    success: boolean;
	    message: string;
	    stdout: string;
	    stderr: string;
	    finishedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new CommandResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.command = source["command"];
	        this.success = source["success"];
	        this.message = source["message"];
	        this.stdout = source["stdout"];
	        this.stderr = source["stderr"];
	        this.finishedAt = source["finishedAt"];
	    }
	}
	export class CommitInfo {
	    hash: string;
	    author: string;
	    relativeTime: string;
	    subject: string;
	
	    static createFrom(source: any = {}) {
	        return new CommitInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.author = source["author"];
	        this.relativeTime = source["relativeTime"];
	        this.subject = source["subject"];
	    }
	}
	export class Settings {
	    lastRoot: string;
	    maxDepth: number;
	    autoRefresh: boolean;
	    refreshIntervalSeconds: number;
	    autoFetch: boolean;
	    autoPullCleanRepos: boolean;
	    onlyPullCleanRepos: boolean;
	    ideaPath: string;
	    diffDisplayByteLimit: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lastRoot = source["lastRoot"];
	        this.maxDepth = source["maxDepth"];
	        this.autoRefresh = source["autoRefresh"];
	        this.refreshIntervalSeconds = source["refreshIntervalSeconds"];
	        this.autoFetch = source["autoFetch"];
	        this.autoPullCleanRepos = source["autoPullCleanRepos"];
	        this.onlyPullCleanRepos = source["onlyPullCleanRepos"];
	        this.ideaPath = source["ideaPath"];
	        this.diffDisplayByteLimit = source["diffDisplayByteLimit"];
	    }
	}
	export class InitialState {
	    settings: Settings;
	    hasGit: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InitialState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settings = this.convertValues(source["settings"], Settings);
	        this.hasGit = source["hasGit"];
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
	export class RepoStatus {
	    added: number;
	    modified: number;
	    deleted: number;
	    renamed: number;
	    copied: number;
	    untracked: number;
	    conflicted: number;
	    staged: number;
	    unstaged: number;
	    files: ChangedFile[];
	
	    static createFrom(source: any = {}) {
	        return new RepoStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.modified = source["modified"];
	        this.deleted = source["deleted"];
	        this.renamed = source["renamed"];
	        this.copied = source["copied"];
	        this.untracked = source["untracked"];
	        this.conflicted = source["conflicted"];
	        this.staged = source["staged"];
	        this.unstaged = source["unstaged"];
	        this.files = this.convertValues(source["files"], ChangedFile);
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
	export class RepoTimings {
	    revParseMs: number;
	    statusMs: number;
	    remoteMs: number;
	    lastCommitMs: number;
	    totalMs: number;

	    static createFrom(source: any = {}) {
	        return new RepoTimings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.revParseMs = source["revParseMs"];
	        this.statusMs = source["statusMs"];
	        this.remoteMs = source["remoteMs"];
	        this.lastCommitMs = source["lastCommitMs"];
	        this.totalMs = source["totalMs"];
	    }
	}
	export class Repository {
	    id: string;
	    name: string;
	    path: string;
	    branch: string;
	    head: string;
	    upstream: string;
	    remoteName: string;
	    remoteUrl: string;
	    hasRemote: boolean;
	    hasUpstream: boolean;
	    isClean: boolean;
	    ahead: number;
	    behind: number;
	    status: RepoStatus;
	    lastCommit: CommitInfo;
	    timings: RepoTimings;
	    inspectedAt: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Repository(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.branch = source["branch"];
	        this.head = source["head"];
	        this.upstream = source["upstream"];
	        this.remoteName = source["remoteName"];
	        this.remoteUrl = source["remoteUrl"];
	        this.hasRemote = source["hasRemote"];
	        this.hasUpstream = source["hasUpstream"];
	        this.isClean = source["isClean"];
	        this.ahead = source["ahead"];
	        this.behind = source["behind"];
	        this.status = this.convertValues(source["status"], RepoStatus);
	        this.lastCommit = this.convertValues(source["lastCommit"], CommitInfo);
	        this.timings = this.convertValues(source["timings"], RepoTimings);
	        this.inspectedAt = source["inspectedAt"];
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
	export class ScanResponse {
	    root: string;
	    maxDepth: number;
	    repositories: Repository[];
	    scannedAt: string;
	    warnings?: string[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ScanResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.maxDepth = source["maxDepth"];
	        this.repositories = this.convertValues(source["repositories"], Repository);
	        this.scannedAt = source["scannedAt"];
	        this.warnings = source["warnings"];
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
	
	export class UpdateRequest {
	    paths: string[];
	    mode: string;
	    onlyClean: boolean;
	    prune: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.paths = source["paths"];
	        this.mode = source["mode"];
	        this.onlyClean = source["onlyClean"];
	        this.prune = source["prune"];
	    }
	}
	export class UpdateResult {
	    path: string;
	    mode: string;
	    skipped: boolean;
	    success: boolean;
	    message: string;
	    stdout: string;
	    stderr: string;
	    before?: Repository;
	    after?: Repository;
	    finishedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.mode = source["mode"];
	        this.skipped = source["skipped"];
	        this.success = source["success"];
	        this.message = source["message"];
	        this.stdout = source["stdout"];
	        this.stderr = source["stderr"];
	        this.before = this.convertValues(source["before"], Repository);
	        this.after = this.convertValues(source["after"], Repository);
	        this.finishedAt = source["finishedAt"];
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
