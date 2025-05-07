/**
 *
 * Copyright © 2024-2025 MaximAL
 *
 */

package main

import (
	"maximal/simon/cmd"
	"runtime"
)

func init() {
	runtime.GOMAXPROCS(2)
	//runtime.LockOSThread()
}

func main() {
	cmd.Execute()
}
