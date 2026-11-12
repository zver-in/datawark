# Tasks

## 1. Canonical contract

- [ ] 1.1 Add public-contract regression tests for accepted `projection`/`order` fields and rejected `select`, `columns`, `orderBy`, and `order_by` aliases; verify `go test ./internal/query/...` passes.
- [ ] 1.2 Document the canonical top-level, filter, ordering, and scalar JSON forms alongside the Query Plan API contract; verify examples conform to `query.Decode`.

## 2. Documentation alignment

- [ ] 2.1 Update PRD Query Plan examples and terminology to use `projection` and `order`; verify no conflicting Query Plan field aliases remain in PRD.
- [ ] 2.2 Update ARCH Query Plan example and transport-contract references to the canonical form; verify documentation examples match the decoder and `go test ./...` passes.

## Workflow follow-up

- Archive the change after review, before publishing a transport that accepts Query Plans.
