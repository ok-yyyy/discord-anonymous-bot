# CLAUDE.md

Discord の匿名メッセージ Bot。パネルのボタンから投稿すると、投稿者を秘匿したまま
（毎日変わる匿名の表示名 / アイコンで）チャンネルにメッセージが送信される。

- インフラ: AWS CDK (TypeScript) / アプリ: Go (Lambda `provided.al2023` arm64)
- Discord クライアント: `disgoorg/disgo`
- 受信: Lambda Function URL（API Gateway は使わない）
- 非同期処理: SQS を挟んだ 2 段 Lambda
- **永続ストアは持たない**（DynamoDB / RDS 等は導入しない）

## アーキテクチャ

```
Discord ──HTTP POST──> Lambda Function URL
                             │
                       [interaction Lambda]  Go
                         - Ed25519 署名検証 / PING に即 PONG
                         - Registry を引いて 2 通りに分岐
                             │
              ┌──────────────┴───────────────┐
        （Sync）                        （Async）
   その場で type 4 を返す           SQS へ enqueue して type 5
                                           │
                                    SQS (FIFO) ──> DLQ
                                           │
                                    [worker Lambda]  Go
                                      - 匿名 ID を導出
                                      - Webhook の username / avatar_url を
                                        上書きして投稿
                                      - followup で結果を返す
```

**Discord は 3 秒以内に応答がないと interaction を失敗扱いにする。** これが構成上の制約の
ほぼすべての理由になっている。Discord REST への往復は 3 秒に収まる保証がないため SQS に逃がすが、
外部 I/O のないコマンドまで経由させると「考え中…」が挟まって体感が悪いので、
**どちらで処理するかはコマンド単位で選ぶ**。

Bot 本体として投稿すると匿名にならない。`username` / `avatar_url` の上書きは
**Execute Webhook でしかできない**ため、投稿経路は Webhook に限られる。

## コマンドのルーティング

`internal/handler` に 1 つの表を持ち、interaction Lambda と worker Lambda が同じ表を参照する。
コマンド追加はここに 1 行足すだけで済ませる。

```go
type Mode int

const (
    Sync  Mode = iota // interaction Lambda 内で完結し、type 4 を返す
    Async             // SQS に積み、type 5 を返す。実処理は worker
)

type Command struct {
    Mode       Mode
    Ephemeral  bool                        // 応答を本人にだけ見せるか
    Definition *discord.SlashCommandCreate // Discord に登録する定義。ボタン等では nil

    Validate func(discord.Interaction) error                                 // enqueue 前の検査
    Handle   func(discord.Interaction) (*discord.InteractionResponse, error) // Mode == Sync
    Work     func(context.Context, *Deps, discord.Interaction) error         // Mode == Async
}

// キーはコマンド名、またはコンポーネント / モーダルの custom_id。
var Registry = map[string]Command{ /* 識別子 -> 定義 */ }
```

- **外部 I/O があるなら `Async`。** Discord REST を呼ぶ、処理時間が読めない、リトライしたい、
  失敗を DLQ に残したい、のいずれかに当てはまれば `Async`。固定文言や入力検証だけなら `Sync`。
- **`Sync` ハンドラからネットワーク I/O をしない。** タイムアウトすると interaction 自体が
  失敗し「アプリケーションが応答しませんでした」が出る。`Handle` に `context.Context` を
  渡していないのはこれを書けなくするため。I/O が必要になったら `Async` に変える。
- `Sync` でも入力検証は行い、エラーは即 ephemeral で返す。不正入力を SQS に流さない。
- SQS に積むのは **interaction の生 JSON + 受信時刻**。worker 側で再パースする。
  interaction Lambda に業務ロジックを持ち込まない。
- worker は未知の識別子をエラーにして DLQ へ送る（デプロイ順の食い違いを検知するため）。
- `discord.Interaction` は disgo では**インターフェース**なので値で渡す。生 JSON からの復元は
  `discord.UnmarshalInteraction([]byte)`。種類ごとの中身は型アサーションで取り出す。

