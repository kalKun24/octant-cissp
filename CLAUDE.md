# Octant

CISSP試験対策のための共有学習ノートアプリ。問題ノートと知識ノートをMarkdownで書き、
4択のフラッシュカードで演習し、SM-2の間隔反復で復習日が決まる。
解説・タグ・問題はClaudeに生成させられる。UIは日本語、モバイルファースト。

旧実装（`../octant` ディレクトリ。単一HTML + Firebase JS SDK 直叩き）を、
**Go REST API + React SPA** として作り直したものがこのリポジトリ。
旧実装は参照用であり、**このリポジトリのコードは旧実装から生成しない**。

## 技術スタック

| 領域 | 採用 | 備考 |
|---|---|---|
| API | Go 1.26 / chi / oapi-codegen v2 | クリーンアーキテクチャ。`api/openapi.yaml` が正 |
| API仕様 | OpenAPI 3.1（仕様先行） | 手で書いた yaml から Go と TS の型を生成 |
| フロント | React 19 / TypeScript / Vite | React Router + TanStack Query + shadcn/ui + Tailwind CSS |
| 永続化 | Cloud Firestore (Native) | RDB・Redis・キューは使わない |
| 認証 | Firebase Authentication（Googleサインイン） | 独自パスワードは持たない |
| AI | Claude API（Anthropic 直接） | サーバ側のみ。同期REST |
| 実行基盤 | Cloud Run（APIコンテナ） | 東京 `asia-northeast1` |
| 配信 | Firebase Hosting | `/api/**` を rewrites で Cloud Run へ |
| オブジェクト | Cloud Storage | 添付画像・Firestoreバックアップ・tfstate |
| IaC | Terraform ~> 1.9 | dev / prod の2環境 |
| CI/CD | GitHub Actions + Workload Identity Federation | キーレス。`main` push で prod 自動デプロイ |

**この一覧にないミドルウェアを勝手に足さない。** 必要になったらまず提案する。

## 全体構成

```
                 ┌──────────────── Firebase Auth（Googleサインイン）
                 │  ① IDトークン取得
                 ▼
ブラウザ ──▶ Firebase Hosting ──/api/**──▶ Cloud Run (Go)
             （React SPA / CDN）    rewrites   │ ② IDトークン検証 + 許可メール確認
                                              │
                                              ├──▶ Cloud Firestore
                                              ├──▶ Cloud Storage（添付）
                                              └──▶ Claude API（Secret Manager のキー）
```

**ブラウザは Firestore に直接アクセスしない。** Firebase SDK はフロントで
**認証のためだけに**使う。データの読み書きはすべて Go の REST API を通す。
そのため `firebase/firestore.rules` は全ドキュメントを `false` で拒否し、
サーバのサービスアカウント（ルールを迂回する Admin 権限）だけが実際の読み書きを行う。

この設計の理由は3つ。バリデーションと認可をGoの1箇所に集約できること、
セキュリティルールという第2の認可ロジックを保守しなくてよいこと、
AI呼び出しとデータ更新を同じトランザクション境界で扱えること。

## ディレクトリ

```
api/
  openapi.yaml               ★API仕様の正。エンドポイント変更はここが先
backend/
  cmd/server/main.go         エントリポイント。DIの配線はここだけ
  internal/
    config/                  環境変数の読み込みと検証
    domain/
      entity/                エンティティとドメインルール（SM-2の計算はここ）
      repository/            リポジトリのインターフェース定義
    usecase/                 note/ tag/ drill/ ai/ stats/ user/
    interface/
      handler/               生成された ServerInterface の実装
      dto/                   API境界の型変換（entity ⇔ 生成型）
    infrastructure/
      firestore/             リポジトリの具体実装
      firebaseauth/          IDトークン検証
      claudeapi/             Claude API クライアント
      gcs/                   署名付きURL発行
      middleware/            認証・ログ・リカバリ・レート制限
  pkg/                       外部公開してよい汎用ユーティリティのみ
  test/                      E2E・テストヘルパ
frontend/
  src/
    api/                     openapi.yaml から生成した型 + fetchクライアント
    pages/ components/ hooks/
    firebase/                Firebase Auth の初期化のみ
infra/
  modules/                   cloud-run-service/ firestore/ artifact-registry/
                             firebase-auth/ storage/ 等の再利用単位
  environments/dev/ prod/    環境ごとの値。tfstate は GCS バックエンド
firebase/
  firebase.json              Hosting 設定と /api/** の rewrites
  firestore.rules            全拒否（上記の理由による）
  firestore.indexes.json     複合インデックス定義
docs/                        設計メモ・運用手順
tickets/                     チケット（backlog / in-progress / done）
```

