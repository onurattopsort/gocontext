---
name: gocontext
description: Explore Go codebase structure using AST extraction. Use when you need to understand packages, find symbols, read function bodies, find references, or navigate a Go project's structure progressively.
user-invocable: true
allowed-tools: Bash, Read
argument-hint: <tree|package|symbol|body|refs|overview> [args...] [--json]
---

# gocontext — Go AST Explorer

You have access to the `gocontext` CLI tool built from this repository. Use it for progressive disclosure of Go codebases — start broad with `overview` or `tree`, narrow to a package, then drill into specific symbols or function bodies.

The binary is at the repository root: `./gocontext` (build with `go build -o gocontext .` if missing).

All commands support `--json` for structured output.

## Commands

### 1. `overview [dir]` — Full codebase snapshot
Single-call summary of every package with exported types and function signatures. Best starting point.
```bash
./gocontext overview .
./gocontext overview . --json
```

### 2. `tree [dir]` — List packages
Shows all packages with their doc synopses.
```bash
./gocontext tree .
./gocontext tree ./internal
```

### 3. `package <path>` — Inspect a package
Shows exported types, functions, and their signatures with docs. Does NOT show function bodies.
Accepts filesystem paths or full import paths.
```bash
./gocontext package ./internal/inspect
./gocontext package github.com/onurattopsort/gocontext/cmd
```

### 4. `symbol <path> <name>` — Get a type definition
Shows the exact Go source for a type, struct, interface, const, or var.
```bash
./gocontext symbol ./internal/inspect PackageDetail
./gocontext symbol ./cmd Execute
```

### 5. `body <path> <name>` — Get function source
Shows the full source code of a function or method. Use `Type.Method` syntax for methods.
```bash
./gocontext body ./internal/inspect ResolveImportPath
./gocontext body ./internal/inspect Body
```

### 6. `refs <dir> <name>` — Find all references
Finds every usage of a symbol across the codebase with file, line, column, context, and classification.
```bash
./gocontext refs . ResolveImportPath
./gocontext refs . PackageDetail --json
```

## Workflow

Follow this progressive disclosure pattern:

1. **Snapshot**: `./gocontext overview .` for the full picture in one call
2. **Orient**: `./gocontext tree .` to see all packages
3. **Narrow**: `./gocontext package <path>` to see what a package exports
4. **Inspect**: `./gocontext symbol <path> <Name>` to read a type definition
5. **Deep dive**: `./gocontext body <path> <Name>` to read implementation
6. **Trace**: `./gocontext refs . <Name>` to find where a symbol is used

## Handling the user's request

When the user invokes `/gocontext $ARGUMENTS`:

- If `$ARGUMENTS` is empty, run `./gocontext overview .` and present the codebase summary
- If `$ARGUMENTS` starts with `tree`, `package`, `symbol`, `body`, `refs`, or `overview`, run `./gocontext $ARGUMENTS`
- Otherwise, interpret the argument as a question about the codebase and use the appropriate gocontext commands to answer it

Always present the output cleanly. If a command fails, read the error message and try to correct the path or symbol name.
