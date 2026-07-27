---
name: ticket
description: tickets/ 配下のチケットライフサイクル操作（起票・着手・PR記入・完了）の標準手順。チケットの作成・移動・ステータス更新・採番を行うときは必ずこのスキルに従う。「チケットを起票して」「TICKET-XXX に着手」「チケットを完了にして」などで使用。Dev Team / Design Team オーケストレーターからも参照される。
---

# チケット管理スキル

`tickets/` 配下のチケット操作の標準手順。**チケット = PR単位、サブチケット = コミット単位**（詳細は CLAUDE.md「ブランチ・チケット・コミット」参照）。

## 共通ルール

- ステータスは配置ディレクトリで表現する: `tickets/backlog/` → `tickets/in-progress/` → `tickets/done/`
- ファイル命名: `TICKET-{連番3桁}_{概要}.md`
- ステータス表記: 🔴 未着手 / 🟡 作業中 / 🟢 完了
- ファイルの移動は必ず `git mv` で行う（履歴を保持するため）
- 日付は `YYYY-MM-DD` 形式

## 1. 起票

1. **採番**: `ls tickets/backlog tickets/in-progress tickets/done` で全ディレクトリを確認し、既存の最大番号 + 1 を採る（番号空間は3ディレクトリで共有）
2. `tickets/TICKET_TEMPLATE.md` をベースに `tickets/backlog/TICKET-{連番3桁}_{概要}.md` を作成する
3. 基本情報（チケットID・ステータス 🔴 未着手・作成日）、概要、背景・目的、受け入れ条件、サブチケット（`<type>(<scope>): 件名` 形式のコミット計画）を埋める
4. 未確定の項目（着手日・完了日・PR番号など）は `-` または「（PR作成後に記入）」のままにする

## 2. 着手

1. `git mv` で `tickets/backlog/TICKET-XXX_*.md` → `tickets/in-progress/` に移動する
2. ファイル内を更新する: ステータス → 🟡 作業中、着手日 → 今日の日付
3. `git checkout -b feature/TICKET-XXX` でブランチを作成し（develop を最新化してから切る）、ファイルの「ブランチ名」に記入する
4. `git add` → `git commit -m "chore(TICKET-XXX): 作業開始"` でコミットする

## 3. 作業中

- サブチケットを1つ消化（=1コミット）するたびに、チケットのチェックボックスを ON にする（対応する実装コミットに含めてよい）

## 4. PR記入

- PR作成後、チケットの「PR番号」（`#xx`）と「PRリンク」を記入し、`docs(TICKET-XXX): PR情報を記入` 等でコミット・プッシュする
- PRの向き先は `develop`（CLAUDE.md ブランチ戦略参照）

## 5. マージ前のゲート（承認の代替）

このプロジェクトは1人体制で承認者がいないため、**人間のレビュー承認の代わりに QA Team の判定をゲートにする**。

1. PR作成後、**QA Team を起動する**（「品質チェックして」）
2. 判定が **READY** または **CONDITIONAL** であればマージしてよい（**セルフマージ可**）
   - CONDITIONAL の場合は、残った指摘を「対応する」か「v2へ回す」かをユーザーが決めてからマージする
3. **NEEDS WORK のままマージしない。** Dev Team に修正を依頼し、再度 QA Team を通す
4. マージ判定の根拠（QA Team の最終判定）を PR コメントに残す

## 6. 完了（マージ後）

1. develop を pull してから新しいブランチ（例: `chore/close-ticket-XXX`）を切る
2. `git mv` で `tickets/in-progress/TICKET-XXX_*.md` → `tickets/done/` に移動する
3. ステータス → 🟢 完了、完了日 → 今日の日付に更新する
4. `chore(TICKET-XXX): マージ完了に伴いチケットを done へ移動` でコミットし、PR を作成する
