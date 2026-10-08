// Collections support both Classic ASP call syntax and enumeration properties.
// Item values remain permissive for multi-valued fields and COM compatibility.
interface ASPRequestCollection {
    (nameOrIndex: string | number): any;
    readonly Count: number;
    Item(nameOrIndex: string | number): any;
    Key(index: number): string;
}
interface ASPCookieCollection extends ASPRequestCollection {
    (): any;
}
interface ASPRequest {
    (name: string): any;
    QueryString: ASPRequestCollection;
    Form: ASPRequestCollection;
    ServerVariables: ASPRequestCollection;
    Cookies: ASPCookieCollection;
    [name: string]: any;
}
interface ASPResponse {
    Write(value: any): void;
    Redirect(url: string): void;
    End(): void;
    AddHeader(name: string, value: string): void;
    AppendToLog(value: string): void;
    BinaryWrite(value: any): void;
    Clear(): void;
    Flush(): void;
    Buffer: boolean;
    CacheControl: string;
    Charset: string;
    // Classic ASP/IIS supports this casing alias for the same response property.
    CharSet: string;
    CodePage: number;
    ContentType: string;
    Cookies: ASPCookieCollection;
    Expires: number;
    ExpiresAbsolute: any;
    readonly IsClientConnected: boolean;
    PICS: string;
    Status: string;
}
// The setter overloads exist only in the virtual checker representation.
// Returning T preserves the value/type of a JavaScript assignment expression.
interface ASPContents {
    (key: string | number): any;
    <T>(key: string, value: T): T;
    Count: number;
    Remove(key: string): void;
    RemoveAll(): void;
    Item: any;
    Key: any;
}
interface ASPApplication {
    (key: string): any;
    <T>(key: string, value: T): T;
    Contents: ASPContents;
    StaticObjects: any;
    Lock(): void;
    Unlock(): void;
}
interface ASPSession {
    (key: string): any;
    <T>(key: string, value: T): T;
    Contents: ASPContents;
    StaticObjects: any;
    Abandon(): void;
    CodePage: number;
    LCID: number;
    readonly SessionID: any;
    Timeout: number;
}
interface ASPServer {
    MapPath(path: string): string;
    CreateObject(progId: string): any;
    HTMLEncode(value: any): string;
    [name: string]: any;
}
declare const Request: ASPRequest;
declare const Response: ASPResponse;
declare const Server: ASPServer;
// JScript COM activation. Object members stay permissive until typed per ProgID.
declare class ActiveXObject {
    constructor(progId: string);
    [name: string]: any;
}
declare const Session: ASPSession;
declare const Application: ASPApplication;
declare function __aspWrite(value: any): void;
// Virtual-only indexed setter: the getter call still checks target and indices;
// the assignment result has the exact RHS type. This is not a runtime API.
declare function __aspSetIndexed<T>(indexedValue: any, value: T): T;
declare function __aspDeleteIdentifier(value: any): true;
