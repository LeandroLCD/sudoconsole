package cli

import (
	"runtime"

	"github.com/spf13/cobra"
)

// newVersionCmd builds `sudoconsole version`.
func newVersionCmd(app *App, version, commit, buildDate string) *cobra.Command {
	deps := &versionDeps{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
		Fmt:       app.Formatter,
		Now:       app.Now,
	}
	return &cobra.Command{
		Use:   "version",
		Short: "Print build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runVersion(cmd, deps)
		},
	}
}

type versionDeps struct {
	Version, Commit, BuildDate string
	GoVersion                  string
	GOOS, GOARCH               string
	Fmt                        Formatter
	Now                        func() string
}

func runVersion(cmd *cobra.Command, d *versionDeps) error {
	if d.GoVersion == "" {
		d.GoVersion = runtime.Version()
	}
	if d.GOOS == "" {
		d.GOOS = runtime.GOOS
	}
	if d.GOARCH == "" {
		d.GOARCH = runtime.GOARCH
	}
	if d.Now == nil {
		d.Now = Now
	}
	return d.Fmt.Print(cmd.OutOrStdout(), &VersionInfo{
		Version:   d.Version,
		Commit:    d.Commit,
		BuildDate: d.BuildDate,
		GoVersion: d.GoVersion,
		GOOS:      d.GOOS,
		GOARCH:    d.GOARCH,
		Time:      d.Now(),
	})
}