### 応答順序: enqueue が先、ack は後

`Async` は **SQS に積んでから、HTTP レスポンスのボディで type 5 を返す**。
Discord は `POST /interactions/{id}/{token}/callback` で先に ack して元のリクエストに 202 を
返す方式も認めているが、採用しない。ack 先行だと 3 秒の予算に discord.com への往復を
持ち込むことになるうえ、enqueue 失敗時のエラーを followup で別途送る必要が出る。

ただし enqueue が遅延すると「キューには残っているのに失敗が見える」ゴースト投稿が起きうる。
**SendMessage には 1 秒程度の context タイムアウトを張り、SDK の既定リトライに任せない。**

### interaction type ごとの扱い

| type | 内容                | 扱い                                                             |
| ---- | ------------------- | ---------------------------------------------------------------- |
| 1    | PING                | 同期で `{"type":1}`。Registry を通さない                         |
| 2    | APPLICATION_COMMAND | Registry に従って分岐                                            |
| 3    | MESSAGE_COMPONENT   | モーダルを開くので**必ず同期**で type 9（defer 不可 = I/O 禁止） |
| 4    | AUTOCOMPLETE        | 使っていない。使う場合も defer できないので `Async` 不可         |
| 5    | MODAL_SUBMIT        | Registry に従って分岐（匿名投稿は `Async`）                      |

## ディレクトリ構成

**root が CDK アプリ**（`cdk init app --language typescript` の生成物をそのまま使う）。
Go は `lambda/` 配下の独立した Go モジュール。

```
.
├── bin/ lib/ test/               # CDK（cdk init の生成名のままにする）
├── cdk.json / package.json / tsconfig.json / jest.config.js
├── lambda/                       # Go モジュール
│   ├── cmd/
│   │   ├── interaction/          # Function URL ハンドラ（同期・軽量）
│   │   ├── worker/               # SQS コンシューマ（Discord REST 呼び出し）
│   │   └── registercmd/          # コマンド登録 CLI（手動実行）
│   ├── internal/
│   │   ├── discord/              # disgo をそのまま使えない部分だけ（後述）
│   │   ├── anon/                 # 匿名 ID 導出
│   │   ├── handler/              # Registry とコマンドごとのロジック
│   │   ├── config/               # 環境変数の読み取りと検証
│   │   └── queue/                # SQS 送受信のラッパ
│   ├── tools/build/              # Lambda バイナリのビルドスクリプト
│   └── build/                    # ビルド成果物（gitignore 対象）
└── .env                          # 秘匿値（コミット禁止）
```

- **Go を `lambda/` の独立モジュールにする理由**: root に `go.mod` を置くと、`node_modules` に
  含まれる CDK の Go テンプレート（`%name%.template.go`）を Go が読もうとして
  `go vet ./...` / `go test ./...` が失敗する。`lambda/` に切れば探索範囲から外れる。
- `internal/discord` は **disgo で足りない部分だけ**を持つ。Interaction の型や REST 呼び出しを
  包み直さない（薄いラッパを重ねても読みにくくなるだけ）。具体的には Function URL の
  リクエストから署名検証に渡す形への変換と、よく使う応答の組み立て。
- `internal/config` で環境変数を起動時に一括ロードし、**必須値が欠けていたら `log.Fatal`**。
  各 `main.go` で `os.Getenv` を直接読まない（鍵や salt が空のまま動く事故を防ぐ）。
  読み取りは `caarlos0/env/v11` のタグで宣言し、**`required` ではなく `notEmpty` を使う**。
  `required` は「変数が設定されていること」しか見ないため、空文字が通ってしまう。
  `registercmd` はローカル実行なので `joho/godotenv` で `.env` を読む。
  godotenv は既に設定されている変数を上書きしないので、シェルでの差し替えが効く。

## 開発コマンド

