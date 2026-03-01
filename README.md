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
hb config init
```

対話形式で Hatena ID・Blog ID・API キーを入力します。`~/.config/hb/config.yaml` に保存されます。

フラグで一括指定することもできます:

```sh
hb config init --hatena-id YOUR_ID --blog-id YOUR_BLOG.hateblo.jp --api-key YOUR_KEY
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
hb pull [--force] [--dir <directory>]
```

- `--force`: 既存ファイルを上書き（デフォルトは editUrl が一致するファイルをスキップ）
- `--dir`: 保存先ディレクトリ（デフォルト: カレントディレクトリ）

### `hb fetch <file>`

指定したローカルファイルのリモート最新版を取得します。差分を表示してから上書き確認します。

```sh
hb fetch 20260301_my-article.md
```

### `hb push <file>`

ローカルファイルをリモートへ送信します。

- `editUrl` が frontmatter に**ない**場合 → 新規投稿（POST）
- `editUrl` が frontmatter に**ある**場合 → 差分があれば更新（PUT）

```sh
hb push 20260301_my-article.md
```

### `hb diff <file>`

ローカルファイルとリモートのunified diffを表示します。

```sh
hb diff 20260301_my-article.md
```

### `hb config init`

グローバル設定を初期化します。

### `hb init`

プロジェクトローカル設定を初期化します。

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

## ファイル名規則

```
[draft_]<YYYYmmdd>_<title>.md
```

例: `20260301_My Article Title.md` / `draft_20260301_未公開記事.md`

## 設定ファイルの優先度

グローバル設定（`~/.config/hb/config.yaml`）をベースとして、プロジェクト設定（`.hb/config.yaml`）の非空フィールドでオーバーライドします。

## 開発

```sh
make test   # 全テスト実行
make build  # バイナリビルド
make lint   # go vet 実行
```
