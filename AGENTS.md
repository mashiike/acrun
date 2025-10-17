# acrun - AI Agents 向けプロジェクトコンテキスト

## プロジェクトアイデンティティ

**acrun** - AWS Bedrock AgentCore Runtime 専用の**軽量・特化型デプロイツール**

### 名前の由来

- **AgentCore Runtime** (AWS サービス名) + **run** (実行・デプロイアクション)

## プロジェクト哲学：Simple Made Easy

### Easy vs Simple - 本質的な設計思想

acrunは **Simple（シンプル）** を追求します。**Easy（簡単）** ではなく。

#### Easy（簡単）とは
- **複雑性が隠蔽されている** - でも編み込まれている
- 1コマンドで全部やってくれる - 何が起きているかは見えにくい
- すぐ始められる - でもカスタマイズや細かい制御は難しい
- 依存関係が多い - 特定の環境やフレームワークに依存

#### Simple（シンプル）とは
- **複雑性が分離されている** - 各要素が独立
- 単一責務の小さなコマンド - 何が起きているか明確で予測可能
- 理解が必要 - でも組み合わせや制御が自由
- 依存関係が少ない - 環境やフレームワークに非依存

```
Easy but Complex          Simple but Requires Understanding
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
AWS SAM                   lambroll (Lambda deployment)
Serverless Framework      ecspresso (ECS deployment)
starter-toolkit           acrun (AgentCore deployment)
```

**acrunの選択**: 長期的な保守性、予測可能性、組み合わせやすさのために **Simple** を選ぶ。

### 姉妹ツールからの影響

acrunは2つの成功したAWSデプロイツールの設計哲学を継承しています：

#### 1. **lambroll** (`github.com/fujiwara/lambroll`)

- AWS Lambda 向けの最小限のデプロイツール
- **デプロイ操作のみ**に特化し、フルインフラ管理は行わない
- Jsonnet テンプレート対応
- バージョン管理とロールバック機能
- 哲学：「インフラ管理が必要なら他のツール（AWS SAM、Serverless Framework 等）を推奨」

#### 2. **ecspresso** (`github.com/kayac/ecspresso`)

- Amazon ECS 向けデプロイツール
- **インフラ管理**と**アプリケーションデプロイ**の分離
- JSON/YAML/Jsonnet による Infrastructure as Code
- CI/CD 統合重視
- 拡張可能なプラグインアーキテクチャ

### AWS公式ツールキットとの差別化

**vs. `aws/bedrock-agentcore-starter-toolkit`**

| 観点             | bedrock-agentcore-starter-toolkit        | acrun                                            |
| ---------------- | ---------------------------------------- | ------------------------------------------------ |
| **スコープ**     | 包括的なスキャフォールディング＆デプロイ | デプロイ操作のみ                                 |
| **言語**         | Python 製 CLI                            | Go 製 CLI                                        |
| **対象ユーザー** | 初学者・クイックプロトタイピング         | 本番デプロイワークフロー                         |
| **インフラ**     | 自動作成（Dockerfile、ECR、IAM）         | インフラは既存または別途管理を想定               |
| **哲学**         | Easy（簡単） - すぐ始められる           | Simple（シンプル） - 理解しやすく組み合わせやすい |
| **設定管理**     | `.bedrock_agentcore.yaml`                | Jsonnet/JSON による柔軟なテンプレート            |
| **統合**         | フレームワーク特化（LangGraph、Strands） | フレームワーク非依存（任意の AgentCore Runtime） |
| **ワークフロー** | `agentcore launch` - 一括実行           | `init` / `diff` / `deploy` - ステップ分割        |
| **依存関係**     | Python エコシステム                      | 静的バイナリ（依存関係なし）                     |

#### 詳細な違い

**starter-toolkit を選ぶべき場合:**
- AgentCore Runtimeを初めて使う
- 素早くプロトタイプを作りたい
- インフラ管理は気にせず、とにかく動かしたい
- LangGraph/Strandsなど特定フレームワークを使う
- Pythonエコシステムに慣れている

