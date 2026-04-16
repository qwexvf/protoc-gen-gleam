package generator

import (
	"fmt"
	"strings"

	gleamfmt "github.com/qwexvf/protoc-gen-gleam/internal/gleam"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func generateEncoder(ctx *genContext, msg *protogen.Message, typeName string, oneofs []oneofInfo) {
	fnName := "encode_" + gleamfmt.ToSnakeCase(typeName)
	regular := regularFields(msg)

	if len(regular) == 0 && len(oneofs) == 0 {
		ctx.w.P("pub fn %s(_msg: %s) -> BitArray {", fnName, typeName)
		ctx.w.P("  <<>>")
		ctx.w.P("}")
		ctx.w.P("")
		return
	}

	ctx.w.P("pub fn %s(msg: %s) -> BitArray {", fnName, typeName)

	// Destructure.
	var fieldNames []string
	for _, f := range regular {
		fieldNames = append(fieldNames, gleamfmt.FieldName(string(f.Desc.Name())))
	}
	for _, oo := range oneofs {
		fieldNames = append(fieldNames, gleamfmt.FieldName(string(oo.Desc.Name())))
	}
	ctx.w.P("  let %s(%s) = msg", typeName, joinComma(fieldNames))

	ctx.w.P("  <<")
	for _, f := range regular {
		fieldName := gleamfmt.FieldName(string(f.Desc.Name()))
		constName := "field_" + gleamfmt.ToSnakeCase(typeName) + "_" + string(f.Desc.Name())
		ctx.w.P("    %s,", encodeFieldExpr(ctx, f, fieldName, constName))
	}
	for _, oo := range oneofs {
		ooFieldName := gleamfmt.FieldName(string(oo.Desc.Name()))
		encoderName := "encode_" + gleamfmt.ToSnakeCase(string(oo.Desc.Name())) + "_payload"
		ctx.w.P("    %s(%s):bits,", encoderName, ooFieldName)
	}
	ctx.w.P("  >>")
	ctx.w.P("}")
	ctx.w.P("")

	for _, oo := range oneofs {
		generateOneofEncoder(ctx, oo, typeName)
	}
}

func generateOneofEncoder(ctx *genContext, oo oneofInfo, parentTypeName string) {
	ooTypeName := gleamfmt.ToPascalCase(string(oo.Desc.Name()))
	fnName := "encode_" + gleamfmt.ToSnakeCase(string(oo.Desc.Name())) + "_payload"

	ctx.w.P("fn %s(p: %s) -> BitArray {", fnName, ooTypeName)
	ctx.w.P("  case p {")
	for _, f := range oo.Fields {
		variantName := gleamfmt.OneofVariantName(string(f.Desc.Name()))
		constName := oneofConstName(parentTypeName, f)
		if f.Message != nil {
			// Message variant — encode as length-delimited sub-message.
			msgEncoder := encoderFnName(ctx, f.Message)
			ctx.w.P("    %s(m) -> wire.encode_message_field(%s, %s(m))", variantName, constName, msgEncoder)
		} else {
			// Scalar variant — encode directly.
			expr := encodeScalarExpr(ctx, f, "m", constName)
			ctx.w.P("    %s(m) -> <<%s>>", variantName, expr)
		}
	}
	ctx.w.P("  }")
	ctx.w.P("}")
	ctx.w.P("")
}

func encodeFieldExpr(ctx *genContext, f *protogen.Field, fieldName, constName string) string {
	// Map fields.
	if isMap(f) {
		return encodeMapExpr(ctx, f, fieldName, constName)
	}

	// Optional fields.
	if isOptional(f) {
		return encodeOptionalExpr(ctx, f, fieldName, constName)
	}

	// Repeated fields.
	if f.Desc.IsList() {
		return encodeRepeatedExpr(ctx, f, fieldName, constName)
	}

	// Scalar fields.
	return encodeScalarExpr(ctx, f, fieldName, constName)
}

func encodeScalarExpr(ctx *genContext, f *protogen.Field, fieldName, constName string) string {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return fmt.Sprintf("wire.encode_string_field(%s, %s):bits", constName, fieldName)
	case protoreflect.BoolKind:
		return fmt.Sprintf("wire.encode_bool_field(%s, %s):bits", constName, fieldName)
	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		return fmt.Sprintf("wire.encode_int_field(%s, %s):bits", constName, fieldName)
	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		return fmt.Sprintf("wire.encode_sint_field(%s, %s):bits", constName, fieldName)
	case protoreflect.Fixed32Kind:
		return fmt.Sprintf("wire.encode_fixed32_field(%s, %s):bits", constName, fieldName)
	case protoreflect.Fixed64Kind:
		return fmt.Sprintf("wire.encode_fixed64_field(%s, %s):bits", constName, fieldName)
	case protoreflect.Sfixed32Kind:
		return fmt.Sprintf("wire.encode_sfixed32_field(%s, %s):bits", constName, fieldName)
	case protoreflect.Sfixed64Kind:
		return fmt.Sprintf("wire.encode_sfixed64_field(%s, %s):bits", constName, fieldName)
	case protoreflect.FloatKind:
		return fmt.Sprintf("wire.encode_float_field(%s, %s):bits", constName, fieldName)
	case protoreflect.DoubleKind:
		return fmt.Sprintf("wire.encode_double_field(%s, %s):bits", constName, fieldName)
	case protoreflect.BytesKind:
		return fmt.Sprintf("wire.encode_bytes_field(%s, %s):bits", constName, fieldName)
	case protoreflect.EnumKind:
		prefix := enumFnPrefix(ctx, f)
		return fmt.Sprintf("wire.encode_int_field(%s, %s_to_int(%s)):bits", constName, prefix, fieldName)
	case protoreflect.MessageKind:
		enc := encoderFnName(ctx, f.Message)
		return fmt.Sprintf("wire.encode_message_field(%s, %s(%s)):bits", constName, enc, fieldName)
	default:
		return fmt.Sprintf("<<>>:bits // unsupported kind: %s", f.Desc.Kind())
	}
}

