# Release Linux And Windows Arm64 Packages Plan

## Objective

- Enable official release distribution for `linux/arm64` and `windows/arm64` in addition to the currently shipped targets.
- Align GitHub release assets, install scripts, Scoop manifest behavior, and release documentation around the expanded platform matrix.
- Avoid a repeat of the recent release gap where supported targets in config and published assets diverged.

## Evidence Snapshot

- `.github/workflows/release.yml:31` currently builds only `linux/amd64`, `darwin/amd64`, `darwin/arm64`, and `windows/amd64`.
- `.goreleaser.yml:16` already declares `linux`, `darwin`, `windows` with `amd64` and `arm64`, so the declarative packaging intent is broader than the active workflow.
- `scripts/install.sh:22` already maps `arm64|aarch64` to `GOARCH=arm64`, which means install-script support exists if matching release assets are published.
- `scripts/install.ps1:8` currently assumes only one Windows architecture path and would need verification or adjustment for arm64 consumers.
- `packaging/scoop/agentcom.json:5` exposes only a `64bit` Windows manifest entry today.

## Problem Statement

The repository currently expresses partial support for arm64 in some places, but release automation and package-manager metadata do not yet publish or expose `linux/arm64` and `windows/arm64` artifacts consistently. That mismatch creates ambiguous support claims and forces manual workarounds.

## Acceptance Criteria

1. GitHub release automation can produce `agentcom_<version>_linux_arm64.tar.gz`.
2. GitHub release automation can produce `agentcom_<version>_windows_arm64.zip`.
3. Release documentation and install paths do not claim unsupported platforms.
4. Scoop metadata has a defined strategy for Windows arm64 users, either explicit support or an intentionally documented limitation if Scoop cannot express the target as desired.
5. The release process defines how checksums for the new artifacts flow into package-manager updates.
6. Verification covers build success, asset presence, and package-manager metadata correctness for the new targets.

## Tasks

### Task 1 - Map Current Release Constraints

#### Subtasks

- T1.1: Audit `.github/workflows/release.yml` for the exact build matrix and platform-specific setup steps.
- T1.2: Audit `.goreleaser.yml` to determine whether it should become the source of truth or remain secondary.
- T1.3: Audit `scripts/install.sh`, `scripts/install.ps1`, and `packaging/scoop/agentcom.json` for architecture assumptions.
- T1.4: Audit README files to find every place the supported release targets are described.
- T1.5: Confirm whether the current Windows toolchain setup can cross-compile or natively build `windows/arm64` with CGO enabled.

#### Done When

- The repo has a single written summary of where `linux/arm64` and `windows/arm64` are blocked today.

### Task 2 - Decide Release Architecture Strategy

#### Subtasks

- T2.1: Decide whether the project should keep the hand-written GitHub Actions matrix or switch the release pipeline to Goreleaser-driven asset publication.
- T2.2: Decide how `windows/arm64` will be built: native runner, cross-compile, or explicitly unsupported pending toolchain proof.
- T2.3: Decide whether `linux/arm64` should be produced directly in the main workflow or via a secondary packaging path.
- T2.4: Decide the source of truth for supported targets so workflow, docs, install scripts, and package managers stay in sync.

#### Done When

- There is one concrete release architecture with no ambiguous ownership between workflow and packaging config.

### Task 3 - Expand Asset Generation

#### Subtasks

- T3.1: Update release automation to add `linux/arm64` artifact generation.
- T3.2: Update release automation to add `windows/arm64` artifact generation if the chosen strategy supports it.
- T3.3: Ensure archive naming and checksum generation follow the existing release naming scheme.
- T3.4: Ensure any Windows-specific toolchain setup installs the correct compiler/runtime dependencies for the chosen arm64 path.
- T3.5: Keep existing `amd64` and macOS asset generation unchanged.

#### Done When

- A tagged release can publish the new arm64 assets without regressing existing targets.

### Task 4 - Align Installers And Package Managers

#### Subtasks

- T4.1: Update `scripts/install.sh` only if needed so `linux/arm64` users resolve the correct artifact path.
- T4.2: Update `scripts/install.ps1` only if needed so Windows arm64 users resolve the correct artifact path or receive an explicit limitation message.
- T4.3: Update `packaging/scoop/agentcom.json` with the chosen Windows arm64 strategy.
- T4.4: Update Homebrew-related documentation only if the supported macOS matrix or wording changes indirectly.
- T4.5: Define the exact release-day workflow for refreshing hashes after the new assets are uploaded.

#### Done When

- Every supported installer/package-manager path has a deterministic mapping to the expanded release asset set.

### Task 5 - Update Documentation

#### Subtasks

- T5.1: Update `README.md` supported-platform sections.
- T5.2: Update `README.ko.md`, `README.ja.md`, and `README.zh.md` to match the same platform matrix.
- T5.3: Update any release notes or internal memory/process docs that currently describe the old matrix.
- T5.4: Ensure documentation distinguishes between released support and future intent.

#### Done When

- The documented platform matrix matches the actual published asset matrix.

### Task 6 - Verification And Release Drill

#### Subtasks

- T6.1: Run targeted build verification for each new target path.
- T6.2: Run `go test ./...` before release.
- T6.3: Perform a dry run or staging release rehearsal that confirms artifact names and checksums.
- T6.4: Confirm the final release contains `linux/arm64` and `windows/arm64` assets.
- T6.5: Confirm Scoop/Homebrew/install-script references resolve to the intended artifacts.
- T6.6: Record any manual fallback required if GitHub-hosted runners still block a target.

#### Done When

- The team has command-level evidence that the new targets build, publish, and map correctly through distribution metadata.

### Task 7 - Memory Closure

#### Subtasks

- T7.1: Record the chosen release architecture and why it won.
- T7.2: Record any billing/runner/toolchain constraints discovered during implementation.
- T7.3: Record the final supported asset matrix for future releases.
- T7.4: Record the verification and release commands used.

#### Done When

- `.agents/MEMORY.md` captures the new release process and support matrix clearly.

## Execution Order

1. Task 1 - Map Current Release Constraints
2. Task 2 - Decide Release Architecture Strategy
3. Task 3 - Expand Asset Generation
4. Task 4 - Align Installers And Package Managers
5. Task 5 - Update Documentation
6. Task 6 - Verification And Release Drill
7. Task 7 - Memory Closure

## Verification Commands

```bash
go test ./... -count=1
go build ./...
GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build ./cmd/agentcom
GOOS=windows GOARCH=arm64 CGO_ENABLED=1 go build ./cmd/agentcom
gh release view <tag> --json assets
```

## Risks

- `windows/arm64` with CGO may require a toolchain that is not available on the current GitHub-hosted runner setup.
- Scoop may not support the exact architecture mapping desired without a manifest-format compromise.
- The repo can drift again if `.goreleaser.yml`, release workflow, and docs are all edited independently.

## Mitigations

- Prove the build path for each new target before promising support in docs.
- Choose one source of truth for the release matrix and derive the rest from it.
- Keep release-day verification explicit, including asset presence and checksum propagation.
