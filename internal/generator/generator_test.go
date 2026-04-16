package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGenerateScanProto runs protoc-gen-gleam against the aegis scan.proto
// and verifies the output matches the golden file.
func TestGenerateScanProto(t *testing.T) {
	// Build the plugin binary.
	tmpDir := t.TempDir()
	pluginBin := filepath.Join(tmpDir, "protoc-gen-gleam")
	root := findProjectRoot(t)
	build := exec.Command("go", "build", "-o", pluginBin, "./cmd/protoc-gen-gleam/")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building plugin: %s\n%s", err, out)
	}

	// Run protoc with our plugin.
	outDir := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	protoc := exec.Command("protoc",
		"--plugin=protoc-gen-gleam="+pluginBin,
		"--gleam_out="+outDir,
		"--gleam_opt=package_prefix=aegis_api/infra/proto",
		"--proto_path="+filepath.Join(root, "testdata"),
		filepath.Join(root, "testdata", "aegis", "scan", "v1", "scan.proto"),
	)
	if out, err := protoc.CombinedOutput(); err != nil {
		t.Fatalf("protoc: %s\n%s", err, out)
	}

	// Read generated output.
	gotPath := filepath.Join(outDir, "aegis_api", "infra", "proto", "scan.gleam")
	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("reading generated file: %s", err)
	}

	// Compare against golden file.
	goldenPath := filepath.Join(root, "testdata", "golden", "scan.gleam")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("Golden file updated")
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file (run with UPDATE_GOLDEN=1 to create): %s", err)
	}

	if string(got) != string(golden) {
		t.Errorf("generated output differs from golden file.\n"+
			"Run: UPDATE_GOLDEN=1 go test ./internal/generator/ -run TestGenerateScanProto\n"+
			"Then diff testdata/golden/scan.gleam to review changes.")
	}
}

// TestGeneratedGleamCompiles verifies the generated Gleam code compiles
// in a temporary Gleam project.
func TestGeneratedGleamCompiles(t *testing.T) {
	if _, err := exec.LookPath("gleam"); err != nil {
		t.Skip("gleam not in PATH")
	}

	root := findProjectRoot(t)
	goldenPath := filepath.Join(root, "testdata", "golden", "scan.gleam")
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Skip("no golden file; run TestGenerateScanProto with UPDATE_GOLDEN=1 first")
	}

	tmpDir := t.TempDir()

	// Set up a minimal Gleam project.
	gleamToml := `name = "proto_compile_test"
version = "0.1.0"

[dependencies]
gleam_stdlib = ">= 0.44.0 and < 2.0.0"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "gleam.toml"), []byte(gleamToml), 0o644); err != nil {
		t.Fatal(err)
	}

	// Copy wire runtime.
	wireDir := filepath.Join(tmpDir, "src", "gleam_protobuf")
	if err := os.MkdirAll(wireDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wireSrc, err := os.ReadFile(filepath.Join(root, "runtime", "src", "gleam_protobuf", "wire.gleam"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wireDir, "wire.gleam"), wireSrc, 0o644); err != nil {
		t.Fatal(err)
	}

	// Copy generated code.
	genDir := filepath.Join(tmpDir, "src", "aegis_api", "infra", "proto")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(genDir, "scan.gleam"), golden, 0o644); err != nil {
		t.Fatal(err)
	}

	// Run gleam build.
	cmd := exec.Command("gleam", "build")
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gleam build failed:\n%s\n%s", err, out)
	}
}

// TestComprehensiveProtoCompiles generates from the comprehensive test proto
// and verifies the output compiles. This exercises maps, optional, nested
// messages, all scalar types, repeated messages/enums, oneof, and empty messages.
func TestComprehensiveProtoCompiles(t *testing.T) {
	if _, err := exec.LookPath("gleam"); err != nil {
		t.Skip("gleam not in PATH")
	}

	root := findProjectRoot(t)

	// Build plugin.
	tmpBuild := t.TempDir()
	pluginBin := filepath.Join(tmpBuild, "protoc-gen-gleam")
	build := exec.Command("go", "build", "-o", pluginBin, "./cmd/protoc-gen-gleam/")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building plugin: %s\n%s", err, out)
	}

	// Generate.
	outDir := filepath.Join(tmpBuild, "gen")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	protoc := exec.Command("protoc",
		"--plugin=protoc-gen-gleam="+pluginBin,
		"--gleam_out="+outDir,
		"--gleam_opt=package_prefix=example/proto",
		"--proto_path="+filepath.Join(root, "testdata"),
		filepath.Join(root, "testdata", "test", "v1", "comprehensive.proto"),
	)
	if out, err := protoc.CombinedOutput(); err != nil {
		t.Fatalf("protoc: %s\n%s", err, out)
	}

	got, err := os.ReadFile(filepath.Join(outDir, "example", "proto", "comprehensive.gleam"))
	if err != nil {
		t.Fatalf("reading generated file: %s", err)
	}

	// Set up Gleam project.
	gleamDir := t.TempDir()
	gleamToml := `name = "comprehensive_test"
version = "0.1.0"
[dependencies]
gleam_stdlib = ">= 0.44.0 and < 2.0.0"
`
	if err := os.WriteFile(filepath.Join(gleamDir, "gleam.toml"), []byte(gleamToml), 0o644); err != nil {
		t.Fatal(err)
	}
	wireDir := filepath.Join(gleamDir, "src", "gleam_protobuf")
	if err := os.MkdirAll(wireDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wireSrc, err := os.ReadFile(filepath.Join(root, "runtime", "src", "gleam_protobuf", "wire.gleam"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wireDir, "wire.gleam"), wireSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	genDir := filepath.Join(gleamDir, "src", "example", "proto")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(genDir, "comprehensive.gleam"), got, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("gleam", "build")
	cmd.Dir = gleamDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gleam build failed:\n%s\n%s", err, out)
	}
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the test file to find go.mod.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}
