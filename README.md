# protoc-gen-gleam

A [protoc](https://grpc.io/docs/protoc-installation/) / [buf](https://buf.build/) plugin that generates idiomatic [Gleam](https://gleam.run/) types and encode/decode functions from proto3 definitions.

The first protoc plugin for Gleam. Integrates into existing `buf generate` pipelines so Go, Rust, Python, and Gleam types all come from the same `.proto` source.

## Features

- Generates Gleam record types for proto3 messages
- Sum types for `oneof` fields and enums
- `optional` fields mapped to `Result(T, Nil)`
- `map<K, V>` fields mapped to `dict.Dict(K, V)`
- Nested message support with flattened type names
- Cross-file imports (types from other `.proto` files)
- Well-known types (`Timestamp`, `Duration`, `Empty`, wrapper types)
- All scalar types including zigzag (`sint32/64`) and fixed-width (`fixed32/64`, `sfixed32/64`)
- Forward-compatible: unknown fields are skipped on decode
- Proto3 default-value semantics: zero values omitted from wire
- Works with `protoc` and `buf generate`

### Supported proto3 types

| Proto3 | Gleam | Wire |
|--------|-------|------|
| `string` | `String` | LEN |
| `bool` | `Bool` | VARINT |
| `int32`, `int64`, `uint32`, `uint64` | `Int` | VARINT |
| `sint32`, `sint64` | `Int` | VARINT (zigzag) |
| `fixed32`, `fixed64` | `Int` | I32 / I64 |
| `sfixed32`, `sfixed64` | `Int` | I32 / I64 (signed) |
| `float` | `Float` | I32 |
| `double` | `Float` | I64 |
| `bytes` | `BitArray` | LEN |
| `enum` | Sum type | VARINT |
| `message` | Record type | LEN |
| `oneof` | Sum type | per-variant |
| `optional T` | `Result(T, Nil)` | same as T |
| `repeated T` | `List(T)` | unpacked LEN |
| `map<K, V>` | `dict.Dict(K, V)` | repeated MapEntry |

### Well-known types

Fields typed as `google.protobuf.*` are automatically mapped to pre-built types in the `gleam_protobuf/well_known` runtime module:

| Proto | Gleam |
|-------|-------|
| `google.protobuf.Timestamp` | `well_known.Timestamp` |
| `google.protobuf.Duration` | `well_known.Duration` |
| `google.protobuf.Empty` | `well_known.Empty` |
| `google.protobuf.StringValue` | `well_known.StringValue` |
| `google.protobuf.Int64Value` | `well_known.Int64Value` |
| `google.protobuf.BoolValue` | `well_known.BoolValue` |
| `google.protobuf.FloatValue` | `well_known.FloatValue` |
| `google.protobuf.DoubleValue` | `well_known.DoubleValue` |
| `google.protobuf.BytesValue` | `well_known.BytesValue` |

### Not supported

- Services / RPC definitions (no gRPC transport in Gleam)
- `google.protobuf.Struct` / `Value` (dynamic JSON mapping — poor fit for Gleam's type system)
- Proto2 syntax

## Install

```sh
go install github.com/qwexvf/protoc-gen-gleam/cmd/protoc-gen-gleam@latest
```

## Usage

### With buf

Add to your `buf.gen.yaml`:

```yaml
version: v2
plugins:
  - local: protoc-gen-gleam
    out: services/api/src/my_app/proto
    opt: package_prefix=my_app/proto
```

Then run:

```sh
buf generate
```

### With protoc

```sh
protoc \
  --gleam_out=src/my_app/proto \
  --gleam_opt=package_prefix=my_app/proto \
  --proto_path=proto \
  proto/my/package/v1/messages.proto
```

### Options

| Option | Description | Example |
|--------|-------------|---------|
| `package_prefix` | Gleam module path prefix prepended to the output file path | `my_app/proto` |

## Runtime dependency

Generated code imports `gleam_protobuf/wire` for wire-format primitives. Add the runtime to your Gleam project:

**Option A: From Hex** (once published)

```toml
# gleam.toml
[dependencies]
gleam_protobuf = ">= 0.1.0 and < 1.0.0"
```

**Option B: Vendor the runtime**

Copy `runtime/src/gleam_protobuf/` into your project at `src/gleam_protobuf/`.

## Example

Given this proto:

```proto
syntax = "proto3";
package example.v1;

enum Status {
  STATUS_UNSPECIFIED = 0;
  STATUS_ACTIVE = 1;
  STATUS_INACTIVE = 2;
}

message User {
  string name = 1;
  Status status = 2;
  repeated string tags = 3;
  optional string bio = 4;
  map<string, string> metadata = 5;
}
```

The plugin generates:

```gleam
pub type Status {
  StatusUnspecified
  StatusActive
  StatusInactive
}

pub type User {
  User(
    name: String,
    status: Status,
    tags: List(String),
    bio: Result(String, Nil),
    metadata: dict.Dict(String, String),
  )
}

pub fn encode_user(msg: User) -> BitArray { ... }
pub fn decode_user(buf: BitArray) -> Result(User, wire.DecodeError) { ... }

pub fn status_to_int(v: Status) -> Int { ... }
pub fn status_from_int(i: Int) -> Result(Status, wire.DecodeError) { ... }
pub fn status_to_string(v: Status) -> String { ... }
```

## Development

```sh
# Build
go build ./cmd/protoc-gen-gleam/

# Run all tests
go test ./...

# Update golden files after changing codegen
UPDATE_GOLDEN=1 go test ./internal/generator/ -run TestGenerateScanProto

# Test that generated Gleam compiles (requires gleam in PATH)
go test ./internal/generator/ -run TestGeneratedGleamCompiles

# Test comprehensive proto (maps, optional, nested, all types)
go test ./internal/generator/ -run TestComprehensiveProtoCompiles
```

## Architecture

```
cmd/protoc-gen-gleam/        # Plugin binary (reads CodeGeneratorRequest from stdin)
internal/
  generator/                 # Code generation logic
    file.go                  # Per-file orchestration, enum generation, type resolution
    encode.go                # Encoder generation (maps, optional, repeated, scalar)
    decode.go                # Decoder generation (accumulator pattern, map entries)
  gleam/                     # Gleam language formatting utilities
    naming.go                # snake_case, PascalCase, enum variant naming
runtime/                     # gleam_protobuf Hex package
  src/gleam_protobuf/
    wire.gleam               # Wire-format primitives (varint, tag, LEN, float, zigzag, skip)
    well_known.gleam          # Pre-built types for google.protobuf.* types
testdata/                    # Test proto files + golden outputs
  aegis/scan/v1/scan.proto   # Real-world proto from the Aegis project
  test/v1/comprehensive.proto # Exercises every supported feature
```

## License

MIT
