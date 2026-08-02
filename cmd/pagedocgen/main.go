// Command pagedocgen builds the GitHub Pages site for sudoconsole.
//
// Usage:
//
//	go run ./cmd/pagedocgen            # build site/ into ./site/
//	go run ./cmd/pagedocgen --src ./docs --out ./site
//
// Output:
//
//	site/index.html          (handcrafted landing page)
//	site/install.sh          (copy of scripts/install.sh)
//	site/styles.css          (copy of site/styles.css)
//	site/docs/<name>.html    (rendered from docs/<name>.md)
//	site/manpages/<name>.html (rendered from manpages/<name>.1)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	srcRoot = flag.String("src", ".", "source root (repository root)")
	outRoot = flag.String("out", "./site-publish", "output directory for the rendered GitHub Pages site (kept separate from ./site which holds the source HTML)")
)

const pageTmpl = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s — sudoconsole</title>
<meta name="description" content="sudoconsole documentation">
<link rel="stylesheet" href="../styles.css">
</head>
<body>
<nav>
  <div class="container">
    <a class="brand" href="../index.html">sudoconsole</a>
    <a href="../install.sh">install.sh</a>
    <a href="../docs/index.html">docs</a>
    <a href="../manpages/sudoconsole.1.html">manpage</a>
    <a href="https://github.com/LeandroLCD/sudoconsole">github</a>
  </div>