```bash
# Go（lambda/ で実行）
go run ./tools/build     # 両 Lambda を linux/arm64 で build/<関数名>/bootstrap に出力
go test ./...
go vet ./... && gofmt -l .
go run ./cmd/registercmd # スラッシュコマンド登録（デプロイとは独立した手動操作）

# CDK（root で実行）
npm ci
npm test
npx cdk diff
npx cdk deploy           # 事前に lambda/ でのビルドが必要
```

CDK Toolkit は devDependency なのでグローバルインストールは要らない。
Makefile は置かない（Windows に `make` が無いため、ビルドも Go で書く）。

### デプロイ

**ローカルからの `cdk deploy` のみ。** CI/CD は組まない。
認証情報は profile ではなく `aws login`（AWS CLI v2）で取得する。

1. `cd lambda && go run ./tools/build`
2. root で `npx cdk deploy`
3. Function URL が変わったときだけ Discord Developer Portal の
   **Interactions Endpoint URL** を更新（保存時に Discord が PING を投げて検証する）
4. コマンド定義を変えたときだけ `go run ./cmd/registercmd`

Lambda のコードだけ直すときも `cdk deploy` でよい（`aws lambda update-function-code` を
手で叩くと CDK の状態とずれる）。

## コーディング規約

### Go

- **Discord との通信は `disgo` を使う**（自前実装しない）。使うのは次の 3 つだけで、
  `bot` / `gateway` / `cache` は使わない。
  - `disgo/discord` — Interaction・応答・コマンド定義の型
  - `disgo/rest` — REST クライアント
  - `disgo/httpserver` — `DefaultVerifier` による署名検証**のみ**。`httpserver.New` は
    自前の HTTP サーバを立てる作りなので Lambda では使わない
- **`disgo` は v0.x で破壊的変更がありうる。** `go.mod` で固定し、更新時は型変更を確認する。
- 許可する外部依存はこれだけ: `disgoorg/disgo`、`aws/aws-lambda-go`、`aws/aws-sdk-go-v2/*`、
  `caarlos0/env/v11`、`joho/godotenv`。追加したくなったら、まず標準ライブラリで
  書けないか検討する。
- エラーは `fmt.Errorf("...: %w", err)` でラップして返す。ログ出力はハンドラ最上位だけ。
- ログは `log/slog` の JSON ハンドラ。**salt とトークンは絶対に出さない。**
  ユーザー ID・表示名・メッセージ本文は、**匿名投稿の監査ログとして意図的に記録している**
  （`runPost` の `anonymous message received`）。それ以外の箇所では出さない。
  この記録があるため、匿名性は「同じサーバーの他の参加者に対するもの」であり
  運営者に対するものではない。変更するときは `docs/privacy-policy.md` も直す。
- `internal/` 配下はテスト必須。Discord REST は `rest.WithHTTPClient` に stub を渡してテストする。

### CDK (TypeScript)

- **スタックは 1 つ、環境も 1 つ。** dev/prod の出し分けや環境名のパラメータ化はしない。
- Lambda は `lambda.Function` + `Code.fromAsset('lambda/build/<関数名>')`。
  `GoFunction` / Docker バンドルは使わない（ビルドは `lambda/tools/build` に一本化する）。
  CDK は root で動くのでアセットのパスも root 起点で書く。
- `Runtime.PROVIDED_AL2023` / `Architecture.ARM_64` / ハンドラ名 `bootstrap`。
- Function URL は `authType: NONE`（Discord は署名で認証するため）。
  **必ず Lambda 側で署名検証すること。**
- SQS は FIFO。`MessageGroupId` = チャンネル ID、`MessageDeduplicationId` = interaction ID。
  同一チャンネルへの投稿は直列化されるが、個人利用の規模では問題にならない。

