# MEMORY.md — 작업 기억

> 세션 간 컨텍스트를 유지하기 위한 파일. 작업 시작/종료 시 반드시 읽고 업데이트한다.

## 현재 상태

- **Phase**: P11 구현/문서 완료, release 준비 중
- **마지막 작업**: P11 `up`/`down` 라이프사이클 구현 + README 전면 갱신 + atomic commit 정리
- **현재 브랜치**: `feature/P11-up-down-agent-lifecycle-plan`
- **현재 버전**: v0.1.7이 최신 공개 릴리스, 다음 릴리스 버전은 아직 미확정
- **P10 상태**: 구현/문서/테스트 완료, 관련 변경은 현재 브랜치에 포함됨
- **P11 상태**: 구현 완료, 테스트/수동 QA/README 반영 완료, develop 머지 및 release 대기
- **계획 문서 상태**: `AGENTCOM_IMPROVEMENT_PROPOSAL.md` 기반 후속 개선안 분석 완료, PH1~PH4 상세 실행 계획 문서 작성 완료
- **다음 작업**: PH1~PH4 계획 문서 검토 후 우선순위 확정, 구현 시작 시 task 단위로 feature 브랜치 분리, release workflow와 README 기준을 맞춰 `linux/arm64`·`windows/arm64` 지원 여부를 확정하고 문서/배포 설정을 정리
- **워킹트리**: 계획 문서 4개와 MEMORY 업데이트가 존재, 그 외 release/README 관련 별도 수정도 남아 있음

## 완료된 태스크

- P0-01~P0-05 완료
- P1-01~P1-09 완료
- P2-01~P2-09 완료
- P3-01~P3-11 완료
- P4-01~P4-09 완료
- P5-01~P5-03 완료
- P6-01~P6-12 완료
- P7-01~P7-04 완료
- P8-01-01~09 완료 (onboard/setup wizard MVP)
- P10 project column 핵심 구현 완료

## 이번 세션에서 마무리한 작업

- 개선 제안서 기반 후속 계획 문서화 완료
  - `.agents/plans/AGENTCOM_IMPROVEMENT_PROPOSAL.md`를 전체 검토하고 실제 소스 기준으로 불일치/과장 포인트를 재정리
  - plan agent 결과를 바탕으로 `.agents/plans/PHASE1-critical-fixes.md` 작성 완료
  - `.agents/plans/PHASE2-core-improvements.md` 작성 완료
  - `.agents/plans/PHASE3-documentation-polish.md` 작성 완료
  - `.agents/plans/PHASE4-enhanced-ux.md` 작성 완료
  - Phase 번호는 기존 P0~P11과 충돌하지 않도록 `PH1`~`PH4` 체계 사용
  - 총 계획 규모: 22 tasks / 75 subtasks / 약 68시간 추정
  - 핵심 정정 사항: overwrite 버그는 1개가 아니라 3개 write 함수에 존재, `generateInstructionFiles()`가 아니라 `writeAgentInstructions()`가 실제 엔트리포인트, `doctor`와 `verify`는 통합 대상으로 재정의

- v0.1.7 릴리스 요약
  - `feature/init-step3-project-onboarding`를 `develop`에 머지한 뒤 `main`에 릴리스 머지 완료
  - GitHub release/tag `v0.1.7` 발행 및 기본 release asset(`darwin/amd64`, `linux/amd64`, `windows/amd64`) 업로드 확인
  - 누락됐던 `darwin/arm64` asset을 수동 빌드/업로드해 install script와 Homebrew arm64 설치 경로 복구
  - `scripts/install.sh`, `scripts/install.ps1`, `packaging/scoop/agentcom.json`, `malleus35/homebrew-tap` formula를 모두 `0.1.7` 기준으로 갱신
  - Homebrew `brew upgrade agentcom`과 shell install script로 `0.1.7` 수동 QA 완료

- 템플릿 스캐폴드 up/down 문구 정렬 완료
  - `internal/cli/agents.go`의 shared `agentcom/SKILL.md` 생성 문구에 기본 흐름 `init -> up -> down` 추가
  - `internal/cli/agents.go`의 role skill 생성 문구에 template 기본 lifecycle과 `register`의 고급/standalone 위치를 명시
  - `internal/cli/agents.go`의 built-in template `CommonBody` (`company`, `oh-my-opencode`)를 `register` 중심에서 `init -> up -> down` 중심으로 조정
  - `internal/cli/agents_test.go`의 scaffold assertion을 새 문구 기준으로 최소 갱신
  - 검증 완료: `gofmt -w internal/cli/agents.go internal/cli/agents_test.go`, `go test ./internal/cli/...`, `go test ./...`, `go build ./...`
  - 수동 QA 완료: 임시 프로젝트에서 `agentcom init --batch --project demo-company --template company`, `agentcom init --batch --project demo-omo --template oh-my-opencode` 실행 후 generated `COMMON.md`, shared `SKILL.md`, role `SKILL.md`에 기본 흐름/보조적 register 문구 반영 확인

