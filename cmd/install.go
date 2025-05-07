package cmd

import (
	"fmt"
	simon "maximal/simon/modules"
	"os"
	"os/exec"
	"os/user"

	"github.com/spf13/cobra"
)

// `installCmd` represents the `install` command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Linux service",
	Long:  "Install " + simon.APP_NAME + " as a system (root) service or as a user service.",
	Run: func(cmd *cobra.Command, args []string) {
		executable, err := os.Executable()
		if err != nil {
			panic(err)
		}

		userObj, err := user.Current()
		if err != nil {
			panic(err)
		}

		println("THIS COMMAND IS NOT READY FOR THE DIRECT USE.")
		println("MAKE THE STEPS LISTED BELOW MANUALLY.")
		println()

		currentUsername := userObj.Username
		if currentUsername != "root" {
			println("This command must be run as root.")
			simon.Exit(simon.StatusGeneralError)
		}

		username, _ := cmd.Flags().GetString("user")

		forUser, err := user.Lookup(username)
		if err != nil {
			println("Unknown user:", username)
			simon.Exit(simon.StatusGeneralError)
		}

		homeDir := forUser.HomeDir

		root := username == "root"
		println(fmt.Sprintf("Installing service for user `%s`...", username))
		println()

		var executableFile = homeDir + "/.local/bin/simon"
		var serviceFile = homeDir + "/.local/share/systemd/user/simon.service"
		var configFile = homeDir + "/.config/simon/config.yml"
		if root {
			executableFile = "/usr/local/bin/simon"
			serviceFile = "/lib/systemd/system/simon.service"
			configFile = "/etc/simon/config.yml"
		}

		copyCommand := exec.Command("cp", executable, executableFile)
		println("Copying executable file...")
		simon.PrintLnComment(copyCommand.String())
		println()

		chownCommand := exec.Command("chown", username+":"+username, executableFile)
		println("Setting executable owner...")
		simon.PrintLnComment(chownCommand.String())
		println()

		chmodCommand := exec.Command("chmod", "0755", executableFile)
		println("Setting executable rights...")
		simon.PrintLnComment(chmodCommand.String())
		println()

		configCommand := exec.Command(executableFile, "config", ">", configFile)
		println("Generating configuration file...")
		simon.PrintLnComment(configCommand.String())
		println()

		serviceCommand := exec.Command(executableFile, "service", ">", serviceFile)
		println("Generating service file...")
		simon.PrintLnComment(serviceCommand.String())
		println()

		if root {
			// Install for root user
			enableCommand := exec.Command("systemctl", "enable", "simon")
			println("Enabling system’s service...")
			simon.PrintLnComment(enableCommand.String())
			println()

			println("Service successfully installed.")

			startCommand := exec.Command("systemctl", "start", "simon")
			println("Edit the config file: " + configFile)
			println("And start system service when you are ready:")
			simon.PrintLnComment("\t" + startCommand.String())
			return
		}

		// Install for regular user
		enableCommand := exec.Command("sudo", "-u", username, "systemctl", "--user", "enable", "simon")
		println("Enabling user’s service...")
		simon.PrintLnComment(enableCommand.String())
		println()

		lingerCommand := exec.Command("sudo", "loginctl", "enable-linger", username)
		println("Extending service life...")
		simon.PrintLnComment(lingerCommand.String())
		println()

		println("Service successfully installed.")

		startCommand := exec.Command("sudo", "-u", username, "systemctl", "--user", "start", "simon")
		println("Edit the config file: " + configFile)
		println("And start user service when you are ready:")
		simon.PrintLnComment("\t" + startCommand.String())
	},
}

func init() {
	rootCmd.AddCommand(installCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	//installCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	installCmd.Flags().StringP("user", "u", "root", "install for the user")
}
