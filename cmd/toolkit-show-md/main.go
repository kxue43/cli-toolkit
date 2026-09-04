package main

import (
	"bytes"
	_ "embed"
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

	frontmatterPattern = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---[ \t]*\r?\n?`)
	bareURLPattern     = regexp.MustCompile(`^https?://\S+$`)
)

// splitFrontmatter separates a leading YAML front matter block (delimited by
// `---` lines) from the rest of the Markdown source. If no front matter is
// present, yamlPart is nil and body is the entire input.
func splitFrontmatter(mdContents []byte) (yamlPart []byte, body []byte) {
	match := frontmatterPattern.FindSubmatch(mdContents)
	if match == nil {
		return nil, mdContents
	}

	return match[1], mdContents[len(match[0]):]
}

// renderFrontmatter renders a YAML front matter block as an HTML table,
// preserving key order, instead of letting it fall through to the Markdown
// converter where it would be misinterpreted as regular prose.
func renderFrontmatter(yamlPart []byte) (string, error) {
	var items yaml.MapSlice

	if err := yaml.Unmarshal(yamlPart, &items); err != nil {
		return "", err
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

	yamlPart, body := splitFrontmatter(mdContents)

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
