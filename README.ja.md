# agentcom

`agentcom` は、並列 AI コーディングエージェントセッション間のリアルタイム通信のための Go 製 CLI ツールです。

永続状態は SQLite WAL モードに保存し、低遅延なローカル配送には Unix Domain Socket を使います。対象ソケットが利用できない場合は、SQLite inbox polling をフォールバック経路として使用します。

## 主な機能

- 長時間動作するエージェントセッションの登録と解除
- エージェント間のダイレクトメッセージとブロードキャスト
- メッセージ、タスク、エージェント状態の SQLite 永続化
- シンプルな状態遷移に基づくタスク委譲
- STDIO 経由の MCP JSON-RPC サーバー
- SQLite を唯一の外部依存とする単一マシン向け構成

## 必要条件

- Go 1.22+
- CGO 有効
- `github.com/mattn/go-sqlite3` 用の SQLite3 ビルド環境

## インストール

### 最も簡単なインストール方法

各プラットフォームで最短の導入方法は次のとおりです。

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/malleus35/agentcom/main/scripts/install.sh | sh
```

```powershell
# Windows PowerShell
irm https://raw.githubusercontent.com/malleus35/agentcom/main/scripts/install.ps1 | iex
```

パッケージマネージャーを使うなら:

```bash
# macOS / Linux
brew tap malleus35/tap && brew install agentcom
```

```powershell
# Windows (bucket add なし)
scoop install https://raw.githubusercontent.com/malleus35/agentcom/main/packaging/scoop/agentcom.json
```

Scoop の URL 直接インストールは公式にサポートされていますが、登録済み bucket 由来ではないため通常の `scoop update agentcom` フローは使えません。

ローカルビルド:

```bash
make build
./bin/agentcom version
```

Go の bin パスにインストール:

```bash
make install
agentcom version
```

Go から直接インストール:

```bash
go install github.com/malleus35/agentcom/cmd/agentcom@latest
agentcom version
```

これは Go ユーザーには最も簡単ですが、対象環境に Go と CGO/SQLite のビルド環境が必要です。

## クロスプラットフォームのインストール方法

macOS、Linux、Windows で楽に配布するなら、通常は次のいずれかを使います。

### 1. GitHub Releases のバイナリ

一般ユーザーにはこれが最も扱いやすい方法です。現在のリリース設定では次のアーカイブを作成できます。

- macOS: `darwin/amd64`, `darwin/arm64`
- Linux: `linux/amd64`, `linux/arm64`
- Windows: `windows/amd64`, `windows/arm64`

GitHub Releases から自分の OS/アーキテクチャに合うアーカイブを取得し、展開したバイナリを `PATH` に置いてください。

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

### 2. Homebrew

このリポジトリの `.goreleaser.yml` には Homebrew 用設定が含まれています。リリース公開後に tap が用意されれば、macOS / Linux では Homebrew でインストールできます。

```bash
brew tap malleus35/tap
brew install agentcom
```

`brew tap` なしで `brew install agentcom` を成立させるには、formula が `homebrew-core` に採用される必要があります。

### 2a. Homebrew core に入れるには？

概要として必要なのは次の条件です。

- 各対応プラットフォーム向けの安定した公開リリースアーカイブとチェックサム
- `brew audit` と `brew test` を通る formula
- `homebrew-core` への提出と受理
- Homebrew の方針に沿った継続的な保守

それまでは tap ベースの導線が現実的です。

### 3. `go install`

Go 開発者やコントリビューター向けです。

```bash
go install github.com/malleus35/agentcom/cmd/agentcom@latest
```

### 4. 手動ローカルビルド

内部配布や完全に自分で制御したい場合に向いています。

```bash
make build
./bin/agentcom version
```

### どの方法を選ぶべきか

- macOS/Linux の一般ユーザー: インストールスクリプトまたは Homebrew tap
- Windows の一般ユーザー: PowerShell インストールスクリプトまたは Scoop URL 直接インストール
- どの OS でも手動で入れたい場合: GitHub Releases バイナリ
- Homebrew 利用者: Homebrew tap
- Go 開発者: `go install`
- ローカル開発や内部配布: `make build`

## agentcom の動作概要

`agentcom` は登録済みエージェント、メッセージ、タスク、状態確認用タイムスタンプをすべて SQLite に保存します。各登録済みプロセスは Unix Domain Socket サーバーも開くため、ダイレクトメッセージはまずソケット配送を試し、失敗した場合は SQLite inbox polling にフォールバックします。

通常の流れは次のとおりです。

1. ローカルの agentcom ホームディレクトリを初期化する
2. 1つ以上の長時間実行エージェントを登録する
3. CLI または MCP ツールでメッセージ、inbox、タスクを扱う
4. プロセスを正常終了して自動 deregister させる

## ローカル状態と設定

デフォルトでは `~/.agentcom` を使います。

- SQLite DB: `~/.agentcom/agentcom.db`
- ソケットディレクトリ: `~/.agentcom/sockets/`

ベースディレクトリは `AGENTCOM_HOME` で変更できます。

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom init
```

## グローバルフラグ

すべてのコマンドで次のグローバルフラグが使えます。

- `--json` - 可能な場合は機械可読な JSON 出力
- `-v`, `--verbose` - `log/slog` ベースのデバッグログ

```bash
agentcom --json list
agentcom --verbose health
```

## クイックスタート

```bash
agentcom init
agentcom init --agents-md
```

別ターミナルで 2 つのエージェントを起動します。

```bash
agentcom register --name alpha --type codex --cap plan,execute
agentcom register --name beta --type claude --cap review,test
```

メッセージ送信:

```bash
agentcom send --from alpha beta '{"text":"hello"}'
agentcom broadcast --from alpha --topic sync '{"status":"ready"}'
```

タスク操作:

```bash
agentcom task create "Implement MCP tests" --creator alpha --assign beta --priority high
agentcom task list --assignee beta
agentcom task delegate <task-id> --to beta
agentcom task update <task-id> --status in_progress --result "started"
```

状態確認:

```bash
agentcom inbox --agent beta --unread
agentcom status
agentcom health
```

## コマンド別の詳細

### `agentcom init`

ホームディレクトリを準備し、必要なサブディレクトリを作成し、SQLite を開いてマイグレーションを適用します。

```bash
agentcom init
agentcom init --agents-md
agentcom --json init
```

- 再実行しても安全です
- `--agents-md` は現在のディレクトリに `AGENTS.md` を生成します
- JSON 出力には `path`, `status`, `agents_md` が含まれる場合があります
- 現在の実装では事前にホームディレクトリを準備するため、新しいパスでも `status` が `already_initialized` になる場合があります

### `agentcom register`

現在のプロセスを live agent として登録し、heartbeat 更新、Unix Domain Socket サーバー、fallback poller を開始します。このコマンドは長時間動作する前提です。

```bash
agentcom register --name alpha --type codex
agentcom register --name alpha --type codex --cap plan,execute --workdir /path/to/project
agentcom --json register --name alpha --type codex
```

- `--name` - 必須、一意なエージェント名
- `--type` - 必須、自由文字列のタイプ
- `--cap` - 任意、カンマ区切り capability
- `--workdir` - 任意、作業ディレクトリ。省略時は現在ディレクトリ

### `agentcom deregister`

名前または ID でエージェントを解除します。

```bash
agentcom deregister alpha
agentcom deregister agt_xxx --force
agentcom --json deregister alpha --force
```

### `agentcom list`

登録済みエージェントを表示します。

```bash
agentcom list
agentcom list --alive
agentcom --json list
```

### `agentcom send`

登録済み sender から target agent へダイレクトメッセージを送ります。

```bash
agentcom send --from alpha beta '{"text":"hello"}'
agentcom send --from alpha beta 'plain text message'
agentcom send --from alpha --type request --topic review beta '{"file":"README.md"}'
agentcom send --from alpha --correlation-id corr-123 beta '{"step":1}'
```

- 第2引数が有効な JSON object/array ならそのまま保存されます
- そうでなければ `{"text":"..."}` 形式でラップされます

### `agentcom broadcast`

sender を除くすべての alive agent に同じメッセージを送ります。

```bash
agentcom broadcast --from alpha '{"status":"ready"}'
agentcom broadcast --from alpha --topic sync '{"phase":"wave-9"}'
```

### `agentcom inbox`

SQLite から特定 agent の inbox メッセージを取得します。

```bash
agentcom inbox --agent beta
agentcom inbox --agent beta --unread
agentcom inbox --agent agt_xxx --from agt_sender_id
agentcom --json inbox --agent beta --unread
```

### `agentcom task`

タスク管理は 4 つのサブコマンドに分かれています。

```bash
agentcom task create "Implement docs" --desc "Expand README" --creator alpha --assign beta --priority high
agentcom task list --status pending
agentcom task update <task-id> --status completed --result "done"
agentcom task delegate <task-id> --to beta
```

- `task create` は新規タスクを `pending` で保存します
- `--assign`, `--creator` は名前または ID を受け付けます
- `task delegate` は `assigned_to` を対象 agent に更新します

### `agentcom status`

エージェント数、メッセージ数、未読数、タスク総数、状態別タスク数を表示します。

```bash
agentcom status
agentcom --json status
```

### `agentcom health`

登録済みエージェントの health view を表示します。

```bash
agentcom health
agentcom --json health
```

空の環境では JSON 出力は `[]` です。

### `agentcom version`

ビルドメタデータを表示します。

```bash
agentcom version
agentcom --json version
```

### `agentcom mcp-server`

STDIO 上で MCP JSON-RPC サーバーを起動します。

```bash
agentcom mcp-server
agentcom mcp-server --register mcp-agent --type mcp
```

- `initialize` の後に `tools/list`, `tools/call` を呼ぶ必要があります
- 提供ツールは agent 一覧、メッセージ送信、broadcast、タスク作成/委譲、タスク一覧、状態取得です

## JSON 出力例

```bash
agentcom --json init
agentcom --json list
agentcom --json send --from alpha beta '{"text":"hello"}'
agentcom --json task list --status pending
```

```json
{
  "path": "/Users/name/.agentcom",
  "status": "initialized or already_initialized"
}
```

## エンドツーエンド例

ターミナル 1:

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom init
agentcom register --name alpha --type codex --cap plan,execute
```

ターミナル 2:

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom register --name beta --type claude --cap review,test
```

ターミナル 3:

```bash
export AGENTCOM_HOME=/tmp/agentcom-demo
agentcom send --from alpha beta '{"text":"please review README"}'
agentcom inbox --agent beta --unread
agentcom task create "Review README" --creator alpha --assign beta --priority high
agentcom task list --assignee beta
agentcom status
agentcom health
```

終了時は `Ctrl+C` で登録プロセスを止めます。

## MCP セットアップ

```bash
agentcom mcp-server
```

ハンドシェイク例:

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
```

利用可能なツール:

- `list_agents`
- `send_message`
- `broadcast`
- `create_task`
- `delegate_task`
- `list_tasks`
- `get_status`

## アーキテクチャ

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

## 開発

```bash
make build
make test
make lint
make vet
```

現在のテストスイートには DB CRUD、registry、transport integration、task manager、MCP roundtrip、end-to-end CLI シナリオが含まれます。