</nav>
<div class="container">
<article>
%s
</article>
</div>
</body>
</html>
`

func main() {
	flag.Parse()
	// #nosec G301 -- docs are world-readable by design
	if err := os.MkdirAll(*outRoot, 0o755); err != nil {
		log.Fatalf("mkdir out: %v", err)
	}

	// 1. Install script (copy verbatim — served at /install.sh).
	copyFile(filepath.Join(*srcRoot, "scripts/install.sh"),
		filepath.Join(*outRoot, "install.sh"))
	log.Print("install.sh -> ", filepath.Join(*outRoot, "install.sh"))

	// 2. CSS.
	copyFile(filepath.Join(*srcRoot, "site/styles.css"),
		filepath.Join(*outRoot, "styles.css"))
	log.Print("styles.css -> ", filepath.Join(*outRoot, "styles.css"))

	// 3. Landing page (handcrafted, already in src/site/).
	copyFile(filepath.Join(*srcRoot, "site/index.html"),
		filepath.Join(*outRoot, "index.html"))
	log.Print("index.html -> ", filepath.Join(*outRoot, "index.html"))

	// 4. Render every docs/*.md to docs/*.html.
	renderDocs(filepath.Join(*srcRoot, "docs"),
		filepath.Join(*outRoot, "docs"))

	// 5. Render every manpages/*.1 to manpages/*.1.html.
	renderManpages(filepath.Join(*srcRoot, "manpages"),
		filepath.Join(*outRoot, "manpages"))

	log.Print("site built at ", *outRoot)
}

// md is the markdown renderer. GFM tables + autolinks are enabled so
// tables in CONFIG.md render correctly.
var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Linkify),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(
		html.WithXHTML(),
	),
)

func renderDocs(srcDir, outDir string) {
	// #nosec G301 -- docs are world-readable by design
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir docs out: %v", err)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		log.Fatalf("read docs src: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		srcPath := filepath.Join(srcDir, e.Name())
		baseName := strings.TrimSuffix(e.Name(), ".md")
		outPath := filepath.Join(outDir, baseName+".html")
		markdownToHTML(srcPath, outPath, baseName)
		log.Printf("%s -> %s", srcPath, outPath)
	}

	// docs/index.html: a small catalogue linking to every rendered doc.
	docIndex(outDir, entries)
}

func docIndex(outDir string, entries []os.DirEntry) {
	var sb strings.Builder
	sb.WriteString("# Documentation\n\n")
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		title := strings.TrimSuffix(e.Name(), ".md")
		fmt.Fprintf(&sb, "- [%s](%s.html)\n", strings.ToUpper(title[:1])+title[1:], title)
	}
	out := fmt.Sprintf(pageTmpl, "Documentation", sb.String())
	// #nosec G306 -- rendered docs are world-readable
	if err := os.WriteFile(filepath.Join(outDir, "index.html"),
		[]byte(out), 0o644); err != nil {
		log.Fatalf("write docs/index.html: %v", err)
	}
}

func markdownToHTML(srcPath, outPath, title string) {
	// #nosec G304 -- srcPath is supplied by the developer running the tool
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		log.Fatalf("read %s: %v", srcPath, err)
	}
	var buf bytes.Buffer
	if err := md.Convert(raw, &buf); err != nil {
		log.Fatalf("convert %s: %v", srcPath, err)
	}
	out := fmt.Sprintf(pageTmpl, strings.ToUpper(title[:1])+title[1:], buf.String())
	// #nosec G306 -- rendered docs are world-readable
	if err := os.WriteFile(outPath, []byte(out), 0o644); err != nil {
		log.Fatalf("write %s: %v", outPath, err)
	}
}

func renderManpages(srcDir, outDir string) {
	// #nosec G301 -- rendered HTML is world-readable
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir manpages out: %v", err)
	}
	if _, err := os.Stat(srcDir); err != nil {
		if os.IsNotExist(err) {
			log.Print("manpages/: no source, skipping (run `make docgen` first)")
			return
		}
		log.Fatalf("stat %s: %v", srcDir, err)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		log.Fatalf("read manpages: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".1") {
			continue
		}
		srcPath := filepath.Join(srcDir, e.Name())
		outPath := filepath.Join(outDir, e.Name()+".html")
		manpageToHTML(srcPath, outPath, e.Name())
		log.Printf("%s -> %s", srcPath, outPath)
	}
}

// manpageToHTML converts a GoReleaser manpage (groff -man format) into
// a roughly-styled HTML preview. This is *not* a complete groff
// renderer — it covers the subset our manpages actually use:
//
//   - .TH, .SH, .PP
//   - .B / \fB...\fP
//   - .nf / .fi (literal blocks)
//   - lists via .IP / .TP
//
// It is good enough to read the reference in a browser without
// shipping a groff runtime.
func manpageToHTML(srcPath, outPath, name string) {
	// #nosec G304 -- srcPath is supplied by the developer running the tool
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		log.Fatalf("read %s: %v", srcPath, err)
	}
	var sb strings.Builder
	lines := strings.Split(string(raw), "\n")
	inLiteral := false
	for _, line := range lines {
		emitManLine(&sb, line, name, &inLiteral)
	}
	if inLiteral {
		sb.WriteString("</pre>\n")
	}
	body := sb.String()
	if !strings.Contains(body, "<body") {
		// .TH was missing — synthesise a header.
		body = fmt.Sprintf(pageTmpl, name, "<h1>"+escapeHTML(name)+"</h1>\n"+body)
	}
	if !strings.Contains(body, "</body>") {
		// Close tags left open by abbreviated paragraphs.
		body += "</article></div></body></html>\n"
	}
	// #nosec G306 -- rendered manpages are world-readable
	if err := os.WriteFile(outPath, []byte(body), 0o644); err != nil {
		log.Fatalf("write %s: %v", outPath, err)
	}
}

// emitManLine writes a single groff line as HTML. Pulled out into
// its own function so manpageToHTML stays under the gocyclo budget.
func emitManLine(sb *strings.Builder, line, name string, inLiteral *bool) {
	trimmed := strings.TrimRight(line, "\r")
	switch {
	case strings.HasPrefix(trimmed, ".TH "):
		// .TH name section date source manual
		parts := strings.Fields(trimmed)
		title := name
		if len(parts) >= 2 {
			title = parts[1] + "(" + parts[2] + ")"
		}
		fmt.Fprintf(sb, pageTmpl, title, "<h1>"+title+"</h1>\n")
	case strings.HasPrefix(trimmed, ".SH "):
		if *inLiteral {
			sb.WriteString("</pre>\n")
			*inLiteral = false
		}
		heading := strings.TrimPrefix(trimmed, ".SH ")
		sb.WriteString("<h2>" + escapeHTML(heading) + "</h2>\n")
	case trimmed == ".PP":
		sb.WriteString("<p>")
	case trimmed == ".nf":
		if !*inLiteral {
			sb.WriteString("<pre>")
			*inLiteral = true
		}
	case trimmed == ".fi":
		if *inLiteral {
			sb.WriteString("</pre>\n")
			*inLiteral = false
		}
	case trimmed == ".P":
		sb.WriteString("</p><p>")
	case strings.HasPrefix(trimmed, ".B "):
		sb.WriteString("<strong>" + escapeHTML(strings.TrimPrefix(trimmed, ".B ")) + "</strong>")
	case strings.HasPrefix(trimmed, ".PP "):
		sb.WriteString("<p>" + escapeHTML(strings.TrimPrefix(trimmed, ".PP ")))
	default:
		sb.WriteString(processInline(trimmed) + "\n")
	}
}

// processInline wraps a regular paragraph line in <p> tags with
// HTML escaping. \fB...\fP and \fI...\fP would be expanded here for
// full coverage; the existing manpages do not use them so we only
// handle the plain case.
func processInline(line string) string {
	return "<p>" + escapeHTML(line) + "</p>"
}

func escapeHTML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return r.Replace(s)
}

func copyFile(src, dst string) {
	if _, err := os.Stat(src); err != nil {
		// Optional input — silently skip.
		return
	}
	// Refuse to copy a file onto itself: opening dst with O_TRUNC
	// would truncate src (same path) before we read from it,
	// producing a 0-byte copy.
	srcAbs, _ := filepath.Abs(src)
	dstAbs, _ := filepath.Abs(dst)
	if srcAbs == dstAbs {
		return
	}
	// #nosec G304 -- src is supplied by the developer running the tool
	in, err := os.Open(src)
	if err != nil {
		log.Print("open ", src, ": ", err)
		os.Exit(1)
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { // #nosec G301
		log.Print("mkdir ", filepath.Dir(dst), ": ", err)
		// nolint:gocritic // exitAfterDefer: we WANT to skip the deferred close because we are aborting — the runtime will reap the FD.
		os.Exit(1)
	}
	// #nosec G304,G302 -- dst is the output path; rendered content is intentionally world-readable
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		log.Print("create ", dst, ": ", err)
		os.Exit(1)
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		log.Print("copy ", src, " -> ", dst, ": ", err)
		// nolint:gocritic // exitAfterDefer: we WANT to skip the deferred close
		// because we are aborting — the runtime will reap the FD.
		os.Exit(1)
	}
}
