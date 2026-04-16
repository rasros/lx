package core

import (
	"testing"
)

type mockStringer string

func (m mockStringer) String() string { return string(m) }

func TestDefaultTokenCounter(t *testing.T) {
	tests := []struct {
		name    string
		size    int64
		content interface{}
		want    int64
	}{
		{
			name:    "Exact String (4 chars)",
			size:    4,
			content: "1234",
			want:    1,
		},
		{
			name:    "Exact String (8 chars)",
			size:    8,
			content: "12345678",
			want:    2,
		},
		{
			name:    "Under 4 chars",
			size:    3,
			content: "123",
			want:    0,
		},
		{
			name:    "Byte Slice",
			size:    100,
			content: make([]byte, 8),
			want:    2,
		},
		{
			name:    "Stringer Interface",
			size:    100,
			content: mockStringer("1234"),
			want:    1,
		},
		{
			name:    "Nil Content (Use Size)",
			size:    12,
			content: nil,
			want:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultTokenCounter(tt.size, tt.content)
			if got != tt.want {
				t.Errorf("DefaultTokenCounter() = %d, want %d", got, tt.want)
			}
		})
	}
}
