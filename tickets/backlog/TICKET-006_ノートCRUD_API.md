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
