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
- **Strict decoding** — `Unmarshal(..., DisallowUnknownFields())` reports unknown keys (as an `*UnknownFieldsError`) instead of ignoring them
- **Rich data types** — strings, integers (decimal/hex/binary/octal), floats, booleans, null, `UtcTs` timestamps
- **Sets** — `map[K]struct{}` marshals/unmarshals as `{ key=; ... }` set syntax
- **Flag entries** — a value-less key `{ verbose; ... }` denotes the boolean `true` (present = true, absent = false)
- **Variable references** — `${VAR_NAME}` syntax resolves environment variables or custom resolver values at unmarshal time, with `?:` fallback chains and string interpolation
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

Output (`Marshal` writes single-quoted strings — setay's stable, non-interpolating string form):

```setay
{
	name = 'my-app';
	port = 8080;
	tags = [ 'web', 'api' ];
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

#### Rejecting unknown keys

By default, a dict key with no matching struct field is silently ignored, just
like `encoding/json`. Pass `DisallowUnknownFields()` to have unknown keys
reported instead. Every unknown key in the whole document is collected and
returned together as an `*UnknownFieldsError`, each as a dotted path from the
root:

```go
var cfg Config
err := setay.Unmarshal(input, &cfg, setay.DisallowUnknownFields())

var ufe *setay.UnknownFieldsError
if errors.As(err, &ufe) {
    // ufe.Keys == []string{"bogus", "server.tls.oops"}, in document order
    for _, key := range ufe.Keys {
        log.Printf("unknown key: %s", key)
    }
}
```

`UnmarshalFile` accepts the same option. This only affects struct targets — a
map target (e.g. `map[string]any`) has no notion of an unknown key.

### File I/O

```go
// Write
setay.MarshalFile("config.setay", cfg)

// Read
var loaded Config
setay.UnmarshalFile("config.setay", &loaded)
```

## Sets

Sets are represented in Go as `map[K]struct{}`. In setay, each element is written as `key =;` — the `=;` suffix is **atomic** (no space between `=` and `;`).

```go
type Firewall struct {
    AllowedPorts map[int]struct{}    `setay:"allowed-ports"`
    BlockedIPs   map[string]struct{} `setay:"blocked-ips"`
}

// Marshal
fw := Firewall{
    AllowedPorts: map[int]struct{}{80: {}, 443: {}, 8080: {}},
    BlockedIPs:   map[string]struct{}{},
}
data, _ := setay.Marshal(fw)
// Output:
// {
// 	allowed-ports =
// 	{
// 		80=;
// 		443=;
// 		8080=;
// 	};
// 	blocked-ips = {=;};
// }

// Unmarshal
input := `{
    allowed-ports = { 80=; 443=; };
    blocked-ips   = { "192.168.1.1"=; "10.0.0.5"=; };
}`
var loaded Firewall
setay.Unmarshal([]byte(input), &loaded)
```

**Rules:**
- `{=;}` is the empty set (distinct from the empty dict `{}`)
- A space before `=;` is allowed; no space inside `=;`
- Set keys must be scalar values (int, float, string, bool, null, timestamp, or variable reference)
- `map[K]struct{}` cannot be used as the top-level document

## Flag Entries

A dict entry may be written as a key alone, with the `= value` part omitted. This
is a **flag entry**, and it denotes the boolean `true`. A flag entry and
`key = true` are two spellings of the same value — handy for DSL-like configs
where presence itself is the meaning.

```go
type Options struct {
    Verbose bool `setay:"verbose"`
    Debug   bool `setay:"debug"`
    Quiet   bool `setay:"quiet"`
}

input := `{
    verbose;        # flag entry — same as verbose = true
    debug = true;   # the value spelled out
}`
var o Options
setay.Unmarshal([]byte(input), &o)
// o.Verbose == true, o.Debug == true, o.Quiet == false (absent)
```

**Rules:**
- A flag entry denotes `true`; an absent key leaves a `bool` field `false`
- Into a struct `bool` field, or a map value type of `bool` or `any`, a flag
  entry yields `true`; into any other target type it is an error
  - (The target must be `bool`: a `null`/pointer cannot tell "unspecified" apart
    from "explicitly false", which is the whole point of a flag)
- `Marshal` is intentionally lossy in reverse: a `true` is always written as
  `= true`, never collapsed to a flag entry. Round-trip editing (the `Document`
  API) preserves whichever spelling the author used, in both directions
- A flag entry (`{ key; }`) is **not** the Set type (`{ key=; }`), which is a
  separate value kind terminated by the atomic token `=;`

## Variable References

At unmarshal time, `${VAR_NAME}` is replaced with the value of the named variable. By default, variables are resolved from the process's environment (via `os.LookupEnv`). You can also register a custom resolver to source values from a database, secrets manager, or any other location.

### Syntax

```setay
{
	# Simple environment variable reference
	redis-host = ${REDIS_HOST};

	# With a fallback value (used when the variable is not set)
	db-port = ${MYSQL_PORT ?: 13306};

	# String interpolation (variable expanded inside a double-quoted string)
	greeting = "Hello, ${USER_NAME ?: 'World'}!";

	# Fallback chain — tries each variable in turn, then the literal
	password = ${PASS_PRIMARY ?: PASS_SECONDARY ?: "default-secret"};
}
```

**Key rules:**

- `${VAR_NAME}` — refers to a variable by name (unquoted identifier, same rules as an unquoted key)
- `?: value` — fallback if the variable is not defined; can chain: `?: A ?: B ?: "final"`
- Fallback values can be: another variable name, an integer/float literal, a quoted string
  > **Note:** A bare word after `?:` (e.g. `${VAR ?: fallback}`) is treated as a **variable name**,
  > not a string literal. Use quotes to specify a literal: `${VAR ?: "fallback"}` or `${VAR ?: 'fallback'}`.
- String interpolation works **only** inside double-quoted strings (`"..."`); single-quoted strings (`'...'`) are always literal
- Inside a double-quoted string, `$`, `{`, `}` are reserved (for interpolation) and must be escaped as `\$`, `\{`, `\}` to appear literally — a bare `$` is a parse error. In a single-quoted string they are written bare (e.g. `'price: $100'`). See the [format spec](docs/setay-format-spec.md) for the full escape table.
- Variable references are an **unmarshal-only** feature; `Marshal` always outputs concrete values

### Resolution Priority

1. **Custom resolver** (if registered and returns `ok=true`)
2. **Environment variable** (`os.LookupEnv`) — used when no resolver is registered, or resolver returns `ok=false`
3. **`?:` fallback value** — used when the variable is not found by either source
4. **Error** — returned if the variable is not found and no fallback is specified

### Custom Resolver

```go
import setay "github.com/aki-kuramoto/setay-go"

// Register a custom resolver (called before os.LookupEnv).
// Return ok=true to use result, ok=false to fall through to os.LookupEnv.
// Return a non-nil error to abort Unmarshal immediately.
setay.RegisterVariableResolver(func(name string) (result string, ok bool, err error) {
	result, ok = mySecretStore.Get(name)
	return result, ok, nil
})

// Reset to default (env-only) behavior:
setay.RegisterVariableResolver(nil)
```

### Type Coercion

Resolved variable values are always strings (from env or the resolver). setay automatically converts them to the target field's type:

| Target type | Conversion |
|---|---|
| `string` | Used as-is |
| `int`, `int64`, … | Parsed as an integer (supports `0x`, `0b`, `0o` prefixes) |
| `uint`, `uint32`, … | Parsed as an unsigned integer |
| `float32`, `float64` | Parsed as a floating-point number |
| `bool` | `"true"`, `"1"`, `"yes"` → `true`; `"false"`, `"0"`, `"no"`, `""` → `false` |
| `interface{}` | Stored as a `string` |
| `*T` | Pointer is allocated, then the value is coerced to `T` |

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
| Variable Reference | `${MY_VAR}`, `${PORT ?: 8080}` |

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
- [Regenerating the Parser](docs/regenerating-the-parser.md) — how the parser is generated, and the two generated copies that must be kept in sync

## License

Copyright (c) 2026-present Akihiro Kuramoto. Setay-go is free and open-source software licensed under the [MIT License](LICENSE.md).

---

# setay-go (日本語)

Go 言語用の **setay** 構造化データ形式ライブラリおよび CLI ツールです。

setay は設定ファイルや構造化データの保存を目的に設計された、人間にとって読みやすいデータシリアライゼーション形式です。ディクト（キーと値のマップ）、リスト、文字列、数値、真偽値、null、UTC タイムスタンプ、柔軟なコメント記法をサポートしています。

## 機能

- **`encoding/json` スタイルの API** — `Marshal`, `Unmarshal`, `MarshalFile`, `UnmarshalFile`
- **構造体タグ** — `setay:"name,omitempty"`, `setay:"-"`
- **厳格デコード** — `Unmarshal(..., DisallowUnknownFields())` で未知のキーを無視せず `*UnknownFieldsError` として報告
- **豊富なデータ型** — 文字列、整数（10進/16進/2進/8進）、浮動小数点、真偽値、null、`UtcTs` タイムスタンプ
- **セット** — `map[K]struct{}` を `{ key=; ... }` のセット記法でマーシャル/アンマーシャル
- **フラグエントリ** — 値を持たないキー `{ verbose; ... }` は真偽値 `true` を表す（存在 = true, 不在 = false）
- **変数参照** — `${VAR_NAME}` 構文でアンマーシャル時に環境変数やカスタムリゾルバーから値を解決。`?:` フォールバックチェーンと文字列補間に対応
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

出力 (`Marshal` はシングルクォート文字列を出力する -- setay の安定・非補間の文字列形式)：

```setay
{
	name = 'my-app';
	port = 8080;
	tags = [ 'web', 'api' ];
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

#### 未知のキーをエラーにする

既定では、対応する struct フィールドが無い dict キーは `encoding/json` と同様に黙って無視されます。`DisallowUnknownFields()` を渡すと、未知のキーをエラーとして報告します。文書全体の未知キーを全て集め、それぞれ root からのドット区切りパスとして 1 つの `*UnknownFieldsError` にまとめて返します:

```go
var cfg Config
err := setay.Unmarshal(input, &cfg, setay.DisallowUnknownFields())

var ufe *setay.UnknownFieldsError
if errors.As(err, &ufe) {
    // ufe.Keys == []string{"bogus", "server.tls.oops"} (文書の出現順)
    for _, key := range ufe.Keys {
        log.Printf("unknown key: %s", key)
    }
}
```

`UnmarshalFile` も同じオプションを受け取ります。これが効くのは struct ターゲットのみです -- map ターゲット (`map[string]any` など) には「未知のキー」という概念がありません。

### ファイル入出力

```go
// 書き込み
setay.MarshalFile("config.setay", cfg)

// 読み込み
var loaded Config
setay.UnmarshalFile("config.setay", &loaded)
```

## セット

Go では `map[K]struct{}` としてセットを表現します。setay では各要素を `key =;` と記述します。`=;` サフィックスは**アトミック**（`=` と `;` の間にスペース不可）です。

```go
type Firewall struct {
    AllowedPorts map[int]struct{}    `setay:"allowed-ports"`
    BlockedIPs   map[string]struct{} `setay:"blocked-ips"`
}

// マーシャル
fw := Firewall{
    AllowedPorts: map[int]struct{}{80: {}, 443: {}, 8080: {}},
    BlockedIPs:   map[string]struct{}{},
}
data, _ := setay.Marshal(fw)
// 出力:
// {
// 	allowed-ports =
// 	{
// 		80=;
// 		443=;
// 		8080=;
// 	};
// 	blocked-ips = {=;};
// }

// アンマーシャル
input := `{
    allowed-ports = { 80=; 443=; };
    blocked-ips   = { "192.168.1.1"=; "10.0.0.5"=; };
}`
var loaded Firewall
setay.Unmarshal([]byte(input), &loaded)
```

**ルール:**
- `{=;}` が空セット（空ディクト `{}` とは別物）
- `=;` の直前にスペースは許可、`=;` 内部にスペース不可
- セットのキーはスカラー値のみ（int, float, string, bool, null, タイムスタンプ、変数参照）
- `map[K]struct{}` はトップレベルのドキュメントには使用不可

## フラグエントリ

ディクトのエントリは、キーだけを書いて `= value` を省略できます。これを**フラグエントリ**と呼び、真偽値 `true` を表します。`key;` と `key = true;` は同じ値の 2 通りの綴りで、「存在すること自体が意味を持つ」DSL 的な設定に向いています。

```go
type Options struct {
    Verbose bool `setay:"verbose"`
    Debug   bool `setay:"debug"`
    Quiet   bool `setay:"quiet"`
}

input := `{
    verbose;        # フラグエントリ -- verbose = true と同じ
    debug = true;   # 値を明示した書き方
}`
var o Options
setay.Unmarshal([]byte(input), &o)
// o.Verbose == true, o.Debug == true, o.Quiet == false (未指定)
```

**ルール:**
- フラグエントリは `true` を表す; キーが無い場合、`bool` フィールドは `false` のまま
- struct の `bool` フィールド、または map の値型が `bool`・`any` の場合、フラグエントリは `true` になる; それ以外の型はエラー
  - (対象は `bool` である必要がある: `null`/ポインタでは「未指定」と「明示的に false」を区別できず、それこそがフラグの目的)
- `Marshal` は逆方向では意図的に lossy: `true` は常に `= true` と出力され、フラグエントリには畳まれない。往復編集 (`Document` API) は作者が書いた綴りを両方向とも保持する
- フラグエントリ (`{ key; }`) はセット型 (`{ key=; }`) とは別物。セットはアトミックなトークン `=;` で終端する独立した値種別

## 変数参照

アンマーシャル時に `${VAR_NAME}` が変数名に対応する値で置き換えられます。デフォルトではプロセスの環境変数（`os.LookupEnv`）から解決します。データベース・シークレットマネージャーなど独自のソースから値を提供したい場合は、カスタムリゾルバーを登録できます。

### 構文

```setay
{
	# シンプルな環境変数参照
	redis-host = ${REDIS_HOST};

	# フォールバック値付き（変数が未定義の場合に使用）
	db-port = ${MYSQL_PORT ?: 13306};

	# 文字列補間（ダブルクォート文字列内で変数を展開）
	greeting = "Hello, ${USER_NAME ?: 'World'}!";

	# フォールバックチェーン — 順番に変数を試し、最後にリテラル
	password = ${PASS_PRIMARY ?: PASS_SECONDARY ?: "default-secret"};
}
```

**主なルール：**

- `${VAR_NAME}` — 変数名（ベアキーと同じ文字規則の識別子）で変数を参照
- `?: value` — 変数が未定義の場合のフォールバック。チェーン可能: `?: A ?: B ?: "final"`
- フォールバック値の種類: 別の変数名、整数/浮動小数点リテラル、クォートされた文字列
  > **注意:** `?:` 直後のクォートなし識別子（例: `${VAR ?: fallback}`）は**変数名**として扱われます。
  > 文字列リテラルとして使う場合はクォートが必要：`${VAR ?: "fallback"}` または `${VAR ?: 'fallback'}`。
- 文字列補間は **ダブルクォート文字列**（`"..."`）内でのみ機能。シングルクォート文字列（`'...'`）は常にリテラル
- ダブルクォート文字列内では `$` `{` `}` は (補間用に) 予約されており、リテラルにするには `\$` `\{` `\}` と書く -- 素の `$` はパースエラー。シングルクォート文字列では素で書ける (例: `'price: $100'`)。完全なエスケープ表は [format spec](docs/setay-format-spec.md) を参照
- 変数参照は **アンマーシャル専用** の機能。`Marshal` は常に具体的な値を出力する

### 解決の優先順位

1. **カスタムリゾルバー**（登録済みで `ok=true` を返した場合）
2. **環境変数**（`os.LookupEnv`）— リゾルバー未登録、または `ok=false` を返した場合
3. **`?:` フォールバック値** — いずれのソースでも見つからなかった場合
4. **エラー** — 変数が見つからずフォールバックも指定されていない場合

### カスタムリゾルバー

```go
import setay "github.com/aki-kuramoto/setay-go"

// カスタムリゾルバーを登録する（os.LookupEnv より先に呼ばれる）。
// ok=true を返すとその値を使用、ok=false を返すと os.LookupEnv にフォールバック。
// error を返すと Unmarshal が即座にそのエラーを返す。
setay.RegisterVariableResolver(func(name string) (result string, ok bool, err error) {
	result, ok = mySecretStore.Get(name)
	return result, ok, nil
})

// デフォルト（環境変数のみ）に戻す：
setay.RegisterVariableResolver(nil)
```

### 型への変換

解決された変数値は常に文字列で、setay がターゲットフィールドの型に自動変換します：

| 変換先の型 | 変換方法 |
|---|---|
| `string` | そのまま使用 |
| `int`, `int64`, … | 整数としてパース（`0x`, `0b`, `0o` プリフィックス対応） |
| `uint`, `uint32`, … | 符号なし整数としてパース |
| `float32`, `float64` | 浮動小数点数としてパース |
| `bool` | `"true"`, `"1"`, `"yes"` → `true`；`"false"`, `"0"`, `"no"`, `""` → `false` |
| `interface{}` | `string` として格納 |
| `*T` | ポインタを確保し、`T` として変換 |

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
| 変数参照 | `${MY_VAR}`, `${PORT ?: 8080}` |

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
- [パーサの再生成](docs/regenerating-the-parser.md) — パーサの生成方法と、同期が必要な2つの生成コピー

## ライセンス

Copyright (c) 2026-present Akihiro Kuramoto. Setay-go は [MIT ライセンス](LICENSE.md) のもとで公開されているフリーかつオープンソースなソフトウェアです。
