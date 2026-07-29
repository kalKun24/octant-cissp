# TICKET-016: GCP プロジェクトのブートストラップ

## 基本情報

| 項目 | 内容 |
|---|---|
| チケットID | TICKET-016 |
| ステータス | 🟡 作業中 |
| 作成日 | 2026-07-28 |
| 着手日 | 2026-07-29 |
| 完了日 | - |
| ブランチ名 | `feature/TICKET-016` |
| PR番号 | - |
| PRリンク | - |

## 概要

**GCP をゼロから立ち上げる。** プロジェクトの作成・課金の紐付け・API の有効化・
tfstate 用バケットの作成・Firebase の有効化までを行い、**Terraform が動き出せる状態**にする。

> **⚠ 実行順序**: このチケットは番号が最後だが、**TICKET-004（Terraform）より先に実施する。**
> 番号は起票順であって実装順ではない。
>
> ```
> 003（認証基盤）→ 016（このチケット）→ 004（Terraform）→ 005（CI/CD）→ 006 以降
> ```

## 背景・目的

TICKET-004 / 005 は「GCP プロジェクトが既にある」ことを暗黙の前提にしていたが、
**それを作るチケットが存在しなかった。** 加えて2つの鶏と卵がある。

1. **tfstate**: Terraform は state を GCS に置くが、**自分の state 置き場を同じ apply では作れない。**
   バケットは Terraform の外で先に用意する必要がある
2. **WIF**: CI から Terraform を動かすには Workload Identity Federation が要るが、
   その WIF を作るのも Terraform。**初回だけは手元の認証情報（ADC）で回す**必要がある

このチケットで両方を解消し、以降を「Terraform と CI に載った状態」で進められるようにする。

## 方針

- **プロジェクト作成は `gcloud` で行う**（Terraform では管理しない）。
  個人アカウント（組織なし）では課金アカウントの権限まわりが煩雑になり、
  state が2層になる割に得るものが少ないため
- **Terraform は「プロジェクトの中身」だけを管理する**（TICKET-004 の担当範囲）
- **`gcloud` でできる部分はスクリプト化して冪等にする。** 手順書だけだと再現できない
- **コンソール操作が必須の部分は手順書に残す**（Firebase Auth の Google プロバイダなど）

## 受け入れ条件

### プロジェクトと課金

- [ ] GCP プロジェクト **`octant-dev`（dev）** と **`octant`（prod）** が存在する
- [ ] 両プロジェクトに課金アカウントが紐付いている
- [ ] **プロジェクト ID が実際に取得できたものと `CLAUDE.md` の記載が一致している**
      （`octant` は既に使われている可能性がある。取れなければ別 ID にして CLAUDE.md を直す）

### API の有効化

- [ ] 両プロジェクトで必要な API が有効になっている。少なくとも以下:
      `cloudresourcemanager` / `serviceusage` / `iam` / `iamcredentials` / `sts` /
      `run` / `artifactregistry` / `firestore` / `firebase` / `firebasehosting` /
      `identitytoolkit` / `secretmanager` / `storage` / `cloudscheduler` / `billingbudgets`
- [ ] 有効化した API の一覧がスクリプトに列挙されており、追加時の変更箇所が1つである

### Terraform の足場

- [ ] **tfstate 用の GCS バケットが環境ごとに存在する**（`octant-dev-tfstate` / `octant-tfstate` 等）
- [ ] tfstate バケットで**バージョニングが有効**、**削除保護が有効**、**公開アクセスが遮断**されている
- [ ] `gcloud auth application-default login` で Terraform が両プロジェクトに `plan` できる
      （この時点では apply する対象が無くてよい。認証が通ることの確認）

### Firebase

- [ ] 両プロジェクトで Firebase が有効になっている
- [ ] **Firebase Auth の Google サインインが有効**になっている
- [ ] 承認済みドメインに `localhost` と Hosting のドメインが入っている
- [ ] **Firebase ウェブアプリが登録され、`firebaseConfig` を取得できている**
- [ ] Firebase Hosting のサイトが存在する
- [ ] Firestore データベースが**未作成**であること（**TICKET-004 の Terraform で作る**。
      ここで作るとリージョンや PITR 設定が Terraform 管理外になる）

### 記録と再現性

- [ ] `scripts/bootstrap-gcp.sh` が **冪等**（2回実行してもエラーにならず、状態が変わらない）
- [ ] スクリプトが**プロジェクト ID と課金アカウントを引数か環境変数で受け取り**、ハードコードしていない
- [ ] `docs/gcp-bootstrap.md` に、スクリプトで自動化できないコンソール操作の手順がある
- [ ] `firebaseConfig` の値の置き場が決まっている（`frontend/.env.example` に
      `VITE_FIREBASE_*` のキーを列挙。**実値はコミットしない**）
- [ ] `.env.example` に `ALLOWED_EMAILS` / `CLAUDE_MODEL` など必要な環境変数が列挙されている
- [ ] **手順どおりに実際に実行し、動くことを確認した**（机上の手順書で終わらせない）

### コスト

- [ ] この時点で**課金が発生していない**ことを確認した
      （プロジェクト作成・API 有効化・空バケットは無料。
      Firestore と Cloud Run は TICKET-004 以降で作る）

## サブチケット（コミット計画）

- [ ] `feat(infra): GCP ブートストラップスクリプトを追加`
- [ ] `feat(infra): tfstate バケットの作成をスクリプトに追加`
- [ ] `docs(infra): コンソール操作（Firebase Auth・ウェブアプリ登録）の手順を追加`
- [ ] `chore(repo): .env.example と frontend/.env.example を追加`
- [ ] `docs(repo): ブートストラップの実行結果と取得した設定値の記録方法を追記`

## 関連情報

- CLAUDE.md「インフラ・デプロイ」「コスト規約」
- リージョンは `asia-northeast1`（東京）
- **サービスアカウントキーの JSON を発行しない。** 認証は ADC（手元）と WIF（CI）のみ
- `firebaseConfig` は**秘密情報ではない**（クライアントに配布される）が、
  プロジェクト固有なので環境変数で渡し、リポジトリに実値を置かない

### 後続チケットとの関係

| チケット | このチケットに依存する内容 |
|---|---|
| **TICKET-004** | プロジェクト・API・tfstate バケットが無いと `terraform init` すら通らない |
| **TICKET-005** | WIF の作成先プロジェクトと、Hosting サイトが必要 |
| **TICKET-008** | フロントの Firebase Auth 初期化に `firebaseConfig` が必要 |
| **TICKET-003** | ローカルは Auth エミュレータで完結するため**依存しない**（先に着手できる） |

### 注意: 失敗しやすい点

- **プロジェクト ID は全世界で一意。** `octant` は取得できない可能性が高い。
  取れなかった場合は `octant-cissp-prod` などにフォールバックし、**CLAUDE.md の環境表を直す**
- **課金アカウントの紐付けを忘れると Cloud Run も Firestore も作れない。**
  API の有効化まではできてしまうため気づきにくい
- **Firestore はここで作らない。** コンソールで「Firestore を使ってみる」を押すと
  リージョン固定でデータベースができてしまい、Terraform 管理外になる
- Firebase の有効化は「既存の GCP プロジェクトを Firebase に追加する」方向で行う。
  Firebase コンソールから新規作成すると GCP プロジェクト ID が自動採番される
