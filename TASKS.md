# RedisMeter Implementation Tasks

> Auto-generated task list for completing remaining features

---

## Phase 1: Wire Up New Services to API Server ✅

### Task 1.1: Initialize logging store in API server ✅
- [x] Add log store initialization in serve.go
- [x] Create adapter for API LogStore interface
- [x] Register log routes

### Task 1.2: Initialize infrastructure profile store in API server ✅
- [x] Add infra profile store initialization
- [x] Create adapter for API InfraProfileStore interface
- [x] Register infra profile routes

### Task 1.3: Bundle export/import (deferred)
- [ ] Bundle routes can be added later as needed

---

## Phase 2: Add CLI Commands ✅

### Task 2.1: Add `redismeter logs` command ✅
- [x] Create logs.go CLI file
- [x] Subcommands: list, show, export, clear, stats

### Task 2.2: Add `redismeter profiles` command ✅
- [x] Create profiles.go CLI file
- [x] Subcommands: list, show, create, delete, export, import, stats

### Task 2.3: Add `redismeter bundle` command ✅
- [x] Create bundle.go CLI file
- [x] Subcommands: export, import, info
- [x] Options for selecting what to export
- [x] Options for merge/overwrite behavior

---

## Phase 3: Add Missing Unit Tests

### Task 3.1: API package tests (deferred)
- [ ] Create api_test.go (deferred - complex interface mocking needed)
- [ ] Test route handlers

### Task 3.2: Reporter package tests ✅
- [x] Create reporter_test.go
- [x] Test HTML, Markdown generation

### Task 3.3: Terraform package tests (deferred)
- [ ] Create terraform_test.go (deferred - requires terraform binary)
- [ ] Test manager operations (mock)

### Task 3.4: CLI package tests (deferred)
- [ ] Create cli_test.go (deferred - CLI tests require running full command)
- [ ] Test new commands

---

## Phase 4: Documentation ✅

### Task 4.1: Update README.md ✅
- [x] Add logging feature documentation
- [x] Add infrastructure profiles documentation
- [x] Add export/import documentation
- [x] Add CLI commands documentation
- [x] Update project structure

---

## Progress Tracking

| Phase | Task | Status |
|-------|------|--------|
| 1.1 | Initialize logging store | ✅ Complete |
| 1.2 | Initialize infra profile store | ✅ Complete |
| 1.3 | Bundle routes | ⏳ Deferred |
| 2.1 | CLI logs command | ✅ Complete |
| 2.2 | CLI profiles command | ✅ Complete |
| 2.3 | CLI bundle command | ✅ Complete |
| 3.1 | API tests | ⏸ Deferred |
| 3.2 | Reporter tests | ✅ Complete |
| 3.3 | Terraform tests | ⏸ Deferred |
| 3.4 | CLI tests | ⏸ Deferred |
| 4.1 | Update README | ✅ Complete |

---

*Last updated: 2026-02-05*
