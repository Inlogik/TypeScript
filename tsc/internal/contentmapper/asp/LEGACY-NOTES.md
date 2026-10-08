# Real application compatibility observations

Initial read-only checks on a legacy JScript application (2026-10-07) exposed
these limitations. This file intentionally contains no proprietary source.

- Native TypeScript defaults enable stricter checking than the intended loose
  MVP. `Strict`, `NoImplicitAny`, and `UseUnknownInCatchVariables` are explicitly
  disabled; a regression test proves untyped parameters/catches remain loose.
- JScript member function declarations (`function Object.member(...)`) are not
  standard JavaScript declarations. They produce syntax errors and cascades.
  The mapper now rewrites recovered member declarations into member assignments
  with synthetic span mappings. Project constructor extensions belong in files
  supplied with `--types`; the native parser/checker remains unchanged.
- Classic ASP indexed-property assignment (`Application(key) = value` and
  similar COM properties) is not a valid JavaScript assignment target.
  Plain call-shaped assignments now map to a generic virtual setter helper,
  keeping assignment-expression value types and checking getter targets/indices.
  Compound/update operators still require dedicated handling. Target writability
  is not verified by this permissive compatibility representation.
- Legacy JSON polyfills collide with standard ES5 JSON declarations, and
  extensions to primitive prototypes require additional ambient declarations.
- The native compiler enforces always-strict JavaScript syntax independently
  of `Strict:false`; `AlwaysStrict:false` is a removed option. Legacy deletion
  of bare identifiers now maps to a true-valued helper only in non-strict source,
  matching AxonASP's verified runtime no-op. Explicit strict directives/classes
  still produce diagnostics. This is not blanket diagnostic suppression.
- Sample unresolved names in shared helpers are implicit-global assignments,
  not include/mapping errors. Automatically declaring every missing identifier
  would conceal typos; distinguish intentional globals from missing local vars
  before adding project declarations or modifying application source.
- Repeated include expansion repeats diagnostics. Cross-page reporting should
  deduplicate original filename, location, code, and message, without deduplicating
  source expansion (which would change program scope/semantics).
- Do not interpret missing identifiers after an unsupported member declaration
  as proven application defects: parsing cascades must be addressed first.
