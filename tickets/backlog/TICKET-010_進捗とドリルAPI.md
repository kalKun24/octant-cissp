# TICKET-010: 進捗・ドリル API（SM-2 間隔反復）

## 基本情報

| 項目 | 内容 |
|---|---|
| チケットID | TICKET-010 |
| ステータス | 🔴 未着手 |
| 作成日 | 2026-07-27 |
| 着手日 | - |
| 完了日 | - |
| ブランチ名 | - |
| PR番号 | - |
| PRリンク | - |

## 概要

SM-2 の間隔反復アルゴリズムを実装し、解答の記録・復習期限の抽出・ドリルセッションの
記録 API を提供する。個人データのため uid で厳密に分離する。

## 背景・目的

学習を継続させる中核。SM-2 は純粋な計算であり、`domain/entity` に純粋関数として置くことで
外部依存なしにテストできる。また個人データを扱う最初のチケットであり、
**uid の取り扱いパターン**をここで確立する。

## 受け入れ条件

- [ ] SM-2 の計算が `domain/entity` の**純粋関数**として実装され、標準ライブラリ以外を import していない
- [ ] SM-2 のテーブル駆動ユニットテストがある（正解 / 不正解、`ef` の下限、初回・2回目・n回目）
- [ ] 解答を記録すると `/users/{uid}/progress/{noteId}` が更新される
      （`attempts` / `correct` / `ef` / `interval` / `reps` / `due` / `history`）
- [ ] 復習期限のノート（`due <= now`）を取得できる
- [ ] ドリルセッションの記録が `/users/{uid}/sessions/{sessionId}` に保存される
- [ ] 日別集計が `/users/{uid}/days/{YYYY-MM-DD}` に更新される
- [ ] **すべての個人データのパスが context の検証済み uid から組み立てられている**
      （リクエストボディ・パスパラメータの uid を信用していない）
- [ ] **他ユーザーの uid を指定しても他人のデータにアクセスできない**ことを統合テストで確認している
- [ ] ドリル対象の抽出（全件 / 期限のみ / タグ絞り込み / 苦手順 / ランダム）が API で選べる
- [ ] 対応するインデックスが `firestore.indexes.json` にあり、統合テストでクエリが実行される

## サブチケット（コミット計画）

- [ ] `feat(api): 進捗・ドリル・セッションのエンドポイントを openapi.yaml に追加`
- [ ] `feat(backend): SM-2 の計算を domain の純粋関数として追加`
- [ ] `feat(backend): Progress / Session のエンティティとリポジトリを追加`
- [ ] `feat(backend): 解答記録と復習期限抽出のユースケースを追加`
- [ ] `feat(backend): 日別集計の更新を追加`
- [ ] `feat(backend): 進捗・ドリルのハンドラを追加`
- [ ] `test(backend): SM-2 のユニットテストと uid 分離の統合テストを追加`

## 関連情報

- CLAUDE.md「データモデル」「認証・認可」
- 進捗: `{ noteId, attempts, correct, ef, interval, reps, due, history[] }`
- 旧実装の SM-2: `../octant/artifact/octant-cissp.html` の `sm2()` / `recordAnswer()`
  （**コードはコピーせず仕様のみ参照**）
- 復習期限の抽出（`due <= now` で絞り `due` 昇順）は単一フィールドインデックスで足りる