func encodeOptionalExpr(ctx *genContext, f *protogen.Field, fieldName, constName string) string {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return fmt.Sprintf("wire.encode_optional_string_field(%s, %s):bits", constName, fieldName)
	case protoreflect.BoolKind:
		return fmt.Sprintf("wire.encode_optional_bool_field(%s, %s):bits", constName, fieldName)
	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		return fmt.Sprintf("wire.encode_optional_int_field(%s, %s):bits", constName, fieldName)
	case protoreflect.FloatKind:
		return fmt.Sprintf("wire.encode_optional_float_field(%s, %s):bits", constName, fieldName)
	case protoreflect.DoubleKind:
		return fmt.Sprintf("wire.encode_optional_double_field(%s, %s):bits", constName, fieldName)
	case protoreflect.BytesKind:
		return fmt.Sprintf("wire.encode_optional_bytes_field(%s, %s):bits", constName, fieldName)
	case protoreflect.EnumKind:
		// For optional enum, we generate a case expression inline.
		prefix := enumFnPrefix(ctx, f)
		return fmt.Sprintf("wire.encode_optional_int_field(%s, case %s { Ok(ev) -> Ok(%s_to_int(ev)) Error(e) -> Error(e) }):bits",
			constName, fieldName, prefix)
	case protoreflect.MessageKind:
		enc := encoderFnName(ctx, f.Message)
		return fmt.Sprintf("wire.encode_optional_message_field(%s, %s, %s):bits", constName, fieldName, enc)
	default:
		return fmt.Sprintf("<<>>:bits // unsupported optional kind: %s", f.Desc.Kind())
	}
}

