package cmd

import (
	"github.com/lambdabaa/dewey/apps/cli/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Printf("dewey %s\n", version.Version)
			cmd.Printf("api    %s\n", version.APIVersion)
			cmd.Printf("commit %s\n", version.Commit)
			cmd.Printf("date   %s\n", version.Date)
		},
	}
}
