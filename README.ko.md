# agentcom

`agentcom`은 병렬 AI 코딩 에이전트 세션 사이의 실시간 통신을 위한 Go CLI 도구입니다.

영속 상태는 SQLite WAL 모드에 저장하고, 저지연 로컬 전달은 Unix Domain Socket으로 처리합니다. 대상 소켓이 없거나 전달에 실패하면 SQLite inbox polling을 폴백 경로로 사용합니다.

## 주요 기능

- `agentcom up` / `agentcom down`으로 템플릿에 정의된 전체 에이전트 일괄 시작 및 종료
- 장시간 실행되는 에이전트 세션 등록 및 해제
- 에이전트 간 1:1 메시지 및 브로드캐스트 전송
- 메시지, 태스크, 에이전트 상태를 SQLite에 영속화
- 간단한 상태 머신 기반 태스크 위임
- 지원하는 코딩 에이전트용 프로젝트/사용자 단위 `SKILL.md` 생성
- 공통 지침과 6개 역할 스킬이 포함된 내장 멀티 에이전트 템플릿 스캐폴드 생성
- `agentcom agents template`로 내장 템플릿 조회 및 TTY에서 interactive search 지원
- STDIO 기반 MCP JSON-RPC 서버 제공
- SQLite만 외부 의존성으로 사용하는 단일 머신용 구조

## 요구 사항

- Go 1.22+
- CGO 활성화
- `github.com/mattn/go-sqlite3`를 위한 SQLite3 툴체인 지원

## 설치

### 가장 쉬운 설치 방법

