# hb — Hatena Blog Management CLI

はてなブログの記事をローカルのMarkdownファイルとして管理し、AtomPub APIで同期するCLIツールです。

## 目次
- [インストール](#インストール)
- [初期設定](#初期設定)
- [設定](#設定)
- [グローバルフラグ](#グローバルフラグ)
- [コマンドリファレンス](#コマンドリファレンス)
- [フロントマター仕様](#フロントマター仕様)
- [ファイル名規則](#ファイル名規則)
- [開発](#開発)

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

### Nix を使う場合

```sh
# ビルドして ./result/bin/hb に配置
nix build github:hirano00o/hb

# インストールせず直接実行
nix run github:hirano00o/hb -- --help
```

## 初期設定

### グローバル設定（初回のみ）

```sh
hb config init -g
```

対話形式で Hatena ID・Blog ID・API キーを入力します。API キー入力時はターミナル上でマスキングされます。
設定は `~/.config/hb/config.yaml` に保存されます。

### プロジェクト設定（任意）

グローバル設定の値をプロジェクトごとに上書きしたい場合は、プロジェクトルートで実行します:

```sh
hb config init
```

対話形式で各フィールドを入力します。空 Enter でスキップしたフィールドはファイルに書き込まれず、グローバル設定が使われます。
設定は `.hb/config.yaml` に保存されます。

## 設定

### 環境変数による設定オーバーライド

コンフィグファイルよりも環境変数が優先されます。CI/CD などでの利用に便利です。

| 環境変数 | 説明 |
|---|---|
| `HB_HATENA_ID` | Hatena ID |
| `HB_BLOG_ID` | Blog ID |
| `HB_API_KEY` | API キー |
| `HB_CONCURRENCY` | pull の並列実行数（デフォルト: 5） |
| `HB_MAX_PAGES` | pull のページ取得上限（デフォルト: 0 = 無制限） |

### 設定ファイルの優先度

環境変数 > プロジェクト設定（`.hb/config.yaml`）> グローバル設定（`~/.config/hb/config.yaml`）

### 設定ファイルの例

```yaml
hatena_id: yourhatenaId
blog_id: yourblog.hateblo.jp
api_key: your_api_key
concurrency: 10  # pull の並列実行数（デフォルト: 5）
max_pages: 5     # pull のページ取得上限（デフォルト: 0 = 無制限）
```

### `hb config init`

プロジェクトローカル設定を対話形式で初期化します。空 Enter でスキップしたフィールドはファイルに書き込まれません。

`-g` フラグでグローバル設定を初期化します（全フィールドの入力が必須）。

### `hb config show`

現在の有効な設定値を表示します（グローバル設定 → プロジェクト設定 → 環境変数の優先順でマージ済みの値）。

```sh
hb config show
```

API キーは末尾4文字のみ表示され、残りは `*` でマスクされます。未設定のフィールドはデフォルト値とともに表示されます。

```
hatena_id: yourid
blog_id:   yourblog.hateblo.jp
api_key:   *********1234
concurrency: 5 (default)
max_pages: unlimited
```

## グローバルフラグ

全サブコマンドで使用できるフラグです。

| フラグ | 説明 |
|---|---|
| `--version` / `-v` | バージョン情報を表示（例: `hb version v0.1.0`。ローカルビルドは `dev`） |
| `--verbose` | スキップされたファイルの警告を詳細表示する。省略時は読み取りエラーのみ件数サマリーを表示 |

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

ファイル名衝突時（`--force` なし）は、カスタム名入力 / Enter（連番suffix自動付与）/ `s`（スキップ）から選択します。

既に `editUrl` が一致するローカルファイルがある記事は自動的にスキップされます。

`.` 始まりのディレクトリ（`.git`、`.hb` 等）はローカルファイル収集の対象から除外されます。

### `hb sync <file>`

指定したローカルファイルのリモート最新版を取得します。差分を表示してから上書き確認します。

```sh
hb sync [--yes|-y] <file>
```

- `--yes` / `-y`: 確認プロンプトをスキップして上書き

```sh
hb sync 20260301_my-article.md
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
hb push 20260301_my-article.md
```

### `hb publish`

指定した記事を公開状態にします。`draft` を `false` に設定し、ファイル名の `draft_` プレフィックスを除去します。

```sh
hb publish <file> [--push|-p]
```

- `--push` / `-p`: 変更後にリモートへ即座に反映

### `hb unpublish`

指定した記事を下書き状態に戻します。`draft` を `true` に設定し、ファイル名に `draft_` プレフィックスを付与します。

```sh
hb unpublish <file> [--push|-p]
```

- `--push` / `-p`: 変更後にリモートへ即座に反映

### `hb rename`

記事のタイトルを変更し、frontmatter の `title` を更新するとともに、ファイル名を新しいタイトルに合わせてリネームします。

```sh
hb rename <file> --title <title> [--force]
```

- `--title`: 新しい記事タイトル（必須）
- `--force`: リネーム先ファイルが既に存在する場合でも上書き

### `hb diff <file>`

ローカルファイルとリモートのunified diffを表示します。

```sh
hb diff 20260301_my-article.md
```

> **注意**: ローカル画像参照（`![alt](photo.jpg)` のように `http`/`https` で始まらないパス）を含む記事では、`hb push` でアップロードが完了するまで画像行に差分が表示されます。このとき stderr に `note: this file contains local images; ...` が出力されます。

### `hb new --title <title>`

フロントマター付きのMarkdownファイルをカレントディレクトリに新規作成します。

```sh
hb new --title|-t <title> [--draft] [--push|-p] [--body|-b <body>]
```

- `--title` / `-t`: 記事タイトル（必須）
- `--draft`: 下書きとして作成。ファイル名に `draft_` プレフィックスを付与し、frontmatter に `draft: true` を設定
- `--push` / `-p`: ファイル作成後にリモートへ新規投稿（POST）。`editUrl`・`url`・`date` をローカルに書き戻し
- `--body` / `-b`: 作成するファイルの本文を指定。省略時は stdin がパイプなら自動で読み込む（`\n` は変換しない）

同名ファイルが既に存在する場合はエラーで中断します。先にファイルをリネームしてから再実行してください。

```sh
# タイトルを指定してファイルを作成
hb new -t "はじめての記事"
# → 20260306_はじめての記事.md を作成

# 作成と同時にリモートへ投稿
hb new --push -t "公開記事"
```

### `hb list`

ローカルの記事一覧をテーブル形式で表示します。日付の降順（新しい記事が上）でソートされます。

```sh
hb list [--dir <directory>] [--draft] [--published] [--category <name>] [--categories]
```

- `--dir`: スキャン先ディレクトリ（デフォルト: カレントディレクトリ）
- `--draft`: 下書きのみ表示
- `--published`: 公開記事のみ表示
- `--category <name>`: 指定カテゴリの記事のみ表示
- `--categories`: 全カテゴリを一覧表示

`--draft` と `--published` は同時に指定できません。`--categories` は `--draft`、`--published`、`--category` と同時に指定できません。

`.` 始まりのディレクトリ（`.git`、`.hb` 等）は走査対象から除外されます。フロントマターのないファイルはサイレントにスキップされます。読み取りに失敗したファイルはデフォルトで件数サマリーを stderr に表示し、`--verbose` で詳細表示できます。

### `hb search`

ローカルの記事をキーワード検索します。大文字小文字を区別しません。

```sh
hb search <query> [--dir <directory>] [--title] [--body]
```

- `--dir`: スキャン先ディレクトリ（デフォルト: カレントディレクトリ）
- `--title`: タイトルのみ検索
- `--body`: 本文のみ検索

フラグなしの場合はタイトルと本文のOR検索を行います。`--title` と `--body` を両方指定するとAND検索（タイトルと本文の両方に一致）になります。

### `hb open <file>`

指定したローカルファイルの公開URLをデフォルトブラウザで開きます。

```sh
hb open [--edit|-e] <file>
```

- `--edit` / `-e`: はてなブログの編集ページ（`https://blog.hatena.ne.jp/{user}/{blog}/edit?entry={id}`）を開く

```sh
hb open 20260301_my-article.md
```

frontmatter に `url`（または `--edit` 時は `editUrl`）が設定されていない（未公開の）記事ではエラーになります。

### `hb status`

ローカル記事とリモートの同期状態を表示します。

```sh
hb status [--dir <directory>]
```

- `--dir`: スキャン先ディレクトリ（デフォルト: カレントディレクトリ）

`.` 始まりのディレクトリ（`.git`、`.hb` 等）は走査対象から除外されます。読み取りに失敗したファイルは `hb list` と同様に `--verbose` で詳細表示できます。

各ローカルファイルを以下の 3 つに分類して表示します。

- **Modified**: `editUrl` があり、ローカルとリモートの内容が異なる
- **Untracked**: `editUrl` がない（未公開記事）
- **Up to date**: ローカルとリモートが一致している

> **注意**: ローカル画像参照（`![alt](photo.jpg)` のように `http`/`https` で始まらないパス）を含む記事は、`hb push` でアップロードが完了するまで **Modified** と表示される場合があります。このとき stderr に `note: <file> contains local images; ...` が出力されます。

```
Modified (2):
  20260301_Article1.md
  ...

Untracked (1):
  draft_NewPost.md

Up to date (3):
  20260201_OldPost.md
  ...
```

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

## 開発

```sh
make test   # 全テスト実行
make build  # バイナリビルド（VERSION は git describe --tags から自動設定）
make lint   # go vet 実行
```

Nix を使う場合は、go / gopls / golangci-lint が揃った開発シェルを起動できます:

```sh
nix develop
```
