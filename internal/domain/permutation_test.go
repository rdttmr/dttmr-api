package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsPermutation(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{name: "both empty", a: nil, b: []string{}, want: true},
		{name: "same order", a: []string{"a", "b", "c"}, b: []string{"a", "b", "c"}, want: true},
		{name: "reordered", a: []string{"a", "b", "c"}, b: []string{"c", "a", "b"}, want: true},
		{name: "different case", a: []string{"ABC", "def"}, b: []string{"DEF", "abc"}, want: true},
		{name: "first is shorter", a: []string{"a", "b"}, b: []string{"a", "b", "c"}, want: false},
		{name: "second is shorter", a: []string{"a", "b", "c"}, b: []string{"a", "b"}, want: false},
		{name: "same length, different element", a: []string{"a", "b"}, b: []string{"a", "c"}, want: false},
		{name: "duplicate in first", a: []string{"a", "a"}, b: []string{"a", "b"}, want: false},
		{name: "duplicate in second", a: []string{"a", "b"}, b: []string{"a", "a"}, want: false},
		{name: "same duplicate on both sides", a: []string{"a", "a"}, b: []string{"a", "a"}, want: false},
		{name: "duplicate differing only in case", a: []string{"a", "b"}, b: []string{"a", "A"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsPermutation(tt.a, tt.b))
			assert.Equal(t, tt.want, IsPermutation(tt.b, tt.a), "must be symmetric")
		})
	}
}