**acrun を選ぶべき場合:**
- 本番環境への定期的なデプロイが必要
- インフラはTerraform/CloudFormationで管理している
- CI/CDパイプラインに組み込みたい
- 環境別設定（dev/staging/prod）をコードで管理したい
- デプロイ前に差分確認とレビューをしたい
- 既存のlambroll/ecspressoワークフローに馴染みがある
- 静的バイナリで依存関係を最小化したい

**acrunのポジショニング**: LambdaにおけるlambrollやECSにおけるecspressoと同様、acrunは以下を求めるチーム向け：

- **関心の分離**: インフラ（Terraform/CloudFormation） vs. デプロイ（acrun）
- **CI/CDパイプライン統合**: スクリプト可能、予測可能、最小限の依存関係
- **本番品質ワークフロー**: `init`、`diff`、`deploy`、`invoke`
- **テンプレートベース設定**: Jsonnetによる環境別デプロイ
- **Simpleの追求**: 長期的な保守性、予測可能性、組み合わせやすさ

## コアコマンド

```bash
acrun init     # 既存runtimeからacrun設定を初期化
acrun diff     # ローカルとリモートの差分表示
acrun deploy   # Agent runtimeをAWSへデプロイ
acrun invoke   # デプロイしたagentのテスト実行
```

## 技術スタック

### 言語とランタイム

- **Go 1.25.1** - 静的バイナリ、クロスプラットフォーム対応、高速実行
- **Node.js 24.1.0** - ツーリングサポート（asdf/mise 経由）

### AWS 統合

- AWS SDK Go v2
  - `bedrockagentcorecontrol` - コントロールプレーン操作（runtime 作成/更新）
  - `bedrockagentcore` - データプレーン操作（agent 実行）
  - `ecr` - コンテナレジストリ操作（URI 解決）

### CLI フレームワーク

- **Kong** (`alecthomas/kong`) - 宣言的なコマンド構造、自動ヘルプ生成

### 設定システム

- **Jsonnet** (`google/go-jsonnet`) - プライマリ設定フォーマット
- **JSON** - フォールバックフォーマット
- **デフォルトファイル名**: `agent_runtime.jsonnet` または `agent_runtime.json`

### ユーザー体験

- `Songmu/prompter` - インタラクティブ確認
- `aereal/jsondiff` - セマンティックな diff 表示（カラー付き）
- `fatih/color` - ターミナルカラー（可読性向上）
- `mashiike/slogutils` - 構造化ログ

## アーキテクチャパターン

### 3 層設計

```
┌─────────────────────────────────────┐
│  CLI層 (cli.go)                     │  Kongベースのコマンドルーティング
│  - Init, Deploy, Diff, Invoke       │  フラグパース、ヘルプ生成
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│  App層 (app.go, *.go)               │  ビジネスロジックオーケストレーション
│  - 設定ロード/バリデーション          │  コアデプロイロジック
│  - Diff計算                          │  ファイルI/O管理
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│  AWS層 (aws.go)                     │  AWS SDKラッパー
│  - AgentRuntime型定義                │  API呼び出し抽象化
│  - AWS APIのJSONマーシャリング       │  エラーハンドリング
└─────────────────────────────────────┘
```

### デザインパターン

#### 1. コンストラクタパターン

```go
New()              // デフォルトAWSクライアント（SDK設定使用）
NewWithClient()    // テスト用の依存性注入
```

#### 2. コマンドパターン（Kong）

```go
type CLI struct {
    GlobalOption
    Init   InitOption   `cmd:""`
    Deploy DeployOption `cmd:""`
    Diff   DiffOption   `cmd:""`
    Invoke InvokeOption `cmd:""`
}
```

#### 3. 設定パイプライン

```
Jsonnet/JSONファイル
  ↓ loadAgentRuntimeFile()
Go構造体 (AgentRuntime)
  ↓ validateAgentRuntime()
バリデーション済み設定
  ↓ marshalJSON(withLowerCamelCase)
AWS APIフォーマット（JSON）
```

#### 4. 構造化終了コード

```go
type ExitError struct {
    Code int
    Err  error
}

// 例：diffコマンドは差分がある場合に終了コード2を返す
var ErrDiff = &ExitError{Code: 2, Err: nil}
```