func encodeRepeatedExpr(ctx *genContext, f *protogen.Field, fieldName, constName string) string {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return fmt.Sprintf("wire.encode_repeated_strings(%s, %s):bits", constName, fieldName)
	case protoreflect.MessageKind:
		enc := encoderFnName(ctx, f.Message)
		return fmt.Sprintf("wire.encode_repeated_messages(%s, %s, %s):bits", constName, fieldName, enc)
	case protoreflect.BytesKind:
		return fmt.Sprintf("wire.encode_repeated_messages(%s, %s, fn(b) { b }):bits", constName, fieldName)
	case protoreflect.EnumKind:
		prefix := enumFnPrefix(ctx, f)
		return fmt.Sprintf("wire.encode_repeated_ints(%s, list.map(%s, %s_to_int)):bits", constName, fieldName, prefix)
	case protoreflect.BoolKind:
		return fmt.Sprintf("wire.encode_repeated_ints(%s, list.map(%s, fn(b) { case b { True -> 1 False -> 0 } })):bits", constName, fieldName)
	default:
		// int32, int64, uint32, uint64, sint32, sint64, etc.
		return fmt.Sprintf("wire.encode_repeated_ints(%s, %s):bits", constName, fieldName)
	}
}

func encodeMapExpr(ctx *genContext, f *protogen.Field, fieldName, constName string) string {
	mapEntry := f.Message
	keyField := mapEntry.Fields[0]
	valField := mapEntry.Fields[1]

	keyEnc := mapKeyEncoder(ctx, keyField)
	valEnc := mapValueEncoder(ctx, valField)

	return fmt.Sprintf("wire.encode_map_entries(%s, dict.to_list(%s), %s, %s):bits",
		constName, fieldName, keyEnc, valEnc)
}

func mapKeyEncoder(_ *genContext, f *protogen.Field) string {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return "wire.encode_string_field"
	case protoreflect.BoolKind:
		return "wire.encode_bool_field"
	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		return "wire.encode_int_field"
	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		return "wire.encode_sint_field"
	case protoreflect.Fixed32Kind:
		return "wire.encode_fixed32_field"
	case protoreflect.Fixed64Kind:
		return "wire.encode_fixed64_field"
	case protoreflect.Sfixed32Kind:
		return "wire.encode_sfixed32_field"
	case protoreflect.Sfixed64Kind:
		return "wire.encode_sfixed64_field"
	default:
		return "wire.encode_string_field // unsupported map key type"
	}
}

func mapValueEncoder(ctx *genContext, f *protogen.Field) string {
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return "wire.encode_string_field"
	case protoreflect.BoolKind:
		return "wire.encode_bool_field"
	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		return "wire.encode_int_field"
	case protoreflect.Sint32Kind, protoreflect.Sint64Kind:
		return "wire.encode_sint_field"
	case protoreflect.FloatKind:
		return "wire.encode_float_field"
	case protoreflect.DoubleKind:
		return "wire.encode_double_field"
	case protoreflect.BytesKind:
		return "wire.encode_bytes_field"
	case protoreflect.Fixed32Kind:
		return "wire.encode_fixed32_field"
	case protoreflect.Fixed64Kind:
		return "wire.encode_fixed64_field"
	case protoreflect.Sfixed32Kind:
		return "wire.encode_sfixed32_field"
	case protoreflect.Sfixed64Kind:
		return "wire.encode_sfixed64_field"
	case protoreflect.MessageKind:
		enc := encoderFnName(ctx, f.Message)
		return fmt.Sprintf("fn(fn_num, v) { wire.encode_message_field(fn_num, %s(v)) }", enc)
	case protoreflect.EnumKind:
		prefix := enumFnPrefix(ctx, f)
		return fmt.Sprintf("fn(fn_num, v) { wire.encode_int_field(fn_num, %s_to_int(v)) }", prefix)
	default:
		return "wire.encode_string_field // unsupported map value type"
	}
}

func joinComma(ss []string) string {
	return strings.Join(ss, ", ")
}
