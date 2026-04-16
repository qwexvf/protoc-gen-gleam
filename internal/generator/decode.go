package generator

import (
	"fmt"
	"strings"

	gleamfmt "github.com/qwexvf/protoc-gen-gleam/internal/gleam"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func generateDecoder(ctx *genContext, msg *protogen.Message, typeName string, oneofs []oneofInfo) {
	fnName := "decode_" + gleamfmt.ToSnakeCase(typeName)
	regular := regularFields(msg)

	if len(regular) == 0 && len(oneofs) == 0 {
		ctx.w.P("pub fn %s(_buf: BitArray) -> Result(%s, wire.DecodeError) {", fnName, typeName)
		ctx.w.P("  Ok(%s)", typeName)
		ctx.w.P("}")
		ctx.w.P("")
		return
	}

	// Accumulator type.
	accName := typeName + "Acc"
	generateAccType(ctx, accName, regular, oneofs)

	// Decoder function.
	ctx.w.P("pub fn %s(buf: BitArray) -> Result(%s, wire.DecodeError) {", fnName, typeName)
	ctx.w.P("  let init = %s(", accName)
	for _, f := range regular {
		ctx.w.P("    %s: %s,", gleamfmt.FieldName(string(f.Desc.Name())), defaultValue(ctx, f))
	}
	for _, oo := range oneofs {
		ctx.w.P("    %s: Error(Nil),", gleamfmt.FieldName(string(oo.Desc.Name())))
	}
	ctx.w.P("  )")

	handlerName := gleamfmt.ToSnakeCase(typeName) + "_field_handler"
	ctx.w.P("  case decode_fields(buf, init, %s) {", handlerName)
	ctx.w.P("    Error(e) -> Error(e)")
	ctx.w.P("    Ok(acc) ->")

	if len(oneofs) > 0 {
		for _, oo := range oneofs {
			ooField := gleamfmt.FieldName(string(oo.Desc.Name()))
			ctx.w.P("      case acc.%s {", ooField)
			ctx.w.P("        Error(_) -> Error(wire.MissingPayload)")
			ctx.w.P("        Ok(p) ->")
		}
		ctx.w.P("          Ok(%s(", typeName)
		for _, f := range regular {
			fieldName := gleamfmt.FieldName(string(f.Desc.Name()))
			if f.Desc.IsList() {
				ctx.w.P("            %s: list.reverse(acc.%s),", fieldName, fieldName)
			} else {
				ctx.w.P("            %s: acc.%s,", fieldName, fieldName)
			}
		}
		for _, oo := range oneofs {
			ctx.w.P("            %s: p,", gleamfmt.FieldName(string(oo.Desc.Name())))
		}
		ctx.w.P("          ))")
		for range oneofs {
			ctx.w.P("      }")
		}
	} else {
		ctx.w.P("      Ok(%s(", typeName)
		for _, f := range regular {
			fieldName := gleamfmt.FieldName(string(f.Desc.Name()))
			if f.Desc.IsList() && !isMap(f) {
				ctx.w.P("        %s: list.reverse(acc.%s),", fieldName, fieldName)
			} else if isMap(f) {
				ctx.w.P("        %s: dict.from_list(acc.%s),", fieldName, fieldName)
			} else {
				ctx.w.P("        %s: acc.%s,", fieldName, fieldName)
			}
		}
		ctx.w.P("      ))")
	}
	ctx.w.P("  }")
	ctx.w.P("}")
	ctx.w.P("")

	generateFieldHandler(ctx, typeName, accName, regular, oneofs)

	// Generate map entry decoders for any map fields.
	for _, f := range regular {
		if isMap(f) {
			generateMapEntryDecoder(ctx, f)
		}
	}
}

func generateAccType(ctx *genContext, accName string, regular []*protogen.Field, oneofs []oneofInfo) {
	ctx.w.P("type %s {", accName)
	ctx.w.P("  %s(", accName)
	var fields []string
	for _, f := range regular {
		fields = append(fields, fmt.Sprintf("    %s: %s", gleamfmt.FieldName(string(f.Desc.Name())), gleamAccFieldType(ctx, f)))
	}
	for _, oo := range oneofs {
		ooType := gleamfmt.ToPascalCase(string(oo.Desc.Name()))
		fields = append(fields, fmt.Sprintf("    %s: Result(%s, Nil)", gleamfmt.FieldName(string(oo.Desc.Name())), ooType))
	}
	ctx.w.P("%s,", strings.Join(fields, ",\n"))
	ctx.w.P("  )")
	ctx.w.P("}")
	ctx.w.P("")
}

// gleamAccFieldType returns the accumulator field type.
// Maps are accumulated as List(#(K, V)) then converted to Dict at the end.
func gleamAccFieldType(ctx *genContext, f *protogen.Field) string {
	if isMap(f) {
		mapEntry := f.Message
		keyField := mapEntry.Fields[0]
		valField := mapEntry.Fields[1]
		return fmt.Sprintf("List(#(%s, %s))", gleamScalarType(ctx, keyField), gleamScalarType(ctx, valField))
	}
	return gleamFieldType(ctx, f)
}

func generateFieldHandler(ctx *genContext, typeName, accName string, regular []*protogen.Field, oneofs []oneofInfo) {
	handlerName := gleamfmt.ToSnakeCase(typeName) + "_field_handler"

	ctx.w.P("fn %s(", handlerName)
	ctx.w.P("  field_number: Int,")
	ctx.w.P("  wire_type: Int,")
	ctx.w.P("  rest: BitArray,")
	ctx.w.P("  acc: %s,", accName)
	ctx.w.P(") -> Result(#(%s, BitArray), wire.DecodeError) {", accName)
	ctx.w.P("  case field_number {")

	constPrefix := "field_" + gleamfmt.ToSnakeCase(typeName)

	for _, f := range regular {
		fieldName := gleamfmt.FieldName(string(f.Desc.Name()))
		constName := constPrefix + "_" + string(f.Desc.Name())
		ctx.w.P("    n if n == %s ->", constName)
		generateFieldDecode(ctx, f, fieldName, accName)
	}

	for _, oo := range oneofs {
		for _, f := range oo.Fields {
			constName := fmt.Sprintf("oneof_%s", string(f.Desc.Name()))
			ctx.w.P("    n if n == %s ->", constName)
			generateOneofFieldDecode(ctx, f, oo, accName)
		}
	}

	ctx.w.P("    _ ->")
	ctx.w.P("      case wire.skip_field(rest, wire_type) {")
	ctx.w.P("        Ok(r) -> Ok(#(acc, r))")
	ctx.w.P("        Error(e) -> Error(e)")
	ctx.w.P("      }")
	ctx.w.P("  }")
	ctx.w.P("}")
	ctx.w.P("")
}

func generateFieldDecode(ctx *genContext, f *protogen.Field, fieldName, accName string) {
	if isMap(f) {
		generateMapDecode(ctx, f, fieldName, accName)
		return
	}
	if f.Desc.IsList() {
		generateRepeatedDecode(ctx, f, fieldName, accName)
		return
	}
	if isOptional(f) {
		generateOptionalDecode(ctx, f, fieldName, accName)
		return
	}
	generateScalarDecode(ctx, f, fieldName, accName)
}

func generateScalarDecode(ctx *genContext, f *protogen.Field, fieldName, accName string) {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		ctx.w.P("      case wire.decode_len_delimited(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(bytes, r)) ->")
		ctx.w.P("          case wire.decode_string(bytes) {")
		ctx.w.P("            Error(e) -> Error(e)")
		ctx.w.P("            Ok(s) -> Ok(#(%s(..acc, %s: s), r))", accName, fieldName)
		ctx.w.P("          }")
		ctx.w.P("      }")

	case protoreflect.BoolKind:
		ctx.w.P("      case wire.decode_varint(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: v != 0), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		ctx.w.P("      case wire.decode_varint(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: v), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		ctx.w.P("      case wire.decode_varint(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: wire.zigzag_decode(v)), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.Fixed32Kind:
		ctx.w.P("      case wire.decode_fixed32(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: v), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.Fixed64Kind:
		ctx.w.P("      case wire.decode_fixed64(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: v), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.Sfixed32Kind:
		ctx.w.P("      case wire.decode_sfixed32(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: v), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.Sfixed64Kind:
		ctx.w.P("      case wire.decode_sfixed64(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: v), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.FloatKind:
		ctx.w.P("      case wire.decode_float32(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: v), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.DoubleKind:
		ctx.w.P("      case wire.decode_float64(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: v), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.BytesKind:
		ctx.w.P("      case wire.decode_len_delimited(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(bytes, r)) -> Ok(#(%s(..acc, %s: bytes), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.EnumKind:
		prefix := enumFnPrefix(ctx, f)
		ctx.w.P("      case wire.decode_varint(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) ->")
		ctx.w.P("          case %s_from_int(v) {", prefix)
		ctx.w.P("            Error(e) -> Error(e)")
		ctx.w.P("            Ok(ev) -> Ok(#(%s(..acc, %s: ev), r))", accName, fieldName)
		ctx.w.P("          }")
		ctx.w.P("      }")

	case protoreflect.MessageKind:
		dec := decoderFnName(ctx, f.Message)
		ctx.w.P("      case wire.decode_len_delimited(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(body, r)) ->")
		ctx.w.P("          case %s(body) {", dec)
		ctx.w.P("            Error(e) -> Error(e)")
		ctx.w.P("            Ok(m) -> Ok(#(%s(..acc, %s: m), r))", accName, fieldName)
		ctx.w.P("          }")
		ctx.w.P("      }")

	default:
		ctx.w.P("      case wire.skip_field(rest, wire.wire_len) {")
		ctx.w.P("        Ok(r) -> Ok(#(acc, r))")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("      }")
	}
}

func generateOptionalDecode(ctx *genContext, f *protogen.Field, fieldName, accName string) {
	// Optional fields wrap value in Ok() on decode.
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		ctx.w.P("      case wire.decode_len_delimited(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(bytes, r)) ->")
		ctx.w.P("          case wire.decode_string(bytes) {")
		ctx.w.P("            Error(e) -> Error(e)")
		ctx.w.P("            Ok(s) -> Ok(#(%s(..acc, %s: Ok(s)), r))", accName, fieldName)
		ctx.w.P("          }")
		ctx.w.P("      }")

	case protoreflect.BoolKind:
		ctx.w.P("      case wire.decode_varint(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: Ok(v != 0)), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		ctx.w.P("      case wire.decode_varint(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: Ok(v)), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.FloatKind:
		ctx.w.P("      case wire.decode_float32(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: Ok(v)), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.DoubleKind:
		ctx.w.P("      case wire.decode_float64(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: Ok(v)), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.BytesKind:
		ctx.w.P("      case wire.decode_len_delimited(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(bytes, r)) -> Ok(#(%s(..acc, %s: Ok(bytes)), r))", accName, fieldName)
		ctx.w.P("      }")

	case protoreflect.EnumKind:
		prefix := enumFnPrefix(ctx, f)
		ctx.w.P("      case wire.decode_varint(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) ->")
		ctx.w.P("          case %s_from_int(v) {", prefix)
		ctx.w.P("            Error(e) -> Error(e)")
		ctx.w.P("            Ok(ev) -> Ok(#(%s(..acc, %s: Ok(ev)), r))", accName, fieldName)
		ctx.w.P("          }")
		ctx.w.P("      }")

	case protoreflect.MessageKind:
		dec := decoderFnName(ctx, f.Message)
		ctx.w.P("      case wire.decode_len_delimited(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(body, r)) ->")
		ctx.w.P("          case %s(body) {", dec)
		ctx.w.P("            Error(e) -> Error(e)")
		ctx.w.P("            Ok(m) -> Ok(#(%s(..acc, %s: Ok(m)), r))", accName, fieldName)
		ctx.w.P("          }")
		ctx.w.P("      }")

	default:
		generateScalarDecode(ctx, f, fieldName, accName)
	}
}

func generateRepeatedDecode(ctx *genContext, f *protogen.Field, fieldName, accName string) {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		ctx.w.P("      case wire.decode_len_delimited(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(bytes, r)) ->")
		ctx.w.P("          case wire.decode_string(bytes) {")
		ctx.w.P("            Error(e) -> Error(e)")
		ctx.w.P("            Ok(s) -> Ok(#(%s(..acc, %s: [s, ..acc.%s]), r))", accName, fieldName, fieldName)
		ctx.w.P("          }")
		ctx.w.P("      }")

	case protoreflect.MessageKind:
		dec := decoderFnName(ctx, f.Message)
		ctx.w.P("      case wire.decode_len_delimited(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(body, r)) ->")
		ctx.w.P("          case %s(body) {", dec)
		ctx.w.P("            Error(e) -> Error(e)")
		ctx.w.P("            Ok(m) -> Ok(#(%s(..acc, %s: [m, ..acc.%s]), r))", accName, fieldName, fieldName)
		ctx.w.P("          }")
		ctx.w.P("      }")

	case protoreflect.BytesKind:
		ctx.w.P("      case wire.decode_len_delimited(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(bytes, r)) -> Ok(#(%s(..acc, %s: [bytes, ..acc.%s]), r))", accName, fieldName, fieldName)
		ctx.w.P("      }")

	case protoreflect.EnumKind:
		prefix := enumFnPrefix(ctx, f)
		ctx.w.P("      case wire.decode_varint(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) ->")
		ctx.w.P("          case %s_from_int(v) {", prefix)
		ctx.w.P("            Error(e) -> Error(e)")
		ctx.w.P("            Ok(ev) -> Ok(#(%s(..acc, %s: [ev, ..acc.%s]), r))", accName, fieldName, fieldName)
		ctx.w.P("          }")
		ctx.w.P("      }")

	default:
		// Repeated scalars (int, bool, etc.) — varint.
		ctx.w.P("      case wire.decode_varint(rest) {")
		ctx.w.P("        Error(e) -> Error(e)")
		ctx.w.P("        Ok(#(v, r)) -> Ok(#(%s(..acc, %s: [v, ..acc.%s]), r))", accName, fieldName, fieldName)
		ctx.w.P("      }")
	}
}

func generateMapDecode(ctx *genContext, f *protogen.Field, fieldName, accName string) {
	mapEntry := f.Message
	keyField := mapEntry.Fields[0]
	valField := mapEntry.Fields[1]

	// Decode the map entry as a length-delimited message, then parse key/value.
	ctx.w.P("      case wire.decode_len_delimited(rest) {")
	ctx.w.P("        Error(e) -> Error(e)")
	ctx.w.P("        Ok(#(body, r)) ->")
	ctx.w.P("          case decode_map_entry_%s(body) {", string(f.Desc.Name()))
	ctx.w.P("            Error(e) -> Error(e)")
	ctx.w.P("            Ok(#(k, v)) -> Ok(#(%s(..acc, %s: [#(k, v), ..acc.%s]), r))", accName, fieldName, fieldName)
	ctx.w.P("          }")
	ctx.w.P("      }")

	// Also generate the map entry decoder after the field handler.
	// We'll store it to emit later. For simplicity, emit it right here
	// since Gleam allows functions in any order within a module.
	_ = keyField
	_ = valField
}

// generateMapEntryDecoder emits a dedicated decoder for a map entry.
// Called after the field handler for the parent message.
func generateMapEntryDecoder(ctx *genContext, f *protogen.Field) {
	mapEntry := f.Message
	keyField := mapEntry.Fields[0]
	valField := mapEntry.Fields[1]

	keyType := gleamScalarType(ctx, keyField)
	valType := gleamScalarType(ctx, valField)
	keyDefault := scalarDefault(ctx, keyField)
	valDefault := scalarDefault(ctx, valField)

	fnName := "decode_map_entry_" + string(f.Desc.Name())
	ctx.w.P("fn %s(buf: BitArray) -> Result(#(%s, %s), wire.DecodeError) {", fnName, keyType, valType)
	ctx.w.P("  decode_map_entry_loop_%s(buf, %s, %s)", string(f.Desc.Name()), keyDefault, valDefault)
	ctx.w.P("}")
	ctx.w.P("")

	loopName := "decode_map_entry_loop_" + string(f.Desc.Name())
	ctx.w.P("fn %s(buf: BitArray, key: %s, val: %s) -> Result(#(%s, %s), wire.DecodeError) {", loopName, keyType, valType, keyType, valType)
	ctx.w.P("  case buf {")
	ctx.w.P("    <<>> -> Ok(#(key, val))")
	ctx.w.P("    _ ->")
	ctx.w.P("      case wire.decode_tag(buf) {")
	ctx.w.P("        Error(e) -> Error(e)")
	ctx.w.P("        Ok(#(field_number, wire_type, rest2)) ->")
	ctx.w.P("          case field_number {")

	// Field 1 = key
	ctx.w.P("            1 ->")
	emitMapFieldDecode(ctx, keyField, "key", "val", loopName, true)

	// Field 2 = value
	ctx.w.P("            2 ->")
	emitMapFieldDecode(ctx, valField, "key", "val", loopName, false)

	ctx.w.P("            _ ->")
	ctx.w.P("              case wire.skip_field(rest2, wire_type) {")
	ctx.w.P("                Ok(r) -> %s(r, key, val)", loopName)
	ctx.w.P("                Error(e) -> Error(e)")
	ctx.w.P("              }")
	ctx.w.P("          }")
	ctx.w.P("      }")
	ctx.w.P("  }")
	ctx.w.P("}")
	ctx.w.P("")
}

func emitMapFieldDecode(ctx *genContext, f *protogen.Field, keyVar, valVar, loopName string, isKey bool) {
	target := valVar
	other := keyVar
	if isKey {
		target = keyVar
		other = valVar
	}

	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		ctx.w.P("              case wire.decode_len_delimited(rest2) {")
		ctx.w.P("                Error(e) -> Error(e)")
		ctx.w.P("                Ok(#(bytes, r)) ->")
		ctx.w.P("                  case wire.decode_string(bytes) {")
		ctx.w.P("                    Error(e) -> Error(e)")
		if isKey {
			ctx.w.P("                    Ok(s) -> %s(r, s, %s)", loopName, other)
		} else {
			ctx.w.P("                    Ok(s) -> %s(r, %s, s)", loopName, other)
		}
		ctx.w.P("                  }")
		ctx.w.P("              }")

	case protoreflect.BoolKind:
		ctx.w.P("              case wire.decode_varint(rest2) {")
		ctx.w.P("                Error(e) -> Error(e)")
		if isKey {
			ctx.w.P("                Ok(#(v, r)) -> %s(r, v != 0, %s)", loopName, other)
		} else {
			ctx.w.P("                Ok(#(v, r)) -> %s(r, %s, v != 0)", loopName, other)
		}
		ctx.w.P("              }")

	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		ctx.w.P("              case wire.decode_varint(rest2) {")
		ctx.w.P("                Error(e) -> Error(e)")
		if isKey {
			ctx.w.P("                Ok(#(v, r)) -> %s(r, v, %s)", loopName, other)
		} else {
			ctx.w.P("                Ok(#(v, r)) -> %s(r, %s, v)", loopName, other)
		}
		ctx.w.P("              }")

	case protoreflect.EnumKind:
		prefix := enumFnPrefix(ctx, f)
		ctx.w.P("              case wire.decode_varint(rest2) {")
		ctx.w.P("                Error(e) -> Error(e)")
		ctx.w.P("                Ok(#(v, r)) ->")
		ctx.w.P("                  case %s_from_int(v) {", prefix)
		ctx.w.P("                    Error(e) -> Error(e)")
		if isKey {
			ctx.w.P("                    Ok(ev) -> %s(r, ev, %s)", loopName, other)
		} else {
			ctx.w.P("                    Ok(ev) -> %s(r, %s, ev)", loopName, other)
		}
		ctx.w.P("                  }")
		ctx.w.P("              }")

	case protoreflect.MessageKind:
		dec := decoderFnName(ctx, f.Message)
		ctx.w.P("              case wire.decode_len_delimited(rest2) {")
		ctx.w.P("                Error(e) -> Error(e)")
		ctx.w.P("                Ok(#(body, r)) ->")
		ctx.w.P("                  case %s(body) {", dec)
		ctx.w.P("                    Error(e) -> Error(e)")
		if isKey {
			ctx.w.P("                    Ok(m) -> %s(r, m, %s)", loopName, other)
		} else {
			ctx.w.P("                    Ok(m) -> %s(r, %s, m)", loopName, other)
		}
		ctx.w.P("                  }")
		ctx.w.P("              }")

	default:
		// Fallback: varint
		ctx.w.P("              case wire.decode_varint(rest2) {")
		ctx.w.P("                Error(e) -> Error(e)")
		_ = target
		if isKey {
			ctx.w.P("                Ok(#(v, r)) -> %s(r, v, %s)", loopName, other)
		} else {
			ctx.w.P("                Ok(#(v, r)) -> %s(r, %s, v)", loopName, other)
		}
		ctx.w.P("              }")
	}
}

func generateOneofFieldDecode(ctx *genContext, f *protogen.Field, oo oneofInfo, accName string) {
	variantName := gleamfmt.OneofVariantName(string(f.Desc.Name()))
	ooField := gleamfmt.FieldName(string(oo.Desc.Name()))
	dec := decoderFnName(ctx, f.Message)

	ctx.w.P("      case wire.decode_len_delimited(rest) {")
	ctx.w.P("        Error(e) -> Error(e)")
	ctx.w.P("        Ok(#(body, r)) ->")
	ctx.w.P("          case %s(body) {", dec)
	ctx.w.P("            Error(e) -> Error(e)")
	ctx.w.P("            Ok(m) -> Ok(#(%s(..acc, %s: Ok(%s(m))), r))", accName, ooField, variantName)
	ctx.w.P("          }")
	ctx.w.P("      }")
}

func generateDecodeFieldsHelper(w *writer) {
	w.P("// --- Shared decode helpers -------------------------------------------")
	w.P("")
	w.P("fn decode_fields(")
	w.P("  buf: BitArray,")
	w.P("  acc: a,")
	w.P("  handler: fn(Int, Int, BitArray, a) ->")
	w.P("    Result(#(a, BitArray), wire.DecodeError),")
	w.P(") -> Result(a, wire.DecodeError) {")
	w.P("  case buf {")
	w.P("    <<>> -> Ok(acc)")
	w.P("    _ ->")
	w.P("      case wire.decode_tag(buf) {")
	w.P("        Error(e) -> Error(e)")
	w.P("        Ok(#(field_number, wire_type, rest)) ->")
	w.P("          case handler(field_number, wire_type, rest, acc) {")
	w.P("            Error(e) -> Error(e)")
	w.P("            Ok(#(new_acc, new_rest)) ->")
	w.P("              decode_fields(new_rest, new_acc, handler)")
	w.P("          }")
	w.P("      }")
	w.P("  }")
	w.P("}")
	w.P("")
}

// defaultValue returns the Gleam default for an accumulator field.
// Note: map accumulators use List(#(K,V)) internally, converted to Dict later.
func defaultValue(ctx *genContext, f *protogen.Field) string {
	if isMap(f) {
		return "[]" // Accumulated as list, converted to dict at the end.
	}
	if f.Desc.IsList() {
		return "[]"
	}
	if isOptional(f) {
		return "Error(Nil)"
	}
	return scalarDefault(ctx, f)
}

// publicDefaultValue returns the Gleam default for a public type field.
// Used in zero-value constructors. Maps are Dict, not List.
func publicDefaultValue(ctx *genContext, f *protogen.Field) string {
	if isMap(f) {
		return "dict.new()"
	}
	if f.Desc.IsList() {
		return "[]"
	}
	if isOptional(f) {
		return "Error(Nil)"
	}
	return scalarDefault(ctx, f)
}

func scalarDefault(ctx *genContext, f *protogen.Field) string {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return "\"\""
	case protoreflect.BoolKind:
		return "False"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return "0.0"
	case protoreflect.BytesKind:
		return "<<>>"
	case protoreflect.EnumKind:
		if f.Enum != nil && len(f.Enum.Values) > 0 {
			return gleamfmt.EnumVariantName(string(f.Enum.Desc.Name()), string(f.Enum.Values[0].Desc.Name()))
		}
		return "0"
	case protoreflect.MessageKind:
		if f.Message != nil {
			return zeroConstructor(ctx, f.Message)
		}
		return "0"
	default:
		return "0"
	}
}

// zeroConstructor generates a zero-value constructor call for a message.
func zeroConstructor(ctx *genContext, msg *protogen.Message) string {
	typeName := fullMsgTypeName(msg)

	// Check if it's from another file.
	if msg.Desc.ParentFile().Path() != ctx.file.Desc.Path() {
		mod := gleamfmt.ModuleName(string(msg.Desc.ParentFile().Path()), ctx.packagePrefix)
		shortMod := mod
		if idx := strings.LastIndex(mod, "/"); idx >= 0 {
			shortMod = mod[idx+1:]
		}
		typeName = shortMod + "." + typeName
	}

	fields := regularFields(msg)
	oneofs := collectOneofs(msg)

	if len(fields) == 0 && len(oneofs) == 0 {
		return typeName
	}

	parts := []string{}
	for _, f := range fields {
		parts = append(parts, fmt.Sprintf("%s: %s", gleamfmt.FieldName(string(f.Desc.Name())), publicDefaultValue(ctx, f)))
	}
	for _, oo := range oneofs {
		// Use the first variant with a zero-constructed inner message.
		if len(oo.Fields) > 0 {
			first := oo.Fields[0]
			variantName := gleamfmt.OneofVariantName(string(first.Desc.Name()))
			innerZero := zeroConstructor(ctx, first.Message)
			parts = append(parts, fmt.Sprintf("%s: %s(%s)", gleamfmt.FieldName(string(oo.Desc.Name())), variantName, innerZero))
		}
	}

	return fmt.Sprintf("%s(%s)", typeName, strings.Join(parts, ", "))
}