| 対象                   | 設定                                                     |
| ---------------------- | -------------------------------------------------------- |
| interaction Lambda     | timeout **3 秒** / memory 256 MB                         |
| worker Lambda          | timeout 30 秒 / memory 256 MB                            |
| SQS visibilityTimeout  | **180 秒**（worker timeout の 6 倍。短いと二重実行する） |
| SQS の Lambda トリガー | `batchSize: 1`                                           |
| SQS DLQ                | `maxReceiveCount: 1`（再配信すると投稿が重複する）       |
| CloudWatch Logs        | `retention: ONE_MONTH`                                   |

interaction Lambda を 3 秒より長くしても Discord が先に諦めるだけで、課金が増え異常に
気付きにくくなる。変更後は `npx cdk diff` で意図しないリソース置換がないか見る。

## Discord Interaction の制約（必ず守る）

1. **署名検証**: `X-Signature-Ed25519` / `X-Signature-Timestamp` + raw body を
   `httpserver.DefaultVerifier{}.Verify(publicKey, append(timestamp, body...), sig)` で検証。
   失敗時は **HTTP 401**（403 や 400 だとエンドポイント検証に通らない）。
   body は整形せず受け取ったバイト列のまま使う。Function URL の `isBase64Encoded` が
   true のケースを考慮すること。
2. **PING**（type 1）は同期で `{"type":1}`。SQS には流さない。
3. **3 秒ルール**: `Async` は即 ack する。方式は Interaction の種類で決まるので
   コマンド側で選ばない（`discord.Ack` / `discord.AckShowsThinking`）。
   - コンポーネント / MODAL_SUBMIT → **type 6（DEFERRED_UPDATE_MESSAGE）で無言 ack**。
     「考え中…」が出ない。`@original` は**元のメッセージ（パネル）を指す**ので、
     結果は `CreateFollowupMessage` で伝える（編集するとパネルが書き換わる）。
   - スラッシュコマンド → type 5（deferred, ephemeral）。type 6 は使えない。
     `@original` が「考え中…」を指すので、**必ず `UpdateInteractionResponse` で埋める**
     （放置すると「考え中…」が消えない）。
   投稿者への応答は ephemeral にして、投稿内容が公開チャンネルに漏れないようにする。
4. **followup トークンの有効期限は 15 分。** worker はこれを超えないようリトライを絞る。
5. **Discord への投稿は 1 回だけ試行する。** Discord は interaction を再送しないが、
   SQS の再配信で重複投稿が起きうる（投稿してから落ちるケース）。失敗は DLQ に送る。
6. レート制限: 429 の `retry_after` を尊重する。Webhook ごとの制限に注意。

## 匿名化の仕様

投稿者の識別子は保存せず、投稿のたびに導出する（`internal/anon`）。

```
sum := sha256.Sum256([]byte(userID + ":" + date + ":" + salt))   // 32 バイト
        ├─ sum[0:8]  ──> namePrefix のインデックス
        ├─ sum[8:16] ──> nameSuffix のインデックス
        └─ sum[16:32] ─> DiceBear の seed（hex 32 文字）
```

- `date` は **UTC** の `YYYY-MM-DD`。日が変われば匿名 ID も変わり、日をまたいだ追跡ができない。
  `now.UTC()` で丸める（`now` のロケーションに任せると、渡された時刻の場所で境界がずれる）。
- インデックスは `binary.BigEndian.Uint64(sum[0:8]) % uint64(len(namePrefix))`。
- 表示名は **namePrefix + nameSuffix**（例:「しずかなうさぎ」）。リストは `internal/anon` に持つ。
  **順序は変更しない**（途中に挿入するとその日の全員の名前がずれる）。追加は末尾のみ。
- 語彙にネガティブな語・人を指す語は入れない（匿名の名前が侮辱に読めないように）。
- アイコンは `https://api.dicebear.com/10.x/shapes/png?size=128&seed=%s` を `avatar_url` に
  そのまま渡す（`%s` は `hex.EncodeToString(sum[16:32])`）。画像を取りに行くのは Discord 側なので
  Lambda から DiceBear への通信は発生しない。