## コード構成

### ルートパッケージ構造

```
acrun/
├── cmd/acrun/main.go       # CLIエントリーポイント
├── cli.go                  # Kongコマンド定義
├── app.go                  # コアAppオーケストレーター
├── deploy.go               # Deployコマンド実装
├── init.go                 # Initコマンド実装
├── invoke.go               # Invokeコマンド実装
├── diff.go                 # Diffコマンド実装
├── aws.go                  # AWS SDK統合
├── jsonnet.go              # Jsonnet処理
├── json.go                 # JSONマーシャリングユーティリティ
├── errors.go               # カスタムエラー型
├── version.go              # バージョン情報
├── *_test.go               # テスト（テーブル駆動パターン）
├── mock_test.go            # 生成されたモック
├── testdata/               # テストフィクスチャ
└── _examples/agent/        # サンプルagentプロジェクト
```

### ファイル責務

| ファイル     | 目的                                                          |
| ------------ | ------------------------------------------------------------- |
| `cli.go`     | Kong 構造体定義、コマンドルーティング                         |
| `app.go`     | コアオーケストレーション、ファイル I/O、AWS クライアント管理  |
| `deploy.go`  | `(*App).Deploy()` - runtime/endpoint 作成/更新                |
| `init.go`    | `(*App).Init()` - AWS から現在の設定をダウンロード            |
| `invoke.go`  | `(*App).Invoke()` - agent 実行テスト                          |
| `diff.go`    | `(*App).Diff()` - 差分計算と表示                              |
| `aws.go`     | `AgentRuntime`型、AWS SDK ラッパー、アンマーシャル            |
| `jsonnet.go` | Jsonnet テンプレート評価                                      |
| `json.go`    | カスタム JSON マーシャリング（フック、lower camel case 変換） |

## 規約とスタイル

### Go イディオム

- 標準`gofmt`フォーマット
- 状態変更メソッドはポインタレシーバー: `(*App).Deploy()`
- 非エクスポートヘルパー: `coalesce()`, `fillEndpointName()`
- `testify/require`によるテーブル駆動テスト

### 命名規則

- **コマンド**: `InitOption`, `DeployOption`（フラグを持つ構造体）
- **コア型**: `AgentRuntime`, `App`, `CLI`
- **定数**: `AppName`, `DefaultEndpointName`, `DefaultAgentRuntimeFilenames`
- **エラー**: `ExitError`, `ErrDiff`, `ErrAgentRuntimeNotFound`

### JSON 処理

- **フックシステム**: マーシャリング時の柔軟な変換
- **lower camel case 変換**: AWS API 互換性（`agentRuntimeName`）
- **パスベースフィルタリング**: `ignoreLowerCamelPaths`による選択的変換

## 開発ワークフロー

### よく使うコマンド

```bash
# テスト
go test ./...                    # 全テスト実行
go test -cover ./...             # カバレッジ付き

# コード品質
gofmt -w .                       # コードフォーマット
golangci-lint run                # Lint（利用可能な場合）

# ビルド
go build ./cmd/acorun            # バイナリビルド
go install ./cmd/acorun          # $GOPATH/binへインストール

# タスクランナー
task                             # 利用可能タスク表示（Taskfile v3）
```

### タスク完了チェックリスト

機能実装やバグ修正時：

1. **フォーマット**: `gofmt -w .`
2. **テスト**: `go test ./...`
3. **ビルド**: `go build ./cmd/acorun`
4. **整理**: `go mod tidy`

新機能の場合：

- `*_test.go`にテーブル駆動テストを追加
- フィクスチャは`testdata/`へ配置
- エクスポート API の godoc コメント更新

## テスト戦略

### テーブル駆動テスト

```go
func TestUnmarshalAgentRuntime(t *testing.T) {
    cases := []struct {
        Name      string
        File      string
        Expected  *AgentRuntime
        ShouldErr bool
    }{
        // テストケース
    }
    for _, c := range cases {
        t.Run(c.Name, func(t *testing.T) {
            // テストロジック
        })
    }
}
```

### モック戦略

