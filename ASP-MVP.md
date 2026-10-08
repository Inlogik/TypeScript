# Classic ASP / JScript check-only MVP

## Local layout

- This repository: TypeScript `asp-tsc` branch.
- `tsc/go.mod` preserves the AxonASP module import path and replaces it with
  `github.com/Inlogik/axonasp/v2 v2.0.0-20261007235150-a4d4d2b02ebd`, pinned to
  published `origin/main` commit `a4d4d2b02ebd52e52e2e45e9eaa5a8df8a7b7955`.
  No neighbouring AxonASP checkout is required for compiler builds. OSV returned
  no known vulnerabilities for this exact version on 2026-10-08.
- Use `GOWORK=off` for this spike so the upstream tools workspace does not
  introduce unrelated tooling dependencies.
- The AxonASP module graph selects `compress v1.20.1` and `cpuid/v2 v2.4.0`
  instead of upstream's older pins; these exact versions were checked with OSV.

## Companion declarations and shared-global cleanup lint

For every entry/include source, the checker automatically discovers
`<original filename>.d.ts` (for example `customer.inc.d.ts`). Global-script
ambient declarations and interfaces in that companion are loaded only into entry
programs containing that source. They are also included in IntelliSense and tracked
as dependencies. This is an external contract, not an automatic implementation
annotation; use JSDoc/source-type rules to check matching implementations.

`--warn-shared-globals` enables opt-in resolved-symbol variable dependency lint
(code 90001). Global variable reads/writes across original source files warn;
local/parameter shadowing, same-file globals, host/global project declarations and
function dependencies are excluded. A companion's `declare var` acknowledges the
dependency only for its corresponding source; sibling references still warn.
Variable types nevertheless participate in the whole entry's shared global scope.
Keep types compatible with actual parent declarations. Companion contracts assume
the external value exists and do not verify runtime initialization or call order.

Example companion:

```ts
// customer.inc.d.ts
interface Customer { id: number; name: string; }
declare var customers: Customer[];
```

Use global-script companions (no import/export) for this MVP. Imported/module
declaration resolution in the concatenated IntelliSense view is not yet supported.

## Batch checking

```powershell
asp-tsc --project tools/asp-typecheck/asp-check.json --scan site --jobs 2 --errors-only
```

`--scan` recursively checks .asp files case-insensitively as independent entry
programs. Shared includes are never merged across page scopes. `--jobs` bounds
concurrency (1-16; default 2). Project paths resolve relative to the config file;
explicit CLI options override project defaults. `--project` without a page uses
its configured entry. Scan paths are relative to the caller's working directory.

Text prints unique original-source diagnostics with affected-page counts and one
summary line. JSON adds per-page reports, unique diagnostics with entry lists,
clean/error/failed counts, error/warning counts, and elapsed seconds. Warnings-only
pages are clean. `--errors-only` hides warning details, not their summary count.
Exit 0 means no errors/failures, 1 means compiler errors, 2 means scan/input failures.
All discovered .asp files are entries, even those normally used as includes; no
exclude patterns or shared-file caching are implemented in this first version.

## Build and check (WSL)

```bash
cd /mnt/d/Dev/typescript/tsc
GOWORK=off ~/.local/go/bin/go build -o ../built/local/asp-tsc ./cmd/asp-tsc
../built/local/asp-tsc --noEmit \
  --root internal/contentmapper/asp/testdata \
  internal/contentmapper/asp/testdata/default.asp
```

The fixture deliberately reports TS2339 at `default.asp(9,18)` and
`helpers.asp(4,21)`. A separate regression test checks `Response.YouWrite`.
Exit codes: 0 = clean, 1 = compiler diagnostics,
2 = usage, input, include, or unsupported-language error.

`--errors-only` hides warnings from console output. Checking and exit codes are
unchanged: warnings alone return 0, errors return 1, input/usage failures return 2.
Diagnostics are sorted after mapping: errors first, then warnings; each group
uses original filename, line, column, code, and message. Fileless errors precede
file diagnostics. Filename ordering is lexical; expanded include order is ignored.

`--json` emits a version-1 report with entry, dependency paths, diagnostics, and
optional error. Diagnostic start/end positions are one-based UTF-16. Includes,
type files and project rule files are dependencies for check-on-save clients.
Input/mapper failures are structured errors (exit 2); invalid CLI usage remains
human-readable flag output. The extension invokes known valid arguments.

The VS Code client is maintained in a separate repository. Application-specific
declarations and rules belong in the application's own versioned configuration.

`--virtual-json` provides inference-enabled JS, separate host/project declarations,
original source text and UTF-16 span offsets for editor forwarding. It deliberately
does not use loose-variable diagnostic inference. No source is emitted to disk by
the compiler; the editor stores temporary documents locally for its JS service.

`--source-overlay FILE.json` accepts an absolute-filename/source-text map for
virtual export or normal checking. Included ASP source can use editor snapshots
without writing production files. Declaration/project rule files load from disk.

