package simon

import "os"

type ExitStatus uint8

const (
	StatusOk ExitStatus = 0
	// Unknown errors, service should be restarted
	StatusFatal ExitStatus = 1
	StatusPanic ExitStatus = 2
	// Known errors, restart won’t be successful
	StatusGeneralError        ExitStatus = 3
	StatusInvalidConfig       ExitStatus = 4
	StatusUnsupportedPlatform ExitStatus = 5
	StatusAlreadyRunning      ExitStatus = 6
	// ... ... ...
)

func Exit(status ExitStatus, err ...error) {
	for _, e := range err {
		println("error:", e.Error())
	}
	os.Exit(int(status))
}
