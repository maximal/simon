package cmd

import (
	"fmt"
	simon "maximal/simon/modules"
	"os"
	"path"

	"github.com/spf13/cobra"
)

// `serviceCmd` represents the `service` command
var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Print Linux service file",
	Long: "`service`" + ` command prints ` + simon.APP_NAME + `’s service file in Linux systemd format.

Run ` + simon.APP_NAME + ` in background as a current user’s (username) service:
1. Create service dir:   mkdir -p ~/.local/share/systemd/user
2. Create service file:  /path/to/` + simon.APP_COMMAND + ` service  >  ~/.local/share/systemd/user/simon.service
3. Enable the service:   systemctl --user enable simon
4. Start the service:    systemctl --user start  simon
5. Extend service life:  sudo loginctl enable-linger <username>
6. Check service logs:   journalctl --user -fu simon

Uninstall current user’s (username) service:
1. Stop the service:     systemctl --user stop    simon
2. Disable the service:  systemctl --user disable simon
3. Remove service file:  rm  ~/.local/share/systemd/user/simon.service
4. Disable lingering:    sudo loginctl disable-linger <username>
`,
	Run: func(cmd *cobra.Command, args []string) {
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		config := path.Dir(ex) + "/" + simon.APP_COMMAND + ".yml"
		text := fmt.Sprintf(
			simon.SERVICE_TEMPLATE,
			simon.REPOSITORY_URL,
			simon.APP_NAME,
			simon.REPOSITORY_URL,
			ex,
			config,
		)
		simon.PrintColorful(text)
	},
}

func init() {
	rootCmd.AddCommand(serviceCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
