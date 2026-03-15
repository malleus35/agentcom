# agentcom

`agentcom` は、並列 AI コーディングエージェントセッション間のリアルタイム通信のための Go 製 CLI ツールです。

永続状態は SQLite WAL モードに保存し、低遅延なローカル配送には Unix Domain Socket を使います。対象ソケットが利用できない場合は、SQLite inbox polling をフォールバック経路として使用します。

## 主な機能

- 長時間動作するエージェントセッションの登録と解除
- エージェント間のダイレクトメッセージとブロードキャスト
- メッセージ、タスク、エージェント状態の SQLite 永続化
- シンプルな状態遷移に基づくタスク委譲
- 対応するコーディングエージェント向けに project / user スコープの `SKILL.md` を生成
- 共通指示と 6 つの役割スキルを含む内蔵マルチエージェントテンプレートを生成
- `agentcom agents template` で内蔵テンプレートを確認でき、TTY では interactive search も可能
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
agentcom init --batch
agentcom init --batch --agents-md claude,codex
```

共通指示と 6 つの役割スキルを含む内蔵テンプレートを生成:

```bash
agentcom init --template company
agentcom init --template oh-my-opencode
agentcom init --template custom
```

生成前に内蔵テンプレートを確認:

```bash
agentcom agents template
agentcom agents template company
agentcom --json agents template oh-my-opencode
```

インタラクティブ端末でテンプレート名なしに `agentcom agents template` を実行すると、検索語を入力して番号でテンプレートを選択できます。

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

対応エージェント CLI すべて向けに project スキルを生成:

```bash
agentcom skill create review-pr --agent all --scope project --description "一貫した PR レビュー"
```

## コマンド別の詳細

### `agentcom init`

ホームディレクトリを準備し、必要なサブディレクトリを作成し、SQLite を開いてマイグレーションを適用します。

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

- 再実行しても安全です
- インタラクティブ端末では `agentcom init` が既定でオンボーディング wizard を起動します
- `--batch` は非対話フローを強制し、`--json` 時にも自動適用されます
- `--agents-md` は `all` または `claude,codex,cursor` のようなカンマ区切りの agent 一覧を受け取ります。`agentcom init --batch --agents-md` のように値なしで使うと従来どおり `AGENTS.md` を生成します
- `--template` は `.agentcom/templates/<template>/COMMON.md`、`.agentcom/templates/<template>/template.json`、各対応 agent 向けの shared `agentcom/SKILL.md`、および `agentcom/<template>-frontend` 形式の 6 つの namespaced role skill を生成します
- 組み込みテンプレートは `company` と `oh-my-opencode` で、`custom` はインタラクティブなテンプレート作成 wizard を起動します
- `agentcom agents template --list` は built-in/custom テンプレートをまとめて表示し、`agentcom agents template --delete <name>` は確認後に custom テンプレートを削除します
- JSON 出力には必要に応じて `path`, `status`, `instruction_files`, `agents_md`, `memory_files`, `template`, `custom_template_path`, `generated_files` が含まれます
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

### `agentcom skill`

対応するコーディングエージェント CLI 向けのスキルファイルを生成します。

使用例:

```bash
agentcom skill create review-pr --agent claude --scope project --description "一貫した PR レビュー"
agentcom skill create pairing-notes --agent cursor --scope project
agentcom skill create docs-sync --agent github-copilot --scope user
agentcom skill create release-check --agent all --scope user
agentcom --json skill create docs-sync --agent gemini-cli --scope project
```

フラグ:

- `--agent` - 対象エージェント識別子または `all`（既定値 `all`）
- `--scope` - 出力スコープ: `project|user`（既定値 `project`）
- `--description` - 任意のスキル説明。既定値は `Skill generated by agentcom`

生成先パス:

- `claude` - project: `.claude/skills/<name>/SKILL.md`, user: `~/.claude/skills/<name>/SKILL.md`
- `codex` - project: `.agents/skills/<name>/SKILL.md`, user: `~/.agents/skills/<name>/SKILL.md`
- `gemini` - project: `.gemini/skills/<name>/SKILL.md`, user: `~/.gemini/skills/<name>/SKILL.md`
- `opencode` - project: `.opencode/skills/<name>/SKILL.md`, user: `~/.config/opencode/skills/<name>/SKILL.md`
- `cursor` - project: `.cursor/skills/<name>.mdc`, user: `~/.cursor/skills/<name>.mdc`
- `github-copilot` - project: `.github/skills/<name>.md`, user: `~/.github/skills/<name>.md`
- `universal` - project: `skills/<name>/SKILL.md`, user: `~/skills/<name>/SKILL.md`

追加の対応識別子には `claude-code`、`gemini-cli`、`droid` などの alias と、`cline`、`continue`、`roo-code`、`kilo-code`、`windsurf`、`devin`、`replit-agent`、`bolt`、`lovable`、`tabnine`、`tabby`、`amazon-q`、`sourcegraph-cody`、`augment-code`、`factory`、`goose`、`openhands`、`qwen` などがあります。

補足:

- スキル名には小文字、数字、単一ハイフンのみ使えます。
- 既存のスキルファイルは上書きしません。
- `--agent all` は対応する全エージェント向けに書き込みを試み、最初の失敗で停止します。
- 出力形式は agent ごとに異なります。大半は `SKILL.md`、`cursor` は `.mdc`、`github-copilot`、`windsurf`、`devin`、`replit-agent`、`bolt`、`lovable`、`playcode-agent` は `.md` を使います。

### `agentcom agents template`

内蔵テンプレート一覧を表示するか、特定テンプレートの定義を詳しく表示します。

使用例:

```bash
agentcom agents template
agentcom agents template company
agentcom --json agents template oh-my-opencode
```

インタラクティブ動作:

- interactive terminal でテンプレート名なしに実行すると、検索語入力と番号選択のプロンプトが表示されます。
- non-interactive または `--json` モードでは、既存の一覧/詳細出力を維持します。

内蔵テンプレート:

- `company` - Paperclip の役割構造に着想を得た会社型マルチエージェントワークフロー
- `oh-my-opencode` - Prometheus、Momus、Oracle、Sisyphus-Junior のパターンに着想を得た計画重視ワークフロー

生成される scaffold:

- 共通指示: `.agentcom/templates/<template>/COMMON.md`
- テンプレートメタデータ: `.agentcom/templates/<template>/template.json`
- project レベルの shared template skill: `.claude/skills/agentcom/SKILL.md`、`.agents/skills/agentcom/SKILL.md`、`.gemini/skills/agentcom/SKILL.md`、`.opencode/skills/agentcom/SKILL.md`
- role skill は同じ namespace 配下、たとえば `.agents/skills/agentcom/company-frontend/SKILL.md` 形式で生成されます。
- 各 role skill はまず shared `../SKILL.md`、次に template `COMMON.md` を参照し、`frontend`、`backend`、`plan`、`review`、`architect`、`design` 間の communication map を含みます。

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
