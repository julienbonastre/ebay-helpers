# Husky Git Hooks

This directory contains Git hooks managed by Husky to enforce code quality and security checks before commits.

## Setup

Run `npm install` to set up the hooks automatically.

## Hooks

### pre-commit
- Runs ESLint with security plugins on JavaScript files
- Runs `gofmt` and `go vet` on Go files
- Blocks commits if security issues are found

## Bypassing Hooks (Emergency Only)

If you absolutely must bypass the hooks (not recommended):

```bash
git commit --no-verify -m "message"
```

**Warning:** Only use `--no-verify` in emergencies. All bypassed commits will still be caught by CI/CD security checks.
