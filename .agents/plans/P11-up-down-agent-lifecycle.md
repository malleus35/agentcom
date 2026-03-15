# P11 — `agentcom up` / `agentcom down`: 에이전트 일괄 라이프사이클 관리

> **상태**: 계획 단계 (구현 미착수)
> **선행 의존**: P10 project column (완료), P9 init overhaul (완료)
> **목적**: 프로젝트의 모든 에이전트를 한 번에 등록/해제하는 배치 라이프사이클 명령 추가
> **핵심 고민**: init → up 플로우의 직관성 확보

---

## 1. 문제 정의

### 현재 플로우 (as-is)

```
agentcom init                                    # 1회: DB 생성, 소켓 디렉토리, 프로젝트 설정
agentcom init --template oh-my-opencode          # 1회: 템플릿 스캐폴딩 (SKILL.md, COMMON.md)
agentcom register --name oracle --type architect  # 터미널 1: 블로킹
agentcom register --name frontend --type engineer # 터미널 2: 블로킹
agentcom register --name backend --type engineer  # 터미널 3: 블로킹
...                                               # 6개 역할 × 각각 터미널
```

**문제점:**
1. 템플릿에 6개 역할이 정의되어 있는데 수동으로 하나씩 register 해야 함
2. `register`는 블로킹이라 터미널 6개를 열어야 함
3. 에이전트 이름/타입을 `template.json`에서 일일이 확인해서 타이핑해야 함
4. 종료 시에도 각 터미널에서 Ctrl+C를 개별로 해야 함

### 원하는 플로우 (to-be)

```
agentcom init                # 1회: 환경 초기화 + 템플릿 선택 (wizard)
agentcom up                  # 매번: 전체 에이전트 일괄 기동
agentcom down                # 매번: 전체 에이전트 일괄 종료
```

---

## 2. 핵심 설계 고민: init → up 플로우 직관성

### 2.1 현재 `init`이 하는 일이 너무 많다

`agentcom init`은 현재 4개의 서로 다른 관심사를 하나에 묶고 있다:

| 관심사 | 플래그 | 하는 일 |
|--------|--------|---------|
| 플랫폼 초기화 | (기본) | ~/.agentcom, SQLite DB, sockets/ 생성 |
| 프로젝트 설정 | `--project` | .agentcom.json 작성 |
| 지침 파일 생성 | `--agents-md` | CLAUDE.md, .cursorrules 등 에이전트별 instruction 파일 |
| 템플릿 스캐폴딩 | `--template` | COMMON.md, template.json, 6개 role SKILL.md |

여기에 `up`을 추가하면 사용자가 혼란을 느낀다:
```
agentcom init --template oh-my-opencode  # 템플릿 "설정"
agentcom up --template oh-my-opencode    # 또 템플릿? 뭐가 다른 거지?
```

### 2.2 업계 CLI 패턴 벤치마크

| CLI | init이 하는 일 | run/up이 하는 일 | init과 run의 레이어 차이 |
|-----|---------------|-----------------|----------------------|
| **Docker Compose** | 없음 (compose.yaml이 전부) | config 읽어서 컨테이너 기동 | init 없이 바로 up |
| **Terraform** | provider 플러그인 설치, backend 연결 | 인프라 변경 (API 호출) | 설치 vs 실행 (비용/위험도 차이) |
| **Dapr** | 런타임 바이너리/컨테이너 설치 | 앱 프로세스 기동 | 플랫폼 vs 애플리케이션 |
| **PM2** | ecosystem.config.js 스캐폴딩 (선택적) | config 읽어서 프로세스 fork | 파일 생성 vs 프로세스 관리 |
| **systemd** | unit 파일 작성 (사람이) | 프로세스 기동 | 선언 vs 실행 |
| **Kubernetes** | 없음 (kubectl apply가 멱등) | manifest → 컨트롤러 수렴 | init 불필요 |

**공통 통찰:**
- init과 up/run이 **다른 레이어**를 담당할 때 두 단계가 자연스럽다
- init이 "환경/플랫폼 준비", up이 "프로세스 실행"이면 직관적
- **config 파일이 "무엇을 실행할지"의 단일 진실 원천**이 되면 가장 매끄럽다

### 2.3 Oracle 권고 요약

> `up`을 "프로젝트를 원하는 상태로 만들고(필요 시 init 포함) 그 상태를 실행으로 올리는" 단일 진입점으로 만들라.
> `.agentcom.json`에 `template.active`를 기록해서 `up`의 기본 입력으로 사용하라.
> `register`는 고급/단일 에이전트용 "plumbing"으로 자연스럽게 분리된다.