- P11 `up`/`down` 에이전트 라이프사이클 구현 완료
  - `.agentcom.json`에 `template.active` 저장/로드 추가
  - `agentcom init`가 template 선택 시 active template까지 기록하도록 확장
  - `agentcom up` 구현: active template 로드, detach 기본, `--template`, `--only`, `--force` 지원
  - `agentcom down` 구현: `.agentcom/run/up.json` 기반 종료, `--only`, `--timeout`, `--force` 지원
  - subprocess supervisor + 플랫폼별 detach 처리 + 런타임 상태 파일 추가
- 테스트/검증 완료
  - `go test ./...` 통과
  - `go build ./...` 통과
  - 수동 QA: `init --template company`, `up --only frontend,plan`, `down`, `down --only plan` 확인
- README 전체 갱신 완료
  - `README.md`, `README.ko.md`, `README.ja.md`, `README.zh.md` 모두 `init -> up -> down` 기본 흐름으로 재작성
  - `register`를 고급/저수준 인터페이스로 재포지셔닝
- P11 관련 커밋 6개 생성 완료
  - `feat(config): persist active template in project config`
  - `feat(cli): persist active template during init`
  - `feat(cli): add runtime state storage for managed sessions`
  - `feat(cli): add managed up and down commands`
  - `test(cli): cover managed lifecycle commands`
  - `docs: refresh README for init up down workflow`

### 이전 세션 작업 (참고)

