// protoc-gen-gleam is a protoc/buf plugin that generates idiomatic Gleam
// types and encode/decode functions from proto3 definitions.
//
// Install:
//
//	go install github.com/qwexvf/protoc-gen-gleam/cmd/protoc-gen-gleam@latest
//
// Usage with buf (buf.gen.yaml):
//
//	plugins:
//	  - name: gleam
//	    out: src/my_app/proto
//	    opt: package_prefix=my_app/proto
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/qwexvf/protoc-gen-gleam/internal/generator"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

var version = "dev"

func main() {
	var flags flag.FlagSet
	packagePrefix := flags.String("package_prefix", "", "Gleam module path prefix (e.g. my_app/proto)")
	showVersion := flags.Bool("version", false, "print version and exit")

	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("protoc-gen-gleam %s\n", version)
		os.Exit(0)
	}

	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {
		_ = showVersion
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			if err := generator.GenerateFile(gen, f, *packagePrefix); err != nil {
				return fmt.Errorf("generating %s: %w", f.Desc.Path(), err)
			}
		}
		return nil
	})
}