---

## 3. 설계 제안

### 옵션 A: "init = 플랫폼, up = 런타임" 분리 (Terraform/Dapr 패턴)

```bash
# 1단계: 플랫폼 초기화 (한 번만)
agentcom init
# → DB 생성, sockets/, .agentcom.json (프로젝트명 + active template 기록)
# → wizard에서 템플릿 선택 시 → template.json + SKILL.md 스캐폴딩까지 완료

# 2단계: 에이전트 기동 (매번)
agentcom up
# → .agentcom.json의 template.active를 읽음
# → template.json에서 역할 목록 로드
# → 각 역할을 subprocess로 register (heartbeat, UDS, poller 포함)
# → 런타임 상태를 .agentcom/run/up.json에 기록 (PID, socket, role 매핑)

# 에이전트 종료
agentcom down
# → .agentcom/run/up.json 읽어서 SIGINT 전파
# → 각 에이전트 graceful deregister 대기
# → 런타임 파일 정리
```

**장점:**
- `init`과 `up`의 레이어가 명확히 분리됨 (환경 준비 vs 프로세스 실행)
- `init`이 기록한 active template을 `up`이 자동으로 읽으므로 중복 지정 불필요
- 기존 `register` 명령은 "plumbing" 레이어로 자연스럽게 공존

**단점:**
- 여전히 두 단계 (init → up) 필요
- 신규 사용자가 "init 안 하고 up만 치면?" 케이스 처리 필요

**edge case 처리:**
- `up` 실행 시 init 안 됨 → TTY면 init wizard 자동 실행, non-TTY면 에러 + 안내
- `up --template X` → active template을 X로 변경 + .agentcom.json 업데이트 + 기동
- `up --only oracle` → 특정 역할만 기동 (PM2 패턴)
- 템플릿 변경 후 → `down` + `up` 필요 (자동 반영 안 함, 복잡도 제한)

### 옵션 B: "up이 init을 흡수" (Docker Compose 패턴)

```bash
# 이것 하나로 0 → 전체 에이전트 기동
agentcom up
# → .agentcom.json 없으면? → wizard 실행 (init 흡수)
# → DB 없으면? → 자동 생성
# → template 없으면? → 선택 prompt
# → 모든 에이전트 기동

# 종료
agentcom down
```

**장점:**
- 최소 마찰: 명령 하나로 모든 것 해결
- 신규 사용자 경험 최적

**단점:**
- `init`의 존재 이유가 모호해짐 ("up이 다 해주는데 init은 왜?")
- `init`의 순수 설정 기능(agents-md, template scaffold만)이 `up`과 뒤섞임
- `up`이 너무 많은 책임을 가짐

**`init`의 새 역할:**
- "설정만 하고 실행은 안 하는" 모드로 재포지셔닝
- `agentcom init --template company` → 스캐폴딩만, 에이전트 기동 안 함
- CI/스크립트용, 또는 스캐폴딩만 원하는 경우

### 옵션 C: 선언적 설정 파일 중심 (PM2 + k8s 패턴)

```bash
# 설정 파일 생성 (init 또는 수동)
agentcom init
# → agentcom.yaml 생성 (template.json과 별도의 "실행 명세")

# agentcom.yaml
# version: 1
# template: oh-my-opencode
# agents:
#   - name: oracle
#     type: architect
#     capabilities: [review, architecture]
#   - name: frontend
#     type: sisyphus-junior/visual-engineering
#     capabilities: [ui, css, animation]
#   ...

# 기동
agentcom up                # agentcom.yaml 자동 탐색
agentcom up -f custom.yaml # 명시적 파일 지정

# 종료
agentcom down
```

**장점:**
- 설정과 실행의 관심사가 완벽히 분리
- 설정 파일을 git에 커밋 가능 (팀 공유)
- `template.json`과 별도로 "실행 명세"를 관리 → 역할 정의와 런타임 설정 분리

**단점:**
- 새로운 설정 파일 포맷 도입 (template.json과 중복 가능성)
- template.json에 이미 AgentName/AgentType이 있는데 또 다른 파일?

---

## 4. .agentcom.json 확장 vs 새 파일

### 현재 .agentcom.json

```json
{
  "project": "myapp"
}
```

### 확장 방안 A: .agentcom.json에 active template 추가

