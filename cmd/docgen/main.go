// Command docgen generates the man page and markdown reference
// from the cobra command tree defined in internal/transport/cli.
//
// Usage:
//
//	go run ./cmd/docgen man  ./man      → manpages/sudoconsole.1, ...
//	go run ./cmd/docgen md   ./docs/cmd → sudoconsole.md, sudoconsole_install.md, ...
//	go run ./cmd/docgen yaml ./yaml     → sudoconsole.yaml, ...
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/LeandroLCD/sudoconsole/internal/transport/cli"
)

// version/commit/buildDate are populated at build time via
// -ldflags "-X main.version=...". The doc generator uses the
// upstream cobra defaults (empty strings) so the rendered text
// stays stable regardless of the binary's own version stamp.
var (
	version   = ""
	commit    = ""
	buildDate = ""
)

// newRoot constructs the same command tree the binary exposes, but
// without wiring any infrastructure (no App, no gateway, no audit).
// Sub-commands that depend on App.run* helpers fall back to printing
// a "configuration error" — that's fine, we only care about the
// command shape.
func newRoot() *cobra.Command {
	// The internal/cli constructor accepts a nil App; we pass a
	// minimal one so version / install sub-commands don't deref nil
	// when the generator walks the tree (it only reads metadata, but
	// NewVersionCmd eagerly captures deps at construction time).
	app := &cli.App{}
	root := cli.NewRootCmd(app, version, commit, buildDate)
	return root
}

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("usage: docgen {man|md|yaml} <outdir>")
	}
	format, outdir := os.Args[1], os.Args[2]
	// Resolve outdir to an absolute, cleaned path so a
	// relative target like "../../etc" is normalised before
	// we touch the filesystem. The result still trusts the
	// developer running the tool — outdir is not user input.
	clean := filepath.Clean(outdir)
	if !filepath.IsAbs(clean) {
		abs, err := filepath.Abs(clean)
		if err == nil {
			clean = abs
		}
	}
	outdir = clean
	// 0o755 is intentional: docs are user-facing and need to be
	// readable by the same user account that produced them.
	// #nosec G301 -- docs dir is shared, mode is intentional
	// #nosec G703 -- outdir is supplied by the developer running the tool
	if err := os.MkdirAll(outdir, 0o755); err != nil {
		// #nosec G706 -- tool, not daemon
		log.Fatalf("mkdir %s: %v", outdir, err)
	}
	root := newRoot()

	var err error
	switch strings.ToLower(format) {
	case "man":
		err = doc.GenManTreeFromOpts(root, doc.GenManTreeOptions{
			Path:   outdir,
			Header: defaultManHeader(),
		})
	case "md", "markdown":
		err = doc.GenMarkdownTreeCustom(root, outdir,
			func(s string) string { return "" },
			func(s string) string { return "" },
		)
	case "yaml":
		err = doc.GenYamlTreeCustom(root, outdir,
			func(s string) string { return "" },
			func(s string) string { return "" },
		)
	default:
		log.Fatalf("unknown format %q (want man|md|yaml)", format) // #nosec G706 -- tool, not daemon
	}
	if err != nil {
		log.Fatalf("generate %s: %v", format, err) // #nosec G706 -- tool, not daemon
	}
}

func defaultManHeader() *doc.GenManHeader {
	now := time.Now()
	return &doc.GenManHeader{
		Title:   "sudoconsole",
		Section: "1",
		Manual:  "sudoconsole Manual",
		Source:  "sudoconsole " + version,
		Date:    &now,
	}
}
