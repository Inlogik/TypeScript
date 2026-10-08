module github.com/microsoft/TypeScript/tsc

go 1.27

require (
	g3pix.com.br/axonasp/v2 v2.0.0
	github.com/Microsoft/go-winio v0.6.2
	github.com/google/go-cmp v0.7.0
	github.com/klauspost/compress v1.20.1
	github.com/mackerelio/go-osstat v0.2.8
	github.com/peter-evans/patience v0.3.0
	github.com/zeebo/xxh3 v1.1.0
	golang.org/x/sync v0.23.0
	golang.org/x/sys v0.48.0
	golang.org/x/term v0.46.0
	golang.org/x/text v0.42.0
	gotest.tools/v3 v3.5.2
)

require (
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/matryer/moq v0.7.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/tools v0.50.0 // indirect
)

tool (
	github.com/matryer/moq
	golang.org/x/tools/cmd/stringer
)

// Preserve AxonASP's declared module path while pinning the GitHub fork.
// Inlogik/axonasp main at a4d4d2b02ebd52e52e2e45e9eaa5a8df8a7b7955.
replace g3pix.com.br/axonasp/v2 => github.com/Inlogik/axonasp/v2 v2.0.0-20261007235150-a4d4d2b02ebd
