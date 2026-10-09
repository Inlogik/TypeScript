// Collections support both Classic ASP call syntax and enumeration properties.
// Item values remain permissive for multi-valued fields and COM compatibility.
interface ASPRequestCollection {
    /** Reads a value by name or numeric index. Multi-valued items remain permissive COM values. */
    (nameOrIndex: string | number): any;
    /** Number of keys in this collection. Classic ASP collection indices are normally one-based. */
    readonly Count: number;
    /** Reads a collection item by name or numeric index. */
    Item(nameOrIndex: string | number): any;
    /** Returns the name of the key at the given collection index. */
    Key(index: number): string;
}
interface ASPCookieCollection extends ASPRequestCollection {
    (): any;
}
interface ASPRequest {
    /** Looks up a named request value using the host's combined request collections. Prefer Form or QueryString when the source matters. */
    (name: string): any;
    /** Values supplied in the URL query string. Supports call lookup, Count, Item and Key. */
    QueryString: ASPRequestCollection;
    /** Submitted form values. Multipart uploads may require an application-specific parser. */
    Form: ASPRequestCollection;
    /** Request/environment metadata, for example REQUEST_METHOD, SCRIPT_NAME and HTTP_USER_AGENT. */
    ServerVariables: ASPRequestCollection;
    /** Cookies sent by the client. Item/subkey values remain permissive for COM compatibility. */
    Cookies: ASPCookieCollection;
    [name: string]: any;
}
interface ASPResponse {
    /** Writes a value to the HTTP response body. The host converts the value to text. @param value Text or another value to write. */
    Write(value: any): void;
    /** Sends a redirect to the client. @param url Destination URL, relative or absolute as supported by the host. */
    Redirect(url: string): void;
    /** Stops processing the current ASP request. */
    End(): void;
    /** Adds an HTTP response header. Set headers before the response is committed. */
    AddHeader(name: string, value: string): void;
    /** Appends text to the host's request log, where supported. */
    AppendToLog(value: string): void;
    /** Writes binary data without text encoding. Payload representation depends on the host/COM object. */
    BinaryWrite(value: any): void;
    /** Clears buffered response-body output. It does not undo content already sent to the client. */
    Clear(): void;
    /** Sends buffered output to the client. Later header changes may no longer be possible. */
    Flush(): void;
    /** Whether response output is buffered before being sent to the client. */
    Buffer: boolean;
    /** Cache-control policy, commonly "private" or "public". */
    CacheControl: string;
    /** Character-set name advertised in the response Content-Type, for example "UTF-8". */
    Charset: string;
    // Classic ASP/IIS supports this casing alias for the same response property.
    /** Classic ASP casing alias of Charset; both access the same response property. */
    CharSet: string;
    /** Code page used for response text conversion, for example 65001 for UTF-8. */
    CodePage: number;
    /** Response MIME type, for example "text/html", "text/plain" or "application/json". */
    ContentType: string;
    /** Response cookies. Cookie values and attributes remain permissive COM-style items. */
    Cookies: ASPCookieCollection;
    /** Relative cache expiration time in minutes. */
    Expires: number;
    /** Absolute response cache expiration date/time. */
    ExpiresAbsolute: any;
    /** Whether the client connection is still available, according to the host. */
    readonly IsClientConnected: boolean;
    /** Legacy PICS content-rating label/header value, where supported. */
    PICS: string;
    /** HTTP status line, for example "200 OK" or "404 Not Found". */
    Status: string;
}
// The setter overloads exist only in the virtual checker representation.
// Returning T preserves the value/type of a JavaScript assignment expression.
interface ASPContents {
    /** Reads an application/session value by key or collection index. */
    (key: string | number): any;
    /** Checker-only setter representation. Production ASP uses Contents(key) = value. Returns the assigned value. */
    <T>(key: string, value: T): T;
    /** Number of stored entries. */
    Count: number;
    /** Removes the entry with this key. */
    Remove(key: string): void;
    /** Removes all entries from the collection. */
    RemoveAll(): void;
    /** COM-style item access; indexing/call behavior is host-dependent. */
    Item: any;
    /** COM-style key enumeration by collection index. */
    Key: any;
}
interface ASPApplication {
    /** Reads shared application state by key. Values may be visible to other requests. */
    (key: string): any;
    /** Checker-only setter overload for Application(key) = value; not a portable runtime two-argument API. */
    <T>(key: string, value: T): T;
    /** Shared application key/value collection. */
    Contents: ASPContents;
    /** Objects declared with application scope in the host's application configuration. */
    StaticObjects: any;
    /** Acquires the host's application-state lock. Pair with Unlock, including error paths. */
    Lock(): void;
    /** Releases a lock previously acquired through Application.Lock(). */
    Unlock(): void;
}
interface ASPSession {
    /** Reads a value from the current user's session by key. */
    (key: string): any;
    /** Checker-only setter overload for Session(key) = value; not a portable runtime two-argument API. */
    <T>(key: string, value: T): T;
    /** Current session's key/value collection. */
    Contents: ASPContents;
    /** Objects declared with session scope by the host. */
    StaticObjects: any;
    /** Abandons the current session according to host lifecycle behavior. */
    Abandon(): void;
    /** Session text-conversion code page. */
    CodePage: number;
    /** Locale identifier used for locale-sensitive operations. */
    LCID: number;
    /** Host-issued identifier for the current session; representation is host-dependent. */
    readonly SessionID: any;
    /** Session inactivity timeout in minutes. */
    Timeout: number;
}
interface ASPServer {
    /** Resolves a virtual/relative application path to a physical filesystem path. */
    MapPath(path: string): string;
    /** Creates a registered COM/compatibility object by ProgID. Members remain any unless typed by a project contract. */
    CreateObject(progId: string): any;
    /** Encodes a value for HTML text. This is not JavaScript, URL or SQL encoding. */
    HTMLEncode(value: any): string;
    [name: string]: any;
}
/** Current HTTP request: query/form values, cookies and server metadata. Supplied by the ASP host. */
declare const Request: ASPRequest;
/** Current HTTP response: output, headers, redirects and cookies. Supplied by the ASP host. */
declare const Response: ASPResponse;
/** ASP server utilities for path resolution, HTML encoding and COM object creation. */
declare const Server: ASPServer;
// JScript COM activation. Object members stay permissive until typed per ProgID.
declare class ActiveXObject {
    /** Creates a registered COM/compatibility object. @param progId Registered identifier such as "Scripting.FileSystemObject". */
    constructor(progId: string);
    [name: string]: any;
}
/** Per-user session state supplied by the ASP host. */
declare const Session: ASPSession;
/** Shared application state supplied by the ASP host; use locking for coordinated mutations where required. */
declare const Application: ASPApplication;
declare function __aspWrite(value: any): void;
// Virtual-only indexed setter: the getter call still checks target and indices;
// the assignment result has the exact RHS type. This is not a runtime API.
declare function __aspSetIndexed<T>(indexedValue: any, value: T): T;
declare function __aspDeleteIdentifier(value: any): true;
