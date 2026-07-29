# TICKET-006: ノート CRUD API

## 基本情報

| 項目 | 内容 |
|---|---|
| チケットID | TICKET-006 |
| ステータス | 🔴 未着手 |
| 作成日 | 2026-07-27 |
| 着手日 | - |
| 完了日 | - |
| ブランチ名 | - |
| PR番号 | - |
| PRリンク | - |

## 概要

ノート（問題ノート / 知識ノート）の作成・取得・一覧・更新・削除 API を実装する。
Firestore の `/notes/{noteId}` に 1ノート = 1ドキュメントで保存する。

## 背景・目的

アプリの土台となるエンティティ。以降のドリル・AI・統計はすべてノートに依存する。
クリーンアーキテクチャの各層をこのチケットで一巡させ、以降が踏襲すべきパターンを確立する。

## 受け入れ条件

- [ ] `api/openapi.yaml` にノートのパスとスキーマが定義され、`make gen` の生成物がコミットされている
- [ ] `domain/entity/note.go` に Note エンティティとバリデーションルールがある（標準ライブラリのみ import）
- [ ] `domain/repository/` に NoteRepository インターフェースがある（Firestore の型を露出しない）
- [ ] `infrastructure/firestore/note_repository.go` に実装がある
- [ ] 一覧が**カーソルページネーション**（`limit` / `cursor` → `items` / `next_cursor`）で動く
- [ ] `type` での絞り込みと `updatedAt` 降順のソートができる
- [ ] **`type:"k"`（知識ノート）で保存すると `domain` が `null` に落とされる**
- [ ] `domain` が 1〜8 の範囲外、`type` が `q`/`k` 以外の場合に 400 を返す
- [ ] 壊れた `cursor` を渡すと 500 ではなく **400** を返す
- [ ] 存在しない ID で 404、エラー時も `{data:null, error:{code,message}}` 形式
- [ ] 作成時に 201 と `Location` ヘッダを返す
- [ ] `authorUid` が **context の検証済み uid** から設定される（リクエストの値を使わない）
- [ ] ユースケース層のテーブル駆動ユニットテストがある
- [ ] Firestore リポジトリの統合テスト（エミュレータ）がある
- [ ] 使用するクエリに対応する複合インデックスが `firebase/firestore.indexes.json` にあり、
      統合テストでそのクエリが1度は実行される

## サブチケット（コミット計画）

- [ ] `feat(api): ノートのエンドポイントとスキーマを openapi.yaml に追加`
- [ ] `feat(backend): Note エンティティとドメインルールを追加`
- [ ] `feat(backend): NoteRepository インターフェースを追加`
- [ ] `feat(backend): Firestore の NoteRepository 実装を追加`
- [ ] `feat(backend): ノートのユースケースを追加`
- [ ] `feat(backend): ノートのハンドラと DTO 変換を追加`
- [ ] `feat(backend): カーソルページネーションの実装を追加`
- [ ] `test(backend): ノートのユニットテストと統合テストを追加`

## 関連情報

- CLAUDE.md「データモデル」「API 規約」「アーキテクチャ規約（Go）」
- エンティティ: `{ id, type:"q"|"k", title, domain, tags[], related[], body, choices[], answer, explanation, createdAt, updatedAt, authorUid }`
- **`domain` は問題ノートだけが持つ**（1〜8 = CISSPの試験ドメイン）。知識ノートは常に `null`
- 同時編集はノート単位で後勝ち
- **`Offset()` を使わない**（読み飛ばした分も課金される）

### TICKET-003 からの申し送り: Firestore エミュレータの前提

**このチケットで初めて Firestore エミュレータを実際に使う。**

- ローカル環境には **Java 25（`default-jre-headless`）が導入済み**（2026-07-29）。
  `make dev` が Auth（9099）・Firestore（8808）・API（8080）・Vite（5173）の
  4つを起動し、Ctrl-C で4つとも停止することを実測確認済み
- `make test-integration` は現在 **`--only auth`**。
  **Firestore を使うテストを足す際は `scripts/test-integration.sh` を
  `--only auth,firestore` に変更し、`FIRESTORE_EMULATOR_HOST` を渡すこと**
- **CI に JRE のセットアップを追加する必要がある。**
  現在のワークフローは Auth エミュレータしか使わないため Java を入れていない。
  `--only auth,firestore` にした時点で CI が落ちるので、
  `actions/setup-java` を同じ PR で追加すること

### TICKET-002 の QA からの申し送り（このチケットで必ず対応する）

**1. `openapi.yaml` の制約は実行時に一切効かない。バリデーションは自前で書く。**

oapi-codegen の chi-server はバリデータを組み込まないため、`minimum` / `maximum` /
`maxLength` / `pattern` / `additionalProperties: false` はすべて**ドキュメント上の宣言に留まる**。

**特に `limit` の `default: 50` が効かず、未指定時はゼロ値 0 になる。**
`0` をそのまま Firestore の `Limit()` に渡すと「全件取得」として振る舞う実装があり得るため、
**最も素朴なリクエストが最も重くなる**。読み取り課金なので費用に直結する。

追加すべき受け入れ条件:

- [ ] `limit` 未指定で 50 件が返る（ゼロ値 0 のまま Firestore に渡らないこと）
- [ ] `limit=0` / `limit=-1` / `limit=101` が 400 になる
- [ ] `cursor` が 512 文字を超えると 400 になる
- [ ] 上記の検証が**一箇所**（共通のパラメータ解釈関数）で行われ、ハンドラ個別実装になっていない

**2. カーソルはクライアントから改竄できる入力として扱う。**

- 内部識別子（uid・Firestore のドキュメントパス）をカーソルに含めない。
  カーソルは URL のクエリに載り、ブラウザ履歴・Referer・アクセスログに残る
- リポジトリ層は**コレクションのルートを常に検証済み uid から組み立て**、
  カーソルから復元しない（復元すると IDOR が成立する）
- 復号・検証に失敗したカーソルは 400 で拒否し、先頭にフォールバックしない

**3. `CursorPage` スキーマを直接 `$ref` しない。**

`items` の要素型を差し替えられないため、`$ref` すると Go に `[]interface{}`、
TS に `unknown[]` が残る。`NotePage` のように `items` を具体型にした
専用スキーマを定義すること。

**4. 業務 API の HEAD は 405 のままでよいか、`GET /notes` 追加時に再確認する。**

TICKET-002 では `/health` だけ HEAD を登録した。RFC 9110 §9.1 は
GET を実装するなら HEAD も実装することを MUST としているため、方針の再確認が要る。
