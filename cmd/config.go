package cmd

import (
	"fmt"
	simon "maximal/simon/modules"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// `configCmd` represents the `config` command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print YAML config file",
	Long: `Print YAML config file with the default settings.

You can use it like this:
	` + simon.APP_COMMAND + ` config  >  config.yml

Then edit ` + "`config.yml`" + ` to set InfluxDB settings,
monitored disks (mount points), I/O devices, network interfaces, etc.
By default, ` + simon.APP_NAME + ` will track all networks and disks,
including localhost network and temp filesystems.

Finally, run the monitor with the config:
	` + simon.APP_COMMAND + ` -c config.yml -i 15
`,
	Run: func(cmd *cobra.Command, args []string) {
		now := time.Now()
		text := fmt.Sprintf(
			simon.CONFIG_TEMPLATE,
			simon.APP_NAME,
			now.Format("2006-01-02"),
			now.Format("15:04:05-07:00"),
			simon.REPOSITORY_URL,
			getHostname(),
			getMounts(),
			getIoDevices(),
			getNetworkInterfaces(),
		)
		simon.PrintColorful(text)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func getHostname() string {
	hostnameBytes, err := os.ReadFile("/etc/hostname")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(hostnameBytes))
}

func getMounts() string {
	cmd := exec.Command("df", "-kP")
	stdout, err := cmd.Output()
	if err != nil {
		return "#- /"
	}
	result := []string{}
	for _, line := range strings.Split(string(stdout), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 6 {
			continue
		}
		result = append(result, parts[5])
	}
	if len(result) == 0 {
		return "#- /"
	}
	//sort.Slice(result, func(i, j int) bool {
	//	return result[i] < result[j]
	//})
	return "- " + strings.Join(result, "\n    - ")
}

func getIoDevices() string {
	contents, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return "#- vda\n    #- vda1\n    #- vdb"
	}
	result := []string{}
	for _, line := range strings.Split(string(contents), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 14 {
			continue
		}
		// Skip devices with all "zeroes", nothing to calcucate
		var allZeroes bool = true
		for index, part := range parts {
			if index < 3 {
				continue
			}
			if part != "0" {
				allZeroes = false
				break
			}
		}
		if allZeroes {
			continue
		}
		result = append(result, parts[2])
	}
	if len(result) == 0 {
		return "#- vda\n    #- vda1\n    #- vdb"
	}
	return "- " + strings.Join(result, "\n    - ")
}

func getNetworkInterfaces() string {
	contents, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return "#- lo\n    #- eth0"
	}
	result := []string{}
	for _, line := range strings.Split(string(contents), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 17 {
			continue
		}
		result = append(result, strings.TrimRight(parts[0], ":"))
	}
	if len(result) == 0 {
		return "#- lo\n    #- eth0"
	}
	return "- " + strings.Join(result, "\n    - ")
}