- 表示名は組み合わせ数が限られるので衝突しうる。**同じ名前でも別人の可能性がある**ことは
  `/help` に明記する。アイコンは 128 ビット由来なので実質衝突しない。
- salt を変えると過去の匿名 ID と紐づかなくなる。

## コマンド / インタラクション仕様

| トリガー       | モード  | 応答                                                              |
| -------------- | ------- | ----------------------------------------------------------------- |
| `/ping`        | `Sync`  | `pong` を返すだけ。疎通確認用                                     |
| `/help`        | `Sync`  | ephemeral。使い方と匿名化の説明（名前の衝突・日替わりを含む）     |
| `/setup`       | `Async` | Webhook を用意しパネルを投稿。実行者にのみ ephemeral で結果を返す |
| パネルのボタン | `Sync`  | type 9 でモーダルを返す。**defer 不可なので I/O 禁止**            |
| モーダル送信   | `Async` | **type 6 で無言 ack** → worker が匿名投稿（成功時は何も表示しない）|

### コマンド定義と登録

- 定義は `discord.SlashCommandCreate` で書き、**`Registry` に一緒に持たせる**。
  `registercmd` はそれを送るだけにして二重管理しない。
- **global 登録のみ**を `rest.SetGlobalCommands`（bulk overwrite）で行う。
  guild 登録は使わない。個別の `Create` だと削除したコマンドが残る。
- global 登録は**即時反映を期待しない**。クライアント側にキャッシュがあるので、変更が
  見えなくてもまず待つ・クライアントを再起動する（実装を疑う前に）。
- すべてサーバー内限定にする。DM では Webhook を作れず機能しないため。
  - `Contexts: []discord.InteractionContextType{discord.InteractionContextTypeGuild}`
  - `IntegrationTypes: []discord.ApplicationIntegrationType{discord.ApplicationIntegrationTypeGuildInstall}`
  - `dm_permission` は**非推奨**。`Contexts` で指定する。

### Bot に必要な権限

OAuth2 スコープ `bot` + `applications.commands`、権限は **Manage Webhooks / Send Messages /
View Channel** = `permissions=536873984`。Manage Webhooks が無いと `/setup` が失敗する。
権限を変えたら招待 URL も更新して入れ直す必要がある。

### `/setup`

- チャンネルの Webhook を `GetWebhooks` で探し、**自アプリ所有のものが無ければ**作成する
  （名前は固定にして再利用可能にする）。**冪等にする**（二度実行しても Webhook は増やさない）。
- ボタン 1 つ（`style: PRIMARY`、ラベル「メッセージを送信」、`custom_id: "anon:open"`）を持つ
  パネルを投稿する。パネル自体は Bot として投稿してよい（匿名投稿ではない）。
- `default_member_permissions` で **Manage Guild** を要求し、一般ユーザーが実行できないようにする。

### ボタン → モーダル → 匿名投稿

- `custom_id` は `anon:` 名前空間で分岐する（`anon:open` / `anon:submit`）。
- モーダルの入力欄は `TextInputComponent`（`TextInputStyleParagraph`）1 つ。
  **`LabelComponent` で包む**。disgo の `TextInputComponent` は `Label` を持たないため、
  ラベルを付ける方法はこれだけ。
- **`MaxLength: 2000`** を設定して Discord のメッセージ上限を入力段階で弾く
  （設定しないと 4000 文字まで入力でき、送信後にエラーになる）。
  `MinLength: 1` / `Required: true` も付ける。`title` とラベルは 45 文字以内。
- 投稿先は**モーダル送信時に渡される `channel_id`**。パネルの位置を覚える必要はない。
- interaction Lambda は本文の空文字と長さだけ検証して SQS に積む。
- worker は匿名 ID を導出し、`CreateWebhookMessage` を `username` / `avatar_url` 上書きで呼ぶ。
  **`allowed_mentions` は `{"parse": []}` を明示**し、`@everyone` やロールメンションが
  匿名投稿から飛ばないようにする。
