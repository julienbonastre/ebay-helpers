## Summary

<!-- Brief description of changes -->

## Type of Change

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Security fix (addresses security vulnerability)
- [ ] Performance improvement
- [ ] Code refactoring (no functional changes)
- [ ] Documentation update

## Security Review Checklist

**Frontend JavaScript:**
- [ ] No `innerHTML` usage with unescaped user/database data
- [ ] All dynamic HTML uses `escapeHtml()` or programmatic DOM creation
- [ ] No inline event handlers (`onclick`, `onchange`, etc.)
- [ ] No `<option>` elements created via template literals (use `createElement()`)
- [ ] All template literals in HTML contexts escape variables

**Backend Go:**
- [ ] All foreign key references validated before INSERT/UPDATE
- [ ] All database queries use parameterized statements (no string concatenation)
- [ ] URL parameters/path variables properly validated
- [ ] No sensitive data logged or exposed in error messages
- [ ] Authentication/authorization checks where required

**General Security:**
- [ ] No hardcoded credentials, API keys, or secrets
- [ ] No new uses of `eval()`, `new Function()`, or similar dynamic code execution
- [ ] CSP headers maintained or improved (no weakening)
- [ ] No SQL injection vulnerabilities
- [ ] No command injection vulnerabilities

## Code Quality Checklist

- [ ] No inline styles (extracted to CSS where possible)
- [ ] No magic numbers/strings (use constants or config)
- [ ] No global namespace pollution (`window.x` - use scoped vars)
- [ ] Consistent code formatting (`gofmt`, `eslint --fix` applied)
- [ ] No code duplication (DRY principle followed)
- [ ] Error handling implemented appropriately

## Testing

- [ ] Manually tested all changes
- [ ] Tested on multiple browsers (if frontend changes)
- [ ] Tested on mobile devices (if UI changes)
- [ ] No console errors or warnings
- [ ] Build completes successfully (`go build`)

## Breaking Changes

<!-- Describe any breaking changes and migration path -->

## Related Issues

<!-- Link to related issues using #issue_number -->
Fixes #
Relates to #

## Screenshots/Videos

<!-- If applicable, add screenshots or videos demonstrating the changes -->

## Additional Notes

<!-- Any other information reviewers should know -->

---

**For Reviewers:**
- [ ] Code follows project security guidelines (see `CLAUDE.md`)
- [ ] All security checklist items verified
- [ ] No new technical debt introduced
- [ ] Documentation updated if needed