**`backend/internal/interface/repository/` は作らない。** リポジトリの
インターフェースは `domain/repository/` に置く（依存を内側に向けるため）。

## コマンド

```bash
make setup            # 依存インストール（Go / npm / ツール類）
make gen              # openapi.yaml → Goサーバ型 + TS型 を生成
make dev              # エミュレータ + API + Vite を同時起動
make test             # backend の全テスト（エミュレータ利用）
make test-front       # frontend のテスト
make lint             # golangci-lint + eslint + tsc --noEmit
make fmt              # gofmt + prettier
make build            # フロントのビルドとAPIコンテナのビルド
make deploy-dev       # dev環境へ手動デプロイ（通常はCIに任せる）
make tf-plan ENV=dev  # Terraform の差分確認
make tf-apply ENV=dev # Terraform の適用
```

`make dev` が起動するもの:

| プロセス | ポート |
|---|---|
| API（Go） | 8080 |
| Vite（フロント） | 5173 |
| Firestore エミュレータ | 8080 とは別（既定 8808） |
| **Firebase Auth エミュレータ** | **9099** |

**Auth エミュレータは必須。** ローカルで本物の ID トークンと同形のトークンを
発行でき、Go 側は `FIREBASE_AUTH_EMULATOR_HOST` を読むだけで本番と同じ
検証コードを通せる。**認証をスキップするバイパスをアプリに実装しない。**

**`make gen` の生成物はコミットする。** CI が `make gen` 後に差分がないことを
検証するので、`openapi.yaml` を変えたら必ず生成してからコミットする。

## API 規約

### API First

順序を守る。**逆順で書き始めない。**

1. `api/openapi.yaml` を更新する
2. `make gen` で Go と TypeScript の型を生成する
3. ハンドラ → ユースケース → リポジトリ の順に実装する
4. フロントは生成された TS 型を使う（手書きの型定義を作らない）

**生成コードを手で編集しない。** 直したいときは `openapi.yaml` を直して再生成する。

### レスポンス形式

すべてのレスポンスは以下の封筒に入れる。片方は必ず `null`。

```json
{ "data": { ... }, "error": null }
{ "data": null, "error": { "code": "NOTE_NOT_FOUND", "message": "ノートが見つかりません" } }
```

`code` は機械可読な SCREAMING_SNAKE_CASE、`message` はそのまま画面に出せる日本語。

### ページネーション

**カーソル方式**を使う。`?limit=50&cursor=<opaque>` を受け、
`{ "items": [...], "next_cursor": "..." }` を返す（末尾なら `next_cursor` は `null`）。

オフセット方式を使わないのは、Firestore の `Offset()` が
**読み飛ばした分も課金対象になる**ため。深いページほど無駄に高くなる。

### ステータスコード

| 状況 | コード |
|---|---|
| 取得・更新成功 | 200 |
| 作成成功 | 201（`Location` ヘッダを付ける） |
| 削除成功 | 204 |
| 入力不正 | 400 |
| 未認証（トークンなし・失効） | 401 |
| 認証済みだが許可されていない | 403 |
| 対象なし | 404 |
| レート制限 | 429（`Retry-After` を付ける） |

## データモデル

Firestore のコレクション構成。**旧実装のキーバリュー方式（`bank:body:v1:{0-3}` 等）は
引き継がない。** あれは Claude の `window.storage` の制約への回避策であり、
このリポジトリには存在しない。

| パス | 共有/個人 | 内容 |
|---|---|---|
| `/notes/{noteId}` | 共有 | ノート本体（メタ + 本文 + 選択肢 + 解説） |
| `/tags/{tagId}` | 共有 | `{name, color, createdAt}` |
| `/prompts/{promptId}` | 共有 | AIプロンプトのテンプレート |
| `/prompts/{promptId}/revisions/{revId}` | 共有 | プロンプトの変更履歴 |
| `/users/{uid}` | 個人 | `{email, displayName, settings, createdAt}` |
| `/users/{uid}/progress/{noteId}` | 個人 | SM-2の状態。1ノート1ドキュメント |
| `/users/{uid}/sessions/{sessionId}` | 個人 | ドリル1回分の記録 |
| `/users/{uid}/days/{YYYY-MM-DD}` | 個人 | 日別集計（ヒートマップ・連続日数用） |
| `/usage/{uid}` | 個人 | AI呼び出しのレート制限カウンタ |

### エンティティ

