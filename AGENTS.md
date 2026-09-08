# AGENTS.md

`krknctl` is the Krkn Go CLI and shared Go library. This file covers this
repository; the marked ecosystem section is reusable across projects.

## Start Here

Read `PROJECT.toml` if present, then `go.mod` and the relevant build/test/CI
definitions. Use them to verify versions, commands, and paths; do not treat old
architecture notes as proof of current behavior. Flag conflicts that change the
task's requirements. Read more-specific instructions before editing a subtree.

## Shared Krkn Ecosystem Structure

<!-- SHARED ECOSYSTEM CONTEXT
Copy this entire section unchanged into each ecosystem AGENTS.md.
Keep repository-specific instructions outside these markers.
-->

Paths are relative to the common workspace; confirm which checkouts are available.

| Repository path | Responsibility |
| --- | --- |
| `krkn-operator-ecosystem/krkn-operator` | Main Kubernetes/OpenShift operator and REST API. |
| `krkn-operator-ecosystem/krkn-operator-console` | Operator web UI. |
| `krkn-operator-ecosystem/krkn-operator-acm` | ACM integration operator. |
| `krknctl` | Krkn CLI and shared Go library used across the Go projects. |
| `krkn` | Python core, containerized and orchestrated by `krknctl` and `krkn-operator`. |

- Put a change in the repository that owns the behavior. Share Go logic through
  existing `krknctl` APIs when appropriate; do not duplicate it in consumers or
  introduce speculative shared abstractions.
- For exported Go API changes, inspect affected consumers and their pinned module
  versions. A sibling checkout does not mean a build uses its local code.
- Changes to core scenario/container contracts can affect both orchestrators.
  REST API changes require checking affected console and ACM consumers.
- Preserve compatibility unless the task includes a coordinated breaking change.
  Read each affected repository's instructions before working there; this map
  does not authorize unrelated edits, dependency upgrades, or releases.
- Inspect only consumers relevant to the changed contract. Report unavailable
  checkouts and unverified integration assumptions. Do not commit temporary local
  module replacements.
- When explicitly updating this shared block, keep authorized copies consistent
  and identify copies that could not be updated.

<!-- END SHARED ECOSYSTEM CONTEXT -->

## Repository Guide

Use this map to locate the implementation, not as an exhaustive file inventory.

| Area | Starting points |
| --- | --- |
| CLI entry, commands, flags | `main.go`, `cmd/` |
| Embedded configuration and image selection | `pkg/config/` |
| Registry providers | `pkg/provider/`: `factory/`, `quay/`, `registryv2/`, `models/` |
| Runtime abstraction and scenario lifecycle | `pkg/scenarioorchestrator/`, including `podman/` and `docker/` |
| Workflow ordering and random generation | `pkg/dependencygraph/`, `pkg/randomgraph/` |
| Shared validation and helpers | `pkg/typing/`, `pkg/utils/` |
| Lightspeed deployment and GPU detection | `pkg/assist/`, `pkg/gpudetect/` |
| Assistance container builds and indexing | `containers/assist/` |

## Contracts to Preserve

- Keep CLI wiring in `cmd/` and reusable behavior in appropriate packages.
  Exported library code must return errors, not terminate its caller's process.
  Preserve CLI flags, environment variables, exit codes, and output contracts.
- Reuse embedded configuration for images, endpoints, ports, and timeouts.
  Validate new inputs; do not silently disable authentication or TLS checks.
- Keep Podman/Docker differences behind the runtime abstraction. Check both
  implementations when changing shared lifecycle behavior, including cancellation,
  partial startup failure, and cleanup. Account for Linux/Darwin socket paths.
- Preserve host-side GPU detection, CPU fallback, and `--no-gpu` without device
  mounting. Verify current image mappings in code. Do not conflate macOS arm64's
  Podman/Apple-Silicon path, Podman's NVIDIA CDI path, and Docker DeviceRequest.
- Preserve Lightspeed's cached/offline indexing path. Changes to indexing stages
  must account for dependencies and verify the resulting indexed sources.
- Preserve dependency ordering and documented failure, parallelism, and seed
  semantics in workflows. Test the changed execution behavior, not only parsing.
- Add regression tests for fixes and tests for new public behavior, including
  error paths. Document changed exported APIs and user-visible options.

## Search and RTK

