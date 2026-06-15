package cmd

import (
	"github.com/spf13/cobra"

	simon "maximal/simon/modules"
)

// `rootCmd` represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Version: simon.APP_VERSION,
	Use:     "simon",
	Short:   simon.APP_NAME + " — Linux system monitor with InfluxDB metrics",
	Long: simon.APP_NAME + ` is a simple Linux system monitor.
It gathers various performance metrics and prints or sends them to InfluxDB.

Author: MaximAL 2024—2026
Website: ` + simon.REPOSITORY_URL + `
Original PHP version: ` + simon.REPOSITORY_PHP_URL,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		config, err := cmd.Flags().GetString("config")
		if err != nil {
			simon.Exit(simon.StatusGeneralError, err)
		}
		//_, err := os.OpenFile(config, os.O_RDONLY, 0644)
		//if errors.Is(err, os.ErrNotExist) {
		//	log.Fatal("File not found", config)
		//}

		interval, err := cmd.Flags().GetFloat32("interval")
		if err != nil {
			simon.Exit(simon.StatusGeneralError, err)
		}
		if interval < 0.1 || interval > 60*60 {
			println("interval must be in range from 0.1 to 3600")
			simon.Exit(simon.StatusGeneralError)
		}

		simon.Work(config, interval)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		simon.Exit(simon.StatusGeneralError, err)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringP("config", "c", "simon.yml", "Config file")
	// rootCmd.PersistentFlags().Float32P("interval", "i", 15.0, "Refresh interval in seconds: 0.1 .. 600")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().StringP("config", "c", "config.yml", "Config file")
	rootCmd.Flags().StringP("config", "c", "simon.yml", "config file")
	rootCmd.Flags().Float32P("interval", "i", 15.0, "refresh interval in seconds: 0.1 .. 3600")
}