```
Note      { id, type: "q"|"k", title, domain, tags[], related[],
            body, choices[], answer, explanation,
            createdAt, updatedAt, authorUid }
Progress  { noteId, attempts, correct, ef, interval, reps, due, history[] }
Tag       { id, name, color }
```

- **`domain` は1〜8**（CISSPの試験ドメイン）で、**問題ノート（`type:"q"`）だけが持つ**。
  知識ノートは常に `null`。保存時にサーバ側で落とす
- `tags` はドメインとは無関係な横断テーマ用。`related` は相互参照するノートIDの配列
- **1ノート = 1ドキュメント**。同時編集はノート単位で後勝ち

### インデックス

- 復習期限の抽出（`/users/{uid}/progress` を `due <= now` で絞り `due` 昇順）は
  単一フィールドインデックスで足りる
- **ノート一覧の `type` 絞り込み + `updatedAt` 降順は複合インデックスが要る。**
  クエリを足したら `firebase/firestore.indexes.json` を必ず更新する。
  更新漏れは本番でだけ失敗するので、統合テストでクエリを1度は通すこと

## 認証・認可

1. フロントで Firebase Auth の Googleサインイン → IDトークンを取得
2. 全APIリクエストに `Authorization: Bearer <IDトークン>` を付ける
3. Go の認証ミドルウェアが Firebase Admin SDK でトークンを検証する
4. `email_verified == true` と**許可メールのホワイトリスト**を確認する
5. `uid` と `email` を context に入れて下流に渡す

- 許可メールは環境変数 `ALLOWED_EMAILS`（カンマ区切り）で与える。**コードに埋めない**
- ロールは持たない。許可された利用者は全員が共有ノートを読み書きできる
  （閲覧者/編集者の分離は将来の課題）
- **個人データ（`/users/{uid}/**`）は必ず context の `uid` でパスを組み立てる。**
  リクエストボディやパスパラメータから来た `uid` を信用しない
- トークン検証は認証ミドルウェア1箇所だけで行う。ハンドラで再実装しない

## AI 機能

- 呼び先は Anthropic API 直接。キーは Secret Manager の `anthropic-api-key` を
  Cloud Run の環境変数 `ANTHROPIC_API_KEY` として注入する
- **キーがフロントに渡る経路を作らない。** レスポンスにも含めない
- モデルIDは環境変数 `CLAUDE_MODEL` で指定（既定 `claude-sonnet-5`）。
  ハードコードしない。切り替えはCloud Runの環境変数更新だけで済むこと
- **応答は同期REST。** ストリーミング（SSE）は使わない。
  生成完了まで待ち、パース済みの構造化データを返す
- Claudeの出力JSONは**必ず検証してから**保存する。パース失敗・スキーマ不一致は
  502 で返し、Firestoreには何も書かない
- レート制限は `/usage/{uid}` のカウンタで行う（Cloud Runのインスタンス数に
  依存しないため）。上限超過は 429 + `Retry-After`
- プロンプトは `/prompts` に置き、編集時は `revisions` に旧版を残す

## アーキテクチャ規約（Go）

依存の向きは内側へ一方向。**逆流させない。**

```
infrastructure ──▶ interface ──▶ usecase ──▶ domain
       └────────────────────────────────────────┘
              （domain のインターフェースを実装する）
```

- `domain/entity`: 外部パッケージへの import 禁止（標準ライブラリのみ）。
  SM-2 の計算はここに置き、純粋関数としてテストする
- `domain/repository`: インターフェースのみ。Firestoreの型を露出させない
- `usecase`: `domain` のみに依存。リポジトリはコンストラクタで受け取る（DI）。
  `net/http` や Firestore の型をここに持ち込まない
- `interface/handler`: 生成された `ServerInterface` を実装する。
  HTTPの関心事（ステータスコード・ヘッダ）はここで完結させる
- `infrastructure`: 具体実装。ここだけが外部SDKを知っている
- 配線は `cmd/server/main.go` の1箇所。DIフレームワークは使わない

**新しいコードを書く前に、同種の既存実装を読んでそのパターンに合わせる。**

### 品質

- エラーは `fmt.Errorf("...: %w", err)` で文脈を付けて伝播する。握り潰さない
- ログは `log/slog` の構造化JSON。`trace_id` / `uid` / `path` / `latency_ms` を入れる。
  **ノート本文・メールアドレス・トークンをログに出さない**
- ユースケース層のユニットテストは必須（テーブル駆動、同パッケージに `_test.go`）
- Firestore を触るテストはエミュレータ前提（`FIRESTORE_EMULATOR_HOST`）。
  ファイル名は `*_integration_test.go`
- 提出前に `make fmt` `make lint` `make test` を通す

