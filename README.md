# setay-go

[![CI](https://github.com/aki-kuramoto/setay-go/actions/workflows/ci.yml/badge.svg)](https://github.com/aki-kuramoto/setay-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/aki-kuramoto/setay-go.svg)](https://pkg.go.dev/github.com/aki-kuramoto/setay-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE.md)

> **Note:** The second half of this document is available in Japanese (日本語).

A Go library and CLI tool for the **setay** structured data format.

Setay is a human-friendly data serialization format designed for configuration files and structured data storage. It features support for dicts (key–value maps), lists, strings, numbers, booleans, null, UTC timestamps, and flexible comment syntax — all wrapped in a clean, readable notation.

## Features

- **`encoding/json`-style API** — `Marshal`, `Unmarshal`, `MarshalFile`, `UnmarshalFile`
- **Struct tags** — `setay:"name,omitempty"`, `setay:"-"`
- **Rich data types** — strings, integers (decimal/hex/binary/octal), floats, booleans, null, `UtcTs` timestamps
- **[wantai](https://github.com/aki-kuramoto/wantai) integration** — Native support for wantai's typed UTC timestamp wrappers
- **CLI formatter** — `setay fmt` to format `.setay` files in the standard style

## Installation

### Library

```bash
go get github.com/aki-kuramoto/setay-go
```

### CLI Tool

Download a prebuilt binary from the [Releases](https://github.com/aki-kuramoto/setay-go/releases) page — no Go installation required.

Or build from source:

```bash
go install github.com/aki-kuramoto/setay-go/cmd/setay@latest
```

## Quick Start

### Marshaling (Go → Setay)

```go
package main

import (
    "fmt"
    setay "github.com/aki-kuramoto/setay-go"
)

type Config struct {
    Name     string   `setay:"name"`
    Port     int      `setay:"port"`
    Debug    bool     `setay:"debug,omitempty"`
    Tags     []string `setay:"tags"`
}

func main() {
    cfg := Config{
        Name: "my-app",
        Port: 8080,
        Tags: []string{"web", "api"},
    }
    data, err := setay.Marshal(cfg)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(data))
}
```

Output:

```setay
{
	name = "my-app";
	port = 8080;
	tags = [ "web", "api" ];
}
```

### Unmarshaling (Setay → Go)

```go
input := []byte(`{
	name = "my-app";
	port = 8080;
	tags = [ "web", "api" ]
}`)

var cfg Config
if err := setay.Unmarshal(input, &cfg); err != nil {
    panic(err)
}
fmt.Printf("%+v\n", cfg)
// {Name:my-app Port:8080 Debug:false Tags:[web api]}
```

### File I/O

```go
// Write
setay.MarshalFile("config.setay", cfg)

// Read
var loaded Config
setay.UnmarshalFile("config.setay", &loaded)
```
## Timestamps with [wantai](https://github.com/aki-kuramoto/wantai)

setay-go natively supports [wantai](https://github.com/aki-kuramoto/wantai)'s typed UTC timestamp wrappers. Use any wantai timestamp type (`UtcNanoTs`, `UtcMicroTs`, `UtcMilliTs`, `UtcSecTsS32`, etc.) in your struct fields — they will be automatically marshaled to/from `UtcTs("...")` in setay format.

```go
import (
    setay "github.com/aki-kuramoto/setay-go"
    "github.com/aki-kuramoto/wantai"
    "time"
)

type Event struct {
    Name      string           `setay:"name"`
    CreatedAt wantai.UtcNanoTs `setay:"created-at"`
    ExpiresAt wantai.UtcMilliTs `setay:"expires-at"`
}

func main() {
    ev := Event{
        Name:      "deployment",
        CreatedAt: wantai.FromTime(time.Now()),
        ExpiresAt: wantai.FromTimeMillis(time.Now().Add(24 * time.Hour)),
    }
    data, _ := setay.Marshal(ev)
    // UtcTs("2026-03-22 12:28:27") format

    var loaded Event
    setay.Unmarshal(data, &loaded)
    // loaded.CreatedAt.ToTime() returns time.Time
}
```

## CLI Tool

The `setay` CLI includes a formatter for `.setay` files:

```bash
# Format files (creates .bak backup by default)
setay fmt config.setay

# Format in-place without backup
setay fmt -w config.setay

# Format all .setay files in a directory recursively
setay fmt ./configs/
```

## The Setay Format

A brief overview of the format. See [`docs/setay-format-spec.md`](docs/setay-format-spec.md) for the full specification.

### Basic Structure

The top level of a setay document is always a dict `{ ... }`. Dicts hold key–value entries separated by `;`, and lists hold values separated by `,`.

```setay
{
	name = "John S.";
	age = 47;
	is-student = false;
	hobbies = [ "Anime", "Games", "Rock'n' roll" ];
	address =
	{
		city = "Shibuya";
		country = "Japan";
	};
}
```

### Value Types

| Type | Example |
|------|---------|
| Null | `null` |
| Boolean | `true`, `false` |
| Integer | `42`, `-1`, `0xFF`, `0b1010`, `0o77` |
| Float | `3.14`, `-0.5`, `1.5e10` |
| String | `"hello"`, `'world'` |
| UTC Timestamp | `UtcTs("2026-01-02 15:04:05")` |

### Comments

```setay
# Single-line comment (hash + space)
## Double-hash comment
#! Shebang / reserved expression
#{ Multi-line
   comment }#
```

## Documentation

- [Format Specification](docs/setay-format-spec.md) — Full setay format specification
- [Grammar Definition](docs/setay.bp) — PEG grammar (boompaw format)

## License

Copyright (c) 2026-present Akihiro Kuramoto. Setay-go is free and open-source software licensed under the [MIT License](LICENSE.md).

---

# setay-go (日本語)

Go 言語用の **setay** 構造化データ形式ライブラリおよび CLI ツールです。

setay は設定ファイルや構造化データの保存を目的に設計された、人間にとって読みやすいデータシリアライゼーション形式です。ディクト（キーと値のマップ）、リスト、文字列、数値、真偽値、null、UTC タイムスタンプ、柔軟なコメント記法をサポートしています。

## 機能

- **`encoding/json` スタイルの API** — `Marshal`, `Unmarshal`, `MarshalFile`, `UnmarshalFile`
- **構造体タグ** — `setay:"name,omitempty"`, `setay:"-"`
- **豊富なデータ型** — 文字列、整数（10進/16進/2進/8進）、浮動小数点、真偽値、null、`UtcTs` タイムスタンプ
- **[wantai](https://github.com/aki-kuramoto/wantai) 連携** — wantai の型付き UTC タイムスタンプラッパーをネイティブサポート
- **CLI フォーマッター** — `setay fmt` で `.setay` ファイルを標準スタイルに整形

## インストール

### ライブラリ

```bash
go get github.com/aki-kuramoto/setay-go
```

### CLI ツール

[Releases](https://github.com/aki-kuramoto/setay-go/releases) ページからビルド済みバイナリをダウンロードできます（Go のインストールは不要です）。

ソースからビルドする場合：

```bash
go install github.com/aki-kuramoto/setay-go/cmd/setay@latest
```

## クイックスタート

### マーシャリング（Go → Setay）

```go
package main

import (
    "fmt"
    setay "github.com/aki-kuramoto/setay-go"
)

type Config struct {
    Name     string   `setay:"name"`
    Port     int      `setay:"port"`
    Debug    bool     `setay:"debug,omitempty"`
    Tags     []string `setay:"tags"`
}

func main() {
    cfg := Config{
        Name: "my-app",
        Port: 8080,
        Tags: []string{"web", "api"},
    }
    data, err := setay.Marshal(cfg)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(data))
}
```

出力：

```setay
{
	name = "my-app";
	port = 8080;
	tags = [ "web", "api" ];
}
```

### アンマーシャリング（Setay → Go）

```go
input := []byte(`{
	name = "my-app";
	port = 8080;
	tags = [ "web", "api" ]
}`)

var cfg Config
if err := setay.Unmarshal(input, &cfg); err != nil {
    panic(err)
}
fmt.Printf("%+v\n", cfg)
// {Name:my-app Port:8080 Debug:false Tags:[web api]}
```

### ファイル入出力

```go
// 書き込み
setay.MarshalFile("config.setay", cfg)

// 読み込み
var loaded Config
setay.UnmarshalFile("config.setay", &loaded)
```

## タイムスタンプと [wantai](https://github.com/aki-kuramoto/wantai)

setay-go は [wantai](https://github.com/aki-kuramoto/wantai) の型付き UTC タイムスタンプラッパーをネイティブサポートしています。構造体フィールドに wantai のタイムスタンプ型（`UtcNanoTs`、`UtcMicroTs`、`UtcMilliTs`、`UtcSecTsS32` 等）を使用すると、setay 形式の `UtcTs("...")` との間で自動的に変換されます。

```go
import (
    setay "github.com/aki-kuramoto/setay-go"
    "github.com/aki-kuramoto/wantai"
    "time"
)

type Event struct {
    Name      string           `setay:"name"`
    CreatedAt wantai.UtcNanoTs `setay:"created-at"`
    ExpiresAt wantai.UtcMilliTs `setay:"expires-at"`
}

func main() {
    ev := Event{
        Name:      "deployment",
        CreatedAt: wantai.FromTime(time.Now()),
        ExpiresAt: wantai.FromTimeMillis(time.Now().Add(24 * time.Hour)),
    }
    data, _ := setay.Marshal(ev)
    // UtcTs("2026-03-22 12:28:27") の形式で出力

    var loaded Event
    setay.Unmarshal(data, &loaded)
    // loaded.CreatedAt.ToTime() で time.Time を取得可能
}
```

## CLI ツール

`setay` CLI には `.setay` ファイル用のフォーマッターが含まれています：

```bash
# ファイルをフォーマット（デフォルトで .bak バックアップを作成）
setay fmt config.setay

# バックアップなしでその場で上書きフォーマット
setay fmt -w config.setay

# ディレクトリ内の全 .setay ファイルを再帰的にフォーマット
setay fmt ./configs/
```

## setay 形式について

形式の詳細は [`docs/setay-format-spec.md`](docs/setay-format-spec.md) を参照してください。

### 基本構造

setay ドキュメントのトップレベルは常にディクト `{ ... }` です。ディクトはキーと値のエントリーを `;` で区切り、リストは値を `,` で区切ります。

```setay
{
	name = "John S.";
	age = 47;
	is-student = false;
	hobbies = [ "アニメ", "ゲーム", "Rock'n' roll" ];
	address =
	{
		city = "渋谷区";
		country = "Japan";
	};
}
```

### 値の型

| 型 | 例 |
|---|---|
| ナル | `null` |
| 真偽値 | `true`, `false` |
| 整数 | `42`, `-1`, `0xFF`, `0b1010`, `0o77` |
| 浮動小数点 | `3.14`, `-0.5`, `1.5e10` |
| 文字列 | `"hello"`, `'world'` |
| UTC タイムスタンプ | `UtcTs("2026-01-02 15:04:05")` |

### コメント

```setay
# 単行コメント（ハッシュ + 空白）
## ダブルハッシュコメント
#! Shebang / 予約表現
#{ 複数行
   コメント }#
```

## ドキュメント

- [形式仕様](docs/setay-format-spec.md) — setay 形式の完全な仕様書
- [文法定義](docs/setay.bp) — PEG 文法（boompaw 形式）

## ライセンス

Copyright (c) 2026-present Akihiro Kuramoto. Setay-go は [MIT ライセンス](LICENSE.md) のもとで公開されているフリーかつオープンソースなソフトウェアです。
