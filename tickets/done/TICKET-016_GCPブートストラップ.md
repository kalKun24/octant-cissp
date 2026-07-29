# TICKET-016: GCP プロジェクトのブートストラップ

## 基本情報

| 項目 | 内容 |
|---|---|
| チケットID | TICKET-016 |
| ステータス | 🟢 完了 |
| 作成日 | 2026-07-28 |
| 着手日 | 2026-07-29 |
| 完了日 | 2026-07-29 |
| ブランチ名 | `feature/TICKET-016` |
| PR番号 | #11 |
| PRリンク | https://github.com/kalKun24/octant-cissp/pull/11 |

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

- [x] GCP プロジェクト **`octant-dev`（dev）** と **`octant-prod`（prod）** が存在する
- [x] 両プロジェクトに課金アカウントが紐付いている
- [x] **プロジェクト ID が実際に取得できたものと `CLAUDE.md` の記載が一致している**
      （`octant` は取得できず `octant-prod` にフォールバック。CLAUDE.md を更新済み）

### API の有効化

- [x] 両プロジェクトで必要な API が有効になっている。少なくとも以下:
      `cloudresourcemanager` / `serviceusage` / `iam` / `iamcredentials` / `sts` /
      `run` / `artifactregistry` / `firestore` / `firebase` / `firebasehosting` /
      `identitytoolkit` / `secretmanager` / `storage` / `cloudscheduler` / `billingbudgets`
- [x] 有効化した API の一覧がスクリプトに列挙されており、追加時の変更箇所が1つである

### Terraform の足場

- [x] **tfstate 用の GCS バケットが環境ごとに存在する**（`octant-dev-tfstate` / `octant-prod-tfstate`）
- [x] tfstate バケットで**バージョニングが有効**、**削除保護が有効**、**公開アクセスが遮断**されている
- [x] `gcloud auth application-default login` で Terraform が両プロジェクトに `plan` できる
      （この時点では apply する対象が無くてよい。認証が通ることの確認）

### Firebase

- [x] 両プロジェクトで Firebase が有効になっている
- [x] **Firebase Auth の Google サインインが有効**になっている
- [x] 承認済みドメインに `localhost` と Hosting のドメインが入っている
- [x] **Firebase ウェブアプリが登録され、`firebaseConfig` を取得できている**
- [x] Firebase Hosting のサイトが存在する
- [x] Firestore データベースが**未作成**であること（**TICKET-004 の Terraform で作る**。
      ここで作るとリージョンや PITR 設定が Terraform 管理外になる）

### 記録と再現性

- [x] `scripts/bootstrap-gcp.sh` が **冪等**（2回実行してもエラーにならず、状態が変わらない）
- [x] スクリプトが**プロジェクト ID と課金アカウントを引数か環境変数で受け取り**、ハードコードしていない
- [x] `docs/gcp-bootstrap.md` に、スクリプトで自動化できないコンソール操作の手順がある
- [x] `firebaseConfig` の値の置き場が決まっている（`frontend/.env.example` に
      `VITE_FIREBASE_*` のキーを列挙。**実値はコミットしない**）
- [x] `.env.example` に `ALLOWED_EMAILS` / `CLAUDE_MODEL` など必要な環境変数が列挙されている
- [x] **手順どおりに実際に実行し、動くことを確認した**（机上の手順書で終わらせない）

### コスト

- [x] この時点で**課金が発生していない**ことを確認した
      （プロジェクト作成・API 有効化・空バケットは無料。
      Firestore と Cloud Run は TICKET-004 以降で作る）

## サブチケット（コミット計画）

- [x] `feat(infra): GCP ブートストラップスクリプトを追加`
- [x] `feat(infra): tfstate バケットの作成をスクリプトに追加`
- [x] `docs(infra): コンソール操作（Firebase Auth・ウェブアプリ登録）の手順を追加`
- [x] `chore(repo): .env.example と frontend/.env.example を追加`
- [x] `docs(repo): ブートストラップの実行結果と取得した設定値の記録方法を追記`

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

- **プロジェクト ID は全世界で一意。** 実際に `octant` は取得できなかった。
  取れなかった場合は `octant-cissp-prod` などにフォールバックし、**CLAUDE.md の環境表を直す**
- **課金アカウントの紐付けを忘れると Cloud Run も Firestore も作れない。**
  API の有効化まではできてしまうため気づきにくい
- **Firestore はここで作らない。** コンソールで「Firestore を使ってみる」を押すと
  リージョン固定でデータベースができてしまい、Terraform 管理外になる
- Firebase の有効化は「既存の GCP プロジェクトを Firebase に追加する」方向で行う。
  Firebase コンソールから新規作成すると GCP プロジェクト ID が自動採番される

## 完了時メモ

### 確定した構成

| 環境 | プロジェクト ID | tfstate |
|---|---|---|
| prod | `octant-prod` | `gs://octant-prod-tfstate` |
| dev | `octant-dev` | `gs://octant-dev-tfstate` |

課金アカウント `013D28-C16A68-E3B1DA` / リージョン `asia-northeast1`。
**旧 `octant-cissp` には触れていない**（旧 octant が稼働中のため分離）。

### 実測で判明した落とし穴（すべて docs/gcp-bootstrap.md に記録済み）

**1. `gcloud projects describe` では ID の空き状況を判別できない。**
存在しない ID でも他者所有でも同じ `PERMISSION_DENIED` を返す（存在の有無を秘匿する仕様）。
一度「`octant-dev` は取得できない」と誤判断したが、実際は取得できた。
`octant` だけが取れず `octant-prod` にフォールバックしている。

**2. ブラウザ操作が必要なのは Google サインインの有効化1箇所だけだった。**
当初4項目をコンソール操作としていたが、3項目は firebase CLI で完結する
（`projects:addfirebase` / `apps:create` / `apps:sdkconfig`）。
Hosting サイトは Firebase 追加時に自動作成される。

**3. Identity Toolkit Admin API には `x-goog-user-project` ヘッダが必須。**
付けないと「quota project 未設定」の 403 になり、API 無効のエラーと紛らわしい。

**4. `gcloud auth application-default login` はクォータプロジェクトを
gcloud の既定プロジェクトに合わせる。** 既定が旧 `octant-cissp` のままだと
ADC もそちらを向く。既定とクォータの両方を `octant-dev` に付け替え済み。

### 意図的にやらなかったこと

- **Firestore データベースを作らない。** コンソールで作るとリージョンと PITR が
  Terraform の管理外になるため、TICKET-004 の Terraform が作る
- **サービスアカウントキーを発行しない。** ADC と WIF だけを使う

### TICKET-004 への申し送り

- ADC は設定済み。**初回の apply は手元の ADC で行う**（WIF を作るのが Terraform 自身のため）
- プロバイダに `billing_project` と `user_project_override` を設定し、
  ADC のクォータプロジェクト設定に依存しない形にすること
- この時点で**課金は発生していない**（プロジェクト・API・空バケットは無料）
