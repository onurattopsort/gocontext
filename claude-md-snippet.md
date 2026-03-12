## Go Codebase Exploration

When working in this Go project, use the `gocontext` CLI for structural queries instead of grep or reading whole files. It is faster, more precise, and produces token-efficient output.

Use it proactively — do not wait for the user to ask. Pick the right command for the task:

- `gocontext overview .` — start here when orienting in an unfamiliar codebase. Returns every package with exported types and function signatures in one call.
- `gocontext tree .` — list all packages with doc synopses.
- `gocontext package <path>` — see all exported symbols and docs for a single package. Use instead of reading multiple files.
- `gocontext symbol <path> <Name>` — get the exact source of a type, struct, interface, const, or var definition.
- `gocontext body <path> <Func>` — get the full source of a function or method. Use `Type.Method` for methods.
- `gocontext refs . <Name>` — find every usage of a symbol across the codebase with file:line and context.

Add `--json` when you need to parse the output programmatically.

### When to use gocontext vs built-in tools

| Task | Use |
|---|---|
| "What does this codebase do?" | `gocontext overview .` |
| "What does package X export?" | `gocontext package X` |
| "Show me the Config struct" | `gocontext symbol <path> Config` |
| "Show me the handler function" | `gocontext body <path> Handler` |
| "Where is Foo used?" | `gocontext refs . Foo` |
| Searching for a string literal or log message | `Grep` (gocontext is for structural queries, not text search) |
| Reading non-Go files | `Read` |

If `gocontext` is not installed, fall back to Grep/Read and suggest: `go install github.com/onurattopsort/gocontext/cmd/gocontext@latest`