- `mock_test.go` - `go.uber.org/mock/mockgen`で生成
- `NewWithClient()`による依存性注入
- テスト可能性のための AWS SDK クライアントインターフェース

## CI/CD 統合

### 典型的なワークフロー

```yaml
# 例：GitHub Actions、GitLab CI等
steps:
  - name: acrun設定初期化
    run: acrun init --endpoint-name production

  - name: 差分確認
    run: acrun diff || exit 0  # 終了コード2は差分ありを意味

  - name: AWSへデプロイ
    run: acrun deploy --dry-run=false

  - name: デプロイ検証
    run: acrun invoke --input '{"query": "health check"}'
```

### 環境変数

- 標準 SDK 設定による AWS 認証情報（`AWS_PROFILE`, `AWS_REGION`等）
- Jsonnet 外部変数（今後の拡張領域）

## 拡張ポイント

### 現在の拡張性

- **Jsonnet テンプレート**: 複雑な設定ロジック
- **JSON マーシャリングフック**: カスタムフィールド変換
- **App クライアント注入**: テスト用のカスタム AWS クライアント実装

### 今後の拡張候補

1. **プラグインシステム**: ecspresso のテンプレート関数のように
2. **ロールバックサポート**: lambroll のバージョン管理のように
3. **Blue/Green デプロイ**: ecspresso から着想
4. **状態管理**: Terraform state ファイルとの統合
5. **バリデーションルール**: デプロイ前カスタムチェック

## AWS サービスとの関係

### 主要サービス

- **Bedrock AgentCore Runtime** - AI エージェントのホスティング
- **Bedrock AgentCore Control** - runtime ライフサイクル管理 API
- **ECR** - コンテナイメージレジストリ（URI 解決）
- **STS** - AWS 認証情報検証
- **IAM** - ロール/権限管理（別途管理を想定）

### 設定フィールド

`agent_runtime.jsonnet`の主要フィールド：

```jsonnet
{
  agentRuntimeName: "my-agent",
  roleArn: "arn:aws:iam::123456789012:role/MyAgentRole",
  agentRuntimeArtifact: {
    containerConfiguration: {
      containerUri: "123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v1"
    }
  },
  environmentVariables: { /* ... */ },
  networkConfiguration: { /* ... */ },
  protocolConfiguration: { /* ... */ }
}
```

## オープンソースコンテキスト

### ライセンス

通常は MIT または Apache 2.0（LICENSE ファイルで確認）

### mashike の姉妹プロジェクト

同作者の他ツールを確認し、一貫したパターンと統合機会を探る

### コミュニティ

- fujiwara の lambroll（AWS Lambda デプロイ）から着想
- kayac の ecspresso（ECS デプロイ）から着想
- Go AWS tooling エコシステムの一部

## 成功要因

1. **単一目的への集中**: デプロイのみ、スキャフォールディングは行わない
2. **予測可能な動作**: lambroll や ecspresso と同様
3. **テンプレートサポート**: Jsonnet による環境別設定
4. **CI/CD フレンドリー**: スクリプト可能、明確な終了コード、最小限の依存関係
5. **本番品質**: デプロイ前 diff、構造化ログ、エラーハンドリング

---

## AI Agents 向けクイックリファレンス

### 機能実装時：

- lambroll/ecspresso のパターンに従い一貫性を保つ
- 3 層アーキテクチャ（CLI → App → AWS）を維持
- testify によるテーブル駆動テスト使用
- jsonnet 互換性を維持
- CI/CD 向けの明確な終了コード提供

### デバッグ時：

- slog による構造化ログを確認
- `diff`コマンドでローカルとリモート比較
- AWS 権限と認証情報を確認
- `invoke`コマンドで素早く検証

### 拡張時：

- ecspresso のようなプラグインアーキテクチャを検討
- インフラ管理からの分離を維持
- CLI インターフェースは最小限かつ予測可能に
- エクスポート API には godoc ドキュメント記載

### コミット・PR：

- **コミットメッセージと PR は英語で記述**
- プロジェクトドキュメント（AGENTS.md 等）は日本語
- コードコメント（godoc）は英語推奨
