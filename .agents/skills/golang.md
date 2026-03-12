# Go 코딩 컨벤션 — agentcom

## 에러 처리

```go
// 에러는 항상 컨텍스트를 포함하여 래핑
if err != nil {
    return fmt.Errorf("db.InsertAgent: %w", err)
}

// sentinel 에러는 패키지 레벨 변수로 정의
var ErrAgentNotFound = errors.New("agent not found")
var ErrDuplicateName = errors.New("agent name already exists")
```

## 함수 시그니처

```go
// context.Context는 항상 첫 번째 파라미터
func (r *Registry) Register(ctx context.Context, name, agentType string) (*Agent, error)

// 옵션이 많으면 Options 패턴 사용
type RegisterOptions struct {
    Capabilities []string
    WorkDir      string
}
```

## 테스트

```go
// 테이블 기반 테스트 (Table-Driven Tests)
func TestInsertAgent(t *testing.T) {
    tests := []struct {
        name    string
        agent   Agent
        wantErr error
    }{
        {"정상 등록", Agent{Name: "backend", Type: "claude-code"}, nil},
        {"중복 이름", Agent{Name: "backend", Type: "codex"}, ErrDuplicateName},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}

// DB 테스트는 in-memory SQLite 사용
func setupTestDB(t *testing.T) *DB {
    t.Helper()
    db, err := Open(":memory:")
    require.NoError(t, err)
    t.Cleanup(func() { db.Close() })
    return db
}
```

## 인터페이스

```go
// 인터페이스는 사용하는 쪽(consumer)에서 정의
// transport 패키지가 agent 패키지의 인터페이스를 정의
type AgentFinder interface {
    FindByName(ctx context.Context, name string) (*agent.Agent, error)
}
```

## 패키지 구조

- `internal/` 하위만 사용 (외부 노출 없음)
- 순환 의존 금지: db ← agent ← message ← transport ← cli 방향만 허용
- config 패키지는 어디서든 임포트 가능

## SQLite 사용 규칙

```go
// 쿼리는 prepared statement 사용
stmt, err := db.PrepareContext(ctx, "SELECT * FROM agents WHERE name = ?")

// 트랜잭션은 명시적으로
tx, err := db.BeginTx(ctx, nil)
defer tx.Rollback()
// ... 작업 ...
return tx.Commit()
```

## 로깅

- 표준 `log/slog` 사용
- CLI 출력(사용자용)과 로그(디버깅용) 분리
- `--verbose` 플래그로 디버그 로그 활성화
