package main

import (
	"bytes"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	yaml "github.com/goccy/go-yaml"
	"github.com/pkg/browser"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var (
	logger = log.New(os.Stderr, "toolkit-show-md: ", 0)

	helpMsg = `Usage: %s <PATH>

Convert Markdown file to GitHub style HTML and display HTML in the default browser.

Arguments:
  <PATH>    Path to the Markdown file to convert.
`
	//go:embed .github.style.tmplt
	gitHubMarkdownTemplate []byte

	bareURLPattern = regexp.MustCompile(`^https?://\S+$`)
)

// splitFrontmatter separates a leading YAML front matter block (delimited by
// "---" lines) from the rest of the Markdown source. If no leading "---"
// fence is present, yamlPart is nil and body is the entire input. A leading
// "---" fence that never closes is a real error rather than a silent
// pass-through, since it means the input claims to have front matter but is
// malformed.
func splitFrontmatter(mdContents []byte) (yamlPart []byte, body []byte, err error) {
	trimmed := bytes.TrimLeft(mdContents, "\r\n\t ")

	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return nil, mdContents, nil
	}

	after := trimmed[3:]

	if len(after) > 0 && after[0] != '\n' && !bytes.HasPrefix(after, []byte("\r\n")) {
		return nil, mdContents, nil
	}

	idx := indexClosingFence(after)
	if idx == -1 {
		return nil, nil, errors.New(`front matter starts with "---" but has no closing fence`)
	}

	return after[:idx], bytes.TrimLeft(after[idx+4:], "\n\r"), nil
}

// indexClosingFence returns the byte offset within s of the newline
// immediately preceding the first line that consists solely of "---",
// terminated by "\n", "\r\n", or the end of s. Returns -1 if s has no such
// line. s[idx+4:] is always exactly what follows the 3 closing dashes,
// since idx always points at a "\n" and the fence line itself is always
// "\n---" -- 4 bytes.
func indexClosingFence(s []byte) int {
	for i, b := range s {
		if b != '\n' {
			continue
		}

		rest := s[i+1:]
		if !bytes.HasPrefix(rest, []byte("---")) {
			continue
		}

		after := rest[3:]
		if len(after) == 0 || after[0] == '\n' || bytes.HasPrefix(after, []byte("\r\n")) {
			return i
		}
	}

	return -1
}

// renderFrontmatter renders a YAML front matter block as an HTML table,
// preserving key order, instead of letting it fall through to the Markdown
// converter where it would be misinterpreted as regular prose.
func renderFrontmatter(yamlPart []byte) (string, error) {
	var items yaml.MapSlice

	if err := yaml.Unmarshal(yamlPart, &items); err != nil {
		return "", err
	}

	if items == nil {
		return "", errors.New("front matter block did not parse to a YAML mapping")
	}

	var b strings.Builder

	b.WriteString("<table>\n<thead>\n<tr><th>Key</th><th>Value</th></tr>\n</thead>\n<tbody>\n")

	for _, item := range items {
		b.WriteString("<tr><td>")
		b.WriteString(html.EscapeString(fmt.Sprintf("%v", item.Key)))
		b.WriteString("</td><td>")
		b.WriteString(formatYAMLValue(item.Value))
		b.WriteString("</td></tr>\n")
	}

	b.WriteString("</tbody>\n</table>\n")

	return b.String(), nil
}

// formatYAMLValue renders a decoded YAML value as an HTML table cell body.
func formatYAMLValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		if trimmed := strings.TrimSpace(typed); bareURLPattern.MatchString(trimmed) {
			escaped := html.EscapeString(trimmed)

			return fmt.Sprintf(`<a href="%s">%s</a>`, escaped, escaped)
		}

		return strings.ReplaceAll(html.EscapeString(typed), "\n", "<br>")
	case []any:
		parts := make([]string, len(typed))
		for i, item := range typed {
			parts[i] = formatYAMLValue(item)
		}

		return strings.Join(parts, ", ")
	case yaml.MapSlice:
		out, err := yaml.Marshal(typed)
		if err != nil {
			return html.EscapeString(fmt.Sprintf("%v", typed))
		}

		return fmt.Sprintf("<pre><code>%s</code></pre>", html.EscapeString(string(out)))
	default:
		return html.EscapeString(fmt.Sprintf("%v", typed))
	}
}

func main() {
	var stat os.FileInfo

	var err error

	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), helpMsg, os.Args[0])
	}

	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		logger.Fatalln("Exactly one argument is permitted and it is <PATH>.")
	}

	inpath := filepath.Clean(args[0])

	if stat, err = os.Stat(inpath); os.IsNotExist(err) || stat.IsDir() {
		logger.Fatalf("The input path %q doesn't exist or is a directory.", inpath)
	}

	mdContents, err := os.ReadFile(filepath.Clean(inpath))
	if err != nil {
		logger.Fatalf("Failed to read the contents of the input file at %q: %s", inpath, err)
	}

	tmplt, err := template.New("github.style").Parse(string(gitHubMarkdownTemplate))
	if err != nil {
		logger.Fatalf("Failed to load template file: %s", err)
	}

	yamlPart, body, err := splitFrontmatter(mdContents)
	if err != nil {
		logger.Fatalf("Failed to parse Markdown front matter: %s", err)
	}

	var frontmatterHTML string

	if yamlPart != nil {
		frontmatterHTML, err = renderFrontmatter(yamlPart)
		if err != nil {
			logger.Fatalf("Failed to parse YAML front matter: %s", err)
		}
	}

	md := goldmark.New(goldmark.WithExtensions(extension.GFM))

	var htmlBuf, out bytes.Buffer

	err = md.Convert(body, &htmlBuf)
	if err != nil {
		logger.Fatalf("Encountered error while converting Markdown to HTML: %s", err)
	}

	err = tmplt.Execute(&out, frontmatterHTML+htmlBuf.String())
	if err != nil {
		logger.Fatalf("Failed to insert converted HTML into template: %s", err)
	}

	err = browser.OpenReader(&out)
	if err != nil {
		logger.Fatalf("Failed to open rendered HTML in default browser: %s", err)
	}
}