## フロントエンド規約

- ルーティングは **React Router**。画面は `src/pages/`、部品は `src/components/`
- **サーバ状態は TanStack Query が持つ。** グローバルな状態ストアを別に作らない。
  クライアント固有の状態（開いているシート、入力途中の値）だけ `useState` / Context
- API呼び出しは `src/api/` の生成済み型を使う。**手書きのレスポンス型を作らない**
- UIは shadcn/ui + Tailwind CSS v4。**色・間隔・角丸を任意の値でハードコードしない**
  （テーマトークンを使い、必要なら `frontend/src/index.css` の `@theme` に足す。
  **v4 は `tailwind.config` を持たない。** 色を足すときは `:root` と `.dark` の両方に
  CSS 変数を定義し、`@theme inline` でユーティリティに結びつける）
- ダークテーマとライトテーマの両方で成立させる
- **モバイルファースト。** タップ領域44px以上、キーボードフォーカス可視、
  `prefers-reduced-motion` 対応を維持する
- Markdown レンダリングは `marked` + `DOMPurify` + `highlight.js` + `mermaid`。
  **`dangerouslySetInnerHTML` に渡してよいのは DOMPurify を通した後のHTMLだけ。**
  `marked` の出力を直接渡さない。サニタイズ設定を緩める変更は単独では入れない
  （ノート本文はユーザー入力であり、XSSの主要な入口）
- `[[ノートタイトル]]` の wikiリンクとバックリンク表示を維持する
- UI文言は日本語。常体は使わず「〜します」。エラーは**何が起きたかと次にどうするか**を
  書く（謝らない）。v1は日本語のみで、i18nライブラリは導入しない

## インフラ・デプロイ

### 環境

| 環境 | GCPプロジェクト | ブランチ | デプロイ |
|---|---|---|---|
| dev | `octant-dev` | `develop` | push で自動 |
| prod | `octant` | `main` | push で自動 |

**GCP はゼロから立ち上げる。** プロジェクトも課金の紐付けも存在しない状態が出発点で、
TICKET-016（ブートストラップ）で作る。**上記のプロジェクト ID は希望値**であり、
プロジェクト ID は全世界で一意のため取得できない可能性がある。
**実際に取れた ID を正とし、この表を書き換えること。**

**手で `gcloud run deploy` を打たない。** すべての変更は Terraform と
GitHub Actions を経由する。緊急時に手で当てた場合は、その日のうちに
Terraform 側へ反映して差分を消す。

- 認証は Workload Identity Federation。**サービスアカウントキーのJSONを
  発行しない・リポジトリに置かない**
- tfstate は GCS バックエンド（環境ごとに別バケット、バージョニング有効）
- Cloud Run のリビジョンにはコミットSHAをタグ付けし、ロールバックできるようにする

### コスト規約

個人利用のため、**固定費ゼロを維持する**ことを設計上の制約として扱う。

- **`min_instances = 0` を必ず守る。** 1にするとアイドル課金で月2,000円規模になる。
  コールドスタート（Go なら1〜2秒）は許容する
- **`max_instances` に上限を設ける**（dev: 2 / prod: 5）。暴走時の青天井を防ぐ
- **Artifact Registry にクリーンアップポリシーを設定する**（最新3世代のみ保持）。
  イメージが溜まると無料枠0.5GBを超えて課金される
- GCS のバックアップバケットはライフサイクルルールで90日削除
- 予算アラートを設定する（月1,000円、50/90/100%でメール通知）
- Firestore の読み取り回数を意識する。**一覧取得で全ドキュメントを引かない**

依存する無料枠（この規模なら超えない）:
Cloud Run 月200万リクエスト、Firestore 日50,000読み取り/20,000書き込み、
Firebase Hosting 10GB保存・日360MB転送。

### 運用

- Firestore は PITR と削除保護を有効にする
- Cloud Scheduler が週次で Firestore を GCS へエクスポートする
- 復元は `gcloud firestore import gs://<バケット>/<フォルダ>`

## ブランチ・チケット・コミット

- ブランチ: `main`（本番） ← `develop`（統合） ← `feature/TICKET-XXX`
- **`main` と `develop` に直接 push しない。** 必ずPR経由
- PRの向き先は `develop`。`develop` → `main` のマージが本番リリース
- **セルフマージ可**（1人体制のため承認者を置かない）。ただし
  **マージ前に QA Team を起動し、判定が READY または CONDITIONAL であること**を
  確認する。**NEEDS WORK のままマージしない。** これが人間の承認の代替である
- チケット運用は `.claude/skills/ticket` に従う（チケット=PR単位、
  サブチケット=コミット単位）