```json
{
  "project": "myapp",
  "template": {
    "active": "oh-my-opencode"
  }
}
```

`up`은 이 파일에서 active template을 읽고 → `.agentcom/templates/<active>/template.json`에서 역할 목록을 로드.

### 확장 방안 B: .agentcom.json에 전체 에이전트 목록 포함

```json
{
  "project": "myapp",
  "template": "oh-my-opencode",
  "agents": [
    {"name": "oracle", "type": "architect", "capabilities": ["review"]},
    {"name": "frontend", "type": "sisyphus-junior/visual-engineering"}
  ]
}
```

### 확장 방안 C: 별도 agentcom.yaml

template.json과 독립적인 "실행 명세" 파일. PM2의 ecosystem.config.js에 해당.

---

## 5. up/down 프로세스 관리 방식

### 5.1 Supervisor 패턴 (권장)

`agentcom up`이 foreground supervisor 프로세스로 동작:

```
agentcom up (supervisor)
├── [subprocess] register --name oracle --type architect
├── [subprocess] register --name frontend --type engineer
├── [subprocess] register --name backend --type engineer
├── [subprocess] register --name plan --type planner
├── [subprocess] register --name review --type reviewer
└── [subprocess] register --name design --type designer
```

- supervisor가 각 자식 프로세스를 관리
- supervisor에 SIGINT → 모든 자식에 SIGINT 전파 → graceful shutdown
- 자식 프로세스 크래시 시 재시작 정책 (optional)

### 5.2 Detach 모드

```bash
agentcom up --detach  # 백그라운드 데몬으로 실행
agentcom down          # 데몬에 종료 신호
```

- `.agentcom/run/supervisor.pid`에 supervisor PID 기록
- `down`은 이 PID에 SIGINT 전송

### 5.3 런타임 상태 파일

```json
// .agentcom/run/up.json
{
  "started_at": "2026-03-15T12:00:00Z",
  "template": "oh-my-opencode",
  "supervisor_pid": 12345,
  "agents": [
    {"name": "oracle", "pid": 12346, "agent_id": "agt_abc123", "socket": "~/.agentcom/sockets/agt_abc123.sock"},
    {"name": "frontend", "pid": 12347, "agent_id": "agt_def456", "socket": "~/.agentcom/sockets/agt_def456.sock"}
  ]
}
```

`down`은 이 파일을 1차 신뢰 원천으로 사용. DB는 2차 확인용.

---

## 6. 명령 시그니처 (확정 필요)

### agentcom up

```
agentcom up [flags]

Flags:
  --template <name>    사용할 템플릿 (기본: .agentcom.json의 template.active)
  --only <roles>       특정 역할만 기동 (쉼표 구분)
  --detach             백그라운드 데몬으로 실행
  --force              이미 실행 중인 세션이 있어도 강제 시작

Global flags:
  --json               JSON 출력
  --project <name>     프로젝트 스코프 override
  --verbose            디버그 로깅
```

### agentcom down

```
agentcom down [flags]

Flags:
  --only <roles>       특정 역할만 종료 (쉼표 구분)
  --force              graceful 대기 없이 즉시 종료 (SIGKILL)
  --timeout <seconds>  graceful shutdown 대기 시간 (기본: 10s)

Global flags:
  --json               JSON 출력
  --project <name>     프로젝트 스코프 override
  --verbose            디버그 로깅
```

---

## 7. register와의 공존

| 명령 | 대상 | 모드 | 사용 시나리오 |
|------|------|------|-------------|
| `agentcom register` | 단일 에이전트 | 블로킹 (현재 터미널) | 디버깅, 임시 에이전트 추가, 스크립트 |
| `agentcom up` | 템플릿 전체 | supervisor (foreground/detach) | 프로젝트 에이전트 일괄 기동 |

README에서의 포지셔닝:
- **Quick Start** → `agentcom init` + `agentcom up` (high-level)
- **Advanced Usage** → `agentcom register --name X --type Y` (low-level, plumbing)
- `register`는 `git plumbing` 처럼 내부 동작 명령으로 문서화

---

## 8. README 내러티브 (안)

```markdown
## Quick Start

**1. Initialize** (once per project)
agentcom init

Sets up the database, selects a template, and scaffolds agent instruction files.

**2. Start all agents**
agentcom up

Reads the active template from .agentcom.json and registers all defined roles.

**3. Stop when done**
agentcom down

Gracefully shuts down all running agents.
```

