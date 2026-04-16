package gleam

import "testing"

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"ScanMode", "scan_mode"},
		{"ScanStartCommand", "scan_start_command"},
		{"XMLParser", "xml_parser"},
		{"scanID", "scan_id"},
		{"Envelope", "envelope"},
		{"ScanFindingEvent", "scan_finding_event"},
		{"LogLevel", "log_level"},
	}
	for _, tt := range tests {
		if got := ToSnakeCase(tt.in); got != tt.want {
			t.Errorf("ToSnakeCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"scan_mode", "ScanMode"},
		{"start_command", "StartCommand"},
		{"scan_finding_event", "ScanFindingEvent"},
	}
	for _, tt := range tests {
		if got := ToPascalCase(tt.in); got != tt.want {
			t.Errorf("ToPascalCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestEnumVariantName(t *testing.T) {
	tests := []struct {
		enumName, valueName, want string
	}{
		{"ScanMode", "SCAN_MODE_UNSPECIFIED", "ScanModeUnspecified"},
		{"ScanMode", "SCAN_MODE_URL", "ScanModeUrl"},
		{"Severity", "SEVERITY_CRITICAL", "SeverityCritical"},
		{"LogLevel", "LOG_LEVEL_DEBUG", "LogLevelDebug"},
	}
	for _, tt := range tests {
		if got := EnumVariantName(tt.enumName, tt.valueName); got != tt.want {
			t.Errorf("EnumVariantName(%q, %q) = %q, want %q", tt.enumName, tt.valueName, got, tt.want)
		}
	}
}

func TestEnumToStringValue(t *testing.T) {
	tests := []struct {
		enumName, valueName, want string
	}{
		{"ScanMode", "SCAN_MODE_URL", "url"},
		{"Severity", "SEVERITY_CRITICAL", "critical"},
		{"LogLevel", "LOG_LEVEL_DEBUG", "debug"},
	}
	for _, tt := range tests {
		if got := EnumToStringValue(tt.enumName, tt.valueName); got != tt.want {
			t.Errorf("EnumToStringValue(%q, %q) = %q, want %q", tt.enumName, tt.valueName, got, tt.want)
		}
	}
}

func TestModuleName(t *testing.T) {
	tests := []struct {
		path, prefix, want string
	}{
		{"aegis/scan/v1/scan.proto", "", "scan"},
		{"aegis/scan/v1/scan.proto", "my_app/proto", "my_app/proto/scan"},
		{"foo.proto", "pkg", "pkg/foo"},
	}
	for _, tt := range tests {
		if got := ModuleName(tt.path, tt.prefix); got != tt.want {
			t.Errorf("ModuleName(%q, %q) = %q, want %q", tt.path, tt.prefix, got, tt.want)
		}
	}
}
