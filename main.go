/**
 *
 * Copyright © 2024-2026 MaximAL
 *
 */

package main

import (
	"runtime"

	"maximal/simon/cmd"
)

func init() {
	runtime.GOMAXPROCS(2)
	//runtime.LockOSThread()
}

func main() {
	cmd.Execute()
}
