# Template Scaffold Up/Down Alignment Plan

## Objective

- Align template scaffold wording with the current recommended workflow: `init -> up -> down`.
- Remove stale `register`-centric guidance from generated shared/role instructions where it no longer matches the intended onboarding flow.
- Keep the change narrowly scoped to scaffolded template content and its verification.

## Scope

### In Scope

- Built-in template scaffold text defined in `internal/cli/agents.go`.
- Shared skill body text generated for supported agent CLIs.
- Role skill text generated from built-in templates.
- Tests and docs only where needed to verify the wording change.

### Out of Scope

- Runtime behavior changes to `init`, `up`, `down`, or `register`.
- Template structure redesign.
- Non-template documentation rewrites unless a generated-output assertion requires it.

## Acceptance Criteria

1. Newly scaffolded built-in templates no longer present `register` as the primary default flow when the intended flow is `init -> up -> down`.
2. Shared skill text and role skill text stay consistent with each other.
3. Any remaining mention of `register` is explicitly framed as low-level or advanced usage, not the default path.
4. Existing template generation tests pass after the wording update.
5. Manual QA confirms generated scaffold files in a fresh project contain the updated guidance.

## Work Breakdown

### Task T0 - Source Mapping

#### Goal

Locate every template-scaffold source string that controls generated workflow wording.

#### Subtasks

- T0.1: Inspect `internal/cli/agents.go` for:
  - shared skill body text
  - role skill text
  - built-in template `CommonBody`
- T0.2: Identify all generated outputs affected by those source strings.
- T0.3: Identify tests that assert current generated wording.
- T0.4: Separate default-flow text from low-level/advanced-flow text.

#### QA

- A complete list exists of every source constant/function that contributes to generated template wording.

### Task T1 - Messaging Rules

#### Goal

Define exact wording rules before editing implementation strings.

#### Subtasks

- T1.1: Set the default lifecycle wording to `init -> up -> down`.
- T1.2: Keep `register` available only as an advanced/manual interface.
- T1.3: Ensure wording is consistent across:
  - shared skill text
  - role skill text
  - template common instructions
- T1.4: Preserve existing structure and tone unless it directly conflicts with the new default flow.
- T1.5: Avoid introducing new capabilities or behavioral promises.

#### QA

- A single wording policy can be applied uniformly to all generated template artifacts.

### Task T2 - Shared Skill Alignment

#### Goal

Update shared scaffolded skill instructions so they describe the correct primary workflow.

#### Subtasks

- T2.1: Edit shared skill source text in `internal/cli/agents.go`.
- T2.2: Replace any default-first `register` guidance with `init -> up -> down` guidance.
- T2.3: Rephrase any remaining `register` mention as optional low-level usage.
- T2.4: Check that wording still reads naturally for each supported agent CLI target.

#### QA

- Generated shared `agentcom/SKILL.md` content reflects the updated default flow without contradictory register-first language.

### Task T3 - Role Skill Alignment

#### Goal

Update role-specific scaffolded skill content so generated agent role files match the shared guidance.

#### Subtasks

- T3.1: Inspect each role-specific template fragment in `internal/cli/agents.go`.
- T3.2: Remove or rewrite stale workflow text that assumes manual `register` is the normal starting point.
- T3.3: Ensure roles still mention communication/coordination responsibilities without changing role semantics.
- T3.4: Verify no role skill contradicts the shared skill text.

#### QA

- Generated role skills consistently point to the same primary workflow as the shared skill.

### Task T4 - Built-in Template CommonBody Alignment

#### Goal

Update built-in template common instructions so scaffolded `COMMON.md` matches the modern workflow.

#### Subtasks

- T4.1: Find built-in template `CommonBody` definitions in `internal/cli/agents.go`.
- T4.2: Replace register-centric onboarding language with template lifecycle guidance centered on `init -> up -> down`.
- T4.3: Preserve template-specific identity and collaboration text.
- T4.4: Keep any advanced/manual note about `register` only if still useful and accurate.

#### QA

- Generated `.agentcom/templates/<template>/COMMON.md` content matches current product guidance.

### Task T5 - Test Updates

#### Goal

Keep scaffold-generation tests aligned with the revised wording and behavior expectations.

#### Subtasks

- T5.1: Find tests covering built-in template generation and generated content snapshots/assertions.
- T5.2: Update assertions only where wording changed.
- T5.3: Avoid broadening test scope beyond scaffold-output verification.
- T5.4: Run targeted tests first for faster feedback.

#### QA

- Relevant scaffold-generation tests pass with no behavioral regressions.

### Task T6 - Manual QA

#### Goal

Verify the generated outputs directly instead of relying only on tests.

#### Subtasks

- T6.1: Run template scaffold generation in a clean temporary project.
- T6.2: Generate at least one built-in template, preferably `company` and/or `oh-my-opencode`.
- T6.3: Open generated shared skill and role skill files.
- T6.4: Open generated `COMMON.md`.
- T6.5: Confirm the default path is described as `init -> up -> down`.
- T6.6: Confirm any `register` mention is secondary and clearly framed as advanced/manual.

#### QA

- Manual inspection of actual generated files matches the intended wording rules.

### Task T7 - Memory Closure

#### Goal

Update project memory after implementation so the next session starts from accurate context.

#### Subtasks

- T7.1: Record what wording sources were changed.
- T7.2: Record which tests and manual QA steps were executed.
- T7.3: Remove the in-progress checklist item if the task is complete.
- T7.4: Record the next actionable task, if any remains.

#### QA

- `.agents/MEMORY.md` accurately reflects the post-change state.

## Execution Order

1. T0 Source Mapping
2. T1 Messaging Rules
3. T2 Shared Skill Alignment
4. T3 Role Skill Alignment
5. T4 Built-in Template CommonBody Alignment
6. T5 Test Updates
7. T6 Manual QA
8. T7 Memory Closure

## Verification Commands

```bash
go test ./...
go test ./internal/cli/...
agentcom init --template company
agentcom init --template oh-my-opencode
```

## Risks

- Over-editing wording could accidentally change intended role semantics.
- Tests may encode exact strings in multiple places, requiring focused assertion updates.
- Shared and role template text may drift again if only one source path is updated.

## Mitigations

- Keep changes limited to workflow wording.
- Update assertions only where generated wording truly changes.
- Verify generated outputs directly after tests.
