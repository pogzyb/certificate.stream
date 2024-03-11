package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "certificate",
	Short: "Stream certificate transparency logs.",
	Long: `The easiest way to stream the certificate transparency logs.
Full description: https://github.com/pogzyb/certificate.stream/
Learn more about the Certificate Transparency: https://certificate.transparency.dev/`,
	Run: func(cmd *cobra.Command, args []string) {},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
