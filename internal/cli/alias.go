package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// registerAliases adds alias commands to the root command.
//
// Aliases are defined in config as:
//
//	aliases:
//	  sg: aws ec2 describe_security_groups
//	  buckets: aws s3 list_buckets
//
// This creates top-level commands (e.g., "cli sg --region us-east-1")
// that expand to the full command path.
func registerAliases(root *cobra.Command, aliases map[string]string) {
	for name, target := range aliases {
		aliasName := name     // capture for closure
		aliasTarget := target // capture for closure

		cmd := &cobra.Command{
			Use:   aliasName,
			Short: fmt.Sprintf("Alias for: %s", aliasTarget),
			// DisableFlagParsing allows all flags to pass through to the target command
			DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				// Split the alias target into parts and append any additional args
				targetParts := strings.Fields(aliasTarget)
				allArgs := append(targetParts, args...)

				// Reset and re-execute the root with the expanded args
				root.SetArgs(allArgs)
				return root.Execute()
			},
		}

		root.AddCommand(cmd)
	}
}