플랫폼별로 가장 짧게 설치하려면 아래 방법을 쓰면 됩니다.

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/malleus35/agentcom/main/scripts/install.sh | sh
```

```powershell
# Windows PowerShell
irm https://raw.githubusercontent.com/malleus35/agentcom/main/scripts/install.ps1 | iex
```

패키지 매니저를 선호하면:

```bash
# macOS / Linux (Homebrew tap)
brew tap malleus35/tap && brew install agentcom
```

```powershell
# Windows (bucket add 없이 Scoop 직접 설치)
scoop install https://raw.githubusercontent.com/malleus35/agentcom/main/packaging/scoop/agentcom.json
```

Scoop의 URL 직접 설치는 공식적으로 지원되지만, 등록된 bucket에서 설치한 것이 아니므로 일반적인 `scoop update agentcom` 흐름은 제공되지 않습니다.

로컬 빌드:

```bash
make build
./bin/agentcom version
```

Go bin 경로로 설치:

```bash
make install
agentcom version
```

Go로 직접 설치:

```bash
go install github.com/malleus35/agentcom/cmd/agentcom@latest
agentcom version
```

이 방식은 Go 사용자에게는 가장 간단하지만, 대상 머신에 Go와 CGO/SQLite 빌드 환경이 준비되어 있어야 합니다.

## 플랫폼 공통 설치 방법

macOS, Linux, Windows에서 가장 편하게 배포하려면 보통 아래 방식 중 하나를 사용합니다.

### 1. GitHub Releases 바이너리

대부분의 일반 사용자에게 가장 적합한 방식입니다. 현재 릴리스 설정 기준으로 다음 타겟 아카이브를 만들 수 있습니다.

- macOS: `darwin/amd64`, `darwin/arm64`
- Linux: `linux/amd64`, `linux/arm64`
- Windows: `windows/amd64`, `windows/arm64`

GitHub Releases에서 자신의 OS/아키텍처에 맞는 압축 파일을 내려받고, 압축을 푼 뒤 바이너리를 `PATH`에 넣으면 됩니다.

예시:

```bash
# macOS / Linux
tar -xzf agentcom_<version>_darwin_arm64.tar.gz
chmod +x agentcom
mv agentcom /usr/local/bin/
```

```powershell
# Windows PowerShell
Expand-Archive .\agentcom_<version>_windows_amd64.zip -DestinationPath .\agentcom
$env:Path += ";C:\path\to\agentcom"
```

### 2. Homebrew (이 저장소에 설정 포함)

이 저장소의 `.goreleaser.yml`에는 Homebrew 설정이 이미 들어 있습니다. 릴리스가 발행되고 tap이 준비되면 macOS와 Linux 사용자는 Homebrew로 설치할 수 있습니다.

예시:

```bash
brew tap malleus35/tap
brew install agentcom
```

이미 Homebrew를 쓰는 환경에서 업데이트까지 편하게 관리하고 싶다면 이 방식이 좋습니다.

`brew tap` 없이 바로 `brew install agentcom`이 되려면 `homebrew-core`에 formula가 들어가야 합니다.

### 2a. Homebrew core에 들어가려면?

핵심 조건만 정리하면 다음과 같습니다.

- 플랫폼별 안정적인 공개 릴리스 아카이브와 체크섬
- `brew audit`, `brew test`를 통과하는 formula
- `homebrew-core` 제출 및 승인
- Homebrew 정책에 맞는 지속적인 유지보수

그 전까지는 tap 기반 설치가 가장 현실적인 Homebrew 경로입니다.

### 3. `go install`

Go 개발자나 기여자에게 적합한 방식입니다.

```bash
go install github.com/malleus35/agentcom/cmd/agentcom@latest
```

장점:

- Go 사용자에게 가장 단순함
- 수동으로 바이너리를 내려받을 필요가 없음

주의점:

- Go가 설치되어 있어야 함
- 로컬 CGO/SQLite 빌드 환경에 의존함

### 4. 수동 로컬 빌드

직접 제어하고 싶거나 사내 배포 패키징을 할 때 적합합니다.

```bash
make build
./bin/agentcom version
```

### 어떤 방식을 선택하면 좋은가?

- macOS/Linux 일반 사용자: 설치 스크립트 또는 Homebrew tap
- Windows 일반 사용자: PowerShell 설치 스크립트 또는 Scoop URL 직접 설치
- 어떤 OS든 수동 설치: GitHub Releases 바이너리
- macOS/Linux + Homebrew 사용 중: Homebrew tap
- Go 개발자: `go install`
- 내부 배포나 로컬 개발: `make build`

## agentcom 동작 방식

`agentcom`은 등록된 에이전트, 메시지, 태스크, 상태 확인용 타임스탬프를 모두 SQLite에 저장합니다. 각 등록된 프로세스는 자신의 Unix Domain Socket 서버도 열기 때문에, direct message는 먼저 소켓 경로로 전달을 시도하고 대상 소켓을 사용할 수 없으면 SQLite inbox polling으로 폴백합니다.

일반적인 사용 흐름은 다음과 같습니다.

1. `agentcom init`으로 로컬 상태를 초기화하고 프로젝트/템플릿을 선택합니다.
2. `agentcom up`으로 active template의 모든 역할을 detached 관리 프로세스로 기동합니다.
3. CLI 명령 또는 MCP 도구로 메시지, inbox, 태스크를 다룹니다.
4. `agentcom down`으로 관리 중인 템플릿 세션을 정상 종료합니다.
5. `agentcom register`는 단일 에이전트를 수동으로 띄우고 싶을 때만 사용하는 저수준 명령으로 남겨 둡니다.

## 로컬 상태와 설정

기본 저장 경로는 `~/.agentcom` 입니다.

- SQLite DB: `~/.agentcom/agentcom.db`
- 소켓 디렉터리: `~/.agentcom/sockets/`

프로젝트별 로컬 메타데이터도 함께 사용합니다.

- 프로젝트 설정: `.agentcom.json`
- 템플릿 스캐폴드: `.agentcom/templates/<template>/COMMON.md`, `.agentcom/templates/<template>/template.json`
- `up` 런타임 상태: `.agentcom/run/up.json`

기본 경로는 `AGENTCOM_HOME` 환경변수로 바꿀 수 있습니다.

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom init
```

이 방식은 테스트, 데모, 프로젝트별 격리, 같은 머신에서 여러 환경을 병렬로 돌릴 때 유용합니다.

