package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVersionCmd builds `sudoconsole version`.
func newVersionCmd(version, commit, buildDate string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "sudoconsole %s\n", version)
			_, _ = fmt.Fprintf(out, "  commit:  %s\n", commit)
			_, _ = fmt.Fprintf(out, "  built:   %s\n", buildDate)
			return nil
		},
	}
}
