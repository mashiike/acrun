---
paths:
  - "deploy.go"
  - "ecr_images.go"
  - "rollback.go"
---

# AgentCore エンドポイントのバージョン取得

## `ListAgentRuntimeEndpoints` は `LiveVersion` / `TargetVersion` を埋めない

SDK の `types.AgentRuntimeEndpoint` にはこの2フィールドが存在し、AWS 公式 API リファレンスの Response Syntax にも `liveVersion` / `targetVersion` が載っている。しかし `types.AgentRuntimeEndpoint` の8フィールド中、**required マークが付いていない唯一の2つがこの2フィールド**であり、List 系 API は実際には値を返さない（`ecr_images.go` が同じ理由で `GetAgentRuntimeEndpoint` を個別に呼んでいる）。

エンドポイントが参照中のバージョンが必要なときは、`ListAgentRuntimeEndpoints` で名前を列挙し、各名前について `GetAgentRuntimeEndpoint` を呼ぶ。List のレスポンスから直接読んではいけない。

この罠が `deploy --keep-versions` で踏まれると、削除してはいけないバージョン（他エンドポイントが現に配信しているバージョン、ロールバック先）が保護対象から漏れて不可逆に消える。`deploy_test.go` の `ListAgentRuntimeEndpoints` モックが `Name` のみを返しているのは、この実 API 挙動を再現して保護ロジックの回帰を検出するためであり、値を足すとテストが実装より弱くなる。

`GetAgentRuntimeEndpoint` が1つでも失敗したら削除を一切行わない。保護集合が不完全なまま削除すると、エンドポイントが現に配信しているバージョンを消す。`ecr_images.go` は同じ失敗を警告して継続するが、あちらは取りこぼしが表示上の問題で済むので事情が違う。

## `deploy --keep-versions` の prune は best-effort

`deleteOldVersions` が error を返さないのは意図的。prune は deploy 本体の後処理（dbt の post_hook / ワークフローの post_process に相当するもの、というのがこのリポジトリのオーナーの解釈）であり、エンドポイント更新が成功した後に掃除だけ失敗しても deploy を失敗扱いにはしない。権限不足・スロットリング・バージョン一覧の取得失敗はすべて warn ログにして継続する。

lambroll の `deleteVersions` は1件目の削除失敗で即 return するが、acrun は全候補を試行する。Throttling が1件出ただけで残りのバージョンが溜まり続けるのを避けるため、この点だけ意図的に逸脱している。

## `deploy --keep-versions` の dry-run は保持枠を1つ予約する

`deleteOldVersions` の `keepFromListed` が dry-run のときだけ `KeepVersions - 1` になるのは、dry-run では `UpdateAgentRuntime` を呼ばないため、これからデプロイされる新バージョンが `ListAgentRuntimeVersions` の結果に含まれないから。減算しないと dry-run は実行時より1つ少ないバージョンしか削除予定に挙げず、「消えない」と表示されたバージョンが実行時に消える。

`deploy_test.go` の `deployedVersionPages()` と `notYetDeployedVersionPages()` はこの1バージョン差を表現していて、両者の削除対象が一致することを検証している。
