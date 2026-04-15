# nippo

Claude Code のセッションログ（`~/.claude/projects/**/*.jsonl`）から、
機械的な事実と ALACT モデルの振り返り用テンプレートを組み合わせた
Markdown を生成する Go 製 CLI。

[nwiizo/nippo](https://github.com/nwiizo/nippo) の Rust collector にインスパイアされ、
**「振り返り（reflection）1用途に絞った最小版」** として書き直したもの。
Claude Code の skill 層は持たず、バイナリ単体で動く。

## インストール

```bash
cd argentina/nippo
go build -o nippo .
```

## 使い方

```bash
# 今日の振り返り（デフォルトは --days=1）
./nippo

# 過去7日
./nippo --days 7

# プロジェクト名で絞り込み（cwd 末尾に対する部分一致）
./nippo --days 7 --project dotfiles

# 名前付き期間
./nippo --period last-week

# 明示的な範囲
./nippo --from 2026-03-01 --to 2026-03-15

# JSON で出力（別ツールに流し込む用）
./nippo --days 7 --format json | jq '.stats.tool_frequency'
```

## 出力内容

Markdown モードは次のセクションで構成される。

1. **事実（機械生成）** — セッション数・メッセージ・ツール呼び出し・トークン・期間
2. **プロジェクト別テーブル**
3. **よく使ったツール top 10**
4. **時間帯ヒートマップ**（ローカル時刻、ASCII 棒グラフ）
5. **意思決定の気配** — 「〜を使う」「instead」などシグナル語を含む発言の抜粋
6. **印象的なプロンプト** — プロジェクト別 top 5
7. **振り返りの問い（ALACT モデル）** — 空欄つきの5つの問い（自分で書く）

## 設計メモ

- 依存は `golang.org/x/sync/errgroup` のみ。JSON は stdlib の `encoding/json`。
- 2パスデシリアライズ: 1パス目で `type` + `timestamp` だけ読んでフィルタし、
  通過した行のみを完全 unmarshal する。
- ファイル単位で `runtime.NumCPU()` 並列読み。
- mtime プレフィルタ: ファイル最終更新が期間開始より古ければ開かない。
- `~/.claude/projects` が存在しない場合は親切な日本語エラーで終了する。

## テスト

```bash
go test ./... -cover
```

全パッケージ80%以上のカバレッジを保つこと。

## 本家との差分

本家 nippo は Rust collector + Claude Code skill（`/nippo` `/nippo reflection` `/nippo insight` ほか8コマンド）の2層構成だが、この Go 版はあえて「振り返り専用の1コマンド」に絞っている。日報・評価面談・長期トレンドなどの用途は本家を推奨。
