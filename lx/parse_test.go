package lx

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	defs := []CommandDef{
		{Name: "head", Short: "h", Type: CmdInterleaved, ValueType: ValueNumber},
		{Name: "tail", Short: "t", Type: CmdInterleaved, ValueType: ValueNumber},
		{Name: "config", Short: "f", Type: CmdGlobal, ValueType: ValueAny},
		{Name: "line-numbers", Short: "l", Type: CmdGlobal, ValueType: ValueNone},
		{Name: "verbose", Short: "v", Type: CmdGlobal, ValueType: ValueNone},
	}

	tests := []struct {
		name      string
		args      []string
		wantOps   []Op
		wantGlo   map[string]string
		wantErr   bool
		errSubstr string
	}{
		// --- Success Cases ---
		{
			name: "sticky number",
			args: []string{"-h5", "file.txt"},
			wantOps: []Op{
				{Action: "head", Value: "5"},
				{Action: "FILE", Value: "file.txt"},
			},
			wantGlo: map[string]string{},
		},
		{
			name: "joined bool flags",
			args: []string{"-lv", "file.txt"},
			wantOps: []Op{
				{Action: "FILE", Value: "file.txt"},
			},
			wantGlo: map[string]string{
				"line-numbers": "true",
				"verbose":      "true",
			},
		},
		{
			name: "joined bool and sticky number",
			args: []string{"-lh5", "file.txt"},
			wantOps: []Op{
				{Action: "head", Value: "5"},
				{Action: "FILE", Value: "file.txt"},
			},
			wantGlo: map[string]string{
				"line-numbers": "true",
			},
		},
		{
			name: "space separated number",
			args: []string{"-h", "5", "file.txt"},
			wantOps: []Op{
				{Action: "head", Value: "5"},
				{Action: "FILE", Value: "file.txt"},
			},
			wantGlo: map[string]string{},
		},
		{
			name: "long flag with equals",
			args: []string{"--head=10", "file.go"},
			wantOps: []Op{
				{Action: "head", Value: "10"},
				{Action: "FILE", Value: "file.go"},
			},
			wantGlo: map[string]string{},
		},
		{
			name: "long flag space separated",
			args: []string{"--config", "my.yaml", "main.go"},
			wantOps: []Op{
				{Action: "FILE", Value: "main.go"},
			},
			wantGlo: map[string]string{
				"config": "my.yaml",
			},
		},

		// --- Error Cases ---
		{
			name:      "unknown short flag",
			args:      []string{"-z"},
			wantErr:   true,
			errSubstr: "unknown short flag: -z",
		},
		{
			name:      "unknown long flag",
			args:      []string{"--foo"},
			wantErr:   true,
			errSubstr: "unknown flag: --foo",
		},
		{
			name:      "missing value for short flag",
			args:      []string{"-h"},
			wantErr:   true,
			errSubstr: "flag -h requires a value",
		},
		{
			name:      "missing value for long flag",
			args:      []string{"--head"},
			wantErr:   true,
			errSubstr: "flag --head requires a value",
		},
		{
			name:    "invalid number format",
			args:    []string{"-h", "foo"},
			wantErr: true,
			// FIXED: Expecting "--head" not "-head"
			errSubstr: "flag --head expects a number, got \"foo\"",
		},
		{
			name:      "bool flag given value with equals",
			args:      []string{"--verbose=true"},
			wantErr:   true,
			errSubstr: "flag --verbose does not take a value",
		},
		{
			name:      "sticky string disallowed (short)",
			args:      []string{"-fconfig.yaml"},
			wantErr:   true,
			errSubstr: "does not support sticky/clustered values",
		},
		{
			name:      "clustered string disallowed (short)",
			args:      []string{"-lf", "config.yaml"},
			wantErr:   true,
			errSubstr: "does not support sticky/clustered values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args, defs)

			// Check Error
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error, got nil")
				} else if tt.errSubstr != "" && !contains(err.Error(), tt.errSubstr) {
					t.Errorf("Parse() error = %q, want substr %q", err.Error(), tt.errSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse() unexpected error: %v", err)
			}

			// Check Ops
			if !reflect.DeepEqual(got.Ops, tt.wantOps) {
				t.Errorf("Ops mismatch.\nGot:  %+v\nWant: %+v", got.Ops, tt.wantOps)
			}

			// Check Globals
			if !reflect.DeepEqual(got.Globals, tt.wantGlo) {
				t.Errorf("Globals mismatch.\nGot:  %+v\nWant: %+v", got.Globals, tt.wantGlo)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
