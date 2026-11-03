# Jina Adapter Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Rename `rerank-proxy` to `jina-proxy` and separate model formatting from serving backend integration, retaining exactly one implementation of each: Qwen3 and OVMS.

**Architecture:** Keep the generated Jina rerank API as the frontend. Compose a configured model formatter and serving backend at startup; the HTTP handler owns Jina validation and response mapping, the formatter owns Qwen3 input formatting, and the backend owns OVMS transport and readiness. Each deployment selects one formatter and backend.

**Tech Stack:** Existing Go version and dependencies, oapi-codegen, net/http, GitHub Actions, Docker Hub, OpenTofu in `infra/github`, and Kubernetes GitOps in `kubernetes/registry`.

**Spec:** The user's requirements in this conversation: project/repository/image name `jina-proxy`; pluggable model formatter and serving backend; only existing Qwen3 and OVMS implementations.

## Global Constraints

- Implement only `qwen3` and `ovms`; no identity formatter, additional backend, dynamic plugin loader, or multi-model routing.
- Preserve Jina rerank behavior, generated-operation selection, Qwen3 prompt bytes, configured model, limits, timeout, health behavior, and pinned deployment images until replacement images are published.
- Keep formatting separate from transport. KServe manages the OVMS container; the backend protocol remains OVMS.
- Preserve GitHub repository identity and history. Rename the managed repository in place through OpenTofu; never destroy and recreate it.
- Keep the current Kubernetes adapter Deployment name `jina-qwen-rerank-adapter` and LibreChat Service `reranker`.
- Preserve unrelated local changes across all repositories. Inspect current branch/remotes and refresh remote state before commits or pushes.

## Review Focus

- Unknown or empty explicit plugin names must not silently select another implementation.
- Query/document prompts must be applied exactly once and originals retained for returned documents.
- Cancellation and timeout must reach both readiness and inference HTTP requests.
- Invalid upstream indices or malformed responses must retain existing error handling.
- GitHub rename and image cutover must preserve repository secrets, workflow ownership, and existing runnable image pins.

## File Map and Interfaces

Paths below are relative to `/Users/al/Work/repo/m11s`; application paths use the current `m11s-io-github/rerank-proxy` directory until its final rename.

- `internal/rerank/types.go`: protocol-neutral `Request{Model string; Query string; Documents []string; TopN *int}` and `Result{Index int; RelevanceScore float64}`.
- `internal/formatter/formatter.go`: `Formatter` interface with `Format(instruction, query string, documents []string) (string, []string)`; `New(name string) (Formatter, error)` recognizes only `qwen3`.
- `internal/formatter/qwen3.go`: existing prompt implementation moved from `internal/qwen/format.go`; return newly allocated document strings.
- `internal/backend/backend.go`: `Backend` interface with `Rerank(context.Context, rerank.Request) ([]rerank.Result, error)` and `Ready(context.Context) error`; typed errors distinguish upstream transport/status failures from malformed responses.
- `internal/backend/ovms.go`: current OVMS request/response encoding, bounded response reads, HTTP transport, and `/v2/health/ready` handling. `New(name string, cfg config.Config) (Backend, error)` recognizes only `ovms` and owns an HTTP client with the configured timeout.
- `internal/server/server.go`: `New(cfg config.Config, formatter formatter.Formatter, backend backend.Backend) http.Handler`; Jina parsing, input limits, formatting invocation, result-index validation, and Jina response generation.
- `internal/config/config.go`: add `MODEL_FORMATTER` (default `qwen3`) and `SERVING_BACKEND` (default `ovms`). Preserve existing configuration and defaults.
- `cmd/jina-proxy/main.go`: construct configuration, formatter, backend, then HTTP handler; fail startup on invalid selectors.

### Task 1: Introduce formatter and backend boundaries

**Files:** Create the interface/implementation files above; move existing Qwen formatter and tests; modify configuration, handler, entrypoint, and their tests.

- [ ] Add formatter contract tests pinning the exact current Qwen3 query/document output, including Unicode, empty strings, ordering, and no mutation of input documents. Observe failures before extracting the existing implementation.
- [ ] Extract the existing Qwen3 implementation behind `Formatter`; no changes to prompt text or score interpretation.
- [ ] Add real `httptest.Server` backend tests that capture the OVMS request, check model/query/documents/top_n and `return_documents:false`, and return representative OVMS results. Cover readiness URL construction, non-2xx status, malformed JSON, bounded reads, cancellation, and timeout. Observe failures before implementing the backend.
- [ ] Move upstream HTTP and readiness operations out of the frontend handler into the OVMS backend. Preserve error messages/statuses exposed by the existing frontend; document the typed backend error mapping.
- [ ] Extend configuration/factory tests for defaults and unknown selectors. Explicit empty selector values must fail; absent variables use defaults. Validate plugin names before starting the HTTP listener.
- [ ] Update frontend tests to exercise the real Qwen3 formatter and OVMS backend against a local HTTP server. Check exact single prompt application, original returned documents, top_n, invalid result indices, configured upstream model versus echoed Jina model, and existing body/document limits.
- [ ] Run `go test -race ./...`, `go vet ./...`, and `go generate ./...`; verify the generated API file has no diff. Commit the interface extraction after these checks pass.