## 전역 플래그

모든 명령은 다음 전역 플래그를 지원합니다.

- `--json` - 가능한 경우 기계가 읽기 쉬운 JSON 출력
- `-v`, `--verbose` - `log/slog` 기반 디버그 로그 활성화
- `--project <name>` - 현재 프로젝트 스코프 override
- `--all-projects` - 프로젝트 스코프를 우회하고 전체 조회

예시:

```bash
agentcom --json list
agentcom --verbose health
```

## 빠른 시작

프로젝트와 active template 초기화:

```bash
agentcom init --project myapp --template company
agentcom --json init --project myapp --template company
```

`.agentcom.json`은 현재 디렉터리 또는 상위 디렉터리에서 자동 탐색되며, `project`와 `template.active`를 저장합니다.

현재 디렉터리에 agent별 instruction 파일 생성:

```bash
agentcom init --agents-md
agentcom init --batch --agents-md claude,codex
```

공통 지침과 6개 역할 스킬이 포함된 내장 템플릿 생성:

```bash
agentcom init --template company
agentcom init --template oh-my-opencode
agentcom init --template custom
```

생성 전에 내장 템플릿 조회:

```bash
agentcom agents template
agentcom agents template company
agentcom --json agents template oh-my-opencode
```

인터랙티브 터미널에서 템플릿 이름 없이 `agentcom agents template`를 실행하면 검색어를 입력한 뒤 번호로 템플릿을 선택할 수 있습니다.

템플릿에 정의된 에이전트 전체 시작:

```bash
agentcom up
agentcom up --only frontend,plan
agentcom --json up
```

기본 `up` 동작은 즉시 detach되며 런타임 메타데이터를 `.agentcom/run/up.json`에 기록합니다. 종료는 `down`으로 처리합니다.

```bash
agentcom down
agentcom down --only plan
agentcom down --timeout 15
agentcom --json down
```

관리 중인 에이전트 확인:

```bash
agentcom list
agentcom health
```

직접 메시지 전송:

```bash
agentcom send --from frontend plan '{"text":"hello"}'
```

브로드캐스트 전송:

```bash
agentcom broadcast --from alpha --topic sync '{"status":"ready"}'
```

태스크 생성, 위임, 갱신:

```bash
agentcom task create "Implement MCP tests" --creator frontend --assign plan --priority high
agentcom task list --assignee plan
agentcom task delegate <task-id> --to plan
agentcom task update <task-id> --status in_progress --result "started"
```

inbox와 상태 확인:

```bash
agentcom inbox --agent plan --unread
agentcom status
```

지원하는 모든 에이전트 CLI용 프로젝트 스킬 생성:

```bash
agentcom skill create review-pr --agent all --scope project --description "일관된 PR 리뷰"
```

## 명령별 상세 사용법

### `agentcom init`

홈 디렉터리를 초기화하고, 하위 디렉터리를 보장하며, SQLite DB를 열고, 마이그레이션을 적용합니다.

사용 예시:

```bash
agentcom init
agentcom init --agents-md
agentcom init --batch
agentcom init --batch --agents-md claude,codex
agentcom init --template company
agentcom init --template oh-my-opencode
agentcom init --template custom
agentcom --json init
```

참고:

