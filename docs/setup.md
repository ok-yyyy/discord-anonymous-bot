# 導入手順

Discordアプリの作成からデプロイまで。
導入後の更新やトラブル対応は [運用と開発](./operations.md) を参照してください。

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
以降はリフレッシュトークンが有効な間、CLIが自動で更新します。

`cdk deploy` はデプロイ先を `CDK_DEFAULT_ACCOUNT` / `CDK_DEFAULT_REGION` から決めます。
リージョンが解決できないと失敗するので、設定していない場合は環境変数で渡してください。

```sh
AWS_REGION=ap-northeast-1 npx cdk deploy
```

## 1. Discordアプリを作る

[Developer Portal](https://discord.com/developers/applications) でアプリを作成し、次を行います。

1. **Bot** タブでBotを作成し、**Token** を控える
2. **General Information** から **Application ID** と **Public Key** を控える

## 2. Bot をサーバーに招待する

`<APPLICATION_ID>` を置き換えて開きます。

```
https://discord.com/oauth2/authorize?client_id=<APPLICATION_ID>&scope=bot%20applications.commands&permissions=536939520
```

権限の内訳は **Manage Webhooks / Send Messages / View Channel / Read Message History** です。
Manage Webhooksが無いと `/setup` が失敗します。
Read Message Historyが無いと古いパネルを掃除できず、投稿のたびにパネルが増え続けます
権限を変えたら招待し直してください。

## 3. `.env` を用意する

`.env.example` をコピーして値を埋めます。
**コミットしないでください** (`.gitignore` 済み) 。

| 変数                     | 取得元                              |
| ------------------------ | ----------------------------------- |
| `DISCORD_BOT_TOKEN`      | BotタブのToken                      |
| `DISCORD_PUBLIC_KEY`     | General InformationのPublic Key     |
| `DISCORD_APPLICATION_ID` | General InformationのApplication ID |
| `ANONYMOUS_SALT`         | 任意のランダムな文字列              |

`ANONYMOUS_SALT` は匿名IDの導出に使います。変更すると過去の匿名IDと紐づかなくなります。

4つとも必須で、欠けていると `cdk deploy` がsynthの時点で止まります。

## 4. デプロイする

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

## 5. Interactions Endpoint URL を登録する

Developer Portalの **General Information** -> **Interactions Endpoint URL** に前手順のURLを設定して保存します。
保存時にDiscordがPINGを投げて検証するため、**保存が成功すれば疎通できています。**

## 6. スラッシュコマンドを登録する

```sh
cd lambda && go run ./cmd/registercmd
```

グローバル登録なので即時反映されないことがあります。
コマンドが候補に出ない場合は、少し待つかDiscordクライアントを再起動してください。

## 7. 動作を確認する

投稿したいチャンネルで `/setup` を実行し、設置されたパネルから投稿できれば完了です。
使い方は [README](../README.md#使い方) を参照してください。
