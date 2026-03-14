# MEMORY.md — 작업 기억

> 세션 간 컨텍스트를 유지하기 위한 파일. 작업 시작/종료 시 반드시 읽고 업데이트한다.

## 현재 상태

- **Phase**: 7 완료
- **마지막 작업**: interactive template 선택/`agentcom` namespaced skill 문서화 후 `v0.1.3` 릴리스 및 패키지 매니저 반영 완료
- **현재 브랜치**: `feature/P8-01-onboard-setup-wizard`
- **추가 진행 작업**: `agentcom init --setup` 대화형 wizard MVP 구현 및 검증
- **다음 작업**: 현재 `feature/skill-agent-catalog`에 남아 있는 다중 agent skill 지원 확장 작업 정리 여부 판단

## 완료된 태스크

- P0-01~P0-05 완료
- P1-01~P1-09 완료
- P2-01~P2-09 완료
- P3-01~P3-11 완료
- P4-01~P4-09 완료
- P5-01~P5-03 완료
- P6-01~P6-12 완료
- P7-01~P7-04 완료

## 이번 세션에서 마무리한 작업

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

## 발견된 이슈

- 기존 메모의 PRD 경로 표기는 `.agents/plan/PRD.md`였지만 실제 경로는 `.agents/plans/PRD.md`
- agent 삭제 시 message/task 외래 키가 deregister를 막는 문제를 확인했고, 초기 스키마에서 제약 제거로 수정 완료

## 메모

- PRD 경로: `.agents/plans/PRD.md`
- onboard wizard PRD: `.agents/plans/P8-01-onboard-setup-wizard.md`
- 전체 태스크 수: 62개
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
- 전체 테스트 통과: `go test ./...`
- 전체 빌드 통과: `go build ./...`

## 진행 중 작업 체크리스트

- P8-01-01: onboard wizard PRD 작성 완료
- P8-01-02~09: `init --setup` MVP 구현 및 문서 반영 진행 중
- 구현 범위는 onboarding wizard, home dir 선택, template/AGENTS.md 선택, 기존 init 동작 보존에 한정
