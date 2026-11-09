---
paths:
  - "cmd/codegen/**"
  - "aws.gen.go"
  - "aws.gen_test.go"
---

# union 変換コードの生成（cmd/codegen）

## SDK の union メンバーはポインタレシーバーで interface を満たす

`types.AgentRuntimeArtifactMemberContainerConfiguration` などの union メンバーは、ポインタ型だけが union interface を実装する。collector はメンバーを `types.NewPointer(t)` で判定し、テンプレートは `case *types.X:` と `&types.X{...}` を出力する。SDK がこの規約を変えると、メンバーが 1 つも見つからず codegen はエラーで止まる。

## 未対応の形は生成を止める

`CreateAgentRuntimeInput` から到達する union は、次の形のときだけ変換コードを生成できる。

- union そのもの、union のスライス
- union を含む構造体へのポインタ、そのスライス
- union メンバーの `Value` が union、または union を含む構造体

これ以外の位置（map の値、スライスのスライス、非ポインタの構造体フィールド、再帰型）に union が現れたら、collector は error を返し codegen は exit 1 になる。auto-codegen ワークフローが赤くなるのは意図した挙動で、黙って `{"Value": ...}` 形式の JSON を出して往復を壊すのを防いでいる。

## map のキーは大文字小文字を変えない

`marshalJSON` / `unmarshalJSON` はキーの先頭文字を lower / upper camel に変換する。map のキー（`tags`、`environmentVariables`、`advertisedScopeMapping` など）はデータなので、`mapPaths` が SDK の型から導出したパスを ignore に渡している。新しい変換経路を足すときは、その経路の起点の型から `mapPaths` を取り直す。

## テストのサンプル値は先に登録した union から取る

生成テストは、union 型のフィールドに `collector.samples` の値（その union の先頭メンバーのテスト値）を使う。`union()` は `memberValue()` で内側の union を登録してから `extractMemberInfo()` を呼ぶ。この順を逆にすると、内側の union のサンプルが nil になり、値が union のメンバー（`selfManagedLatticeResource` など）の生成テストが失敗する。
