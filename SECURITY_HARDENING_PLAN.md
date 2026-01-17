# Security Hardening Plan

**Created:** 2026-01-17
**Based on:** Gemini Code Assist Review Analysis
**Priority:** CRITICAL

---

## Executive Summary

Gemini identified **44 issues** across 5 closed PRs, with **95.5% unaddressed**. This plan focuses on:

1. **Immediate** (Critical): Fix 5 XSS vulnerabilities
2. **Short-term** (High): Add backend validation, fix race condition
3. **Medium-term**: Code quality improvements
4. **Long-term**: Architectural improvements

---

## Phase 1: Critical Security Fixes (THIS PR)

**Estimated effort:** 2-3 hours
**Priority:** CRITICAL

### 1.1 Fix All XSS Vulnerabilities (5 instances)

#### Files Affected:
- `cmd/server/web/app.js`

#### Vulnerable Functions:

1. **`renderMobileZoneResults()`** - Lines ~564-622
   - `result.zones[0].inputs.countryOfOrigin` - unescaped
   - `zone.zoneName` - unescaped

2. **`renderDesktopZoneResults()`** - Similar pattern
   - Same zone data unescaped

3. **`openAddBrandModal()` / `editBrand()`** - Lines ~1853-1902
   - `<option>` tags created via template literals
   - `t.countryName` rendered without escaping

4. **`populateReferenceTables()`** - Lines ~366-427
   - `countryName`, `notes`, `brandName` all unescaped in table rows

#### Fix Strategy:

**Option A: Use escapeHtml() (Quick fix)**
```javascript
// Before
html += `<div>${zoneName}</div>`;

// After
html += `<div>${escapeHtml(zoneName)}</div>`;
```

**Option B: Programmatic DOM (Best practice)**
```javascript
// Instead of innerHTML, create elements
const option = document.createElement('option');
option.value = t.id;
option.textContent = t.countryName; // Safe - uses textContent
selectElement.appendChild(option);
```

**Recommendation:** Use Option A for immediate fix, plan Option B migration.

### 1.2 Fix Race Condition in Confirmation Modal

**Location:** `cmd/server/web/app.js` - `showConfirm()` function

**Issue:** Global `confirmResolve` variable overwritten by second call

**Fix:**
```javascript
// Before - VULNERABLE
let confirmResolve = null;

function showConfirm(message, options) {
    return new Promise((resolve) => {
        confirmResolve = resolve; // ❌ Overwrites previous
        // ...
    });
}

// After - SAFE
function showConfirm(message, options) {
    return new Promise((resolve) => {
        let localResolve = resolve; // ✅ Scoped

        const confirmBtn = document.getElementById('confirmButton');
        const cancelBtn = overlay.querySelector('.btn:not(#confirmButton)');

        // Create unique handlers
        const handleConfirm = () => {
            overlay.classList.remove('active');
            cleanup();
            localResolve(true);
        };

        const handleCancel = () => {
            overlay.classList.remove('active');
            cleanup();
            localResolve(false);
        };

        const cleanup = () => {
            confirmBtn.removeEventListener('click', handleConfirm);
            cancelBtn.removeEventListener('click', handleCancel);
            // Remove escape handler too
        };

        confirmBtn.addEventListener('click', handleConfirm);
        cancelBtn.addEventListener('click', handleCancel);
    });
}
```

### 1.3 Add Backend Validation for Foreign Keys

**Location:** `internal/handlers/handlers.go` - `createBrand()`, `updateBrand()`

**Current Issue:** No validation that `primaryCoo` exists in `tariff_rates` table

**Fix:**

1. Add method to `internal/database/db.go`:
```go
func (db *DB) TariffCountryExists(countryName string) (bool, error) {
    var count int
    err := db.QueryRow(
        `SELECT COUNT(*) FROM tariff_rates
         WHERE LOWER(country_name) = LOWER(?)`,
        countryName,
    ).Scan(&count)

    if err != nil {
        return false, err
    }

    return count > 0, nil
}
```

2. Update handlers in `internal/handlers/handlers.go`:
```go
func (h *Handlers) createBrand(w http.ResponseWriter, r *http.Request) {
    // ... existing code to decode request ...

    // NEW: Validate foreign key
    exists, err := h.DB.TariffCountryExists(req.PrimaryCoo)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    if !exists {
        http.Error(w,
            fmt.Sprintf("Invalid country: %s does not exist in tariff rates", req.PrimaryCoo),
            http.StatusBadRequest)
        return
    }

    // Proceed with creation
}
```

---

## Phase 2: Code Quality Improvements (NEXT PR)

**Priority:** MEDIUM
**Estimated effort:** 4-6 hours

### 2.1 Extract Inline Styles to CSS

**Issue:** 8 instances of inline styles in HTML strings

**Files:**
- `cmd/server/web/app.js` - `displayCalculationResult()`, modal headers
- `cmd/server/web/index.html` - Card headers

**Fix:** Create utility classes in CSS, replace inline styles

### 2.2 Replace Inline Event Handlers

**Issue:** 3 instances of `onclick` attributes

**Fix:** Use event delegation pattern

### 2.3 Replace Hardcoded Values

**Issue:** 4 instances - colours, zone IDs, paths