- `internal/cli/agents.go`: `agentcom agents template` 추가, `company`/`oh-my-opencode` 내장 템플릿 정의, 공통 markdown/manifest/role skill 스캐폴드 로직 추가
- `internal/cli/init.go`: `agentcom init --template <company|oh-my-opencode>` 지원 추가, 프로젝트 템플릿 파일 및 role skill 생성 결과 출력/JSON 응답 확장
- `internal/cli/agents_test.go`: 템플릿 해상도, scaffold 생성, JSON 출력, init 연동 테스트 추가
- `internal/cli/cli_test.go`: root 커맨드에 `agents` 등록 검증 추가
- `internal/db/agent.go`: `InsertAgent`가 preset ID를 덮어쓰지 않도록 수정
- `internal/db/*_test.go`: DB CRUD 테스트 추가
- `internal/agent/registry_test.go`: register/deregister/heartbeat/stale detection 테스트 추가
- `internal/transport/transport_test.go`: UDS roundtrip, stale socket, fallback poller 테스트 추가
- `internal/task/manager_test.go`: 상태 전이와 manager/query 테스트 추가
- `internal/mcp/server_test.go`: STDIO JSON-RPC handshake + tool roundtrip 테스트 추가
- `cmd/agentcom/e2e_test.go`: 실제 바이너리 기반 E2E 시나리오 추가
- `internal/cli/init.go`: `agentcom init --agents-md` 지원 추가
- `internal/cli/skill.go`: `agentcom skill create` 추가 (`project|user`, `claude|codex|gemini|opencode|all`)
- `internal/cli/skill_test.go`: skill 이름 검증, 경로 계산, JSON 출력, 파일 생성 테스트 추가
- `README.md`: 설치, 퀵스타트, CLI/MCP 사용법, 아키텍처 문서화
- `.goreleaser.yml`: 릴리스 설정 추가
- `.github/workflows/ci.yml`: lint/test/build CI 추가
- `scripts/install.sh`, `scripts/install.ps1`: 기본 설치 버전을 `v0.1.2`로 상향
- `packaging/scoop/agentcom.json`: `v0.1.2` Windows asset URL/hash로 갱신
- GitHub release/tag: `v0.1.2` 생성 후 `main` 최신 커밋으로 태그 재지정
- Homebrew tap(`malleus35/homebrew-tap`): `Formula/agentcom.rb`를 `0.1.2` asset/hash로 갱신
- `feature/template-search-select`: `agentcom agents template`에 검색어 입력 + 번호 선택 기반 인터랙티브 템플릿 선택 추가 후 `develop` 머지
- `feature/agentcom-shared-skills`: 템플릿 role skill을 각 agent의 `agentcom/<template>-<role>/SKILL.md` 구조로 생성하고 shared `agentcom/SKILL.md` 참조 추가 후 `develop` 머지
- `README*.md`: interactive template selection과 `agentcom` namespace scaffold 구조 문서화
- `scripts/install.sh`, `scripts/install.ps1`: 기본 설치 버전을 `v0.1.3`으로 상향
- `v0.1.3` release/tag 생성 및 GitHub release asset 업로드 완료
- `packaging/scoop/agentcom.json`: `v0.1.3` Windows asset URL/hash로 갱신
- Homebrew tap(`malleus35/homebrew-tap`): `Formula/agentcom.rb`를 `0.1.3` asset/hash로 갱신
- `internal/cli/instruction.go`: agent별 instruction/memory 파일 레지스트리, 렌더링, 쓰기 로직 추가
- `internal/cli/init.go`, `internal/cli/init_setup.go`, `internal/cli/init_prompter.go`: `--setup` 제거, `--batch` 추가, interactive wizard 기본화, agent 선택/ instruction/memory / custom template 흐름 통합
- `internal/cli/template_store.go`: custom template 저장/로드/병합 로직 추가
- `internal/cli/agents.go`: built-in + custom template 통합 조회와 custom template 삭제 지원 추가
- `internal/cli/*_test.go`, `internal/onboard/result_test.go`: instruction, batch mode, custom template, wizard 통합 시나리오 TDD 테스트 추가
- `README*.md`: 새 `init` UX와 `--agents-md`/`--template custom`/`agents template --delete` 문서 반영
- `feature/skill-agent-catalog`: 추가 agent identifier/alias(`cursor`, `github-copilot`, `gemini-cli`, `droid` 등)와 agent별 파일 확장자 지원 추가 후 `develop` 머지
- `feature/P8-01-onboard-setup-wizard`: `agentcom init --setup` + `--accessible` 기반 onboarding wizard 구현 후 `develop`/`main` 반영
- `v0.1.4` release/tag 생성 및 GitHub release asset 업로드 완료
- `v0.1.4` release에는 누락됐던 `darwin/arm64` asset을 수동 빌드/업로드로 추가 완료
- `packaging/scoop/agentcom.json`: `v0.1.4` Windows asset URL/hash로 갱신
- Homebrew tap(`malleus35/homebrew-tap`): `Formula/agentcom.rb`를 `0.1.4` asset/hash로 갱신하고 macOS arm64 분기 복원
- 로컬 Homebrew 설치에서 `agentcom` formula가 `pinned at 0.1.3` 상태였고, `brew unpin agentcom && brew upgrade agentcom`으로 `0.1.4` 업그레이드 확인
- `internal/db/migrations.go`, `internal/db/migrations_test.go`: `PRAGMA user_version` 기반 schema versioning과 `agents.project` 재생성 마이그레이션 추가
- `internal/db/agent.go`, `internal/db/agent_test.go`: `Agent.Project` 필드, project-aware CRUD/query/list, 동일 이름의 cross-project 등록 테스트 추가
- `internal/config/project.go`, `internal/config/project_test.go`: `.agentcom.json` 읽기/쓰기/상위 탐색/프로젝트명 검증/우선순위 해석 추가
- `internal/agent/registry.go`, `internal/agent/registry_test.go`: register/find/list/deregister에 project 컨텍스트 반영
- `internal/cli/root.go`, `internal/cli/init.go`, `internal/cli/init_setup.go`, `internal/cli/*_test.go`: `--project`/`--all-projects`, init project config 생성, project-scoped list/send/broadcast/inbox/status/health 반영
- `internal/message/router.go`, `internal/message/router_test.go`: router에 project 컨텍스트 추가
- `internal/mcp/server.go`, `internal/mcp/handler.go`, `internal/mcp/tools.go`, `internal/mcp/server_test.go`: MCP 기본 project와 tool-level `project` 파라미터 반영
- `README.md`: project scope, `.agentcom.json`, 새 global flags 문서화

## 설계 결정 로그