- 여러 번 실행해도 안전합니다.
- 인터랙티브 터미널에서는 `agentcom init`가 기본적으로 onboarding wizard를 실행합니다.
- `--batch`는 비대화형 흐름을 강제하며, `--json`일 때도 자동 적용됩니다.
- `--agents-md`는 `all` 또는 `claude,codex,cursor` 같은 콤마 구분 agent 목록을 받습니다. `agentcom init --batch --agents-md`처럼 값 없이 쓰면 기존처럼 `AGENTS.md`를 생성합니다.
- `--template`는 `.agentcom/templates/<template>/COMMON.md`, `.agentcom/templates/<template>/template.json`, 각 지원 agent별 shared `agentcom/SKILL.md`, 그리고 `agentcom/<template>-frontend` 형태의 6개 namespaced role skill을 생성합니다.
- `--template`를 지정하면 `.agentcom.json`에 `template.active`도 함께 기록되며, 이후 `agentcom up`이 이를 기본 입력으로 사용합니다.
- 내장 템플릿은 `company`, `oh-my-opencode`이며, `custom`은 인터랙티브 템플릿 생성 wizard를 엽니다.
- `agentcom agents template --list`는 built-in/custom 템플릿을 함께 보여주고, `agentcom agents template --delete <name>`는 확인 후 custom 템플릿을 삭제합니다.
- JSON 출력에는 상황에 따라 `path`, `status`, `project`, `project_config_path`, `template`, `active_template`, `instruction_files`, `agents_md`, `memory_files`, `custom_template_path`, `generated_files`가 포함됩니다.
- 현재 구현상 홈 디렉터리를 먼저 준비한 뒤 `init` 상태를 검사하므로, 새 경로에서도 `status`가 `already_initialized`로 보일 수 있습니다.

### `agentcom up`

현재 프로젝트의 active template을 기동합니다. 기본 동작은 detach이며, 백그라운드 supervisor가 각 역할마다 `register` 자식 프로세스를 하나씩 시작합니다.

사용 예시:

```bash
agentcom up
agentcom up --template company
agentcom up --only frontend,plan
agentcom up --force
agentcom --json up
```

플래그:

- `--template <name>` - `.agentcom.json`의 `template.active`를 override하고 새 값으로 저장
- `--only <roles>` - 지정한 역할명만 시작
- `--force` - 기존 관리 세션이 있으면 먼저 종료하고 다시 시작

참고:

- interactive terminal에서 프로젝트가 아직 초기화되지 않았다면 `up`이 먼저 `agentcom init`을 실행합니다.
- non-interactive 환경에서는 `.agentcom.json`이 없으면 에러를 반환합니다.
- 런타임 메타데이터는 `.agentcom/run/up.json`에 저장됩니다.
- 템플릿 기반 팀을 시작할 때의 기본 고수준 명령은 `agentcom up`입니다.

### `agentcom down`

`agentcom up`으로 시작한 에이전트를 종료합니다.

사용 예시:

```bash
agentcom down
agentcom down --only plan
agentcom down --timeout 15
agentcom down --force
agentcom --json down
```

플래그:

- `--only <roles>` - 지정한 역할명만 종료
- `--timeout <seconds>` - graceful shutdown 대기 시간
- `--force` - graceful shutdown 없이 즉시 종료

참고:

- `down`은 `.agentcom/run/up.json`을 기본 런타임 정보 원천으로 사용합니다.
- 모든 역할이 종료되면 런타임 상태 파일을 제거합니다.

### `agentcom register`

현재 프로세스를 live agent로 등록하고, heartbeat 갱신, Unix Domain Socket 서버, fallback poller를 시작합니다. 이 명령은 의도적으로 오래 실행되는 명령이며, 인터럽트될 때까지 종료되지 않습니다.

전체 템플릿 팀을 띄우려는 경우에는 `register`보다 `agentcom up`을 권장합니다. `register`는 단일 에이전트를 수동으로 띄우는 고급/저수준 인터페이스입니다.

사용 예시:

```bash
agentcom register --name alpha --type codex
agentcom register --name alpha --type codex --cap plan,execute --workdir /path/to/project
agentcom --json register --name alpha --type codex
```

플래그:

- `--name` - 필수, 고유한 에이전트 이름
- `--type` - 필수, 자유 문자열 타입
- `--cap` - 선택, 쉼표로 구분한 capability 목록
- `--workdir` - 선택, 작업 디렉터리. 기본값은 현재 디렉터리

참고:

- `SIGINT`/`SIGTERM`을 받으면 자동으로 deregister 됩니다.
- `--json` 사용 시 먼저 등록 메타데이터를 출력한 뒤 프로세스는 계속 실행됩니다.
- 등록된 에이전트는 `agt_...` 형식 ID와 소켓 디렉터리 아래의 socket path를 가집니다.

