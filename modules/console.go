package simon

import (
	"fmt"
	"os"
	"strings"
)

func PrintColorful(text string) {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			PrintLnComment(line)
		} else {
			PrintLnNormal(line)
		}
	}
}

func PrintLnComment(line string) {
	o, _ := os.Stdout.Stat()
	if (o.Mode() & os.ModeCharDevice) == os.ModeCharDevice {
		//Terminal
		//Display info to the terminal
		// Blue
		//fmt.Println("\033[0;34m" + line + "\033[0m")
		// Green
		fmt.Println("\033[0;32m" + line + "\033[0m")
	} else { //It is not the terminal
		// Display info to a pipe
		fmt.Println(line)
	}
}

func PrintLnNormal(line string) {
	fmt.Println(line)
}
