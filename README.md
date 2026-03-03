# hb — Hatena Blog Management CLI

はてなブログの記事をローカルのMarkdownファイルとして管理し、AtomPub APIで同期するCLIツールです。

## インストール

```sh
go install github.com/hirano00o/hb/cmd/hb@latest
```

またはリポジトリをクローンしてビルドします:

```sh
git clone https://github.com/hirano00o/hb
cd hb
make build
```

## 初期設定

### グローバル設定（初回のみ）

```sh
hb config init -g
```

対話形式で Hatena ID・Blog ID・API キーを入力します。API キー入力時はターミナル上でマスキングされます。
設定は `~/.config/hb/config.yaml` に保存されます。

フラグで Hatena ID と Blog ID を指定することもできます（API キーは必ずマスキングされた対話入力):

```sh
hb config init -g --hatena-id YOUR_ID --blog-id YOUR_BLOG.hateblo.jp
```

### プロジェクト設定（任意）

グローバル設定の値をプロジェクトごとに上書きしたい場合は、プロジェクトルートで実行します:

```sh
hb config init
```

対話形式で各フィールドを入力します。空 Enter でスキップしたフィールドはファイルに書き込まれず、グローバル設定が使われます。
設定は `.hb/config.yaml` に保存されます。フラグでも指定できます:

```sh
hb config init --hatena-id YOUR_ID --blog-id YOUR_BLOG.hateblo.jp
```

### 環境変数による設定オーバーライド

コンフィグファイルよりも環境変数が優先されます。CI/CD などでの利用に便利です。

| 環境変数 | 説明 |
|---|---|
| `HB_HATENA_ID` | Hatena ID |
| `HB_BLOG_ID` | Blog ID |
| `HB_API_KEY` | API キー |
| `HB_CONCURRENCY` | pull の並列実行数（デフォルト: 5） |
| `HB_MAX_PAGES` | pull のページ取得上限（デフォルト: 0 = 無制限） |

```sh
export HB_API_KEY=your_api_key
hb push article.md
```

### プロジェクト設定（オプション）

特定ディレクトリ配下でグローバル設定を上書きしたい場合:

```sh
cd ~/blog-dir
hb init
```

`.hb/config.yaml` が作成されます。空欄のフィールドはグローバル設定が使用されます。

## コマンドリファレンス

### `hb pull`

リモートの全記事をローカルのMarkdownファイルとして取得します。

```sh
hb pull [--force|-f] [--dir <directory>] [--from <date>] [--to <date>]
```

- `--force` / `-f`: ファイル名が衝突したときに確認なしで自動リネーム（連番suffix付与: `_1`, `_2`, …）
- `--dir`: 保存先ディレクトリ（デフォルト: カレントディレクトリ）
- `--from`: 指定日以降に投稿された記事のみ取得（形式: `YYYY-mm-dd`, `YYYY/mm/dd`, `YYYYmmdd`）
- `--to`: 指定日以前に投稿された記事のみ取得（形式: `YYYY-mm-dd`, `YYYY/mm/dd`, `YYYYmmdd`）

ファイル名が既存ファイルと衝突した場合（`--force` なし）:
1. **カスタム名**を入力 → そのファイル名で保存
2. **Enter（空入力）** → 連番suffix（`_1`, `_2`, …）を自動付与してリネーム
3. **`s`** → この記事のダウンロードをスキップ

既に `editUrl` が一致するローカルファイルがある記事は自動的にスキップされます。

### `hb fetch <file>`

指定したローカルファイルのリモート最新版を取得します。差分を表示してから上書き確認します。

```sh
hb fetch 20260301_my-article.md
```

### `hb push <file>`

ローカルファイルをリモートへ送信します。

- `editUrl` が frontmatter に**ない**場合 → 新規投稿（POST）
- `editUrl` が frontmatter に**ある**場合 → 差分表示後に確認してから更新（PUT）

```sh
hb push [--yes|-y] [--draft] <file>
```

- `--yes` / `-y`: 確認プロンプトをスキップ
- `--draft`: 下書きとして投稿。frontmatter の `draft` 値と異なる場合は確認プロンプトを表示

```sh
# 差分を確認しながらpush
hb push 20260301_my-article.md

# 確認なしでpush
hb push --yes 20260301_my-article.md

# 下書きとして強制push（確認あり）
hb push --draft 20260301_my-article.md
```

### `hb diff <file>`

ローカルファイルとリモートのunified diffを表示します。

```sh
hb diff 20260301_my-article.md
```

### `hb config init`

プロジェクトローカル設定を対話形式で初期化します。空 Enter でスキップしたフィールドはファイルに書き込まれません。

`-g` フラグでグローバル設定を初期化します（全フィールドの入力が必須）。

## フロントマター仕様

```yaml
---
title: 記事タイトル
date: 2026-03-01T12:00:00+09:00
draft: false
category:
  - Go
  - CLI
url: https://example.hateblo.jp/entry/2026/03/01/120000
editUrl: https://blog.hatena.ne.jp/user/example.hateblo.jp/atom/entry/123456
customUrlPath: my-custom-path
scheduledAt: 2026-04-01T12:00:00+09:00
---
```

| フィールド | 説明 |
|---|---|
| `title` | 記事タイトル |
| `date` | 投稿日時（RFC3339形式） |
| `draft` | 下書きフラグ |
| `category` | カテゴリ（複数可） |
| `url` | 公開URL（pullで自動設定） |
| `editUrl` | AtomPub編集URL（pullで自動設定） |
| `customUrlPath` | カスタムURLパス（オプション） |
| `scheduledAt` | 予約投稿日時（RFC3339形式、オプション）。設定すると `draft` が `false` でも下書きとして投稿される |

## ファイル名規則

```
[draft_]<YYYYmmdd>_<title>.md
```

例: `20260301_My Article Title.md` / `draft_20260301_未公開記事.md`

## 設定ファイルの優先度

環境変数 > プロジェクト設定（`.hb/config.yaml`）> グローバル設定（`~/.config/hb/config.yaml`）

### 設定ファイルの例

```yaml
hatena_id: yourhatenaId
blog_id: yourblog.hateblo.jp
api_key: your_api_key
concurrency: 10  # pull の並列実行数（デフォルト: 5）
max_pages: 5     # pull のページ取得上限（デフォルト: 0 = 無制限）
```

## 開発

```sh
make test   # 全テスト実行
make build  # バイナリビルド
make lint   # go vet 実行
```