### `agentcom deregister`

이름 또는 ID로 에이전트를 해제합니다.

사용 예시:

```bash
agentcom deregister alpha
agentcom deregister agt_xxx --force
agentcom --json deregister alpha --force
```

플래그:

- `--force` - 대화형 확인 프롬프트 생략

참고:

- 기본 동작은 삭제 전에 확인을 묻습니다.
- 메시지와 태스크 이력은 deregister 이후에도 DB에 유지됩니다.

### `agentcom list`

등록된 에이전트를 조회합니다.

사용 예시:

```bash
agentcom list
agentcom list --alive
agentcom --json list
```

플래그:

- `--alive` - 현재 `alive` 상태인 에이전트만 표시

### `agentcom send`

등록된 sender에서 target agent로 direct message를 전송합니다.

사용 예시:

```bash
agentcom send --from alpha beta '{"text":"hello"}'
agentcom send --from alpha beta 'plain text message'
agentcom send --from alpha --type request --topic review beta '{"file":"README.md"}'
agentcom send --from alpha --correlation-id corr-123 beta '{"step":1}'
agentcom --json send --from alpha beta '{"text":"hello"}'
```

플래그:

- `--from` - 필수, sender agent 이름
- `--type` - 선택, 메시지 타입. 기본값은 `notification`
- `--topic` - 선택, topic 문자열
- `--correlation-id` - 선택, correlation ID

payload 처리 방식:

- 두 번째 인자가 유효한 JSON object/array 문자열이면 그대로 저장합니다.
- 아니면 `{"text":"..."}` 형태로 감싸서 저장합니다.

### `agentcom broadcast`

sender를 제외한 모든 alive agent에게 동일 메시지를 전송합니다.

사용 예시:

```bash
agentcom broadcast --from alpha '{"status":"ready"}'
agentcom broadcast --from alpha --topic sync '{"phase":"wave-9"}'
agentcom --json broadcast --from alpha '{"status":"ready"}'
```

플래그:

- `--from` - 필수, sender agent 이름
- `--topic` - 선택, topic 문자열

### `agentcom inbox`

특정 agent의 inbox 메시지를 SQLite에서 조회합니다.

사용 예시:

```bash
agentcom inbox --agent beta
agentcom inbox --agent beta --unread
agentcom inbox --agent agt_xxx --from agt_sender_id
agentcom --json inbox --agent beta --unread
```

플래그:

- `--agent` - 필수, agent 이름 또는 ID
- `--unread` - unread 메시지만 표시
- `--from` - sender agent ID로 필터링

### `agentcom task`

태스크 관리는 네 개의 하위 명령으로 구성됩니다.

태스크 생성:

```bash
agentcom task create "Implement docs" --desc "Expand README" --creator alpha --assign beta --priority high
agentcom task create "Ship release" --blocked-by P7-01,P7-02
agentcom --json task create "Implement docs" --creator alpha
```

태스크 조회:

```bash
agentcom task list
agentcom task list --status pending
agentcom task list --assignee beta
agentcom --json task list --status in_progress
```

태스크 상태 갱신:

```bash
agentcom task update <task-id> --status in_progress --result "started"
agentcom task update <task-id> --status completed --result "done"
```

태스크 위임:

```bash
agentcom task delegate <task-id> --to beta
agentcom --json task delegate <task-id> --to agt_xxx
```

중요한 동작:

- `task create`는 새 태스크를 `pending` 상태로 저장합니다.
- `--assign`, `--creator`는 agent 이름 또는 ID를 받을 수 있습니다.
- `task list --assignee`는 조회 전에 이름을 ID로 해석하려고 시도합니다.
- `task update`는 status/result를 직접 기록합니다.
- `task delegate`는 `assigned_to`를 대상 에이전트로 갱신합니다.

### `agentcom status`

에이전트 수, 메시지 수, unread 메시지 수, 전체 태스크 수, 상태별 태스크 수를 요약해서 보여줍니다.

