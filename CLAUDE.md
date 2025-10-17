# Claude Code Context

このプロジェクトの包括的なコンテキスト情報は **AGENTS.md** に記載されています。

@AGENTS.md

## 追加のAI協働指示

このプロジェクトはOSSとして開発されています。以下の点に注意してください:

### コミット前チェックリスト
```bash
gofmt -w .        # コードフォーマット
go test ./...     # テスト実行
go mod tidy       # 依存関係整理
```

### 開発哲学
- **軽量特化型**: lambroll/ecspressoと同じく、デプロイ操作に特化
- **Infrastructure as Code**: Jsonnetによる柔軟な設定管理
- **CI/CD統合**: 予測可能な動作、明確な終了コード、最小限の依存関係
