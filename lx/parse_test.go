package lx

import (
	"reflect"
	"strings"
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
			name: "double dash delimiter",
			args: []string{"-h", "5", "--", "-file-with-dash.txt", "normal.txt"},
			wantOps: []Op{
				{Action: "head", Value: "5", Type: CmdInterleaved},
				{Action: "FILE", Value: "-file-with-dash.txt", Type: CmdAction},
				{Action: "FILE", Value: "normal.txt", Type: CmdAction},
			},
			wantGlo: map[string]string{},
		},
		{
			name: "sticky number",
			args: []string{"-h5", "file.txt"},
			wantOps: []Op{
				{Action: "head", Value: "5", Type: CmdInterleaved},
				{Action: "FILE", Value: "file.txt", Type: CmdAction},
			},
			wantGlo: map[string]string{},
		},
		{
			name: "grouped bools with sticky number (-lvh5)",
			args: []string{"-lvh5", "file.txt"},
			wantOps: []Op{
				{Action: "head", Value: "5", Type: CmdInterleaved},
				{Action: "FILE", Value: "file.txt", Type: CmdAction},
			},
			wantGlo: map[string]string{
				"line-numbers": "true",
				"verbose":      "true",
			},
		},
		{
			name: "grouped bools with sticky string (-lvfconfig.yaml)",
			args: []string{"-lvfconfig.yaml", "main.go"},
			wantOps: []Op{
				{Action: "FILE", Value: "main.go", Type: CmdAction},
			},
			wantGlo: map[string]string{
				"line-numbers": "true",
				"verbose":      "true",
				"config":       "config.yaml",
			},
		},
		{
			name:    "sticky string alone (-fconfig.yaml)",
			args:    []string{"-fconfig.yaml"},
			wantOps: []Op{},
			wantGlo: map[string]string{
				"config": "config.yaml",
			},
		},
		{
			name: "grouped bools space separated number (-lvh 5)",
			args: []string{"-lvh", "5", "file.txt"},
			wantOps: []Op{
				{Action: "head", Value: "5", Type: CmdInterleaved},
				{Action: "FILE", Value: "file.txt", Type: CmdAction},
			},
			wantGlo: map[string]string{
				"line-numbers": "true",
				"verbose":      "true",
			},
		},
		{
			name: "long flag with equals",
			args: []string{"--head=10", "file.go"},
			wantOps: []Op{
				{Action: "head", Value: "10", Type: CmdInterleaved},
				{Action: "FILE", Value: "file.go", Type: CmdAction},
			},
			wantGlo: map[string]string{},
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
			name:      "invalid number format inside sticky",
			args:      []string{"-hfoo"},
			wantErr:   true,
			errSubstr: "flag --head expects a number, got \"foo\"",
		},
		{
			name:      "bool flag given value with equals",
			args:      []string{"--verbose=true"},
			wantErr:   true,
			errSubstr: "flag --verbose does not take a value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args, defs)

			// Check Error
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error, got nil")
				} else if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
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
