# test-plan.md — Example

## Strategy

### Testing Pyramid

- **Unit (60%)**: Business logic, utilities, component behavior
- **Integration (30%)**: API endpoints, DB operations, service boundaries
- **E2E (10%)**: Critical execution paths only

---

## Test Categories

### Unit Tests

**Coverage Target**: 80%
**Command**: `make test-unit`
**Pass Condition**: coverage ≥ 80%, exit code 0

**Scope**:
- Authentication logic and token handling
- Data validation and transformation functions
- Business rule calculations
- Error handling branches
- All utility functions

---

### Integration Tests

**Coverage Target**: 70% of API surface
**Command**: `make test-integration`
**Pass Condition**: all assertions pass, exit code 0
**Isolation**: each suite runs against a clean DB state (transaction rollback or test container)

**Scope**:
- All API endpoints: happy path + error cases
- Database operations: CRUD, constraints, rollback behavior
- External service calls: use mocks, verify contract compliance

---

### E2E Tests

**Command**: `make test-e2e`
**Pass Condition**: all critical paths complete, exit code 0
**Environment**: staging or ephemeral environment with seeded data

**Critical Paths** (must all pass before deploy):
1. User registration → email verification → login
2. [Primary feature flow from spec]
3. Error recovery: invalid auth token → redirect to login

---

### Performance Tests

**Command**: `make test-perf`
**Pass Condition**: all thresholds met

```javascript
// k6 thresholds (non-negotiable)
thresholds: {
  http_req_duration: ['p(95)<500'],  // 95th percentile under 500ms
  http_req_failed:   ['rate<0.01'],  // error rate under 1%
}
```

---

### Security Tests

**Command**: `make test-security`
**Pass Condition**: 0 high/critical vulnerabilities, exit code 0

**Automated Checks**:
- Dependency vulnerability scan (`govulncheck ./...`)
- OWASP ZAP baseline scan against staging
- Auth bypass attempt suite
- Rate limiting verification
- Input sanitization (SQL injection, XSS payloads)

---

## Test Data

**Strategy**: each test suite manages its own data lifecycle

| Level       | Data Source                               | Reset Strategy              |
|-------------|-------------------------------------------|-----------------------------|
| Unit        | inline fixtures, no DB                    | n/a                         |
| Integration | factory functions + transaction rollback  | per-test rollback           |
| E2E         | seeded staging DB                         | full reset between runs     |

**Isolation requirement**: No test may depend on state left by another test.