사용 예시:

```bash
agentcom status
agentcom --json status
```

### `agentcom skill`

지원하는 코딩 에이전트 CLI용 스킬 파일을 생성합니다.

사용 예시:

```bash
agentcom skill create review-pr --agent claude --scope project --description "일관된 PR 리뷰"
agentcom skill create pairing-notes --agent cursor --scope project
agentcom skill create docs-sync --agent github-copilot --scope user
agentcom skill create release-check --agent all --scope user
agentcom --json skill create docs-sync --agent gemini-cli --scope project
```

플래그:

- `--agent` - 대상 에이전트 식별자 또는 `all` (기본값 `all`)
- `--scope` - 생성 범위: `project|user` (기본값 `project`)
- `--description` - 선택적 스킬 설명. 기본값은 `Skill generated by agentcom`

생성 경로:

- `claude` - project: `.claude/skills/<name>/SKILL.md`, user: `~/.claude/skills/<name>/SKILL.md`
- `codex` - project: `.agents/skills/<name>/SKILL.md`, user: `~/.agents/skills/<name>/SKILL.md`
- `gemini` - project: `.gemini/skills/<name>/SKILL.md`, user: `~/.gemini/skills/<name>/SKILL.md`
- `opencode` - project: `.opencode/skills/<name>/SKILL.md`, user: `~/.config/opencode/skills/<name>/SKILL.md`
- `cursor` - project: `.cursor/skills/<name>.mdc`, user: `~/.cursor/skills/<name>.mdc`
- `github-copilot` - project: `.github/skills/<name>.md`, user: `~/.github/skills/<name>.md`
- `universal` - project: `skills/<name>/SKILL.md`, user: `~/skills/<name>/SKILL.md`

추가 지원 식별자로는 `claude-code`, `gemini-cli`, `droid` 같은 alias와 `cline`, `continue`, `roo-code`, `kilo-code`, `windsurf`, `devin`, `replit-agent`, `bolt`, `lovable`, `tabnine`, `tabby`, `amazon-q`, `sourcegraph-cody`, `augment-code`, `factory`, `goose`, `openhands`, `qwen` 등이 있습니다.

참고:

- 스킬 이름은 소문자, 숫자, 단일 하이픈만 사용할 수 있습니다.
- 기존 스킬 파일은 덮어쓰지 않습니다.
- `--agent all`은 지원하는 모든 에이전트에 대해 파일을 생성하고, 첫 번째 쓰기 실패에서 즉시 중단합니다.
- 출력 포맷은 agent마다 다릅니다. 대부분은 `SKILL.md`를 쓰고, `cursor`는 `.mdc`, `github-copilot`, `windsurf`, `devin`, `replit-agent`, `bolt`, `lovable`, `playcode-agent`는 `.md`를 사용합니다.

### `agentcom agents template`

내장 템플릿 목록을 보거나, 특정 템플릿 정의를 자세히 확인합니다.

사용 예시:

```bash
agentcom agents template
agentcom agents template company
agentcom --json agents template oh-my-opencode
```

인터랙티브 동작:

- interactive terminal에서 템플릿 이름 없이 실행하면 검색어 입력 후 번호 선택 프롬프트가 나옵니다.
- non-interactive 또는 `--json` 모드에서는 기존 목록/상세 출력 동작을 유지합니다.

내장 템플릿:

- `company` - Paperclip 역할 구조에서 영감을 받은 회사형 멀티 에이전트 워크플로우
- `oh-my-opencode` - Prometheus, Momus, Oracle, Sisyphus-Junior 패턴에서 영감을 받은 planning 중심 워크플로우

생성되는 scaffold:

