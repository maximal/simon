package simon

import "os"

type ExitStatus uint8

const (
	StatusOk ExitStatus = 0
	// Unknown errors, service should be restarted
	StatusFatal ExitStatus = 1
	StatusPanic ExitStatus = 2
	// Known errors, restart won’t be successful
	StatusGeneralError        ExitStatus = 10
	StatusInvalidConfig       ExitStatus = 11
	StatusUnsupportedPlatform ExitStatus = 12
	StatusAlreadyRunning      ExitStatus = 13
	// ... ... ...
)

func Exit(status ExitStatus, err ...error) {
	for _, e := range err {
		println("error:", e.Error())
	}
	os.Exit(int(status))
}
