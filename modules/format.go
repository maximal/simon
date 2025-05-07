package simon

import (
	"fmt"
	"math"
)

var BYTE_UNITS_IEC = [...]string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB", "RiB", "QiB"}
var BYTE_UNITS_SI = [...]string{"B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB", "RB", "QB"}

var BYTEPS_UNITS_IEC = [...]string{"B/s", "KiB/s", "MiB/s", "GiB/s", "TiB/s", "PiB/s", "EiB/s", "ZiB/s", "YiB/s", "RiB/s", "QiB/s"}
var BYTEPS_UNITS_SI = [...]string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s", "PB/s", "EB/s", "ZB/s", "YB/s", "RB/s", "QB/s"}

var BIT_UNITS_IEC = [...]string{"b", "Kib", "Mib", "Gib", "Tib", "Pib", "Eib", "Zib", "Yib", "Rib", "Qib"}
var BIT_UNITS_SI = [...]string{"b", "kb", "Mb", "Gb", "Tb", "Pb", "Eb", "Zb", "Yb", "Rb", "Qb"}

var BITPS_UNITS_IEC = [...]string{"b/s", "Kib/s", "Mib/s", "Gib/s", "Tib/s", "Pib/s", "Eib/s", "Zib/s", "Yib/s", "Rib/s", "Qib/s"}
var BITPS_UNITS_SI = [...]string{"b/s", "kb/s", "Mb/s", "Gb/s", "Tb/s", "Pb/s", "Eb/s", "Zb/s", "Yb/s", "Rb/s", "Qb/s"}

func formatDurationSeconds(seconds float64) string {
	sec10th := uint64(math.Round(seconds * 10))
	if sec10th < 10*60*60 {
		// Less than an hour
		// minutes:seconds
		return fmt.Sprintf(
			"%d:%02d.%d",
			sec10th/10/60,
			sec10th/10%60,
			sec10th%10,
		)
	}
	if sec10th < 10*60*60*24 {
		// Less than a day
		// hours:minutes:seconds
		return fmt.Sprintf(
			"%d:%02d:%02d.%d",
			sec10th/10/60/60,
			sec10th/10/60%60,
			sec10th/10%60,
			sec10th%10,
		)
	}
	// days:hours:minutes:seconds
	return fmt.Sprintf(
		"%dd%02d:%02d:%02d.%d",
		sec10th/10/60/60/24,
		sec10th/10/60/60%24,
		sec10th/10/60%60,
		sec10th/10%60,
		sec10th%10,
	)
}

func formatBytes(bytes uint64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d %s", bytes, BYTE_UNITS_IEC[0])
	}
	var index uint8 = 0
	var number float64 = float64(bytes)
	for number > 998 && index < 10 {
		number /= 1024.0
		index++
	}
	//units := [...]string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB", "RiB", "QiB"}
	if number < 10 {
		return fmt.Sprintf("%.2f %s", number, BYTE_UNITS_IEC[index])
	}
	if number < 100 {
		return fmt.Sprintf("%.1f %s", number, BYTE_UNITS_IEC[index])
	}
	return fmt.Sprintf("%.0f %s", number, BYTE_UNITS_IEC[index])
}
