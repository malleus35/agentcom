# PH12: CI Baseline Hardening

> **상태**: 초안 (2026-04-07)
> **브랜치**: `feature/PH12-ci-baseline-hardening`
> **우선순위**: High (develop 머지 차단 해소)
> **추정 공수**: ~18h (2 Track, 11 Tasks, ~34 Subtasks) · 여유 20% 포함 ~22h
> **선행 조건**: PH11 Wave 1 (`feat(ph11): wave 1`) + `fix(ci)` 시리즈 머지 완료

---

## 0. 배경 (Why this cycle)

### 0.1 정(Thesis) — 현재 상태

`fix(ci)` 시리즈(PR #2)에서 lint 파이프라인을 다시 살리고 Ubuntu의 data race를 제거해 develop 기준 CI 신호 중 **lint / ubuntu / macOS 세 축**을 복구했다. 그 과정에서 두 부류의 부채가 남겨졌다.

1. `.golangci.yml`에 `errcheck`를 **임시 비활성**한 상태로 봉인된 14건 이상의 pre-existing 미체크 에러
2. Windows runner에서 `TestSkillTargetPath/user_*`만 수정되고, 그 외의 **Windows-전용 실패 군**이 그대로 남아 있음 (`cmd/agentcom`, `internal/onboard`, `internal/transport` 3개 패키지)

### 0.2 반(Antithesis) — 이상 상태

- `.golangci.yml`에서 `errcheck` 비활성 라인이 제거되고, 기본 v2 린터 전체가 녹색 상태를 유지.
- Windows CI 잡이 주기적으로 green을 낼 수 있도록, UDS·프로세스·경로 관련 Windows 차이가 **각 테스트 스위트에서 개별적으로 해소**되거나 정당하게 skip 되어야 한다.
- develop의 CI가 lint / ubuntu / macOS / **windows** 4축 모두에서 기본 green 상태가 되어, 이후 feature PR이 Windows 깨짐을 일상적으로 무시하지 않게 된다.

### 0.3 합(Synthesis) — 이번 사이클

**기술 부채 클로징과 크로스-OS 복구를 한 사이클에 묶는다.** 두 트랙은 독립적이므로 병렬 worktree가 자연스럽다. 머지 게이트는 단일하다: develop의 CI가 4축 모두 green.

---

## 1. 목표 & 범위

### 1.1 핵심 목표

| # | 목표 | 측정 |
|---|------|------|
| G1 | `.golangci.yml`에서 `errcheck` 비활성 라인 제거 | lint job green without disabled linters |
| G2 | CI 4축(lint/ubuntu/macos/windows) 모두 기본 green | `gh pr view` statusCheckRollup에 FAILURE 0 |
| G3 | Windows CI 실패 원인이 **코드 또는 skip 정당화**로 이관 | 남는 `t.Skip()`은 반드시 이슈 링크 주석 동반 |

### 1.2 범위 밖 (명시적 연기)

| 항목 | 사유 | 재평가 시점 |
|------|------|-------------|
| β1 CGO 제거 | PH11과 동일 기준 (6개월 신호 수집) | 2026-10 |
| `testingcontext` / `slicescontains` / `errorsastype` 모던화 제안 | v2 suggestion(★) 수준, 에러 아님 | PH12 종료 후 별도 cleanup |
| `actions/checkout@v4` → v5 (Node.js 20 deprecation) | 워크플로 인프라 별건 | Node 20 강제 종료 전 |

---

## 2. 트랙 개요

| Track | 이름 | 역할 | 예상 공수 |
|-------|------|------|-----------|
| TA | errcheck 부채 클로징 | 린트 전체화 | ~6h |
| TB | Windows baseline 복구 | 크로스-OS 신뢰성 | ~11h |
| E2E | 머지 게이트 | 검증 | ~1h |
| **합계** | | | **~18h** |

---

## 3. Track A — errcheck 부채 클로징

**현재**: `.golangci.yml`에 `errcheck`가 disabled. `fix(ci)` 시리즈의 CI 로그에서 확인된 **14건의 초기 스캔 결과**는 다음과 같다 (완전한 목록은 TA.0.1에서 재생성).

| # | 파일:라인 | 호출 | 범주 |
|---|---|---|---|
| 1 | `internal/cli/init_prompter.go:121:22` | `database.Close` | resource |
| 2 | `internal/cli/init_prompter_test.go:104:22` | `database.Close` | resource (test) |
| 3 | `internal/cli/register.go:61:16` | `server.Start` | lifecycle |
| 4 | `internal/cli/status.go:111:27` | `messageRows.Close` | resource |
| 5 | `internal/cli/up.go:348:21` | `logFile.Close` | resource |
| 6 | `internal/cli/up.go:587:12` | `io.Copy` | I/O |
| 7 | `internal/cli/user_test.go:23:16` | `os.Chdir` | test-only |
| 8 | `internal/cli/user_test.go:68:16` | `os.Chdir` | test-only |
| 9 | `internal/cli/user_test.go:108:16` | `os.Chdir` | test-only |
| 10 | `internal/cli/version.go:32:15` | `fmt.Fprintf` | output |
| 11 | `internal/cli/version.go:33:15` | `fmt.Fprintf` | output |
| 12 | `internal/cli/version.go:34:15` | `fmt.Fprintf` | output |
| 13 | `internal/db/agent.go:53:18` | `stmt.Close` | resource |
| 14 | `internal/db/agent.go:89:18` | `stmt.Close` | resource |

### TA.0 — 완전 목록 확보

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| TA.0.1 | (로컬) | `.golangci.yml`에서 `errcheck` 한 번 재활성 후 `golangci-lint run ./... > /tmp/errcheck.txt` 로컬 실행, 결과를 본 문서 §3 표에 병합 | [ ] 표가 실제 CI 결과와 1:1 일치 [ ] 표의 총 건수 확정 | `diff` | 0.3h |
| TA.0.2 | `.agents/plans/PH12-ci-baseline-hardening.md` | 표 업데이트 및 각 항목의 **복구 방침**(wrap / `_ =` / refactor)을 1열 추가 | [ ] 모든 항목이 방침을 가짐 | 리뷰 | 0.3h |

### TA.1 — `fmt.Fprintf` 류 (output 계열)

`version.go`처럼 stdout/stderr로 헤더·버전 정보를 출력하는 케이스. 디스크풀 등 희귀 상황 제외하면 실패 확률은 0이지만, `errcheck`는 명시적인 처리를 요구한다.

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| TA.1.1 | `internal/cli/version.go` | `fmt.Fprintf` 3곳을 `if _, err := fmt.Fprintf(...); err != nil { return fmt.Errorf("cli.version: %w", err) }` 패턴으로 변경 | [ ] 3건 모두 처리 [ ] `version` 커맨드 수동 실행 OK | `go test ./internal/cli/... -run Version -count=1` | 0.5h |

### TA.2 — `Close` 류 (resource 계열, 프로덕션 코드)

`defer` 패턴에 맞물려 있는 경우가 대부분이므로 `defer func() { _ = x.Close() }()` 로 명시 무시하거나, 처음 발생한 에러를 우선시하도록 `errors.Join`/shadow 패턴을 쓴다.

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| TA.2.1 | `internal/cli/init_prompter.go:121` | `database.Close` 에러를 상위 error wrap (shadowing 방지 위해 `errors.Join`) | [ ] 기존 성공 경로 불변 [ ] 실패 경로에서 close 에러가 로그/리턴에 나타남 | `go test ./internal/cli/... -run InitPrompter -count=1` | 0.6h |
| TA.2.2 | `internal/cli/status.go:111` | `messageRows.Close` 를 `defer` 로 이동 + error wrap | [ ] rows leak 없음 | `go test ./internal/cli/... -run Status -count=1` | 0.5h |
| TA.2.3 | `internal/cli/up.go:348` | `logFile.Close` 를 `defer func(){ _ = logFile.Close() }()` 로 이동 (로그 파일이라 suppress 허용) | [ ] supervisor 정상 종료 시 log file 닫힘 | `go test ./internal/cli/... -run Up -count=1` | 0.4h |
| TA.2.4 | `internal/db/agent.go:53,89` | `stmt.Close` 2건을 `defer` 이동 + 기존 함수 에러와 합성 | [ ] 두 경로 모두 regression 없음 | `go test ./internal/db/... -count=1` | 0.6h |

### TA.3 — `io.Copy` / `server.Start` (lifecycle & I/O)

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| TA.3.1 | `internal/cli/up.go:587` | `io.Copy`를 `if _, err := io.Copy(...); err != nil && !errors.Is(err, io.EOF) { slog.Warn(...) }` 로 | [ ] EOF는 noise 제거 | `go test ./internal/cli/... -run Up -count=1` | 0.5h |
| TA.3.2 | `internal/cli/register.go:61` | `server.Start` 에러를 명시적 검사 후 상위로 리턴 | [ ] start 실패 시 에러 메시지 명확 | `go test ./internal/cli/... -run Register -count=1` | 0.5h |

### TA.4 — 테스트 코드 정리 (test-only 계열)

프로덕션 코드와 달리 `t.Chdir` 등 `testing.TB` 제공 헬퍼로 대체 가능.

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| TA.4.1 | `internal/cli/user_test.go:23,68,108` | `os.Chdir` 3곳을 `t.Chdir` (Go 1.24+) 로 교체, 리턴값 무시 이슈 자동 해결 | [ ] 3건 모두 `t.Chdir` 사용 [ ] cwd 복원 자동 | `go test ./internal/cli/... -run User -count=1` | 0.4h |
| TA.4.2 | `internal/cli/init_prompter_test.go:104` | `database.Close`를 `t.Cleanup(func(){ _ = database.Close() })` 로 승격 | [ ] 테스트 leak 없음 | `go test ./internal/cli/... -run InitPrompter -count=1` | 0.3h |

### TA.5 — `.golangci.yml` 재활성 + 회귀 방지

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| TA.5.1 | `.golangci.yml` | `disable: [errcheck]` 블록 삭제 + TODO 주석 제거 | [ ] 파일이 `version: "2"` + 기본 설정만 남음 | `golangci-lint run ./...` 로컬 green | 0.2h |
| TA.5.2 | `.golangci.yml` | `settings.errcheck` 섹션에 `check-type-assertions: true`, `check-blank: false` 명시(의도 기록) | [ ] 설정 문서화 | 리뷰 | 0.2h |
| TA.5.3 | `.agents/plans/PH12-ci-baseline-hardening.md` | 이 문서의 G1 체크박스 ON + 사후 표 업데이트 | [ ] 완료 표시 | grep | 0.1h |

### TA.6 — (선택) errcheck 예외 목록

사이클 절반 체크포인트(§11)에서 TA.0 결과가 예상(14)보다 2배 이상 많거나 refactor 비용이 폭발하는 케이스가 나오면, `.golangci.yml`에 **명시적 예외 패턴**을 남기는 것이 정당해진다. 예외는 파일 단위가 아닌 **함수 이름 단위**로 기록한다.

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| TA.6.1 | `.golangci.yml` (조건부) | `settings.errcheck.exclude-functions`에 `fmt.Fprintf, (*os.File).Close` 등을 등록, 각 항목에 사유 주석 | [ ] 각 예외가 1줄 이상의 사유 포함 | 리뷰 | 0.4h |

**TA 총 공수 (TA.6 제외)**: ~4.9h. TA.6 포함 시 ~5.3h. 표 업데이트 여유 포함 ~6h.

---

## 4. Track B — Windows baseline 복구

### 4.0 실패 군집 (3 패키지 · 8+ 테스트)

`fix(ci)` PR의 최종 windows-latest run에서 잔존한 Windows-전용 실패:

| 패키지 | 실패 테스트 | 원인 가설 |
|--------|-------------|----------|
| `cmd/agentcom` | `TestAgentcomE2EFlow` (72s) | `go run ./cmd/agentcom` 자식 프로세스가 Windows에서 signal / exit code / PTY 처리 차이 |
| `cmd/agentcom` | `TestAgentcomUpDownFlow` (2s) | 동일 가설 (supervisor 라이프사이클) |
| `cmd/agentcom` | `TestUserEndpointE2E` (1.6s) | 동일 가설 (user pseudo-agent IPC) |
| `cmd/agentcom` | `TestTaskPriorityAndReviewPolicyE2E` (1.6s) | 동일 가설 |
| `internal/onboard` | `TestResultValidate/*` (6 subtest) | `filepath` 비교 시 `\` vs `/` 또는 `result.Instructions` 기대값에 하드코딩된 `/` |
| `internal/onboard` | `TestWizardRun/*` (2 subtest) | 위저드 출력물의 경로 렌더링 동일 원인 |
| `internal/transport` | `TestServerClientRoundTrip` | Windows에서 UDS (`net.Dial("unix", ...)`) 경로 길이·절대경로 규약 차이 |
| `internal/transport` | `TestServerStartRemovesStaleSocket` | 동일 (소켓 파일 unlink 의미론) |

> **주의**: Windows는 Go 1.17부터 `AF_UNIX`를 네이티브 지원하지만, 소켓 경로 길이(<=104), 권한, `os.Remove` 타이밍 등에서 macOS/Linux와 다르다. UDS 기반 테스트가 Windows에서 flake하는 것은 agentcom 뿐 아니라 Go 생태계 전반의 알려진 난점.

### TB.0 — 실패 재현 & 원인 분리

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| TB.0.1 | (GH Actions debug) | windows-latest runner에서 `actions/upload-artifact`로 `*.log`, `testcase.json`, stderr 전체를 수집하는 debug 워크플로 임시 추가 | [ ] 한 번 실패한 run에서 artifact 확보 | GH Actions run | 0.6h |
| TB.0.2 | — | artifact 분석 후 3 패키지별 `.md` 메모 작성 (이 문서 §4 각 섹션에 붙임) | [ ] 각 군집의 root cause 한 줄 | 리뷰 | 0.7h |

### TB.1 — `internal/onboard` 경로 정규화

`TestResultValidate/*`, `TestWizardRun/*` 는 비교적 쉽다. `filepath.ToSlash` 로 기대값을 정규화하면 해결될 가능성이 높다.

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| TB.1.1 | `internal/onboard/result.go` | 파일 경로 비교가 실제 OS 구분자에 의존하는지 grep, 의존한다면 `filepath.ToSlash`로 비교 전 정규화 | [ ] 변경이 로직이 아닌 비교층에 한정 | `go vet` | 0.6h |
| TB.1.2 | `internal/onboard/result_test.go` | 골든 기대값에 하드코딩된 `/` 가 있다면 `filepath.Join`으로 교체, 또는 비교 시 정규화 | [ ] Windows runner 녹색 (TB.4에서 검증) | `go test ./internal/onboard/... -count=1` (macOS) | 0.8h |
| TB.1.3 | `internal/onboard/wizard_test.go` | 동일 점검 | [ ] 동일 | `go test` | 0.5h |
| TB.1.4 | `internal/onboard/template_definition.go` 또는 템플릿 포맷 | 템플릿에서 문자열 경로를 합성하는 부분이 `/` 를 하드코딩한다면 `filepath.ToSlash(filepath.Join(...))` 패턴 적용 | [ ] 렌더 결과 OS-independent | 골든 diff | 0.5h |

### TB.2 — `internal/transport` UDS on Windows

`fix(ci)` 이후 기본 Unix 경로 이슈가 아니라 **Windows UDS 고유 동작 차이**일 가능성이 높다. Windows UDS 경로는 절대경로를 요구하며, `os.Remove` 후 동일 이름 재사용 타이밍이 unix보다 엄격하다.

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| TB.2.1 | `internal/transport/transport_test.go` | `setupSocketPath` 가 생성하는 경로 길이·절대경로 확인. Windows에서 `t.TempDir()`은 절대경로를 돌려주므로 OK지만 총 길이가 104 바이트를 초과하지 않도록 단축 | [ ] Windows에서 경로 len <= 100 | `go test ./internal/transport/... -run Server -count=1` (macOS) | 0.6h |
| TB.2.2 | `internal/transport/uds.go` | `Start()`에서 stale 소켓 unlink 후 `listen` 실패 시 short backoff+retry (1회) 추가 — Windows에서 unlink가 즉시 반영 안 되는 경우 대비 | [ ] retry 1회, 총 100ms 이내 | 단위 테스트 | 0.8h |
| TB.2.3 | `internal/transport/uds_windows_test.go` (신규, build tag `//go:build windows`) | Windows 전용 회귀 테스트: 긴 경로 / 재사용 시나리오 | [ ] 테스트 존재 [ ] CI Windows에서 실행 | `go test` (windows CI) | 0.7h |
| TB.2.4 | `TestServerStartRemovesStaleSocket` | fake stale 소켓 생성을 `os.WriteFile`에서 `net.Listen` → `Close` 로 전환해 정말 "live한 적 있던" 소켓을 만든 뒤 unlink 검증 | [ ] Windows에서도 unlink 경로 실행 | 동일 | 0.5h |

### TB.3 — `cmd/agentcom` E2E 4건 (가장 묵직한 부분)

E2E는 `agentcom` 바이너리를 서브프로세스로 띄우고 IPC로 상호작용한다. Windows에서 가장 흔한 장애 원인은 ① shebang/PATH 차이 ② signal (`SIGTERM` 없음, `os.Interrupt`만) ③ stdio 버퍼링 ④ 임시 디렉토리 경로 구분자.

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| TB.3.1 | `cmd/agentcom/helpers_test.go` (또는 해당 e2e 헬퍼) | `binPath`를 `filepath.Join(..., "agentcom"+exeSuffix())` 로 통일 (`exeSuffix()`는 Windows에서 `.exe`) | [ ] helper 일관성 | 단위 | 0.4h |
| TB.3.2 | 동 | 프로세스 종료 시 `runtime.GOOS == "windows"` 면 `proc.Kill()`로 폴백 (SIGTERM 미지원) | [ ] Windows에서도 정상 종료 경로 | 수동 | 0.6h |
| TB.3.3 | `TestAgentcomUpDownFlow` | `agentcom up` → 소켓 ready-probe를 Windows에서 polling 기반으로 (UDS 생성 시점 차이) | [ ] `waitForSocket` 헬퍼 신규 | 단위 | 0.7h |
| TB.3.4 | `TestAgentcomE2EFlow` | 장시간(72s) 대기 케이스를 단축하기 위해 메시지 전송 후 `waitForInbox` 헬퍼로 polling — Windows에서 파일 lock 타이밍 완화 | [ ] <15s 내 완료 | 동일 | 1h |
| TB.3.5 | `TestUserEndpointE2E` | user pseudo-agent IPC 경로가 `%TEMP%` 아래 절대경로인지 확인, 길이 이슈 시 단축 | [ ] 경로 len OK | 동일 | 0.5h |
| TB.3.6 | `TestTaskPriorityAndReviewPolicyE2E` | 동일 점검 | [ ] 동일 | 동일 | 0.5h |
| TB.3.7 | 공통 | Windows에서만 flaky하면 `t.Skip("#ph12-windows-e2e", ...)` + 이슈 링크 — **최후의 수단**이며 G3 기준 정당화 주석 필수 | [ ] skip이 있다면 이슈 링크 포함 | 리뷰 | 0.3h |

### TB.4 — Windows CI 회귀 방지

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| TB.4.1 | `.github/workflows/ci.yml` | Windows 잡에 **`go test ./... -count=1 -timeout 5m`** 명시 (현재는 timeout 미지정) | [ ] 일관성 | CI run | 0.2h |
| TB.4.2 | `.github/workflows/ci.yml` | Windows 실패 시 artifact (`testresults/windows.xml`) 업로드 유지 | [ ] artifact 노출 | CI run | 0.2h |
| TB.4.3 | `.golangci.yml` | `//go:build windows` 전용 파일도 lint 대상에 포함되는지 확인 (기본 포함) | [ ] 문서화 | 리뷰 | 0.1h |

**TB 총 공수**: ~10.3h. 여유 포함 ~11h.

---

## 5. 머지 게이트 — E2E

| Subtask | 파일 | 변경 내용 | 수락 기준 | 검증 | 공수 |
|---------|------|-----------|-----------|------|------|
| E2E.1 | (CI 관찰) | PH12 최종 PR의 statusCheckRollup이 lint / ubuntu / macos / **windows** 4축 모두 SUCCESS | [ ] `gh pr view` FAILURE 0 | `gh pr view` | 0.3h |
| E2E.2 | `.agents/MEMORY.md` | 현재 상태/다음 작업 업데이트 | [ ] PH12 반영 | grep | 0.2h |
| E2E.3 | `.agents/plans/PH12-ci-baseline-hardening.md` | 본 문서 §13 완료 체크박스 전체 ON | [ ] 완료 표시 | 리뷰 | 0.1h |
| E2E.4 | `.github/workflows/ci.yml` | (TB 완료 후) Windows 잡을 `fail-fast: false` 유지 + **branch protection 대상에 포함**할지 결정 문서화 | [ ] 결정 기록 | 리뷰 | 0.4h |

**E2E 총 공수**: ~1h.

---

## 6. 의존성 그래프

```
TA.0 (실제 errcheck 목록 확정) ─┐
                               ├──► TA.1 ─┐
                               ├──► TA.2 ─┤
                               ├──► TA.3 ─┼──► TA.5 (.golangci.yml 재활성)
                               ├──► TA.4 ─┘
                               └──► (TA.6 조건부)

TB.0 (원인 분리) ──► TB.1 (onboard) ──┐
                ├──► TB.2 (transport) ┼──► TB.4 (CI 회귀 방지)
                └──► TB.3 (cmd E2E)  ─┘

(TA.5 머지 AND TB.4 머지) ──► E2E 머지 게이트
```

**주의**:
- TA 와 TB 는 상호 독립 → 병렬 worktree 가능.
- TA.0 완료 전에는 TA.1~TA.4 에서 "놓친 건"을 찾지 못할 수 있으므로, TA.0 을 wave 1 최우선으로.
- TB.0 은 실제 artifact 없이 진행하면 추측에 기반한 수정이 되므로, 반드시 artifact 1회 확보 후 착수.

---

## 7. 병렬 실행 가능한 Worktree 조합 (Wave)

### Wave 1 — 진단 (병렬 2 worktree)

| Worktree | 담당 | 선행 |
|----------|------|------|
| `wt-ta0-scan` | TA.0 (errcheck 전수) | 없음 |
| `wt-tb0-repro` | TB.0 (Windows artifact 수집) | 없음 |

**Wave 1 게이트**: 본 문서 §3 표 + §4 표가 **실제 CI 결과로 갱신**되어야 Wave 2 착수.

### Wave 2 — 수정 (병렬 4 worktree)

| Worktree | 담당 | 선행 |
|----------|------|------|
| `wt-ta-cleanup` | TA.1 ~ TA.5 | Wave 1 TA.0 |
| `wt-tb-onboard` | TB.1 | Wave 1 TB.0 |
| `wt-tb-transport` | TB.2 | Wave 1 TB.0 |
| `wt-tb-e2e` | TB.3 | Wave 1 TB.0 |

**주의**: `wt-tb-onboard` 와 `wt-tb-e2e` 는 둘 다 `filepath` 관련 헬퍼를 건드릴 수 있으므로 머지 순서에 주의. `wt-tb-onboard` → `wt-tb-e2e` 권장.

### Wave 3 — CI 회귀 방지 + 머지 게이트 (단일 worktree)

| Worktree | 담당 | 선행 |
|----------|------|------|
| `wt-ph12-gate` | TB.4, E2E.* | Wave 2 전부 |

---

## 8. 총 예상 공수

| Track | Subtasks | 공수 |
|-------|----------|------|
| TA (errcheck 클로징) | 12 (TA.6 제외) | ~5.3h |
| TB (Windows baseline) | 17 | ~10.3h |
| E2E 게이트 | 4 | ~1.0h |
| **합계** | **33+** | **~16.6h** |

여유(리뷰·리워크 20%) 포함 시 **~20h**. 실측에 따라 TA.6 추가 시 +0.4h.

---

## 9. 중간 체크포인트

사이클 공수 50% 시점에 다음을 점검한다. 하나라도 적신호면 범위 조정:

- [ ] TA.0.1 실제 errcheck 건수 ≤ 20 (예상 14의 1.5배 이내)
- [ ] TB.0.2 artifact 분석이 **3 가설 중 2 가설 이상**에 root cause 라벨 부착
- [ ] Wave 2 착수 후 3 worktree가 동일 파일을 건드리는 conflict 0건
- [ ] 자동화 불가능해 `t.Skip` 로 이관한 테스트 ≤ 2건

위 중 2개 이상 실패하면 TB.3 의 하위 4개 중 2개를 **PH13**로 연기하고 Windows CI 잡을 `continue-on-error: true` 로 임시 격하.

---

## 10. 연기 항목 (명시적 기록)

| 항목 | 사유 | 재평가 |
|------|------|--------|
| β1 CGO 제거 | 6개월 신호 수집 대기 | 2026-10 |
| `testingcontext` / `slicescontains` / `errorsastype` 모던화 | ★ 수준, 에러 아님 | PH12 종료 후 |
| `actions/checkout@v5` 업그레이드 | Node.js 20 강제 종료(2026-06) 전에 별건 PR | 2026-05 |
| Windows cmd/agentcom E2E를 Bash 전환 | 스크립트 계층 추가는 스코프 크리프 | PH13 후보 |

---

## 11. 완료 기준

### 11.1 기능

- [ ] TA: `.golangci.yml`에서 `errcheck` 비활성 라인이 사라짐 (`grep -n "errcheck" .golangci.yml` → matches in `settings.errcheck` only)
- [ ] TA: `golangci-lint run ./...` 로컬 실행 결과 issues=0
- [ ] TB: PH12 PR 최종 `gh pr view --json statusCheckRollup` 에서 lint / ubuntu-latest / macos-latest / **windows-latest** 모두 SUCCESS
- [ ] TB: 남은 `t.Skip` 이 있다면 전부 이슈 링크 주석 포함

### 11.2 품질 게이트

- [ ] `go build ./...`
- [ ] `go test ./... -race -count=1` (macOS/Linux)
- [ ] CI 4축 green
- [ ] `golangci-lint run ./...` green

### 11.3 문서

- [ ] 본 문서 §3, §4 표가 실제 결과로 갱신됨
- [ ] `.agents/MEMORY.md` 현재 상태 항목에 "PH12 완료" 기록

### 11.4 연기 기록

- [ ] PH13 후보 항목(있다면) `.agents/plans/PH13-*.md` 에 stub 문서 생성 또는 본 §10 하단에 "이월" 명시

---

## 12. 롤백 / 비상 계획

- **TA 롤백**: `.golangci.yml` 에서 `disable: [errcheck]` 재복원 (1줄).
- **TB 롤백**: CI windows 잡을 `continue-on-error: true` 로 격하하고 "baseline red, tracked in PH13" 주석.
- **최악 시나리오**: Wave 2 착수 후 2일 이내에 3 군집 중 하나라도 원인 미상 → 해당 군집 전체를 `t.Skip` 으로 이관 + PH13 이월. 이때 §11.1 세 번째 체크박스는 **조건부 green** 으로 표시한다 (skip된 테스트 수를 명시).

---

## 13. 참조

- PR #1 (PH11 Wave 1): `feat(ph11): wave 1 — SKILL.md schema_version + install-failure safety net`
- PR #2 (fix/ci-baseline): lint 복구, transport race 수정, Windows `TestSkillTargetPath` 수정
- `.agents/plans/PH11-onboarding-self-healing-sweep.md` — 상위 사이클
- `.agents/plans/NEW-NEXT-PHASE-PLAN.md` — PH5~PH9 baseline
- Go AF_UNIX on Windows: https://learn.microsoft.com/en-us/windows/win32/winsock/unix-domain-sockets (외부 링크, 읽기용 참조만)
