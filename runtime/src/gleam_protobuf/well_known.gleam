//// Pre-built types for Google's well-known protobuf types.
////
//// These types match the field layout of the corresponding .proto files
//// from google/protobuf/. When protoc-gen-gleam encounters a field typed
//// as google.protobuf.Timestamp (etc.), it generates code that references
//// these types and their encode/decode functions.
////
//// Users can also import these directly for manual construction.

import gleam_protobuf/wire

// --- google.protobuf.Timestamp ------------------------------------------
// https://protobuf.dev/reference/protobuf/google.protobuf/#timestamp

pub type Timestamp {
  Timestamp(seconds: Int, nanos: Int)
}

pub fn encode_timestamp(msg: Timestamp) -> BitArray {
  let Timestamp(seconds, nanos) = msg
  <<
    wire.encode_int_field(1, seconds):bits,
    wire.encode_int_field(2, nanos):bits,
  >>
}

pub fn decode_timestamp(
  buf: BitArray,
) -> Result(Timestamp, wire.DecodeError) {
  decode_timestamp_loop(buf, 0, 0)
}

fn decode_timestamp_loop(
  buf: BitArray,
  seconds: Int,
  nanos: Int,
) -> Result(Timestamp, wire.DecodeError) {
  case buf {
    <<>> -> Ok(Timestamp(seconds: seconds, nanos: nanos))
    _ ->
      case wire.decode_tag(buf) {
        Error(e) -> Error(e)
        Ok(#(field_number, wire_type, rest)) ->
          case field_number {
            1 ->
              case wire.decode_varint(rest) {
                Error(e) -> Error(e)
                Ok(#(v, r)) -> decode_timestamp_loop(r, v, nanos)
              }
            2 ->
              case wire.decode_varint(rest) {
                Error(e) -> Error(e)
                Ok(#(v, r)) -> decode_timestamp_loop(r, seconds, v)
              }
            _ ->
              case wire.skip_field(rest, wire_type) {
                Error(e) -> Error(e)
                Ok(r) -> decode_timestamp_loop(r, seconds, nanos)
              }
          }
      }
  }
}

// --- google.protobuf.Duration -------------------------------------------
// https://protobuf.dev/reference/protobuf/google.protobuf/#duration

pub type Duration {
  Duration(seconds: Int, nanos: Int)
}

pub fn encode_duration(msg: Duration) -> BitArray {
  let Duration(seconds, nanos) = msg
  <<
    wire.encode_int_field(1, seconds):bits,
    wire.encode_int_field(2, nanos):bits,
  >>
}

pub fn decode_duration(
  buf: BitArray,
) -> Result(Duration, wire.DecodeError) {
  decode_duration_loop(buf, 0, 0)
}

fn decode_duration_loop(
  buf: BitArray,
  seconds: Int,
  nanos: Int,
) -> Result(Duration, wire.DecodeError) {
  case buf {
    <<>> -> Ok(Duration(seconds: seconds, nanos: nanos))
    _ ->
      case wire.decode_tag(buf) {
        Error(e) -> Error(e)
        Ok(#(field_number, wire_type, rest)) ->
          case field_number {
            1 ->
              case wire.decode_varint(rest) {
                Error(e) -> Error(e)
                Ok(#(v, r)) -> decode_duration_loop(r, v, nanos)
              }
            2 ->
              case wire.decode_varint(rest) {
                Error(e) -> Error(e)
                Ok(#(v, r)) -> decode_duration_loop(r, seconds, v)
              }
            _ ->
              case wire.skip_field(rest, wire_type) {
                Error(e) -> Error(e)
                Ok(r) -> decode_duration_loop(r, seconds, nanos)
              }
          }
      }
  }
}

// --- google.protobuf.Empty ----------------------------------------------

pub type Empty {
  Empty
}

pub fn encode_empty(_msg: Empty) -> BitArray {
  <<>>
}

pub fn decode_empty(
  _buf: BitArray,
) -> Result(Empty, wire.DecodeError) {
  Ok(Empty)
}

// --- Wrapper types (google.protobuf.*Value) ------------------------------

pub type StringValue {
  StringValue(value: String)
}

pub fn encode_string_value(msg: StringValue) -> BitArray {
  wire.encode_string_field(1, msg.value)
}

pub fn decode_string_value(
  buf: BitArray,
) -> Result(StringValue, wire.DecodeError) {
  case buf {
    <<>> -> Ok(StringValue(value: ""))
    _ ->
      case wire.decode_tag(buf) {
        Error(e) -> Error(e)
        Ok(#(1, _, rest)) ->
          case wire.decode_len_delimited(rest) {
            Error(e) -> Error(e)
            Ok(#(bytes, _r)) ->
              case wire.decode_string(bytes) {
                Error(e) -> Error(e)
                Ok(s) -> Ok(StringValue(value: s))
              }
          }
        Ok(#(_, wt, rest)) ->
          case wire.skip_field(rest, wt) {
            Error(e) -> Error(e)
            Ok(_r) -> Ok(StringValue(value: ""))
          }
      }
  }
}

pub type Int64Value {
  Int64Value(value: Int)
}

pub fn encode_int64_value(msg: Int64Value) -> BitArray {
  wire.encode_int_field(1, msg.value)
}

pub type BoolValue {
  BoolValue(value: Bool)
}

pub fn encode_bool_value(msg: BoolValue) -> BitArray {
  wire.encode_bool_field(1, msg.value)
}

pub type FloatValue {
  FloatValue(value: Float)
}

pub fn encode_float_value(msg: FloatValue) -> BitArray {
  wire.encode_float_field(1, msg.value)
}

pub type DoubleValue {
  DoubleValue(value: Float)
}

pub fn encode_double_value(msg: DoubleValue) -> BitArray {
  wire.encode_double_field(1, msg.value)
}

pub type BytesValue {
  BytesValue(value: BitArray)
}

pub fn encode_bytes_value(msg: BytesValue) -> BitArray {
  wire.encode_bytes_field(1, msg.value)
}