| 날짜 | 결정 | 이유 |
|------|------|------|
| 2026-03-12 | SQLite3를 유일한 외부 의존성으로 채택 | 외부 데몬 불필요, WAL 모드로 동시성 확보, 단일 파일 관리 |
| 2026-03-12 | 에이전트 type을 자유 문자열로 | 미래 에이전트 도구에 수정 없이 대응, 유연성 극대화 |
| 2026-03-12 | MessagePack 대신 JSON 직렬화 | 디버깅 편의성 우선, 성능 차이 무시할 수준 (로컬 IPC) |
| 2026-03-12 | CGO 사용 (mattn/go-sqlite3) | modernc.org/sqlite도 고려했으나 mattn이 더 성숙하고 WAL 지원 안정적 |
| 2026-03-12 | message/task 테이블의 agent foreign key 제거 | agent deregister 이후에도 message/task history를 유지하고 E2E 흐름을 막지 않기 위해 |
| 2026-03-12 | `skill create`는 에이전트별 네이티브 스킬 경로에 직접 생성 | Claude/Codex/Gemini/OpenCode의 실제 로딩 경로를 맞춰 즉시 사용 가능하게 하기 위해 |
| 2026-03-14 | 템플릿 스캐폴딩은 기존 `agentcom init`을 확장 | 홈/DB 초기화 흐름을 유지하면서 프로젝트 템플릿 생성을 한 번에 수행하기 위해 |
| 2026-03-14 | `agentcom agents template`는 내장 템플릿 조회 전용으로 시작 | 생성 동작은 `init --template`에 두고, `agents template`는 템플릿 탐색/설명 surface로 분리하기 위해 |
| 2026-03-14 | role skill frontmatter는 `name` + `description`만 사용 | Claude/Codex/Gemini/OpenCode 공통 호환성을 유지하고 OpenCode YAML 파싱 이슈를 피하기 위해 |
| 2026-03-14 | `v0.1.2` 태그는 최종적으로 `main` 최신 커밋을 가리키도록 재설정 | Scoop manifest 후속 커밋까지 동일 릴리스 태그에 포함하기 위해 |
| 2026-03-15 | `agents template` 검색/선택은 `openclaw onboard`의 위저드 UX만 차용하고 실제 명령 호출은 하지 않음 | 공식 `openclaw onboard`는 템플릿 검색 기능이 없고, step-based interactive flow만 유사하게 적용하는 편이 안전하기 때문 |
| 2026-03-15 | 템플릿 role skill은 각 agent의 `agentcom` 네임스페이스 아래 shared `SKILL.md` + role adapter 구조로 생성 | shared/common 지침과 role-specific 지침을 분리해 중복을 줄이고 참조형 구조를 만들기 위해 |
| 2026-03-15 | `v0.1.3` release는 tag 워크플로 자산 + 수동 `darwin/arm64` 업로드 조합으로 마무리 | 현재 GitHub Actions release workflow가 `darwin/arm64`를 자동 생성하지 않기 때문 |
| 2026-03-15 | onboard/setup UI는 full-screen TUI 대신 `huh` wizard로 구현 | 요구 범위가 초기 설정 단계에 한정되고, 기존 CLI 패턴을 최소 변경으로 유지하기 위해 |
| 2026-03-15 | `agentcom init --setup`은 기존 root DB 초기화를 우회한 뒤 선택한 home dir 기준으로 별도 apply | 사용자가 wizard에서 홈 경로를 바꾸기 전에 기본 config/db가 먼저 생성되는 부작용을 막기 위해 |
| 2026-03-15 | skill agent catalog 확장 후에도 템플릿 scaffold는 core agent 집합만 사용 | `skill create --agent all`의 catalog 확장과 템플릿 scaffold 범위를 분리해 기존 템플릿 생성 기대값을 보존하기 위해 |
| 2026-03-15 | `agentcom init`은 interactive TTY에서 wizard를 기본 실행하고 `--batch`/`--json`에서만 비대화형으로 동작 | `--setup`을 제거하고 온보딩을 기본 UX로 만들기 위해 |
| 2026-03-15 | `--agents-md`는 bool 대신 string으로 바꾸고 agent별 instruction 파일 생성을 기본으로 하되 `--batch --agents-md` 무값 호출은 legacy `AGENTS.md`를 유지 | 확장된 instruction 파일 생성과 기존 배치 스크립트 호환성을 동시에 만족시키기 위해 |
| 2026-03-15 | custom template는 `.agentcom/templates/<name>/COMMON.md` + `template.json` 저장소 모델을 사용하고 built-in과 함께 해석한다 | init wizard, template inspect, scaffold가 같은 템플릿 원본을 공유하도록 하기 위해 |
| 2026-03-15 | `v0.1.4` release도 `darwin/arm64` asset은 수동 업로드로 보강 | 현재 GitHub release workflow가 `darwin/arm64` build matrix를 포함하지 않아 Homebrew arm64 formula 지원을 위해 별도 업로드가 필요했기 때문 |
| 2026-03-15 | P11 구현은 `init=플랫폼 설정`, `up/down=런타임 관리` 레이어 분리를 유지하되 `up` 기본 모드는 detach로 선택 | 사용자 결정: 기존 추천안에서 3B만 채택. 일상 사용에서는 즉시 프롬프트 복귀가 더 중요하다고 판단 |
| 2026-03-15 | `up`은 `.agentcom.json`의 `template.active`와 `.agentcom/templates/<name>/template.json`을 직접 사용하고, runtime PID 정보는 `.agentcom/run/up.json`에 기록 | 새 선언 파일을 도입하지 않고 기존 scaffold를 런타임 입력으로 재사용하면서 `down`이 정확히 종료 대상을 찾게 하기 위해 |
| 2026-03-15 | 현재 release workflow는 GitHub release asset 업로드만 자동화하고 Homebrew tap 갱신은 별도 수행이 필요 | `.goreleaser.yml`에 brew 설정은 있으나 `.github/workflows/release.yml`은 goreleaser를 호출하지 않음 |
| 2026-03-15 | P9 `init` 개편: wizard를 기본 UI로, `--setup` 제거, `--batch` 추가 | 대부분의 사용자가 인터랙티브 환경에서 init 실행 — wizard가 기본이어야 함 |
| 2026-03-15 | P9 `--agents-md`: agent별 고유 instruction 파일 생성으로 확장 | AGENTS.md 단일 파일 대신 CLAUDE.md, .cursorrules 등 agent별 파일 생성 필요 |
| 2026-03-15 | P9 `--template custom`: 사용자 정의 템플릿 생성 wizard 추가 | built-in 템플릿만으로는 다양한 팀 구조를 커버할 수 없음 |
| 2026-03-15 | AGENTS.md는 크로스 툴 표준 (AAIF, 15+ agent, 60K+ 저장소) | instruction 파일 생성 시 AGENTS.md를 Priority 1으로, 나머지는 agent별 고유 파일로 |
| 2026-03-15 | 작업 우선순위는 P9 init-overhaul 먼저, P10 project column은 후속으로 진행 | project column 계획이 init wizard 구조를 전제로 일부 단계를 얹는 형태라 P9 이후가 재작업이 적음 |
| 2026-03-15 | `agents` schema는 `PRAGMA user_version` 기반 순차 마이그레이션으로 관리하고 `UNIQUE(name, project)` 전환은 테이블 재생성으로 처리 | SQLite에서 기존 UNIQUE 제약을 직접 변경할 수 없고, 재실행 안전성과 기존 DB 호환을 함께 확보해야 했기 때문 |
| 2026-03-15 | CLI/MCP 기본 project scope는 `.agentcom.json` 또는 `--project`에서 해석하고, `--all-projects`일 때만 필터를 우회 | 기존 글로벌 동작과 프로젝트 격리를 동시에 지원하면서 명시적 override를 유지하기 위해 |

