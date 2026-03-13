## Go Codebase Exploration with gocontext

This project has `gocontext` available — an AST-aware CLI for Go. Before using it, check it is installed by running `which gocontext`. If not found, fall back to Grep/Read and suggest: `go install github.com/onurattopsort/gocontext/cmd/gocontext@latest`

### When to use gocontext

Use gocontext when you need **structural understanding** of Go code — it parses the AST, so it understands packages, exports, types, and function boundaries. It is the right tool when:

- You need to orient in an unfamiliar codebase → `gocontext overview .`
- You need to know what a package exports without reading every file → `gocontext package <path>`
- You need the exact definition of a type/struct/interface → `gocontext symbol <path> <Name>`
- You need a complete function or method body with precise boundaries → `gocontext body <path> <Name>`
- You need to find where a symbol is used across the codebase (AST-level, not text) → `gocontext refs . <Name>`

### When NOT to use gocontext

Do NOT use gocontext for these — use the built-in tools instead:

- **Searching for string literals, log messages, error strings, or config keys** → use `Grep`. gocontext only finds Go identifiers, not arbitrary text.
- **Reading a specific file you already know the path to** → use `Read`. Don't round-trip through gocontext.
- **Editing or writing code** → gocontext is read-only.
- **Non-Go files** (YAML, JSON, Markdown, Dockerfiles, etc.) → use `Read`/`Grep`.
- **You already know exactly which file and function to look at** → just `Read` the file. gocontext helps when you don't know where things are.
- **Small packages with 1-2 files** → reading the file directly is about the same cost.

### Commands reference

All commands support `--json` for structured output.

```
gocontext overview .                    # full codebase snapshot — types + function signatures for every package
gocontext tree .                        # list all packages with doc synopses
gocontext package <path>                # exported symbols and docs for one package
gocontext symbol <path> <Name>          # exact source of a type definition
gocontext body <path> <Func>            # exact source of a function/method (use Type.Method for methods)
gocontext refs . <Name>                 # all usages of a symbol with file:line, context, and kind
```