- 공통 지침: `.agentcom/templates/<template>/COMMON.md`
- 템플릿 메타데이터: `.agentcom/templates/<template>/template.json`
- 프로젝트용 shared template skill: `.claude/skills/agentcom/SKILL.md`, `.agents/skills/agentcom/SKILL.md`, `.gemini/skills/agentcom/SKILL.md`, `.opencode/skills/agentcom/SKILL.md`
- role skill은 같은 namespace 아래 예를 들어 `.agents/skills/agentcom/company-frontend/SKILL.md` 형태로 생성됩니다.
- 각 role skill은 먼저 shared `../SKILL.md`, 그다음 template `COMMON.md`를 읽도록 생성되며 `frontend`, `backend`, `plan`, `review`, `architect`, `design` 간 communication map을 포함합니다.

### `agentcom health`

등록된 에이전트 기준의 health view를 보여줍니다.

사용 예시:

```bash
agentcom health
agentcom --json health
```

전체 태스크/메시지 집계보다 live agent 상태를 빠르게 보고 싶을 때 쓰면 됩니다.

JSON 출력은 health entry 배열이며, 빈 환경에서는 `[]`를 반환합니다.

### `agentcom version`

빌드 메타데이터를 출력합니다.

사용 예시:

```bash
agentcom version
agentcom --json version
```

### `agentcom mcp-server`

STDIO 기반 MCP JSON-RPC 서버를 시작합니다.

사용 예시:

```bash
agentcom mcp-server
agentcom mcp-server --register mcp-agent --type mcp
```

플래그:

- `--register <name>` - 서버를 agent로 함께 등록할 때 사용
- `--type <type>` - `--register`와 함께 쓸 agent type

참고:

- `tools/list`, `tools/call` 전에 반드시 `initialize`가 먼저 와야 합니다.
- 제공 도구는 agent 조회, 메시지 전송, broadcast, 태스크 생성/위임, 태스크 조회, 상태 조회를 포함합니다.

## JSON 출력 예시

초기화:

```bash
agentcom --json init
```

출력 형태 예시:

```json
{
  "active_template": "company",
  "path": "/Users/name/.agentcom",
  "status": "initialized or already_initialized"
}
```

관리 세션 시작:

```bash
agentcom --json up
```

출력 형태 예시:

```json
{
  "status": "started",
  "project": "myapp",
  "template": "company",
  "supervisor_pid": 12345,
  "runtime_state": "/path/to/project/.agentcom/run/up.json"
}
```

에이전트 목록 조회:

```bash
agentcom --json list
```

메시지 전송:

```bash
agentcom --json send --from alpha beta '{"text":"hello"}'
```

태스크 목록 조회:

```bash
agentcom --json task list --status pending
```

## 전체 흐름 예시

프로젝트 터미널:

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom init --project demo --template company
agentcom up --only frontend,plan
```

작업 터미널:

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom send --from frontend plan '{"text":"please review README"}'
agentcom inbox --agent plan --unread
agentcom task create "Review README" --creator frontend --assign plan --priority high
agentcom task list --assignee plan
agentcom status
agentcom health
agentcom down
```

## MCP 사용법

MCP 서버 시작:

```bash
agentcom mcp-server
```

핸드셰이크 순서 예시:

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
```

사용 가능한 도구:

- `list_agents`
- `send_message`
- `broadcast`
- `create_task`
- `delegate_task`
- `list_tasks`
- `get_status`

## 아키텍처

```text
                  +----------------------+
                  |     agentcom CLI     |
                  +----------+-----------+
                             |
          +------------------+------------------+
          |                                     |
          v                                     v
 +-------------------+                 +-------------------+
 |   SQLite (WAL)    |                 | Unix Domain Socket|
 | agents/messages/  |                 | server + client   |
 | tasks source      |                 | low-latency IPC   |
 +---------+---------+                 +---------+---------+
           |                                     |
           +------------------+------------------+
                              |
                              v
                    +-------------------+
                    | fallback poller   |
                    | unread inbox scan |
                    +-------------------+
```

## 개발

```bash
make build
make test
make lint
make vet
```

현재 테스트 스위트에는 DB CRUD 테스트, registry 테스트, transport integration 테스트, task manager 테스트, MCP roundtrip 테스트, end-to-end CLI 시나리오 테스트가 포함되어 있습니다.
