---
name: gocontext
description: AST-aware Go codebase explorer. Use ONLY for structural Go queries — understanding packages, finding type definitions, reading function bodies, tracing symbol references. Do NOT use for text search, non-Go files, or when you already know the exact file to read.
user-invocable: true
allowed-tools: Bash, Read
argument-hint: <tree|package|symbol|body|refs|overview> [args...] [--json]
---

# gocontext — Go AST Explorer

`gocontext` is an AST extraction CLI for Go codebases. It parses Go source using `go/ast` and returns structural information.

The binary is at the repository root: `./gocontext` (build with `go build -o gocontext .` if missing).

## When to use gocontext

- Orienting in an unfamiliar Go codebase → `./gocontext overview .`
- Understanding what a package exports → `./gocontext package <path>`
- Getting the exact definition of a type/struct/interface → `./gocontext symbol <path> <Name>`
- Getting a complete function or method body → `./gocontext body <path> <Name>`
- Finding where a Go identifier is used → `./gocontext refs . <Name>`

## When NOT to use — use built-in tools instead

- Searching for string literals, log messages, error strings → `Grep`
- Reading a file you already know the path to → `Read`
- Non-Go files → `Read`/`Grep`
- Editing code → `Edit`/`Write`
- Small packages (1-2 files) where `Read` is just as fast

## Commands

All commands support `--json`. Paths accept filesystem paths or import paths.

```
./gocontext overview .                    # full codebase snapshot
./gocontext tree .                        # list packages with docs
./gocontext package <path>                # exported symbols for one package
./gocontext symbol <path> <Name>          # exact type definition source
./gocontext body <path> <Func>            # exact function source (Type.Method for methods)
./gocontext refs . <Name>                 # all usages with file:line and kind
```

## Handling /gocontext invocations

When the user invokes `/gocontext $ARGUMENTS`:

- If `$ARGUMENTS` is empty → run `./gocontext overview .`
- If it starts with a command name → run `./gocontext $ARGUMENTS`
- Otherwise → interpret as a question and pick the right command(s)

If a command fails, read the error. Use `./gocontext tree .` to find correct package paths.
