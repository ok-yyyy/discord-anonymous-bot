# 運用と開発

導入後に繰り返し行う作業。
初回の導入は [導入手順](./setup.md) を参照してください。

## 開発コマンド

```sh
# Go (lambda/ で実行)
go test ./...
go vet ./... && gofmt -l .
go run ./tools/build     # build/<関数名>/bootstrap を出力
go run ./cmd/registercmd # スラッシュコマンドを登録する

# CDK (rootで実行)
npm test                 # スタックの設定値を検証する
npm run build            # 型チェック
npx cdk diff
npx cdk deploy           # 事前に lambda/ でのビルドが必要
```

[go-task](https://taskfile.dev) を入れている場合は次で実行できます。
実行ディレクトリの移動を含めてまとめてあるだけで、中身は上のコマンドと同じです。

```sh
task test     # GoとCDKのテスト
task deploy   # ビルドしてからデプロイ
```

## 更新するとき

| 変えたもの             | 必要な操作                                      |
| ---------------------- | ----------------------------------------------- |
| Lambdaのコード         | `task deploy` (ビルドしてからデプロイ)          |
| CDKのスタック          | `npx cdk deploy` (事前に `npx cdk diff` で確認) |
| コマンド定義           | 上記に加えて `go run ./cmd/registercmd`         |
| Function URLが変わった | Interactions Endpoint URLを再設定               |

Lambdaのコードだけ直すときも `cdk deploy` を使います。
`aws lambda update-function-code` を手で叩くとCDKの状態とずれます。

## 削除するとき

```sh
npx cdk destroy
```

`/setup` で作成したWebhookとパネルは残るので、必要ならDiscord側で削除してください。

## トラブルシューティング

| 症状                                           | 確認すること                                                        |
| ---------------------------------------------- | ------------------------------------------------------------------- |
| Interactions Endpoint URLの保存に失敗する      | `DISCORD_PUBLIC_KEY` が正しいか。CloudWatch Logsに401が出ていないか |
| コマンドが候補に出ない                         | `registercmd` を実行したか。グローバル登録は反映に時間がかかる      |
| `/setup` が失敗する                            | BotにManage Webhooks権限があるか                                    |
| 投稿できない (`/setup` をやり直すよう言われる) | Webhookが削除されていないか。`/setup` を再実行する                  |
| 投稿が届かない                                 | SQSのDLQにメッセージが入っていないか。workerのCloudWatch Logsを見る |
| パネルが増え続ける                             | BotにRead Message History権限があるか                               |
| `cdk deploy` が認証エラーになる                | `aws login` の有効期限が切れていないか。再実行する                  |

監視やアラートは設けていません。
問題が起きたらCloudWatch Logsを直接確認してください。
