// Package main_test holds repository-level checks that are not about
// Go code: the markdown documentation lint. It lives in the root package so
// `go test ./...` — the gate everyone already runs — enforces the doc rules
// without a second tool.
//
// Rules enforced on every tracked .md file:
//   - relative links resolve to a file (and, for #anchors, to a heading);
//   - in-page anchors match GitHub's heading slugs;
//   - no `---` horizontal rules (misread as frontmatter by some renderers);
//   - no hard-wrapped prose: a paragraph is one physical line.
package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docFiles are the markdown files the lint covers. Listed explicitly rather
// than globbed so a new page must be registered here — and thereby linted —
// on purpose.
var docFiles = []string{
	"README.md",
	"docs/README.md",
	"docs/commands.md",
	"docs/recipes.md",
	"docs/go-api.md",
	"CONTRIBUTING.md",
	"CHANGELOG.md",
	"SECURITY.md",
	"CODE_OF_CONDUCT.md",
}

var (
	// mdLink matches [text](target) but not images (![alt](src)) — images
	// are checked for existence too, via the same regexp with a leading "!".
	mdLink = regexp.MustCompile(`!?\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	// atxHeading matches "# ..." through "###### ...".
	atxHeading = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	// slugStrip removes what GitHub removes from a heading when slugging:
	// everything that is not a letter, digit, space, hyphen or underscore.
	slugStrip = regexp.MustCompile(`[^\p{L}\p{N}\s\-_]`)
	// inlineCode is removed before slugging so backticks do not count.
	inlineCode = regexp.MustCompile("`([^`]*)`")
	// htmlTag is removed before slugging (e.g. <sub>, <kbd>).
	htmlTag = regexp.MustCompile(`<[^>]+>`)
)

func TestDocs(t *testing.T) {
	t.Parallel()
	for _, path := range docFiles {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			if _, err := os.Stat(path); err != nil {
				t.Skipf("%s not present (written by a later phase)", path)
			}
			for _, problem := range lintMarkdown(t, path) {
				t.Error(problem)
			}
		})
	}
}

// lintMarkdown returns every rule violation in one file as "path:line: msg".
func lintMarkdown(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	headings := headingSlugs(lines)
	var problems []string
	report := func(n int, format string, a ...any) {
		problems = append(problems, fmt.Sprintf("%s:%d: %s", path, n, fmt.Sprintf(format, a...)))
	}
	inFence := false
	prevProse := false
	for i, line := range lines {
		n := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			prevProse = false
			continue
		}
		if inFence {
			continue
		}
		if trimmed == "---" {
			report(n, "horizontal rule `---` is banned (use a heading or blank line)")
		}
		// Hard-wrap detection: two consecutive plain prose lines.
		prose := isProseLine(trimmed)
		if prose && prevProse {
			report(n, "hard-wrapped prose: a paragraph must be one physical line")
		}
		prevProse = prose
		for _, m := range mdLink.FindAllStringSubmatch(line, -1) {
			checkLink(path, headings, m[1], func(format string, a ...any) { report(n, format, a...) })
		}
	}
	return problems
}

// isProseLine is true for a line that is body text — not blank, not a
// heading, list item, table row, blockquote, HTML, or link-reference.
func isProseLine(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '#', '-', '*', '>', '|', '<', '[', '!':
		return false
	}
	if len(trimmed) > 1 && trimmed[0] >= '0' && trimmed[0] <= '9' && strings.Contains(trimmed[:min(4, len(trimmed))], ".") {
		return false // ordered list item
	}
	return true
}

// checkLink validates one link target relative to the file that contains it.
func checkLink(path string, headings map[string]bool, target string, report func(string, ...any)) {
	if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
		return
	}
	file, anchor, _ := strings.Cut(target, "#")
	if file == "" {
		if !headings[anchor] {
			report("anchor #%s does not match any heading", anchor)
		}
		return
	}
	resolved := filepath.Join(filepath.Dir(path), file)
	if _, err := os.Stat(resolved); err != nil {
		report("link target %q does not exist (resolved %s)", target, resolved)
		return
	}
	if anchor != "" && strings.HasSuffix(file, ".md") {
		raw, err := os.ReadFile(resolved)
		if err != nil {
			report("read %s: %v", resolved, err)
			return
		}
		if !headingSlugs(strings.Split(string(raw), "\n"))[anchor] {
			report("anchor #%s not found in %s", anchor, file)
		}
	}
}

// headingSlugs computes GitHub-style anchors for every heading in lines,
// including the "-1", "-2" suffixes GitHub adds to duplicates.
func headingSlugs(lines []string) map[string]bool {
	out := map[string]bool{}
	seen := map[string]int{}
	inFence := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		m := atxHeading.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		slug := gitHubSlug(m[2])
		if n := seen[slug]; n > 0 {
			out[fmt.Sprintf("%s-%d", slug, n)] = true
		} else {
			out[slug] = true
		}
		seen[slug]++
	}
	return out
}

// gitHubSlug mirrors github.com's heading → id algorithm: strip markup,
// lowercase, drop punctuation, spaces → hyphens.
func gitHubSlug(heading string) string {
	s := inlineCode.ReplaceAllString(heading, "$1")
	s = htmlTag.ReplaceAllString(s, "")
	s = mdLink.ReplaceAllStringFunc(s, func(l string) string {
		return l[1:strings.Index(l, "]")]
	})
	s = strings.ToLower(s)
	s = slugStrip.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// The lint must catch what it claims to catch — each rule is exercised on a
// synthetic file so a regression in the linter fails loudly.
func TestLintMarkdown_CatchesEachRule(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	body := strings.Join([]string{
		"# Title",
		"",
		"first line of a paragraph",
		"second line hard-wrapped",
		"",
		"---",
		"",
		"[bad anchor](#nope) [good](#title) [missing](gone.md) [ok](x.md#title)",
		"",
		"```",
		"---",
		"wrapped inside",
		"a fence is fine",
		"```",
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := lintMarkdown(t, path)
	wantSubstr := []string{"hard-wrapped prose", "horizontal rule", "anchor #nope", `link target "gone.md"`}
	for _, w := range wantSubstr {
		found := false
		for _, g := range got {
			if strings.Contains(g, w) {
				found = true
			}
		}
		if !found {
			t.Errorf("lint missed %q; got %v", w, got)
		}
	}
	if len(got) != len(wantSubstr) {
		t.Errorf("want exactly %d problems, got %d: %v", len(wantSubstr), len(got), got)
	}
}

func TestGitHubSlug(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"Why browserctrl?":              "why-browserctrl",
		"`find` — resolve a nickname":   "find--resolve-a-nickname",
		"Using it from Claude Code":     "using-it-from-claude-code",
		"Exit codes & JSON":             "exit-codes--json",
		"[Linked](x.md) heading <sub>a": "linked-heading-a",
	} {
		if got := gitHubSlug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}
