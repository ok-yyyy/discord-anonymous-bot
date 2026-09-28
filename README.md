# discord-anonymous-bot

Discordのチャンネルに匿名でメッセージを投稿できるBot。
パネルのボタンから投稿すると、毎日変わる匿名の表示名とアイコンで送信されます。

- 投稿されたメッセージから投稿者は分かりません（運営者は監査用の記録を保持します）
- 表示名は日替わりで、組み合わせに限りがあるため同じ名前でも別の人のことがあります

設計と判断理由は [CLAUDE.md](./CLAUDE.md) にあります。
[利用規約](./docs/terms-of-service.md) / [プライバシーポリシー](./docs/privacy-policy.md)

## 構成

```
Discord --> Lambda Function URL --> [interaction] --> SQS --> [worker] --> Webhook で投稿
                                     署名検証と振り分け        匿名 ID の導出
```

インフラはAWS CDK (TypeScript)、LambdaはGo (`provided.al2023`/arm64) です。

## 必要なもの

|         | バージョン                                   |
| ------- | -------------------------------------------- |
| Go      | `lambda/go.mod` の指定以上 (開発環境は 1.26) |
| Node.js | 22 (LTS)                                     |
| AWS CLI | v2 (`aws login` を使うため 2.36 以上)        |

CDK ToolkitはdevDependencyなのでグローバルインストールは不要です。

### AWS の認証

profileは設定せず、`aws login` でマネジメントコンソールのセッションから一時的な認証情報を取得します。

```sh
aws login
```

ブラウザが開き、ログインして使うアカウントとロールを選びます。
以降はリフレッシュトークンが有効な間、CLI が自動で更新します。

`cdk deploy` はデプロイ先を `CDK_DEFAULT_ACCOUNT` / `CDK_DEFAULT_REGION` から決めます。
リージョンが解決できないと失敗するので、設定していない場合は環境変数で渡してください。

```sh
AWS_REGION=ap-northeast-1 npx cdk deploy
```

## セットアップ

### 1. Discordアプリを作る

[Developer Portal](https://discord.com/developers/applications) でアプリを作成し、次を行います。

1. **Bot** タブでBotを作成し、**Token** を控える
2. **General Information** から **Application ID** と **Public Key** を控える

### 2. Bot をサーバーに招待する

`<APPLICATION_ID>` を置き換えて開きます。

```
https://discord.com/oauth2/authorize?client_id=<APPLICATION_ID>&scope=bot%20applications.commands&permissions=536939520
```

権限の内訳は **Manage Webhooks / Send Messages / View Channel / Read Message History** です。
Manage Webhooksが無いと `/setup` が失敗します。
Read Message Historyが無いと古いパネルを掃除できず、投稿のたびにパネルが増え続けます
(Discordはエラーではなく空のリストを返すため、気づきにくいです) 。
権限を変えたら招待し直してください。

### 3. `.env` を用意する

リポジトリ直下に作成します。
**コミットしないでください** (`.gitignore` 済み) 。

```sh
DISCORD_BOT_TOKEN=...       # BotタブのToken
DISCORD_PUBLIC_KEY=...      # General InformationのPublic Key
DISCORD_APPLICATION_ID=...  # General InformationのApplication ID
ANONYMOUS_SALT=...          # 任意のランダムな文字列
```

`ANONYMOUS_SALT` は匿名IDの導出に使います。変更すると過去の匿名IDと紐づかなくなります。

4つとも必須で、欠けていると `cdk deploy` がsynthの時点で止まります。

### 4. デプロイする

```sh
aws login                                    # 認証情報を取得する
npm ci
cd lambda && go run ./tools/build && cd ..   # Lambdaバイナリをビルド
npx cdk bootstrap                            # このアカウント・リージョンで初回のみ
npx cdk deploy
```

[go-task](https://taskfile.dev) を入れている場合は `task deploy` でビルドとデプロイをまとめて実行できます。
(`go install github.com/go-task/task/v3/cmd/task@latest`)

出力される `InteractionsEndpointUrl` を控えます。

### 5. Interactions Endpoint URL を登録する

Developer Portalの **General Information** -> **Interactions Endpoint URL** に前手順のURLを設定して保存します。
保存時に Discord が PING を投げて検証するため、**保存が成功すれば疎通できています。**

### 6. スラッシュコマンドを登録する

```sh
cd lambda && go run ./cmd/registercmd
```

グローバル登録なので即時反映されないことがあります。
コマンドが候補に出ない場合は、少し待つかDiscordクライアントを再起動してください。

## 使い方

1. 投稿したいチャンネルで `/setup` を実行 (既定ではサーバー管理者のみ)
2. 設置されたパネルの「メッセージを送信」ボタンを押す
3. 入力欄に本文を書いて送信すると、匿名で投稿される

パネルは投稿のたびに作り直され、チャンネルの一番下に移動します。

| コマンド | 内容                                 |
| -------- | ------------------------------------ |
| `/setup` | このチャンネルに投稿パネルを設置する |
| `/help`  | 使い方を表示する                     |
| `/ping`  | 疎通確認                             |

## 開発

```sh
# Go (lambda/ で実行)
go test ./...
go vet ./... && gofmt -l .
go run ./tools/build     # build/<関数名>/bootstrap を出力

# CDK (rootで実行)
npm test                 # スタックの設定値を検証する
npm run build            # 型チェック
npx cdk diff
```

go-taskを使う場合は `task test` でGoとCDKのテストをまとめて実行できます。
実行ディレクトリの移動を含めてまとめてあるだけで、中身は上のコマンドと同じです。

## 更新するとき

| 変えたもの             | 必要な操作                                      |
| ---------------------- | ----------------------------------------------- |
| Lambdaのコード         | `go run ./tools/build` -> `npx cdk deploy`      |
| CDKのスタック          | `npx cdk deploy` (事前に `npx cdk diff` で確認) |
| コマンド定義           | 上記に加えて `go run ./cmd/registercmd`         |
| Function URLが変わった | Interactions Endpoint URL を再設定              |

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
| `cdk deploy` が認証エラーになる                | `aws login` の有効期限が切れていないか。再実行する                  |

監視やアラートは設けていません。
問題が起きたらCloudWatch Logsを直接確認してください。
