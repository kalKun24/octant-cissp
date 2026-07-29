# TICKET-005: CI/CD（WIF・自動デプロイ・Hosting rewrites）

## 基本情報

| 項目 | 内容 |
|---|---|
| チケットID | TICKET-005 |
| ステータス | 🟡 作業中 |
| 作成日 | 2026-07-27 |
| 着手日 | 2026-07-30 |
| 完了日 | - |
| ブランチ名 | `feature/TICKET-005` |
| PR番号 | - |
| PRリンク | - |

## 概要

GitHub Actions から Workload Identity Federation でキーレス認証し、
`develop` → dev 環境、`main` → prod 環境へ自動デプロイする。
Firebase Hosting の `/api/**` rewrites で Cloud Run にプロキシする構成を確定させる。

> **前提条件**: **TICKET-016（GCP ブートストラップ）→ TICKET-004（Terraform）の順で
> 完了していること。** GCP プロジェクトが無ければ WIF の作成先が無く、
> Cloud Run サービスや Artifact Registry が無い状態で自動デプロイが走れば失敗する。
>
> ```
> 016（ブートストラップ）→ 004（Terraform）→ 005（このチケット）
> ```
>
> **鶏と卵に注意**: WIF を作るのは Terraform だが、その Terraform を CI から動かすにも
> WIF が要る。**WIF の初回作成は手元の ADC で `apply` する**こと。
>
> **⚠ `main` へマージした瞬間に prod デプロイが走る。**
> GitHub Actions は `push` イベントで「push されたコミット内のワークフローファイル」を使うため、
> このチケットを `main` にマージしたマージコミット自体が prod デプロイを発火させる。
>
> **方針（2026-07-30 決定）: prod まで一気に出す。**
> 当初は「develop 止まり」としていたが、パイプライン全体を早期に検証する利点を取った。
> **prod にデプロイされるのは `/api/health` だけを返すサーバ**であり、
> 認証は有効、Firestore は空・全拒否ルール適用済みなので実害はない。
> `main` へのマージ前に、**dev で全経路が通ることを必ず確認**すること。

## 背景・目的

手作業デプロイを禁止する方針（CLAUDE.md）を実行可能にする。
また Hosting の rewrites によって CORS 設定が不要になるため、この構成を先に固めておくと
以降のフロント実装が単純になる。

## 受け入れ条件

- [ ] Workload Identity Federation の設定が Terraform で定義されている
- [ ] **サービスアカウントキーの JSON がリポジトリにも Secrets にも存在しない**
- [ ] `develop` への push で dev 環境（Cloud Run + Hosting）へ自動デプロイされる
- [ ] `main` への push で prod 環境へ自動デプロイされる
- [ ] Cloud Run のリビジョンにコミット SHA がタグ付けされる
- [ ] `firebase/firebase.json` の rewrites で `/api/**` が Cloud Run に転送される
- [ ] ブラウザから同一オリジンで API を呼べる（CORS 設定が不要であることを確認）
- [ ] `firebase/firestore.rules` が全ドキュメント拒否の状態でデプロイされる
- [ ] デプロイ前に lint とテストが実行され、失敗したらデプロイされない

### TICKET-004 の QA からの申し送り（このチケットで対応する）

- [ ] **WIF の CI 用 SA は `roles/run.admin` ではなく `roles/run.developer` +
      特定サービスへの `actAs` に絞る。** Cloud Run の `image` は
      `ignore_changes` の対象で `terraform plan` が不正な差し替えを検出しないため、
      デプロイ権限を持つ主体が広いと「`octant-api` SA（Firestore 全アクセス）で
      動く任意のコード」をデプロイできてしまう
- [ ] **既存 CI の `~/go/bin` キャッシュを WIF ワークフローと共有しない。**
      キャッシュ汚染が、デプロイ権限を持つジョブでのコード実行経路になる
- [ ] **`firebase.json` の `/api/**` rewrites は SPA フォールバック
      （`**` → `/index.html`）より前に置く。** 順序を誤ると API が index.html を返す
- [ ] **`make rules-deploy` を CI に組み込む**（TICKET-004 で Makefile に追加済み）。
      現在は手動デプロイのみで、ルールの退行を CI が検知できない
- [ ] Actions を **SHA でピン留め**する（タグ参照はキャッシュ汚染・タグ付け替えの経路）
- [ ] **`default_uri_disabled` の検証**: `*.run.app` の直接アクセスを塞げる可能性があるが、
      **Firebase Hosting の rewrites が動くか未検証**。dev で先に試し、
      動かなければ採用しない。**未検証のまま prod に入れないこと**

## サブチケット（コミット計画）

- [ ] `feat(infra): Workload Identity Federation の定義を追加`
- [ ] `feat(repo): firebase.json と /api rewrites を追加`
- [ ] `feat(repo): firestore.rules（全拒否）と indexes.json を追加`
- [ ] `ci(repo): develop への push で dev へデプロイするワークフローを追加`
- [ ] `ci(repo): main への push で prod へデプロイするワークフローを追加`

## 関連情報

### TICKET-003 からの申し送り: デプロイ後の起動確認と環境変数

**`APP_ENV` / `FIREBASE_PROJECT_ID` / `ALLOWED_EMAILS` の3つが欠けると、
Cloud Run のリビジョンは起動に失敗する**（設定漏れを既定値で黙って埋めると
認証の緩い側へ倒れるため、あえて起動失敗にしてある）。値の設定自体は
TICKET-004 の Terraform 側の責務だが、CI は次の2点に注意すること。

- **デプロイ後に `GET /api/health` が 200 を返すことを確認するステップを入れる。**
  上記の設定漏れは「新リビジョンが起動せず、旧リビジョンにトラフィックが
  残ったまま」という形で現れるため、デプロイジョブ自体は成功してしまう
- ワークフローから環境変数を上書きする場合も **`APP_ENV` を `local` にしない**。
  `local` 以外でのみ `FIREBASE_AUTH_EMULATOR_HOST` の混入ガードが効く
- **`FIREBASE_AUTH_EMULATOR_HOST` を CI からデプロイ先へ渡さない**
  （エミュレータ接続時は ID トークンの署名検証が省略される）

### そのほか

- CLAUDE.md「インフラ・デプロイ」「全体構成」
- **`firestore.rules` は全拒否のままにする。** ブラウザは Firestore に直接アクセスしない設計であり、
  ここに `allow` を書き足すことは設計違反
- 手で `gcloud run deploy` を打たない
