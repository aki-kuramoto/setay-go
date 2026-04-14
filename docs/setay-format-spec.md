# Setay Format Specification

> **Note:** The second half of this document is available in Japanese (日本語).

The `setay` format is designed primarily for saving and loading structured data in text form.

It is built around simple values, **lists** (ordered sequences of values), **dicts** (ordered sequences of key–value pairs called entries), and **sets** (unordered collections of unique scalar elements). Lists, dicts, and sets can contain not only simple values but also other lists, dicts, and sets, enabling nested structures.

# 1. Basic Structure

## Top Level

The top level of a file is always a dict `{ ... }`. A bare value or list at the top level is not permitted.

## List `[ ... ]`

A list holds an ordered sequence of values, e.g. `[ "G", "O", "D", ]`.

## Dict `{ ... }`

A dict holds an ordered sequence of key–value entries, e.g. `{ "key1" = "value1"; "key2" = "value2"; }`.

## Set `{ key=; ... }`

A set holds an unordered collection of unique scalar elements, e.g. `{ 1=; 7=; 42=; }`. See Section 6 for details.

# 2. Value Types

Setay supports several value types.

## Null

`{ "sample" = null; }`

## Boolean

`{ "enabled" = true; }`
`{ "disabled" = false; }`

## Numeric Types

Numeric types include both integers and floating-point numbers.

### Integers

- Decimal: `1979`, `0`, `42`
- Negative: `-42`, `-1`
- Leading zeros are interpreted as decimal, not octal: `008` is the number `8`, not `10`.

### Floating-Point Numbers

- Decimal: `4.11`, `-0.5`
- Exponential notation: `1.5e10`, `2.3E-4`, `-1e+6`

### Prefixed Integers

- `0x` or `0X`: hexadecimal (`0xFF`, `0X1A`)
- `0b` or `0B`: binary (`0b1010`, `0B110`)
- `0o` or `0O`: octal (`0o77`, `0O755`)
- `0d` or `0D`: explicit decimal (`0d42`, `0D100`)

## String Type

`{ "name" = "set-ay"; }`

Strings are enclosed in either double quotes `"..."` or single quotes `'...'`.

- Inside double quotes, single quotes can be used without escaping.
- Inside single quotes, double quotes can be used without escaping.
- In either case, to include the enclosing quote character itself, it must be escaped (`\"` or `\'`).

## UTC Timestamp Type

`{ "born-in" = UtcTs("2006-01-02 15:04:05.999"); }`

- The value inside `UtcTs(...)` is written as a double-quoted or single-quoted string literal.
- This type can represent times with explicit timezone offsets, but regardless of the surface representation, the time should always be interpreted as the corresponding UTC time.
  - That is, `"2026-03-22 21:28:27+09:00"` is an alternative representation of `"2026-03-22 12:28:27"`.
  - Although not required, you may explicitly write `"2026-03-22 12:28:27Z"`.
  - For compatibility with standard formats, `"2026-03-22T12:28:27"` and `"2026-03-22T12:28:27Z"` are also accepted.
- When no timezone is specified, the time is interpreted as UTC.

# 3. Notation Rules

- The standard character encoding is UTF-8 without BOM, with LF line endings.
- Dict keys must be strings (null, booleans, numbers, timestamps, lists, and dicts cannot be used as keys).
- Dict keys may omit quotes if their content follows certain rules (see below).
- Ordinary string values must always be enclosed in single or double quotes.
- Dict entries are separated by `;` and list elements by `,`.
  - The trailing `;` or `,` after the last entry/element may be omitted.

## Bare Keys

Dict keys may omit quotes if all of the following rules are satisfied:

- Allowed characters: `[a-zA-Z0-9_-]` only
- **First character**: `[a-zA-Z_]` only (digits `[0-9]` and hyphens `-` are not allowed at the start)
- **Last character**: `[a-zA-Z0-9_]` only (hyphens `-` are not allowed at the end)
- Escape sequences are not allowed (`\` itself is not in the allowed character set)
- Consecutive `-` and `_` are not forbidden but discouraged

> This matches the `LenientIdentifier` rule from boompaw.bp.

Examples: `name`, `is-student`, `is_neet`, `Address`, `HEIGHT` can all be used without quotes.

# 4. Comment Syntax

## Single-Line Comments

Three forms of single-line comments are available, all starting with `#`:

- `# ` — hash followed by a space
- `#\t` — hash followed by a horizontal tab (U+0009)
- `##` — two consecutive hashes
- Any other character following `#` does not form a single-line comment.

## `#!` Reserved Expression

As a special case, `#!` is a reserved expression that causes the rest of the line to be ignored. This is intended for shebangs (`#!/bin/setay`), but can appear anywhere in the file and is skipped like a comment. Setay does not interpret the meaning of the shebang.

## Multi-Line Comments

- `#{` marks the start of a multi-line comment.
- `}#` marks the end.
- Multi-line comments do not nest by default.
  - Implementations may optionally support nesting, but the standard does not define how opening/closing markers inside string-like content within comments should be handled.

# 5. Variable References

At unmarshal time, `${VAR_NAME}` expressions are replaced with a resolved value.

## 5.1 Basic Syntax

```setay
{
	redis-host = ${REDIS_HOST};
	db-port    = ${MYSQL_PORT ?: 13306};
	greeting   = "Hello, ${USER_NAME ?: 'World'}!";
	password   = ${PASS1 ?: PASS2 ?: "default-secret"};
}
```

## 5.2 Variable Name Rules

Variable names follow the same character rules as bare keys:
- First character: `[a-zA-Z_]`
- Subsequent characters: `[a-zA-Z0-9_-]`
- Last character: `[a-zA-Z0-9_]` (no trailing hyphen)

## 5.3 Fallback Values (`?:`)

If a variable is not defined, a fallback value after `?:` is used instead:

```setay
port    = ${PORT ?: 8080};        # integer literal fallback
mode    = ${MODE ?: "production"}; # string literal fallback
backup  = ${A ?: B ?: "final"};   # chained fallbacks
```

Fallback values can be:
- **Another variable name** — resolved recursively (written without `${}`)
- **An integer or float literal** — used as a string when the target type requires it
- **A single- or double-quoted string literal**

> **Important:** A bare word (unquoted identifier) after `?:` is always interpreted as a
> **variable name**, not as a string literal. To use a fixed string as a fallback, you must
> enclose it in quotes:
>
> ```setay
> # ✅ Correct — "fallback" and 'fallback' are string literals
> v = ${MISSING ?: "fallback"}
> v = ${MISSING ?: 'fallback'}
>
> # ⚠️ This resolves the variable named `fallback` (not the text "fallback")
> # If no variable named `fallback` is defined, this will produce an error.
> v = ${MISSING ?: fallback}
> ```
>
> The same rule applies inside double-quoted string interpolation:
>
> ```setay
> # ✅ Correct
> msg = "Hello, ${NAME ?: "stranger"}!"
>
> # ⚠️ Tries to resolve the variable named `stranger`
> msg = "Hello, ${NAME ?: stranger}!"
> ```

## 5.4 String Interpolation

Inside **double-quoted strings**, `${VAR}` is expanded in place:

```setay
message = "Hello, ${NAME ?: 'stranger'}! Port is ${PORT ?: 8080}.";
```

Rules:
- String interpolation works **only** inside double-quoted strings (`"..."`)
- Single-quoted strings (`'...'`) are always literal — `${VAR}` inside them is not expanded
- A `$` not followed by `{` is treated as a literal `$` character (e.g. `"price: $100"`)
- Inside an interpolation expression (`${...}`), a bare word after `?:` is a **variable name**.
  To use a literal string as fallback, it must be quoted: `${VAR ?: "text"}` or `${VAR ?: 'text'}`.

## 5.5 Resolution Order

The following sources are tried in order:

1. **Custom resolver** (registered via `RegisterVariableResolver`), if it returns `ok=true`
2. **Environment variable** (`os.LookupEnv`) — when no resolver is registered, or the resolver returns `ok=false`
3. **`?:` fallback** — when neither source resolves the variable
4. **Parse error** — when no fallback is provided and the variable is undefined

## 5.6 Unmarshal-Only

Variable references are an **unmarshal-only** feature. `Marshal` (Go → setay) always writes concrete values.

# 6. Sets

A **set** is an unordered collection of unique scalar elements. In Go, sets are represented as `map[ELEMENT_TYPE]struct{}`.

## 6.1 Syntax

Each element is written as `key =;` — the `=;` token is **atomic** (no space is allowed between `=` and `;`). A space *before* `=;` is allowed.

```setay
{
	allowed-ports =
		{
			80=;
			443=;
			8080=;
		};
}
```

Every element must end with `=;`. Omitting the final `=;` is not permitted.

## 6.2 Empty Set

An empty set is written as `{=;}`. This is distinct from the empty dict `{}`.

```setay
{
	blocked-ips = {=;};
}
```

Comments and whitespace are allowed inside an empty set:

```setay
{ flags = { #{ reserved for future use }# =; } }
```

## 6.3 Set Key Types

Set keys are restricted to **scalar (comparable) values**:

| Allowed key types |
|-------------------|
| Integer (`1`, `0xFF`, `0b1010`, …) |
| Float (`3.14`, `-0.5`, …) |
| String (`"hello"`, `'world'`, …) |
| Boolean (`true`, `false`) |
| `null` |
| UTC Timestamp (`UtcTs("2026-01-01 00:00:00")`) |
| Variable reference (`${VAR}`) |

Dicts, lists, and nested sets **cannot** be used as set keys (they are not comparable in Go).

## 6.4 Go Mapping

| Setay | Go |
|-------|----|
| `{ 1=; 2=; }` | `map[int]struct{}` |
| `{ "a"=; "b"=; }` | `map[string]struct{}` |
| `{ 1.5=; 2.5=; }` | `map[float64]struct{}` |
| `{=;}` | `map[K]struct{}` (empty) |

A `map[K]struct{}` **cannot** appear as the top-level document (the top level must always be a dict).

## 6.5 Marshal Output

`Marshal` / `MarshalIndent` automatically detect `map[K]struct{}` and emits set syntax. An empty map emits `{=;}` inline; a non-empty map emits a multi-line block.

# 7. Example

```setay
#!/bin/setay
{
  name       = "John S.";
  "age"      = 47;
  job        = 'aka-madoushi';
  is-student = false;

  # Information he didn't want to share
  is_neet=true;
  'hobbies' = [ "アニメ", 'ゲーム', "Rock'n' roll", ];
  Address = {
	city = "渋谷区";	# Shibuya-ku
	"国または地域" = "Japan"
  };

  HEIGHT = 4.11;
  weight = 180;
  score = -3.14e2;
  mask = 0xFF;
  experiences = null
  #{
    You can write anything inside a comment
  }#;
}
```

# 8. Standard Style

- Indentation uses horizontal tabs by default.
  - However, spaces are used for alignment after the first non-tab character on a line.
- Structure opening/closing tokens are on the same line for single-line content, or on separate lines when line breaks are involved.
- Trailing separators (`;` for dicts, `,` for lists) are omitted for single-line content and included for multi-line content.
- Blank lines are filled with tabs matching the current indentation depth.

```setay
{
	name = "John S.";
	age = 47;
	job = "aka-madoushi";
	is-student = false;
	# The following line is indented with a horizontal tab, as is standard.
	
	# The preceding line is indented with a horizontal tab, as is standard.
	is-neet = true;
	hobbies = [ "アニメ", 'ゲーム', "Rock'n' roll" ];
	address =
	{
		city = "渋谷区";
		"国または地域" = "Japan";
	};
	
	height = 4.11;
	weight = 180;
	score = -3.14e2;
	mask = 0xFF;
	experiences = null;
}
```

# 9. String Escape Sequences

- `\` is used as the escape character.
- To represent `\` itself, use `\\`.

## `\xXX` Form

- The value range is 0 to 127 (i.e., `\x00` to `\x7F`).

## `\uXXXX` Form

- `\uD800` through `\uDFFF` must not appear (surrogate range).

## `\UXXXXXXXX` Form

- The first two digits are expected to be `00`.

## Individual Escape Characters

- `\t` — horizontal tab
- `\r` — carriage return
- `\n` — line feed
- `\\` — backslash (repeated for emphasis)
- `\0` — NUL character
- `\"` — double quote (within double-quoted strings)
- `\'` — single quote (within single-quoted strings)

---

# setay 形式 (日本語)

`setay` 形式は構造化データをテキスト形式でファイルに保存/読み出しする事を主目的に設計されました。

シンプルな値に、
リストという値の並びを保持する構造と
ディクトというキーと値のペア (エントリー) の並びを保持する構造、
そしてセットというスカラー値の重複しない要素の集合を主軸としており、
これらの値として、シンプルな値だけではなくリストやディクト、セットを指定できる事でネストした構造を表現可能です。

# 1. setay の基本構造

## トップレベル

ファイルのトップレベルは必ずディクト `{ ... }` です。リストや裸の値がトップレベルになる事は許されません。

## リスト `[ ... ]`

リストは `[ "G", "O", "D", ]` のように、値の並びを保持する構造です。

## ディクト `{ ... }`

ディクトは `{ "key1" = "value1"; "key2" = "value2"; }` のように、キーと値のペアの並びを保持する構造です。

## セット `{ key=; ... }`

セットは `{ 1=; 7=; 42=; }` のように、スカラー値の重複しない要素の集合を保持する構造です。詳細は Section 6 を参照してください。

# 2. setay の値型

setay では複数の種類の値と値型を利用可能です。

## ナル値

`{ "sample" = null; }`


## 真偽値

`{ "enabled" = true; }`
`{ "disabled" = false; }`


## 数値型

数値型は整数と浮動小数点数の両方を含みます。

### 整数

- 通常の十進数: `1979`, `0`, `42`
- 負の数: `-42`, `-1`
- 不要な先行ゼロは8進数ではなく十進数として解釈: `008` は 数値 `8` であり、`10` ではない。

### 浮動小数点数

- 小数: `4.11`, `-0.5`
- 指数表記: `1.5e10`, `2.3E-4`, `-1e+6`

### プリフィックス付き整数

- `0x` または `0X`: 16進数 (`0xFF`, `0X1A`)
- `0b` または `0B`: 2進数 (`0b1010`, `0B110`)
- `0o` または `0O`: 8進数 (`0o77`, `0O755`)
- `0d` または `0D`: 10進数の別表記 (`0d42`, `0D100`)


## 文字列型

`{ "name" = "set-ay"; }`

文字列はダブルクォート `"..."` またはシングルクォート `'...'` のいずれかで囲みます。

- ダブルクォートで囲んだ場合、内部にシングルクォートをエスケープなしで記述可能
- シングルクォートで囲んだ場合、内部にダブルクォートをエスケープなしで記述可能
- いずれの場合も、囲みに使ったクォート自身を内部で使いたい場合はエスケープが必要 (`\"` または `\'`)


## UTC タイムスタンプ型

`{ "born-in" = UtcTs("2006-01-02 15:04:05.999"); }`

- `UtcTs(...)` の括弧内は、ダブルクォートまたはシングルクォートの文字列リテラルで記述します。
- この型はタイムゾーンを明示した場合には、表面上は UTC 時間以外の表現をすることもできます。
	- 表面上どのように表現されているかに関わらず、この型で表される時間は、それに対応した UTC 時間と解釈するべきです。
	- つまり "2026-03-22 21:28:27+09:00" は "2026-03-22 12:28:27" の別表現です。
	- 行う必要はありませんが "2026-03-22 12:28:27Z" のように明示する事も可能です。
	- また、標準仕様との間の互換性の便宜の為 "2026-03-22T12:28:27" や "2026-03-22T12:28:27Z" のように表記する事も可能です。
- タイムゾーンを明示しない形式の場合には UTC で表記されていると解釈されます。

# 3. 表記ルールの詳細

- 保存する際の文字コードの標準は BOM なし UTF-8 で、改行コードは LF 単体です。
- ディクトのキーは必ず文字列です (null, 真偽値, 数値, UTC タイムスタンプ値などの値や, リストやディクトなどの構造はキーには利用できません)。
- ディクトのキーの場合で、且つ内容が一定のルールを満たす場合に限り、シングルクォートやダブルクォーツを省略した形式が許されます (詳細は後述します)。
- 通常の文字列は、必ずシングルクォートかダブルクォーツで囲まれていなければいけません (どちらを使用するかは任意です)。
- ディクトは1エントリーの終わりに `;` が、リストは1要素の終わりに `,` が継続するのが基本形式です。
	- ディクトとリストでは、その構造内の最後のエントリーまたは要素である場合には、`;` や `,` を省略する事が許されます。

## クォートなしキー

ディクトのキーに限り、内容が以下のルールを全て満たす場合、クォートを省略した形式が許されます。

- 使用可能な文字は `[a-zA-Z0-9_-]` のみ
- **先頭の文字**: `[a-zA-Z_]` のみ (数字 `[0-9]` およびハイフン `-` は先頭に使用不可)
- **末尾の文字**: `[a-zA-Z0-9_]` のみ (ハイフン `-` は末尾に使用不可)
- エスケープ表現は使用不可 (`\` 自体が許可文字に含まれない)
- `-` と `_` の連続は禁止ではないが非推奨

> これは boompaw.bp の `LenientIdentifier` 定義に一致する識別子ルールです。

例: `name`, `is-student`, `is_neet`, `Address`, `HEIGHT` はクォートなしで使用可能。

# 4. コメント記法

## 単行コメント

マークの出現から行末までをコメントとする単行コメント形式が三種類あります。いずれも最初の文字は `#` です。

- `# ` 空白が継続する形です。
- `#(タブ)` 水平タブ (U+0009) が継続する形です。
- `##` `#` が継続し、二文字連続となる形です。
- これら以外の文字が継続した場合は、それは単行コメントではありません。

## `#!` 予約表現

特例として `#!` は予約された表現として、行末までを無視します。
これは shebang (`#!/bin/setay`) の為の仕組みですが、ファイルの先頭に限らずどこにでも出現可能で、一般のコメントと同様に読み飛ばされます。
setay はシェルではないので shebang の意味を解釈しません。

## 複数行コメント

終了マークと共に用いる事で、複数行にまたがってもよい複数行コメント形式があります。

- `#{` を開始マークとします。
- `}#` を終了マークとします。
- 開始マークと終了マークで囲まれた部分をコメントとします。
- 複数行コメントはネストできない解釈を標準とします。
	- 処理系は任意でネストする形式をサポートしてもよいですが、コメント内の文字列に見える部分の開始/終了マークをどう解釈するかなどの答えを標準は提供しません。

# 5. 変数参照

アンマーシャル時に、`${VAR_NAME}` 式が解決された値で置き換えられます。

## 5.1 基本構文

```setay
{
	redis-host = ${REDIS_HOST};
	db-port    = ${MYSQL_PORT ?: 13306};
	greeting   = "Hello, ${USER_NAME ?: 'World'}!";
	password   = ${PASS1 ?: PASS2 ?: "default-secret"};
}
```

## 5.2 変数名のルール

変数名はベアキーと同じ文字規則に従います：
- 先頭文字：`[a-zA-Z_]`
- 2文字目以降：`[a-zA-Z0-9_-]`
- 末尾文字：`[a-zA-Z0-9_]`（ハイフン不可）

## 5.3 フォールバック値 (`?:`)

変数が未定義の場合、`?:` 以降のフォールバック値が使われます：

```setay
port   = ${PORT ?: 8080};         # 整数リテラルのフォールバック
mode   = ${MODE ?: "production"};  # 文字列リテラルのフォールバック
backup = ${A ?: B ?: "final"};    # フォールバックのチェーン
```

フォールバック値として指定できるもの：
- **別の変数名** — 再帰的に解決される（`${}` なしで記述）
- **整数または浮動小数点のリテラル** — 型変換が必要な場合は文字列として使用される
- **シングルまたはダブルクォートの文字列リテラル**

> **重要:** `?:` の直後に書いたクォートなしの識別子は、常に**変数名**として解釈されます。
> 固定の文字列をフォールバックとして使いたい場合は、必ずクォートで囲んでください：
>
> ```setay
> # ✅ 正しい — "fallback" と 'fallback' は文字列リテラル
> v = ${MISSING ?: "fallback"}
> v = ${MISSING ?: 'fallback'}
>
> # ⚠️ これは変数 fallback を解決しようとする（文字列 "fallback" ではない）
> # fallback という変数が未定義の場合はエラーになる
> v = ${MISSING ?: fallback}
> ```
>
> ダブルクォート文字列の補間式内でも同じルールが適用されます：
>
> ```setay
> # ✅ 正しい
> msg = "Hello, ${NAME ?: "stranger"}!"
>
> # ⚠️ stranger という変数を解決しようとする
> msg = "Hello, ${NAME ?: stranger}!"
> ```

## 5.4 文字列補間

**ダブルクォート文字列** 内では `${VAR}` がその場で展開されます：

```setay
message = "Hello, ${NAME ?: 'stranger'}! Port is ${PORT ?: 8080}.";
```

ルール：
- 文字列補間は **ダブルクォート文字列**（`"..."`）内でのみ機能する
- シングルクォート文字列（`'...'`）は常にリテラル — `${VAR}` は展開されない
- `${` が続かない `$` は文字リテラルとして扱われる（例：`"price: $100"`）
- 補間式（`${...}`）内で `?:` の右辺に書いたクォートなしの識別子は**変数名**として扱われる。
  リテラル文字列をフォールバックとして使うにはクォートが必要：`${VAR ?: "テキスト"}` または `${VAR ?: 'テキスト'}`。

## 5.5 解決の優先順位

以下のソースを順番に試みます：

1. **カスタムリゾルバー**（`RegisterVariableResolver` で登録済み）が `ok=true` を返した場合
2. **環境変数**（`os.LookupEnv`）— リゾルバー未登録、または `ok=false` を返した場合
3. **`?:` フォールバック** — いずれのソースでも解決できない場合
4. **パースエラー** — フォールバックが指定されておらず変数が未定義の場合

## 5.6 アンマーシャル専用

変数参照は **アンマーシャル専用** の機能です。`Marshal`（Go → setay）は常に具体的な値を書き出します。

# 6. セット

**セット** はスカラー値の重複しない要素の集合です。Go では `map[ELEMENT_TYPE]struct{}` に対応します。

## 6.1 構文

各要素は `key =;` の形式で記述します。`=;` は**アトミックなトークン**であり、`=` と `;` の間にスペースを入れることはできません。`=;` の直前にスペースを入れることは許されます。

```setay
{
	allowed-ports =
		{
			80=;
			443=;
			8080=;
		};
}
```

すべての要素は `=;` で終わる必要があります。最後の要素の `=;` も省略できません。

## 6.2 空セット

空セットは `{=;}` と記述します。空ディクト `{}` とは区別されます。

```setay
{
	blocked-ips = {=;};
}
```

空セット内にコメントや空白を入れることも可能です：

```setay
{ flags = { #{ 将来のために予約 }# =; } }
```

## 6.3 セットのキー型

セットのキーに使用できるのは**スカラー (比較可能) な値**のみです：

| 使用可能なキー型 |
|----------------|
| 整数 (`1`, `0xFF`, `0b1010`, …) |
| 浮動小数点数 (`3.14`, `-0.5`, …) |
| 文字列 (`"hello"`, `'world'`, …) |
| 真偽値 (`true`, `false`) |
| `null` |
| UTC タイムスタンプ (`UtcTs("2026-01-01 00:00:00")`) |
| 変数参照 (`${VAR}`) |

ディクト、リスト、ネストされたセットは Go のマップキーとして使用できないため、セットのキーにすることも**できません**。

## 6.4 Go との対応

| Setay | Go |
|-------|----|
| `{ 1=; 2=; }` | `map[int]struct{}` |
| `{ "a"=; "b"=; }` | `map[string]struct{}` |
| `{ 1.5=; 2.5=; }` | `map[float64]struct{}` |
| `{=;}` | `map[K]struct{}` (空) |

`map[K]struct{}` をトップレベルのドキュメントとして使用することは**できません**（トップレベルは常にディクトである必要があります）。

## 6.5 マーシャル出力

`Marshal` / `MarshalIndent` は `map[K]struct{}` を自動検出し、セット記法で出力します。空のマップは `{=;}` をインラインで出力し、要素があれば複数行ブロックで出力します。

# 7. 具体的な記述例

```setay
#!/bin/setay
{
  name       = "John S.";
  "age"      = 47;
  job        = 'aka-madoushi';
  is-student = false;

  # 教えたくなかった情報
  is_neet=true;
  'hobbies' = [ "アニメ", 'ゲーム', "Rock'n' roll", ];
  Address = {
	city = "渋谷区";	# Shibuya-ku
	"国または地域" = "Japan"
  };

  HEIGHT = 4.11;
  weight = 180;
  score = -3.14e2;
  mask = 0xFF;
  experiences = null
  #{
    コメントなので何を書いてもよい
  }#;
}
```

# 8. 標準のスタイル

- インデントには原則として水平タブを用います。
	- ただし、行頭から一度でも水平タブ以外の文字が出現した後のレイアウトには空白を用います。
- 構造の開始及び終端は、単行の場合は同じ行に、改行を伴う場合は独立した行になるようにします。
- リスト及びディクトの継続文字は、単行の場合は省略し、複数行になる場合には省略しません。
- 空行はインデントの深さに応じた水平タブで埋めます

```setay
{
	name = "John S.";
	age = 47;
	job = "aka-madoushi";
	is-student = false;
	# 次の行が水平タブでインデントされている状態を標準とします。
	
	# 前の行が水平タブでインデントされている状態を標準とします。
	is-neet = true;
	hobbies = [ "アニメ", 'ゲーム', "Rock'n' roll" ];
	address =
	{
		city = "渋谷区";
		"国または地域" = "Japan";
	};
	
	height = 4.11;
	weight = 180;
	score = -3.14e2;
	mask = 0xFF;
	experiences = null;
}
```

# 9. 文字列内のエスケープ記法

- `\` をエスケープ用の文字として使用します。
- `\` 自体を表したい場合には `\\` のように二重にする事で表現できます。

## "\\xXX" 形式

- 値に指定できるのは 0 から 127 まで (すなわち `\x00` から `\x7F` まで) です。

## "\\uXXXX" 形式

- `\uD800` から `\uDFFF` までは出現してはいけません。

## "\\UXXXXXXXX" 形式

- 先頭二桁は "00" である事が予測されます。

## 個別のエスケープ文字

- `\t` 水平タブ
- `\r` キャリッジリターン
- `\n` ラインフィード
- `\\` バックスラッシュ (再掲)
- `\0` NUL 文字
- `\"` ダブルクォート (ダブルクォート文字列内で自身を表現)
- `\'` シングルクォート (シングルクォート文字列内で自身を表現)