**Fix:**
- Colours → CSS variables
- Zone IDs → Configuration
- Paths → Environment variables or constants

---

## Phase 3: Architectural Improvements (FUTURE)

**Priority:** LOW (Long-term)

### 3.1 Adopt Routing Library

Replace manual URL path parsing with `chi` or `gorilla/mux`

**Benefit:** Cleaner code, automatic parameter extraction

### 3.2 Consider Frontend Framework

**Options:**
- **Htmx** - Minimal JS, server-driven (good fit)
- **Alpine.js** - Lightweight, modern
- **Vue.js** - Full framework with auto-escaping

**Benefit:** Prevents entire class of XSS vulnerabilities

### 3.3 Automated Security Scanning

**Setup:**
- ESLint with `eslint-plugin-no-unsanitized`
- Gosec for Go security scanning
- Pre-commit hooks with Husky
- GitHub Actions CI/CD security checks

---

## Shift-Left Strategy

### 1. IDE/Editor (Instant Feedback)

**ESLint Configuration** (`.eslintrc.json`):
```json
{
  "plugins": ["no-unsanitized"],
  "rules": {
    "no-unsanitized/method": "error",
    "no-unsanitized/property": "error"
  }
}
```

**VS Code Settings** (`.vscode/settings.json`):
```json
{
  "eslint.enable": true,
  "eslint.validate": ["javascript"],
  "editor.codeActionsOnSave": {
    "source.fixAll.eslint": true
  }
}
```

### 2. Pre-Commit Phase (Git Hooks)

**Husky + lint-staged** (`package.json`):
```json
{
  "husky": {
    "hooks": {
      "pre-commit": "lint-staged"
    }
  },
  "lint-staged": {
    "*.js": ["eslint --fix", "git add"],
    "*.go": ["gofmt -w", "go vet", "git add"]
  }
}
```

### 3. PR Phase (CI/CD)

**GitHub Actions** (`.github/workflows/security.yml`):
```yaml
name: Security Checks

on: [pull_request]

jobs:
  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Run ESLint
        run: npm run lint

      - name: Run Gosec
        uses: securego/gosec@master
        with:
          args: './...'

      - name: Run Snyk Security Scan
        uses: snyk/actions/golang@master
        env:
          SNYK_TOKEN: ${{ secrets.SNYK_TOKEN }}
```

### 4. Code Review Phase

**PR Template Checklist** (`.github/pull_request_template.md`):
```markdown
## Security Review

- [ ] No innerHTML usage without escapeHtml()
- [ ] No inline event handlers (onclick, onchange, etc.)
- [ ] Backend validates all foreign key references
- [ ] No hardcoded credentials or secrets
- [ ] No SQL string concatenation (use parameterized queries)

## Code Quality

- [ ] No inline styles (extracted to CSS)
- [ ] No magic numbers/strings (use constants)
- [ ] No global namespace pollution
```

---

## Implementation Checklist

### Phase 1 (Critical - This PR)

- [ ] Fix XSS in `renderMobileZoneResults()`
- [ ] Fix XSS in `renderDesktopZoneResults()`
- [ ] Fix XSS in `openAddBrandModal()` / `editBrand()`
- [ ] Fix XSS in `populateReferenceTables()`
- [ ] Fix race condition in `showConfirm()`
- [ ] Add `TariffCountryExists()` to database
- [ ] Add validation to `createBrand()` handler
- [ ] Add validation to `updateBrand()` handler
- [ ] Test all fixes manually
- [ ] Commit and create PR
- [ ] Update CLAUDE.md with security checklist ✅ (Done)

### Phase 2 (Code Quality - Next PR)

- [ ] Extract inline styles to CSS classes
- [ ] Replace onclick handlers with event delegation
- [ ] Replace hardcoded colour with CSS variable
- [ ] Create zone configuration instead of hardcoded checks
- [ ] Standardize tariff rate precision

### Phase 3 (Tooling - Future)

- [ ] Setup ESLint with security plugins
- [ ] Setup Husky + lint-staged
- [ ] Create GitHub Actions security workflow
- [ ] Create PR template with security checklist
- [ ] Setup Gosec for Go scanning
- [ ] Evaluate frontend framework migration

---

## Success Metrics

**Phase 1 Complete:**
- [ ] All 5 XSS vulnerabilities fixed
- [ ] Race condition resolved
- [ ] Backend validation implemented
- [ ] All manual tests pass
- [ ] No new security issues introduced

**Phase 2 Complete:**
- [ ] No inline styles in HTML strings
- [ ] No onclick attributes in generated HTML
- [ ] All colours use CSS variables
- [ ] Code duplication reduced

**Long-term Success:**
- [ ] Automated security checks in CI/CD
- [ ] Pre-commit hooks prevent vulnerable code
- [ ] Zero Gemini security findings on new PRs

---

## Resources

- **Gemini Analysis:** `GEMINI_CODE_REVIEW_ANALYSIS.md`
- **ESLint Plugin:** https://github.com/mozilla/eslint-plugin-no-unsanitized
- **OWASP XSS Guide:** https://cheatsheetseries.owasp.org/cheatsheets/Cross_Site_Scripting_Prevention_Cheat_Sheet.html
- **Gosec:** https://github.com/securego/gosec
