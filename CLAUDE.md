# CLAUDE.md

Discord の匿名メッセージ Bot。パネルのボタンから投稿すると、毎日変わる匿名の表示名とアイコンで
チャンネルにメッセージが送信される。AWS CDK (TypeScript) と Go の Lambda で動かす。

ここには、コードを読んでも分からず、知らないと間違えることだけを書く。
機能の詳細はコードとテストを、手順は[導入手順](./docs/setup.md)と[運用と開発](./docs/operations.md)を見る。

## アーキテクチャ

```
Discord ──HTTP POST──> Lambda Function URL
                             │
                       [interaction Lambda]  署名検証と振り分け
                             │
              ┌──────────────┴───────────────┐
           （同期）                       （非同期）
      その場で応答を返す             SQS (FIFO) に積んで ack ──> DLQ
                                             │
                                       [worker Lambda]
                                         Discord REST を呼ぶ
```

- **Discord は 3 秒以内に応答がないと interaction を失敗扱いにする。** これが構成上の制約の
  ほぼすべての理由になっている。Discord REST への往復は 3 秒に収まる保証がないため SQS に逃がすが、
  外部 I/O のないコマンドまで経由させると体感が悪いので、どちらで処理するかはコマンド単位で選ぶ。
- **投稿経路は Webhook に限られる。** Bot 本体として投稿すると匿名にならず、`username` /
  `avatar_url` の上書きは Execute Webhook でしかできない。
- 受信は Function URL で、API Gateway は使わない。

## やらないこと

個人開発・小規模運用のため**意図的に実装しない**。気を利かせて足さないこと。

- **永続ストア**（DynamoDB / RDS 等）。パネルやメッセージの ID も保存せず、Webhook は毎回
  Discord から引き直す。
- **投稿者と投稿内容の記録。** 誰が何を投稿したかはログにも残さない。管理者が投稿者を逆算する
  機能も入れない。荒れたときはメッセージの削除とパネルの撤去で対処する。
- **濫用対策**（レート制限・連投抑止・ブロック）。永続ストアが無いため実装できない。
- **監視・アラート**（DLQ 通知、CloudWatch Alarm）。問題が起きたらログを直接見る。
- **ローカル実行環境**（署名付きリクエストのテストハーネス、SAM Local 等）。
  検証はユニットテストで行い、結合は実環境で確認する。
- Gateway（WebSocket）接続、CI/CD、複数環境（dev/prod）の分離。

## 実装の約束

### コマンドの処理

コマンドの追加は `internal/handler` の `registry` に 1 行足す。両方の Lambda が同じ表を参照し、
`registercmd` もここから定義を登録するので、二重管理しない。

- **外部 I/O があるなら `Work` に書く。** Discord REST を呼ぶ、処理時間が読めない、失敗を DLQ に
  残したい、のいずれかに当てはまれば `Work`。固定文言や入力検証だけなら `Respond`。
- **`Respond` からネットワーク I/O をしない。** タイムアウトすると interaction 自体が失敗する。
  `context.Context` を渡していないのは、これを書けなくするため。
- **入力の拒否は `Respond` から `error` で返す。** 拒否の文面を応答として返すと `Work` が実行される。
- **`Work` を持つコマンドの `Respond` は `Ack` を返す。** 別の応答にすると worker の返信先がずれ、
  Discord に拒否される応答なら、失敗と見えるのに `Work` だけが動く。
- **enqueue が先、ack は後。** ack は HTTP レスポンスのボディで返す。先に callback API で ack すると、
  3 秒の予算に discord.com への往復が入り、enqueue 失敗時のエラーも別途送る必要が出る。
- **SQS への送信は約 1 秒で打ち切り、SDK の既定リトライに任せない。** 遅れると、キューには
  残っているのに実行者には失敗が見える状態になる。
- SQS には interaction の生 JSON と受信時刻を積み、worker 側で再パースする。interaction Lambda に
  業務ロジックを持ち込まない。

### 匿名性

- 匿名 ID は保存せず、投稿のたびに利用者 ID・UTC の日付・salt から導出する。
- 語彙を変えると添字の計算（`% len`）が変わるので、末尾への追加でもその日の全員の名前がずれる。
  ネガティブな語や人を指す語は入れない。
- 表示名は衝突しうる。同じ名前でも別人の可能性があることは `/help` に明記する。
- salt を変えると過去の匿名 ID と紐づかなくなる。

### 投稿

- **Discord への投稿は 1 回だけ試行する。** 再配信で投稿が重複するので、`maxReceiveCount` は 1 に
  して、失敗は DLQ に送る。
- **投稿が済んだ後の失敗（パネルの張り替え、通知）はログに残すだけにする。** エラーを返すと
  DLQ 行きになるうえ、再実行されれば投稿が重複する。
- **失敗は実行者に必ず伝える。** deferred のまま放置すると「考え中…」が消えない。
  ただし token 切れなどで届かない場合は除く。