| 2026-03-15 | P11 `up`/`down` 계획: 옵션 A(init=플랫폼, up=런타임)를 기본 방향으로, 옵션 B(up이 init 흡수) 요소도 검토 | Oracle 상담 + 업계 CLI 벤치마크 결과. init→up 레이어 분리가 Terraform/Dapr 패턴과 일치하고 기존 코드 변경 최소 |
| 2026-03-15 | `up`은 .agentcom.json의 template.active를 읽고, template.json에서 역할 목록을 로드하여 subprocess supervisor로 일괄 register | register가 블로킹이므로 supervisor가 자식 프로세스로 각 register를 fork하는 구조가 자연스러움 |
| 2026-03-15 | 런타임 상태는 .agentcom/run/up.json에 PID/socket/role 매핑으로 기록 | SQLite는 진실의 원천이지만 프로세스 종료는 PID 없이 정확히 못 하므로 별도 런타임 파일 필요 |

## 발견된 이슈

- 기존 메모의 PRD 경로 표기는 `.agents/plan/PRD.md`였지만 실제 경로는 `.agents/plans/PRD.md`
- agent 삭제 시 message/task 외래 키가 deregister를 막는 문제를 확인했고, 초기 스키마에서 제약 제거로 수정 완료
- Homebrew는 remote tap이 최신이어도 local tap checkout이 stale하거나 formula가 pin 되어 있으면 `brew upgrade agentcom`이 새 릴리스를 보지 못할 수 있음
- `scripts/install.sh`, `scripts/install.ps1`, `packaging/scoop/agentcom.json`은 아직 `v0.1.5` 기준으로 고정돼 있어 다음 공개 릴리스 전에 버전/URL/hash 갱신이 필요
- 현재 `release.yml` build matrix는 `linux/amd64`, `darwin/amd64`, `windows/amd64`만 자동 생성한다. README의 `arm64` 설명 및 기존 수동 업로드 경험과 차이가 있어 릴리스 시 확인 필요
- 현재 `README*.md`는 실제 공개 릴리스 기준으로 `linux/arm64`, `windows/arm64`를 제외하고 있지만, `.goreleaser.yml`은 여전히 arm64 전반을 암시한다. 다음 작업에서 release workflow/문서/배포 타깃을 한 기준으로 통일할 필요가 있음

