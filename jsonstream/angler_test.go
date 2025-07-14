package jsonstream

import (
	"context"
	"iter"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Zip(first []string, second []any) iter.Seq2[string, any] {
	n := math.Min(float64(len(first)), float64(len(second)))

	return func(yield func(string, any) bool) {
		for i := range int(n) {
			if !yield(first[i], second[i]) {
				return
			}
		}
	}
}

func TestLand(t *testing.T) {
	var tests = []struct {
		contents string
		paths    []string
		expected []any
	}{
		{
			contents: `
			{
				"a": 1,
				"b": [1, 2, 3],
				"c": null,
				"d": {
					"e": "target"
				},
				"f": "x"
			}
			`,
			paths: []string{
				".d.e",
				".f",
				".a",
			},
			expected: []any{
				"target",
				"x",
				float64(1),
			},
		},
		{
			contents: `
			{
				"a": 1,
				"b": {
					"u": "y",
					"v": 2
				},
				"c": null,
				"d": {
					"e f": {
						"g": "z",
						"h": "w"
					},
					"": {
						"s": "here"
					}
				},
				"f": "x"
			}
			`,
			paths: []string{
				".d.e f.g",
				".d.e f.h",
				".b.v",
				".d..s",
			},
			expected: []any{
				"z",
				"w",
				float64(2),
				"here",
			},
		},
		{
			contents: `
			{
				"a": "b",
				"b": "x",
				"c": [1, 2],
				"d": {
					"e": "f",
					"f": "y"
				},
				"g": "z"
			}
			`,
			paths: []string{
				".b",
				".d.f",
			},
			expected: []any{
				"x",
				"y",
			},
		},
	}

	for _, test := range tests {
		for path, expected := range Zip(test.paths, test.expected) {
			angler, err := NewAngler(strings.NewReader(test.contents), path)
			require.NoError(t, err)

			value, err := angler.Land(context.Background())
			require.NoError(t, err)

			assert.Equal(t, expected, value)
		}
	}
}
