package main

import (
	"strings"
	"testing"

	yaml "github.com/goccy/go-yaml"
)

func TestSplitFrontmatter(t *testing.T) {
	t.Run("no front matter", func(t *testing.T) {
		body, rest, err := splitFrontmatter([]byte("# Title\n\nBody text.\n"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if body != nil {
			t.Fatalf("expected nil yamlPart, got %q", body)
		}

		if string(rest) != "# Title\n\nBody text.\n" {
			t.Fatalf("expected body to be the entire input, got %q", rest)
		}
	})

	t.Run("normal front matter", func(t *testing.T) {
		input := "---\ndescription: hi\ncount: 2\n---\n# Title\n"

		yamlPart, body, err := splitFrontmatter([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if string(yamlPart) != "\ndescription: hi\ncount: 2" {
			t.Fatalf("unexpected yamlPart: %q", yamlPart)
		}

		if string(body) != "# Title\n" {
			t.Fatalf("unexpected body: %q", body)
		}
	})

	t.Run("front matter with CRLF line endings", func(t *testing.T) {
		input := "---\r\ndescription: hi\r\n---\r\n# Title\r\n"

		yamlPart, body, err := splitFrontmatter([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if string(yamlPart) != "\r\ndescription: hi\r" {
			t.Fatalf("unexpected yamlPart: %q", yamlPart)
		}

		if string(body) != "# Title\r\n" {
			t.Fatalf("unexpected body: %q", body)
		}
	})

	t.Run("empty front matter with no blank line", func(t *testing.T) {
		yamlPart, body, err := splitFrontmatter([]byte("---\n---\n# Title\n"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if len(yamlPart) != 0 {
			t.Fatalf("expected empty yamlPart, got %q", yamlPart)
		}

		if string(body) != "# Title\n" {
			t.Fatalf("unexpected body: %q", body)
		}
	})

	t.Run("empty front matter with blank line", func(t *testing.T) {
		yamlPart, body, err := splitFrontmatter([]byte("---\n\n---\n# Title\n"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if string(body) != "# Title\n" {
			t.Fatalf("unexpected body: %q", body)
		}

		if _, err := renderFrontmatter(yamlPart); err == nil {
			t.Fatal("expected renderFrontmatter to reject a blank front matter block")
		}
	})

	t.Run("unclosed fence is an error", func(t *testing.T) {
		_, _, err := splitFrontmatter([]byte("---\ndescription: hi\n# Title\n"))
		if err == nil {
			t.Fatal("expected an error for an unclosed front matter fence")
		}
	})

	t.Run("dashes inside a value do not falsely close the fence", func(t *testing.T) {
		input := "---\nnotes: |\n  ---\n  more\n---\n# Title\n"

		yamlPart, body, err := splitFrontmatter([]byte(input))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if string(yamlPart) != "\nnotes: |\n  ---\n  more" {
			t.Fatalf("unexpected yamlPart: %q", yamlPart)
		}

		if string(body) != "# Title\n" {
			t.Fatalf("unexpected body: %q", body)
		}
	})
}

func TestRenderFrontmatter(t *testing.T) {
	t.Run("renders rows in source order", func(t *testing.T) {
		html, err := renderFrontmatter([]byte("\nb: 1\na: 2\n"))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		bIdx := strings.Index(html, "<td>b</td>")
		aIdx := strings.Index(html, "<td>a</td>")

		if bIdx == -1 || aIdx == -1 {
			t.Fatalf("expected both keys to render, got: %s", html)
		}

		if bIdx > aIdx {
			t.Fatalf("expected key %q to render before key %q, got: %s", "b", "a", html)
		}
	})

	t.Run("empty block is an error", func(t *testing.T) {
		_, err := renderFrontmatter([]byte(""))
		if err == nil {
			t.Fatal("expected an error for a front matter block that is not a YAML mapping")
		}
	})

	t.Run("invalid YAML is an error", func(t *testing.T) {
		_, err := renderFrontmatter([]byte("\n[this is not: a mapping"))
		if err == nil {
			t.Fatal("expected an error for invalid YAML")
		}
	})
}

func TestFormatYAMLValue(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{name: "nil", value: nil, want: ""},
		{
			name:  "bare URL",
			value: "https://example.com/path",
			want:  `<a href="https://example.com/path">https://example.com/path</a>`,
		},
		{
			name:  "multiline string",
			value: "line one\nline two",
			want:  "line one<br>line two",
		},
		{
			name:  "escapes HTML in plain strings",
			value: "<script>",
			want:  "&lt;script&gt;",
		},
		{
			name:  "list of scalars",
			value: []any{"a", "b"},
			want:  "a, b",
		},
		{name: "bool", value: true, want: "true"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatYAMLValue(tc.value)
			if got != tc.want {
				t.Fatalf("formatYAMLValue(%#v) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}

	t.Run("nested map renders as a code block", func(t *testing.T) {
		nested := yaml.MapSlice{{Key: "k", Value: "v"}}

		got := formatYAMLValue(nested)
		if !strings.Contains(got, "<pre><code>") {
			t.Fatalf("expected nested map to render inside <pre><code>, got %q", got)
		}
	})
}