Windows build (from the same module directory):

```powershell
$env:GOWORK = 'off'
& C:\Go\bin\go.exe build -o ..\built\local\asp-tsc.exe ./cmd/asp-tsc
..\built\local\asp-tsc.exe --noEmit --root internal/contentmapper/asp/testdata internal/contentmapper/asp/testdata/default.asp
```

## Scope and boundaries

### Legacy loose-variable mode

Unannotated function/method parameters default to any in virtual input, regardless
of loose-variable mode. Explicit @param/inline types and callable project @type
contracts are preserved, including partially typed signatures. A virtual undefined
default preserves optional omitted-argument behavior for unannotated identifier
parameters; extra arguments still fail unless a variadic contract is supplied.
Existing defaults/rest parameters are preserved. No runtime/production signature
is modified. Type annotations inside JSDoc callable signatures are not rewritten.

`--loose-variables` is opt-in and inserts virtual `@type {any}` on unannotated
variable declarations after project source annotations. Variables can change
between string/number/object values. Explicit JSDoc and project annotations are
preserved (conservatively, any attached JSDoc prevents loosening). Parameters,
function declarations, host declarations and production sources are unchanged.

Important: unannotated aliases of typed values lose member/call checks, including
objects holding functions. Annotate important models/helpers to retain checking.
Direct Response typo checks and explicitly annotated models remain checked.
For-in declarations are left inferred: native checking rejects a type annotation
on that binding with TS2404. This mode does not suppress that restriction.

The usual inferred mode remains available by omitting the flag.

- One entry page, represented as in-memory JavaScript, with `allowJs`,
  `checkJs`, `noEmit`, and the ES5 standard library (no browser DOM globals).
  The compiler target is ES2015, its minimum supported target; nothing is emitted.
- Existing JSDoc is preserved. The permissive embedded `classic-asp.d.ts`
  describes runtime globals, not application domain models.
- Response member names are explicit (no string-index catch-all), so misspelled
  methods such as `YouWrite` are errors. Cookies and values remain permissive.
- Request QueryString/Form/ServerVariables and cookies have callable collection
  declarations with Count, Key, and Item members; string names and numeric indices
  are accepted. Item payloads remain any; unknown collection member names fail.
  Known host Count() calls normalize to the numeric Count property, matching a
  verified AxonASP runtime behavior. Cookies also support an omitted lookup key.
- `--source-types FILE.json` injects project-owned variable JSDoc in virtual input
  only (requires `--root`). Entries have root-relative `source`, `variable`, and
  `type`; corresponding interfaces are supplied with `--types`. Initializers
  remain checked, so incompatible contracts surface rather than being cast away.
  Alternatively `source`, `function`, `variadic:true` appends a virtual rest
  parameter to the scoped function/property implementation. Existing parameters
  and body checking remain intact; normal functions are not made variadic globally.
  Function rules may instead supply `jsdoc` containing complete JSDoc comment
  blocks. These are inserted before the scoped declaration/property in virtual
  text. Non-comment/executable text is rejected. Explicit overloaded @type with
  an any implementation return can express native function-expression overloads;
  that intentional return-checking tradeoff belongs to the project configuration.
  Assignment-statement rules can target dotted function paths (for example
  `Array.prototype.indexOf`) for project-owned optional-parameter annotations.
- Repeated includes still expand textually, but identical mapped diagnostic
  filename/location/code/message/severity tuples are reported once per entry.
- TS2403 redeclaration type conflicts in mapped ASP are warnings. External .d.ts
  conflicts and other assignment errors are unchanged; this severity policy does
  not change inferred bindings or cast redeclared values to any.
- TS2367 for equality/inequality comparisons specifically between typeof and
  "date" or "Function" is advisory (including reversed operands). AxonASP dates
  report "object" and callables report lowercase "function"; historical IIS
  behavior is unverified. Other impossible comparisons retain their severity.
- Non-strict `private`/`public` variable identifiers are renamed only in virtual
  input, following verified AxonASP acceptance. Property keys/member names and
  explicit strict scopes remain unchanged; generated names avoid source collisions.
- JScript member declarations such as `function Object.example(...)` become
  virtual member assignments with function expressions. Names, bodies and JSDoc
  retain source positions; converted declarations receive statement terminators.
- Repeatable `--types FILE.d.ts` adds project ambient declarations. Paths are
  relative to the caller's working directory. For example:
  `asp-tsc --types project-legacy.d.ts --root site site/pm/default.asp`.
  No application declarations are added to the generic ASP host types.
- Optional `--dependency-rules FILE.json` loads root-relative conditional include
  contracts owned by the project (requires `--root`). Rules match original source,
  enclosing function, and unresolved identifier; unrelated references stay errors.
  Rules contain `source`, `consumer`, `names`, `provider`, `condition`, and optional
  `requiredEntries`. Known required entries retain errors when dependencies are
  missing. This is explicit project policy, not automatic reachability analysis.
  No rule is enabled by default and application names are not built into the checker.