## 메모

- PRD 경로: `.agents/plans/PRD.md`
- onboard wizard PRD: `.agents/plans/P8-01-onboard-setup-wizard.md`
- **init 개편 PRD: `.agents/plans/P9-init-overhaul.md`** (7 tasks, 43 subtasks)
- **project column PRD: `.agents/plans/P10-add-project-column-plan-prd.md`** (renumber 완료)
- **P11 up/down 계획**: `.agents/plans/P11-up-down-agent-lifecycle.md` (3 옵션, 13 태스크, 8 미결 사항)
- 전체 태스크 수: 62개 (P0-P7) + 9개 (P8) + 43개 (P9) + 63개(+3 sub) (P10) + 13개 (P11) = 190개(+3 sub)
- root 커맨드에 `mcp-server` 등록 완료
- root 커맨드에 `skill` 등록 완료
- root 커맨드에 `agents` 등록 완료
- `agentcom init --template company|oh-my-opencode`는 `.agentcom/templates/<template>/COMMON.md`, `.agentcom/templates/<template>/template.json`, 그리고 6개 role skill을 각 agent CLI 경로에 생성
- `agentcom agents template`는 interactive tty에서 검색어 기반 템플릿 선택을 지원하고, non-interactive/JSON 모드에서는 기존 목록/상세 출력 동작을 유지
- 템플릿 role skill 생성 경로는 `.claude/skills/agentcom/<template>-<role>/SKILL.md` 등 각 agent 네임스페이스 구조로 변경됐고, shared file은 `.claude/skills/agentcom/SKILL.md` 형태로 생성
- onboard wizard PRD: `.agents/plans/P8-01-onboard-setup-wizard.md`
- CEO 중심 라우팅 vs direct-to-user 응답 모델은 아직 계획 단계이며, 현 구현에는 특수 `user` recipient를 추가하지 않음
- `develop`, `release/v0.1.2`, `main`, `feature/init-template-scaffold` 브랜치와 `v0.1.2` 태그는 원격 반영 완료
- `release/v0.1.3`, `main`, `develop`, `v0.1.3` 태그는 원격 반영 완료
- `main`, `develop`, `v0.1.4` 태그와 GitHub release는 원격 반영 완료
- 전체 테스트 통과: `go test ./...`
- 전체 빌드 통과: `go build ./...`
- P11 주요 구현 파일: `internal/cli/up.go`, `internal/cli/up_state.go`, `internal/cli/detach_unix.go`, `internal/cli/detach_windows.go`, `internal/config/project.go`
- P11 수동 QA 완료: `agentcom init --project demo --template company`, `agentcom up --only frontend,plan`, `agentcom down`, `agentcom down --only plan`
- 현재 브랜치의 최신 P11 커밋 체인: `2a09224` → `d639ba2` → `1a3ca6e` → `640149a` → `3056328` → `61471d6`
- P9 주요 구현 파일: `internal/cli/instruction.go`, `internal/cli/init_prompter.go`, `internal/cli/template_store.go`
- P10 주요 구현 파일: `internal/db/migrations.go`, `internal/config/project.go`, `internal/cli/root.go`, `internal/mcp/handler.go`
- 수동 QA 완료: `agentcom init --batch --project`, project별 `list`, `--all-projects list`, project-scoped `send`/`inbox`/`status`

## 진행 중 작업 체크리스트

- 현재 후속 우선순위: 릴리스 버전 확정 → develop 머지 → 태그/릴리스 → Scoop/Homebrew 반영 검증