3문장 요약:
> `agentcom init`으로 프로젝트를 초기화하고 템플릿을 선택합니다.
> `agentcom up`으로 템플릿에 정의된 모든 에이전트를 한 번에 기동합니다.
> `agentcom down`으로 전체 에이전트를 종료합니다.

---

## 9. 미결 사항 (구현 전 결정 필요)

| # | 질문 | 선택지 | 현재 기울기 |
|---|------|--------|-----------|
| 1 | `up` 실행 시 init 안 됨 처리 | A: 자동 init (up이 흡수) / B: 에러 + 안내 | A (TTY) / B (non-TTY) |
| 2 | active template 저장 위치 | A: .agentcom.json 확장 / B: 별도 파일 | A |
| 3 | 프로세스 관리 방식 | A: supervisor foreground / B: detach 기본 | A (--detach로 B 지원) |
| 4 | 런타임 상태 파일 위치 | A: .agentcom/run/up.json / B: SQLite에 통합 | A |
| 5 | 설정 파일 포맷 | A: .agentcom.json 확장 / B: agentcom.yaml 신규 / C: template.json 재사용 | C (기존 template.json을 up의 입력으로 직접 사용) |
| 6 | `register`의 README 포지셔닝 | A: Advanced Usage / B: 별도 섹션 유지 | A |
| 7 | 에이전트 재시작 정책 | A: crash 시 자동 재시작 / B: 단순 종료 | B (초기) → A (후속) |
| 8 | `up --only` granularity | A: 역할명 (oracle, frontend) / B: 에이전트 타입 | A |

---

## 10. 플로우 직관성 평가 매트릭스

각 옵션에 대해 직관성을 5가지 기준으로 평가:

| 기준 | 옵션 A (init=플랫폼, up=런타임) | 옵션 B (up이 init 흡수) | 옵션 C (선언적 yaml) |
|------|------|------|------|
| 신규 사용자 마찰 | ⭐⭐⭐ (2단계) | ⭐⭐⭐⭐⭐ (1단계) | ⭐⭐⭐ (파일 작성 필요) |
| 멘탈 모델 명확성 | ⭐⭐⭐⭐ (레이어 분리) | ⭐⭐⭐ (init 역할 모호) | ⭐⭐⭐⭐⭐ (파일=선언, up=실행) |
| 기존 코드 호환성 | ⭐⭐⭐⭐⭐ (최소 변경) | ⭐⭐⭐ (init 리팩토링 필요) | ⭐⭐⭐ (새 파일 포맷) |
| README 설명 용이성 | ⭐⭐⭐⭐ (Terraform 패턴) | ⭐⭐⭐⭐⭐ (한 줄) | ⭐⭐⭐⭐ (PM2 패턴) |
| 고급 사용 유연성 | ⭐⭐⭐⭐ (register 공존) | ⭐⭐⭐⭐ (register 공존) | ⭐⭐⭐⭐⭐ (yaml 커스텀) |

---

## 11. 태스크 분해 (구현 시)

> 아래는 구현 착수 전 참고용 태스크 분해다. 실제 구현 시 세부 조정 필요.

| ID | 태스크 | 의존성 | 예상 난이도 |
|----|--------|--------|-----------|
| P11-01 | `.agentcom.json`에 `template.active` 필드 추가 | P10 | Low |
| P11-02 | `init` wizard에서 선택한 템플릿을 active로 기록 | P11-01 | Low |
| P11-03 | `up` 명령 scaffold (cobra command, flags) | - | Low |
| P11-04 | template.json 로더 (역할 → 에이전트 정의 변환) | - | Low |
| P11-05 | subprocess supervisor 구현 | P11-03 | **High** |
| P11-06 | 런타임 상태 파일 (.agentcom/run/up.json) 관리 | P11-05 | Medium |
| P11-07 | `down` 명령 구현 (SIGINT 전파, graceful shutdown) | P11-06 | Medium |
| P11-08 | `up --detach` 데몬 모드 | P11-05 | Medium |
| P11-09 | `up --only` / `down --only` 선택적 기동/종료 | P11-05, P11-07 | Low |
| P11-10 | `up` 실행 시 init 미완료 자동 감지/처리 | P11-03 | Low |
| P11-11 | `up`/`down` 테스트 (unit + integration) | P11-05~07 | Medium |
| P11-12 | README 업데이트 (Quick Start 재구성, register 포지셔닝) | P11-11 | Low |
| P11-13 | JSON 출력 지원 (`--json`) | P11-05~07 | Low |
