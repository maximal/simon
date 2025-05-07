package cmd

import (
	"fmt"
	simon "maximal/simon/modules"
	"time"

	"github.com/spf13/cobra"
)

// `metricsCmd` represents the `metrics` command
var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Print metrics example",
	Long:  "Print " + simon.APP_NAME + "’s metrics example in InfluxDB line protocol format.",
	Run: func(cmd *cobra.Command, args []string) {
		text := fmt.Sprintf(
			simon.METRICS_TEMPLATE,
			simon.APP_NAME,
			time.Now().Format("2006-01-02T15:04:05.000Z07:00"),
			simon.REPOSITORY_URL,
			simon.INFLUX_PROTOCOL_URL,
		)
		simon.PrintColorful(text)
	},
}

func init() {
	rootCmd.AddCommand(metricsCmd)
}