- Plain call-shaped indexed assignments (`target(key) = value`, including COM
  members and multiple indices) become `__aspSetIndexed(target(key), (value))`
  only in virtual input. Getter calls still check targets/indices; the helper
  preserves RHS types and original source spans. Nested assignments, parentheses,
  and comma expressions are supported; compound/update and optional-chain targets
  are intentionally not rewritten. Setter writability/value contracts remain
  permissive: this normalization does not prove a target supports COM setters.
- Strict mode, implicit-any errors, and unknown catch variables are explicitly
  disabled: current compiler defaults otherwise make unannotated legacy code noisy.
  Explicit JSDoc type errors are still checked.
- Unresolved names with a plain non-strict assignment in the expanded entry are
  reported as possible-implicit-global warnings, including non-strict references
  to the same name. Names with no such assignment and strict/class references
  remain errors. This is a compatibility heuristic, not proof of execution order.
  Warnings do not fail the CLI exit code; actual errors still return 1.
  Bare non-strict `for (name in object)` / `for (name of values)` targets are
  also assignment evidence. Declared loop variables and strict loops are unchanged.
- String-continuation whitespace is normalized to match AxonASP's runtime rule,
  composing exact original spans. No application whitespace edits are needed.
- Bare-identifier deletion in non-strict source becomes a true-valued virtual
  helper, matching AxonASP's no-op behavior. Identifier names remain checked.
  Explicit strict prologues/class bodies and property deletion are not normalized.
- File and virtual includes are textual, recursively expanded in source order.
  Repeated includes are allowed; cycles and depths beyond 32 are rejected.
  Virtual paths require `--root` and cannot lexically escape it.
- ASP delimiters, language attributes, server script endings, and include
  attribute recognition reuse AxonASP's source parser helpers.
- Every copied script span retains its original filename and byte position;
  diagnostics display one-based lines and UTF-16 columns, including includes.
- No parser/binder/checker changes, runtime execution changes, application-source
  annotations, generated JavaScript files, or compiler emit calls.
- The separate `asp-tsc` entry point uses a compiler host with an in-memory JS
  input. Regular `tsc`, tsconfig `.asp` registration, watch, and LSP integration
  are deliberately not changed in this first spike. Native span maps are
  single-original-file; the MVP's composed span table preserves include filenames.

## Limitations

- UTF-8 input only (optional UTF-8 BOM); other Classic ASP code pages rejected.
- JScript/JavaScript server blocks only. Declare the page language explicitly;
  AxonASP's default is VBScript. Server script tags must specify their language.
- ASP percent endings follow runtime behavior (`%>` ends a block even in a
  JavaScript string). Script-tag endings follow the runtime's quote/comment rules.
- COM/ADODB and most host members remain `any`; no promise of ES3/JScript-runtime
  compatibility for code accepted by the modern JavaScript checker.
- Includes follow current runtime recognition, including its permissive comment
  parser. This is not an untrusted-file sandbox; file includes can use `..`, and
  symlinks are not confined by the virtual-root lexical check.
- Synthetic diagnostic positions anchor to nearby source spans; related
  information chains and editor ranges are future work.

## Focused verification

```bash
cd /mnt/d/Dev/axonasp
~/.local/go/bin/go test ./vbscript -run TestScanASP -count=1
cd /mnt/d/Dev/typescript/tsc
GOWORK=off ~/.local/go/bin/go test ./internal/contentmapper/asp ./cmd/asp-tsc -count=1
```

## Verification recorded 2026-10-07

- AxonASP base: `63c0494b06c543aed37f83bc4247a7665d171a68`.
- TypeScript base: `21b260b4e7e5123727f33d506fa686fecc258b1a`.
- Go: 1.27.1, Windows and WSL.
- New scanner/include-comment focused tests: passed.
- Full `vbscript` package and focused runtime include-resolution/compiler tests:
  passed.
- Broader runtime Include/JScript tests had seven failures, all reproduced on
  the unchanged AxonASP base in a detached worktree: missing real-page fixture,
  cookie fallback, three Node FS sandbox tests, missing form-field behavior,
  and proxy operations. These are baseline failures, not introduced by this spike.
- ASP mapper and CLI tests: passed, including JSDoc, both include modes,
  nested include locations, case-insensitive paths, fragment expansion,
  repeated includes, cycles, traversal rejection, missing files, unsupported
  language, truncated blocks, syntax diagnostics, BOM/CRLF/Unicode mapping,
  clean cross-block control flow, exit codes, and no source writes.
- Native compiler, parser, existing content mapper, and spanmap package tests:
  passed. This is focused verification, not the complete upstream test suite.
- Windows binary built and reported precisely the two expected TS2339 errors.
  Exit 1 is intentional for the deliberately invalid fixture.
- Windows ASP mapper and CLI package tests passed as well as WSL tests.
- WSL `asp-tsc` and regular `tsc` builds passed; focused `go vet` passed.
- `git diff --check` passed in both repositories.