### Task 2: Rename application source and publication name

**Files:** `go.mod`, Go imports, `cmd/rerank-proxy/main.go`, `Dockerfile`, `README.md`, `.github/workflows/build.yml` (confirm actual filename), and any active source references found by `rg`.

- [ ] Change the Go module/import prefix to `github.com/m11s-io/jina-proxy`; rename the command directory and executable to `jina-proxy`; update Docker build/COPY/ENTRYPOINT and startup log text.
- [ ] Publish future images as `m11s/jina-proxy:sha-<12-character-commit>` using the existing workflow and credentials. Preserve immutable digest pinning for deployment.
- [ ] Update README title, build examples, architecture, selector configuration, supported implementations, and the limited Jina rerank surface. Retain the pinned OpenAPI source and generation filter.
- [ ] Run the Go checks from Task 1 and build the renamed Docker image. Exercise `/healthz`, `/readyz`, and one Jina rerank request with a local upstream fixture; confirm expected Qwen3-formatted OVMS payload and mapped response.
- [ ] Commit the rename locally. Coordinate publication with the infrastructure rename in Task 3 before pushing to the renamed repository.

### Task 3: Rename the managed GitHub repository

**Files:** `infra/github/repositories.tf`, `renovate.tf`, `files/renovate-repositories.json`, `files/renovate.example.tfvars`, and any active Renovate settings referencing the old name.

- [ ] Keep the OpenTofu resource address `github_repository.rerank_proxy` initially; change its `name` to `jina-proxy`, description to describe Jina API proxying, and relevant topics. The resource address can remain internal and avoids an unnecessary state-address migration.
- [ ] Update name-based Renovate lists. Inspect resources indexed by repository name, especially `github_repository_file.renovate_workflow` and repository secrets; add explicit `moved` blocks for old-to-new for_each keys where supported and needed. Do not let a renamed key delete a managed workflow from the renamed repository.
- [ ] Run `tofu fmt` on touched files and `tofu validate`; generate and inspect a saved plan with configured credentials. Require an in-place repository rename, preserved repository identity/secrets, and no repository destruction or unrelated changes.
- [ ] Present the reviewed plan before applying external infrastructure changes. Apply the exact reviewed plan once authorized; verify the repository's ID, name, Actions secrets availability, and Renovate workflow ownership.
- [ ] Update the local Git remote to `git@github.com:m11s-io/jina-proxy.git`, then move the local directory to `m11s-io-github/jina-proxy`. Check linked worktree metadata before moving; use the appropriate Git worktree move mechanism if applicable.
- [ ] Push the application commit and confirm its main-branch workflow publishes `m11s/jina-proxy`. Record the published immutable digest before proceeding.

### Task 4: Deploy the renamed image and explicit plugin selection

**Files:** `kubernetes/registry/apps/reranker/base/resources/workloads/deployment.yaml`; `m11s-io-github/helm-charts/charts/ovms/README.md` (upstream adapter link).

- [ ] Refresh registry branch state carefully: the previous migration was pushed as `ced54bf7a`, while the original local checkout may retain the patch-equivalent pre-rebase commit. Preserve unrelated Timestral edits.
- [ ] Point the adapter Deployment at the newly published `m11s/jina-proxy` tag and digest, add `MODEL_FORMATTER=qwen3` and `SERVING_BACKEND=ovms`, and update the source-repository comment. Keep its existing service URL, model pin, instruction, resource settings, and Kubernetes identity.
- [ ] Update the Helm chart README link to the renamed repository; handle publication/versioning under that repository's instructions if publishing a documentation change.
- [ ] Parse changed YAML, run `git diff --check`, inspect the scoped GitOps diff, and run repository-required checks. Report existing unrelated resource-layout failures separately.
- [ ] Commit/push the deployment cutover within the approved delivery scope. Observe Argo CD reconciliation and adapter/predictor readiness, then submit a representative rerank request and verify score/order and returned original documents. Keep old published images available for rollback.

## Completion Criteria

- Repository and newly published image use `jina-proxy`; local directory and Go module agree with the repository name.
- Jina frontend code has no direct Qwen3 or OVMS implementation dependency; startup composes interfaces using configuration.
- Exactly one formatter (`qwen3`) and backend (`ovms`) are implemented, with explicit invalid-selector errors.
- Existing reranking behavior and deployment contracts are preserved, meaningful Go checks pass, and the new image is verified before its deployment pin changes.
- Infrastructure rename is performed through a reviewed OpenTofu plan with repository identity retained; historical design/verification records retain their original names.