- **REST クライアントは必ず `NewRest` で作る。** disgo は `AllowedMentions` が nil の送信で全メンションを
  許可するので、既定を反転させてある。利用者の入力を載せる匿名投稿では、既定に頼らず
  `NoMentions()` を明示する。
- **実行者への応答は ephemeral にする。** 投稿内容や結果が公開チャンネルに漏れないようにする。
- **Webhook が消えていても再作成しない。** 権限の無いチャンネルに投稿しうるので、`/setup` の
  やり直しを案内する。
- **パネルの掃除から Webhook 投稿を必ず除外する。** 匿名メッセージも自アプリの Webhook で
  投稿しているので、除外しないと利用者の投稿を消す。

## Discord の落とし穴

- **署名検証:** Function URL は認証なしで公開するので、必ず Lambda 側で検証する。失敗時は
  **HTTP 401** を返す（他のコードではエンドポイント検証に通らない）。本文は受け取ったバイト列のまま
  使い、`isBase64Encoded` が true の場合も考慮する。
- **ack の方式は Interaction の種類で決まる。** スラッシュコマンドは type 5（「考え中…」）だけ使え、
  `@original` はその「考え中…」を指す。コンポーネントとモーダル送信は type 6（無言）を使い、
  `@original` は元のパネルを指すので、編集せず followup で伝える。
- **モーダルを開く応答と AUTOCOMPLETE は defer できない。** どちらも `Work` に移せない。
- **`Read Message History` 権限が無いと、`GetMessages` はエラーではなく空を返す。** 古いパネルを掃除
  できず増え続ける。権限を変えたら招待 URL も更新して入れ直す。
- **コマンド定義の `Contexts` と `IntegrationTypes` は必ず明示する。** 省略すると意図しない場所で
  使える。`dm_permission` は非推奨なので使わない。
- **`ApplicationIntegrationTypeUserInstall` は Developer Portal の設定が必要。** Installation Contexts で
  User Install を有効にしていないと、登録時に拒否される。
- **登録は global のみで、`SetGlobalCommands`（bulk overwrite）を使う。** 個別の `Create` だと削除した
  コマンドが残る。反映は即時ではないので、変更が見えなくてもまず待つか、クライアントを再起動する。

## 技術上の決まり

### Go

- **外部依存は次に限る:** `disgoorg/disgo`（と、その API が使う `disgoorg/omit` / `disgoorg/snowflake`）、
  `aws/aws-lambda-go`、`aws/aws-sdk-go-v2/*`、`caarlos0/env/v11`、`joho/godotenv`。
  追加したくなったら、まず標準ライブラリで書けないか検討する。
- **disgo で使うのは `discord` と `rest` だけ。** `bot` / `gateway` / `cache` / `httpserver` は使わない。
  `httpserver` は自前の HTTP サーバを立てる作りで、署名検証も `crypto/ed25519` の呼び出しだけなので、
  標準ライブラリを直接使う。v0.x で破壊的変更がありうるので、更新時は型の変更を確認する。
- **Discord との通信を自前で組み立てない。** `internal/discord` は disgo で足りない部分だけを持ち、
  Interaction の型や REST 呼び出しを包み直さない。
- **環境変数は `internal/config` で読む。** 必須値が欠けていたら起動させない。タグは `required` では
  なく `notEmpty` を使う（`required` は空文字を通す）。
- **Go は `lambda/` の独立モジュールにする。** root に `go.mod` を置くと、`node_modules` 内の CDK の
  Go テンプレートを読もうとして `go vet ./...` / `go test ./...` が失敗する。
- エラーは `fmt.Errorf("...: %w", err)` でラップして返す。ログ出力はハンドラ最上位だけにする
  （投稿後の失敗のように、エラーを返さない処理を除く）。
- `internal/` はテスト必須。Discord REST は `rest.WithHTTPClient` に stub を渡してテストする。

### CDK とデプロイ

- **スタックは 1 つ、環境も 1 つ。** 環境名のパラメータ化はしない。
- **Lambda は `lambda.Function` と `Code.fromAsset` で作る。** `GoFunction` / Docker バンドルは使わず、
  ビルドは `lambda/tools/build` に一本化する。
- **秘匿値は `.env` に置き、コミットしない。** CDK のコードやテンプレートに直接書かない。
- **デプロイはローカルからの `cdk deploy` のみ。** コードだけ直すときも同じで、
  `aws lambda update-function-code` は CDK の状態とずれるので使わない。認証は `aws login` で行う。
- **Makefile は置かない。** Windows に `make` が無いので、ビルドも Go で書く。
- **`Taskfile.yml` は薄い入口で、必須にしない。** task が無くても個々のコマンドが動く状態を保つ。
  CDK Toolkit も devDependency にして、グローバルインストールを前提にしない。