- **REST クライアントは `internal/discord.NewRest` で作る。** disgo は `AllowedMentions` が
  nil の送信に users / roles / everyone すべてを許可する値を埋めるため、
  `rest.WithDefaultAllowedMentions` で既定を空に反転させてある。指定漏れが 1 か所
  あるだけでメンションが飛ぶので、呼び出し側の注意に頼らない。
- **Webhook が消えていても再作成しない。** 「`/setup` をやり直してください」と followup で伝える
  （勝手に作り直すと権限の無いチャンネルに投稿しうる）。
- 投稿後、**パネルを作り直してチャンネルの一番下に置く**。投稿が増えるとパネルが上に
  流れてボタンを探しにくくなるため。旧パネルの ID は `ModalSubmitInteraction.Message`
  から取れるので保存は要らない。
  **先に新パネルを作ってから旧パネルを消す。** 逆順だと、削除後に作成が失敗した場合に
  パネルが 1 つも無い状態になり、`/setup` をやり直すまで投稿できなくなる。
- **投稿が済んだ後の失敗（パネルの張り替え、followup）はログに残すだけにする。**
  エラーを返すと DLQ 行きになるうえ、再実行されれば投稿が重複する。
- **worker はどんな経路で失敗しても必ず followup を返す。** deferred のまま放置すると
  「考え中…」が消えず、成功したか分からない状態になる。

### 使う REST メソッド

`disgo/rest` 経由で呼び、直接 HTTP を組み立てない。

| 用途                    | メソッド                        |
| ----------------------- | ------------------------------- |
| deferred の内容を埋める | `UpdateInteractionResponse`     |
| 追加の followup         | `CreateFollowupMessage`         |
| 匿名投稿                | `CreateWebhookMessage`          |
| Webhook 一覧 / 作成     | `GetWebhooks` / `CreateWebhook` |
| パネル投稿              | `CreateMessage`                 |
| コマンド登録            | `SetGlobalCommands`             |

followup と Webhook 実行は **Bot トークン不要**（token 自体が認証）。Bot トークンが要るのは
Webhook の一覧 / 作成、パネル投稿、コマンド登録だけ。

## 環境変数

`.env` に定義し、CDK から Lambda に注入する。**`.env` はコミットしない。**

| 変数                     | 用途                              | 使う場所            |
| ------------------------ | --------------------------------- | ------------------- |
| `DISCORD_BOT_TOKEN`      | Webhook の取得 / 作成、パネル投稿 | worker, registercmd |
| `DISCORD_PUBLIC_KEY`     | Ed25519 署名検証                  | interaction         |
| `DISCORD_APPLICATION_ID` | followup / コマンド登録           | worker, registercmd |
| `ANONYMOUS_SALT`         | 匿名 ID の導出                    | worker              |
| `QUEUE_URL`              | SQS 送信先。CDK が注入する        | interaction         |

`registercmd` はローカル実行なので `.env` を直接読む。global 登録のため、登録先ギルドを
指定する環境変数は持たない。

## やらないこと（スコープ外）

個人開発・小規模運用のため**意図的に実装しない**。気を利かせて足さないこと。

- Gateway（WebSocket）接続。HTTP interaction のみ扱う。
- パネルやメッセージ ID の保存。Webhook は毎回 Discord から引き直す。
- 投稿ログを検索・集計できる形で保存すること（DynamoDB 等）。監査用の記録は
  CloudWatch Logs に出しており（保持 1 か月）、これ以上の永続化はしない。
- **濫用対策**（レート制限・連投抑止・ブロック）。永続ストアが無いため実装できない。
- **監視・アラート**（DLQ 通知、CloudWatch Alarm）。問題が起きたらログを直接見る。
- **ローカル実行環境**（署名付きリクエストのテストハーネス、SAM Local 等）。
  検証は `internal/` のユニットテストで行い、結合は実環境で確認する。
- CI/CD、複数環境（dev/prod）の分離。
- 秘匿値を CDK のコードやテンプレートに直接書くこと。
