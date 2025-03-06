package flags

import (
	"github.com/spf13/cobra"
)

func MustMarkRequired(cmd *cobra.Command, name string) {
	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(err)
	}
}