- コミットメッセージ: `<type>(<scope>): 日本語の件名`
  - type: `feat` / `fix` / `docs` / `refactor` / `test` / `chore` / `perf`
  - scope: `TICKET-XXX` またはモジュール名（`backend` / `frontend` / `infra` / `api` / `repo`）。
    `repo` はリポジトリ全体に関わる作業（Makefile・CI・`.gitignore`・規約文書）に使う

## テスト

| 層 | 対象 | 実行 |
|---|---|---|
| ユニット | domain のドメインルール（SM-2など）、usecase | `make test`（エミュレータ不要） |
| 統合 | 認証・Firestoreリポジトリ | `make test-integration`（エミュレータを起動する） |
| 契約 | 実装が `openapi.yaml` と一致するか | `make gen-check` |
| フロント | フック・コンポーネント（Vitest） | `make test-front` |

**CI は上記4つすべてを実行する。** 統合テストを `make test` に含めないのは、
エミュレータが無い環境で `t.Skip` されると**緑に見えるのに何も検証していない**状態に
なるため。分離しておけば「速い `make test`」と「本当に叩く `make test-integration`」の
区別が実行時に明確になる。

- **`t.Skip` された統合テストを PASS に数えない。** 未検証として扱う
- **Claude API を叩く自動テストを CI に置かない**（課金とレート制限のため）。
  `claudeapi` はインターフェース越しにモックする
- 疎通確認が要る場合は手動のスモークスクリプトを用意し、明示的に実行する
- 統合テストには **firebase-tools** が必要。Auth エミュレータだけなら Java 不要だが、
  **Firestore エミュレータには JRE が要る**（TICKET-006 で必要になる）

## 守るべき制約（まとめ）

- **ブラウザから Firestore を直接読み書きしない。** データは必ず REST API 経由
- **`firestore.rules` は全拒否のまま。** ここに認可ロジックを書き足さない
- **`api/openapi.yaml` を更新せずにエンドポイントを追加・変更しない**
- **生成コードを手で編集しない。** 直したいときは `openapi.yaml` を直して再生成する
- **`min_instances = 0` を変更しない**（コスト制約）
- **Anthropic APIキーをフロントに渡さない・ログに出さない**
- **`uid` はリクエストではなく検証済みトークンから取る**
- **認証をスキップするバイパスをアプリに実装しない**（ローカルは Auth エミュレータ）
- **DOMPurify を通さないHTMLを `dangerouslySetInnerHTML` に渡さない**
- **層をまたぐ依存を逆流させない**（usecase が HTTP や Firestore を知らないこと）
- 旧実装（`../octant`）のコードをコピーして持ち込まない。仕様の参照だけに使う

## 現状

**TICKET-002（OpenAPI 骨子とコード生成）まで完了**（2026-07-28 時点）。

- `make` が一通り動く。React 19 + Tailwind v4 + shadcn/ui のフロント、
  distroless の Dockerfile、PR での CI が揃っている
- **`api/openapi.yaml` が API の正**。`make gen` で Go のサーバインターフェースと
  TypeScript の型を生成し、CI の `make gen-check` が差分を検知する
- **API の基底パスは `/api`。** 実装されているのは `GET /api/health` と `HEAD /api/health` のみ
  （`openapi.yaml` の `paths` は `/health` で、基底は `servers` 側に持たせている）
- **認証はまだ存在しない**
- **GCP のリソースは何も作られていない。** プロジェクトすら存在しない状態から始める
  （TICKET-016 で立ち上げる）

**実装順は `tickets/README.md` を参照。番号順ではない。**
次は **TICKET-003（認証基盤）** または **TICKET-016（GCP ブートストラップ）**。
この2つは互いに独立しているのでどちらからでも着手できる。

v1 のスコープ（すべて実装対象）:

1. ノートCRUD + タグ + Markdown（mermaid・コードハイライト・wikiリンク・バックリンク）
2. ドリル + SM-2 間隔反復
3. AI機能（解説生成・タグ提案・問題生成・プロンプト管理）
4. 統計・ヒートマップ・JSON入出力・テーマ切替

v2以降の候補: 閲覧者/編集者のロール分け、AIによる弱点分析、PWA化、
ノートへの画像添付（Cloud Storage の署名付きURL）。

### 旧実装との既知の相違

- Claude artifact 版（単一HTML + `window.storage`）は**廃止**。Web版に一本化した
- 既存の稼働中Firestoreデータは**移行しない**。ゼロから始める
- 本文のバケット分割（`BUCKETS = 4`）は廃止。1ノート=1ドキュメント
