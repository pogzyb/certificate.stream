package cmd

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"certificate.stream/service/stream"
)

func init() {
	rootCmd.AddCommand(loglist)
}

var (
	loglist = &cobra.Command{
		Use:   "loglist",
		Short: "Lists the Operators and Log sources from Google Chrome's CT log list.",
		Long:  `Lists the Operators and Log sources from Google Chrome's CT log list in a table format.`,
		Run:   func(cmd *cobra.Command, args []string) { LogList() },
	}
)

func LogList() {
	ll, err := stream.GetLogList()
	if err != nil {
		log.Fatal().Msg(fmt.Sprintf("could not get loglist: %v", err))
	}
	fmt.Printf("|%-3s| %-40s| %-25s| %s\n", "", "Operator", "Log Status", "Log URL")
	n := 1
	for _, op := range ll.Operators {
		for _, lg := range op.Logs {
			status := strings.Replace(lg.State.String(), "LogStatus", "", -1)
			fmt.Printf("|%-3d| %-40s| %-25s| %s\n", n, op.Name, status, lg.URL)
			n++
		}
	}
}