- RTK is mandatory whenever it provides a wrapper for the command being run.
  Use `rtk` for all supported search, filesystem, Git, test, lint, build,
  package-manager, and language-tool commands to minimize human-readable output
  and token usage. This includes `rtk rg`, `rtk find`, `rtk git`, `rtk test`,
  `rtk npm`/`rtk npx`, and `rtk go` where applicable.
- Do not use the native command merely out of habit when an RTK wrapper exists.
  Use the native command only when no suitable wrapper exists, exact unfiltered
  output is required, or the command is a file-content/script input operation.
  For a supported command that needs raw output, use `rtk proxy` and state why.
- Start with scoped `rg --files` and `rg -n`; avoid dumping entire repositories.
  Read applicable instruction files completely and inspect relevant code bodies.
- Check RTK availability once when needed. Prefer supported wrappers for noisy
  human-readable output, such as `rtk git status`, `rtk git log -n 10`, or
  `rtk go test` with the project's original arguments.
- Preserve environment variables, build tags, package selection, and exit status.
  Do not add a new linter or change test scope just because RTK supports it.
- Use native commands, or `rtk proxy` when bypassing an installed rewrite hook,
  for exact file contents, final diff review, and output consumed by scripts or
  JSON parsers. Do not use signature-only summaries as editing evidence.
- If a summary is insufficient, inspect the reported raw/tee log first. Rerun only
  a safe, narrow diagnostic; never replay a mutating command just to recover output.
- If RTK is missing or incompatible, use the original command. Do not install or
  reconfigure it as an incidental part of an unrelated task.

## Validation

Use checked-in task/CI commands when available. The commands below are fallbacks
from the supplied project instructions; confirm them against the current checkout.
Use the Go version required by `go.mod`/CI, not a minimum copied into this file.

During iteration, replace `./...` with the actual affected package(s). Broaden to
the full suite for shared API or cross-cutting changes when safe prerequisites are
available. Format only changed Go files with `gofmt`.

```bash
go build -tags containers_image_openpgp ./...
CGO_ENABLED=0 go test -tags containers_image_openpgp ./...
go vet -tags containers_image_openpgp ./...
staticcheck -tags containers_image_openpgp -checks all ./...
gosec -tags containers_image_openpgp -exclude G402 ./...
```

Match CI's environment and configured checks. Do not add lint tools or broaden
security exclusions. For supported RTK versions, the test command becomes:

```bash
CGO_ENABLED=0 rtk go test -tags containers_image_openpgp ./...
```

For a coverage run, preserve the project's coverage configuration:

```bash
CGO_ENABLED=0 go test -tags containers_image_openpgp -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

Add `-json -v` when a report consumer requires it, and keep that stream unfiltered.
The coverage file alone is not a reason to bypass RTK; confirm wrapper support
and inspect the artifact. Do not commit coverage output.

For platform/runtime/build changes, also check the relevant CI cross-builds
(Linux amd64 and Darwin arm64 in the supplied instructions), preserving release
flags and `CGO_ENABLED=0` where configured. Use a supported native race-check
configuration for concurrency changes; do not combine `-race` with CGO disabled.

Container-, cluster-, and GPU-backed tests need matching infrastructure. Inspect
test setup first. Run chaos/integration tests only against an explicitly
designated disposable target; an available kubeconfig is not permission to use it.

## Scope and Completion

- For explanations/reviews, inspect and report without making changes. For an
  implementation request, make focused edits and run safe relevant checks.
- Preserve unrelated user edits. Do not commit, push, publish, install system
  dependencies, or modify live clusters unless the user authorizes that action.
  Keep secrets out of outputs and artifacts.
- If this checkout configures Beads, follow its project label and existing
  workflow; do not initialize another tracker or invent a database location.
- In the supplied ecosystem workspace, `.beads` is an existing symlink to the
  shared central Beads database. Treat the symlink target as authoritative:
  inspect it before writes, and never replace the symlink, run `bd init`, or
  create a repository-local database as incidental setup.
- The repository's `beads/` directory is only a JSONL export for local reference
  or the configured synchronization workflow; it is not an independent Beads
  database. Do not use it as a substitute database or import it automatically.
- Finish after the requested behavior and relevant checks are covered. Review the
  complete diff and report changes, checks passed/failed/not run, and remaining
  compatibility risks. State blockers rather than claiming unexecuted tests passed.
