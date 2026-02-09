package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
// go build -ldflags "-X github.com/reglet-dev/cli/internal/cli.Version=1.0.0"
var Version = "dev"

// Commit is set at build time via ldflags.
var Commit = "unknown"

// BuildTime is set at build time via ldflags.
var BuildTime = "unknown"

// newVersionCommand creates the "version" command.
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("cli version %s\n", Version)
			fmt.Printf("  commit:     %s\n", Commit)
			fmt.Printf("  build time: %s\n", BuildTime)
			fmt.Printf("  go:         %s\n", runtime.Version())
			fmt.Printf("  os/arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
}
