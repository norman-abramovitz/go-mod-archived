package main

import (
	"os"
	"testing"
)

func TestReorderArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "flags already before positional",
			args: []string{"cmd", "--files", "--tree", "path/go.mod"},
			want: []string{"cmd", "--files", "--tree", "path/go.mod"},
		},
		{
			name: "flags after positional",
			args: []string{"cmd", "path/go.mod", "--files", "--tree"},
			want: []string{"cmd", "--files", "--tree", "path/go.mod"},
		},
		{
			name: "mixed flags and positional",
			args: []string{"cmd", "--json", "path/go.mod", "--files", "--tree"},
			want: []string{"cmd", "--json", "--files", "--tree", "path/go.mod"},
		},
		{
			name: "no flags",
			args: []string{"cmd", "path/go.mod"},
			want: []string{"cmd", "path/go.mod"},
		},
		{
			name: "no positional args",
			args: []string{"cmd", "--files", "--json"},
			want: []string{"cmd", "--files", "--json"},
		},
		{
			name: "no args at all",
			args: []string{"cmd"},
			want: []string{"cmd"},
		},
		{
			name: "value flag with separate arg",
			args: []string{"cmd", "path/go.mod", "--workers", "30"},
			want: []string{"cmd", "--workers", "30", "path/go.mod"},
		},
		{
			name: "value flag with equals syntax",
			args: []string{"cmd", "path/go.mod", "--workers=30"},
			want: []string{"cmd", "--workers=30", "path/go.mod"},
		},
		{
			name: "value flag with single dash",
			args: []string{"cmd", "path/go.mod", "-workers", "30"},
			want: []string{"cmd", "-workers", "30", "path/go.mod"},
		},
		{
			name: "value flag between boolean flags",
			args: []string{"cmd", "--json", "path/go.mod", "--workers", "25", "--files"},
			want: []string{"cmd", "--json", "--workers", "25", "--files", "path/go.mod"},
		},
		{
			name: "all flags after positional",
			args: []string{"cmd", "path/go.mod", "--json", "--files", "--tree", "--direct-only", "--all", "--time"},
			want: []string{"cmd", "--json", "--files", "--tree", "--direct-only", "--all", "--time", "path/go.mod"},
		},
		{
			name: "go-version value flag with separate arg",
			args: []string{"cmd", "path/go.mod", "--go-version", "1.21.0"},
			want: []string{"cmd", "--go-version", "1.21.0", "path/go.mod"},
		},
		{
			name: "go-version value flag with equals syntax",
			args: []string{"cmd", "path/go.mod", "--go-version=1.21.0"},
			want: []string{"cmd", "--go-version=1.21.0", "path/go.mod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saved := os.Args
			defer func() { os.Args = saved }()

			os.Args = tt.args
			reorderArgs()

			if len(os.Args) != len(tt.want) {
				t.Fatalf("got %d args %v, want %d args %v", len(os.Args), os.Args, len(tt.want), tt.want)
			}
			for i := range os.Args {
				if os.Args[i] != tt.want[i] {
					t.Errorf("arg[%d] = %q, want %q (full: %v)", i, os.Args[i], tt.want[i], os.Args)
					break
				}
			}
		})
	}
}
