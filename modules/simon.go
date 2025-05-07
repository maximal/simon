package simon

import (
	"errors"
	"fmt"
	"math"
	"maximal/simon/modules/influx"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

// Note: struct fields must be public in order for unmarshal to
// correctly populate the data.
type Config struct {
	Hostname string        `yaml:"hostname" default:""`
	Format   MetricsFormat `yaml:"format" default:"influx"`
	Mode     WorkingMode   `yaml:"mode" default:"push"`
	Server   struct {
		Port          uint16 `yaml:"port" default:"8000"`
		Host          string `yaml:"host" default:""`
		Authorization string `yaml:"authorization" default:""`
	}
	Influx influx.InfluxConfig
	Print  bool `yaml:"print" default:"false"`
	Uptime bool `yaml:"uptime" default:"true"`
	Cpu    bool `yaml:"cpu" default:"true"`
	Memory bool `yaml:"memory" default:"true"`
	Disk   struct {
		Enabled bool     `yaml:"enabled" default:"true"`
		Mounts  []string `yaml:"mounts" default:"nil"`
	}
	Io struct {
		Enabled bool     `yaml:"enabled" default:"true"`
		Devices []string `yaml:"devices" default:"nil"`
	}
	Network struct {
		Enabled    bool     `yaml:"enabled" default:"true"`
		Interfaces []string `yaml:"interfaces" default:"nil"`
	}
	Monitor bool `yaml:"monitor" default:"true"`
}

type WorkingMode string

const (
	Push   WorkingMode = "push"
	Server WorkingMode = "server"
)

type MetricsFormat string

const (
	Influx MetricsFormat = "influx"
)

var config Config
var configLoadedAt time.Time

// Number of minutes to wait before auto-reloading config
const RELOAD_CONFIG_EVERY uint64 = 30

type BasicStats struct {
	UptimeSeconds float64
	Runtime       time.Duration
	CpuUsage      float64
	MemoryUsage   float64
	SwapUsage     float64
	CollectTime   float64
	LastTime      time.Time
	LastSendMs    uint64
	MemoryAlloc   uint64
}

var currentStats BasicStats = BasicStats{
	CpuUsage: -1,
}

type CpuInfo struct {
	LastWorking uint64
	LastIdle    uint64
}

var cpus map[string]CpuInfo = map[string]CpuInfo{}

var neededMounts map[string]bool = map[string]bool{}

type IoInfo struct {
	ReadsCompleted  uint64
	ReadsSectors    uint64
	ReadsTime       uint64
	WritesCompleted uint64
	WritesSectors   uint64
	WritesTime      uint64
	IosTime         uint64
	Time            time.Time
}

var ioDevices map[string]IoInfo = map[string]IoInfo{}
var neededDevices map[string]bool = map[string]bool{}
var DEVICES_REQUIRED_EXPLICITLY []string = []string{"(?i)^loop[0-9]+$"}
var devicesRequiredExplicitly []regexp.Regexp = []regexp.Regexp{}

type NetworkInfo struct {
	ReceivedBytes    uint64
	TransmittedBytes uint64
	Time             time.Time
}

var interfaces map[string]NetworkInfo = map[string]NetworkInfo{}
var neededInterfaces map[string]bool = map[string]bool{}
var NETWORKS_REQUIRED_EXPLICITLY []string = []string{
	// Loopback
	//"(?i)^lo$",
	// Some Docker-specific names
	"(?i)^docker[0-9]+$",
	"(?i)^br-[0-9a-f]+$",
	"(?i)^[v]eth[0-9a-f]+$",
	"(?i)^tunl[0-9]+$",
	"(?i)^ip6tnl[0-9]+$",
	"(?i)^gre[0-9]+$",
	"(?i)^ip6gre[0-9]+$",
	"(?i)^ip_vti[0-9]+$",
	"(?i)^ip6_vti[0-9]+$",
	"(?i)^gretap[0-9]+$",
	"(?i)^erspan[0-9]+$",
	"(?i)^sit[0-9]+$",
}
var networksRequiredExplicitly []regexp.Regexp = []regexp.Regexp{}

var syncMutex sync.Mutex

const WEB_SERVER_THROTTLE_SECONDS float64 = 0.05

// Start SiMon’s main loop.
//
// It loads the configuration, initializes metrics collections,
// and starts the main loop, which runs indefinitely.
//
// The main loop collects metrics, sends them to InfluxDB if enabled,
// and prints some information about the system and the monitor.
// It also reloads the config if needed and calculates the sleep
// interval to maintain the specified interval between metrics
// collection and sending.
//
// The function returns only on panic.
func Work(configFile string, interval float32) {
	start := time.Now()

	validateRunning()
	loadConfig(configFile, start)
	createConfigReloadingSignalChannel(configFile)

	switch config.Mode {
	case Push:
		workPushMode(configFile, interval, start)
	case Server:
		workServerMode(configFile, start)
	default:
		Exit(StatusInvalidConfig)
	}
}

// Push mode
func workPushMode(configFile string, interval float32, start time.Time) {
	usInterval := int64(1_000_000 * interval)
	var index uint64 = 0

	for {
		index++
		loop := time.Now()
		currentStats.Runtime = loop.Sub(start)

		//
		collectMetrics()

		// Print some basic information
		printBasicStats(index, false)

		// If Influx enabled, send metrics
		if config.Influx.Enabled {
			print(" ... ")
			if sendMetrics(index) {
				println("sent in", currentStats.LastSendMs, "ms")
			} else {
				println("failed in", currentStats.LastSendMs, "ms")
			}
		} else {
			println()
		}

		// Print metrics if needed
		printMetrics(index)

		// Reload config if needed
		reloadConfig(configFile, loop)

		// Calculate sleep interval
		diff := usInterval - time.Since(loop).Microseconds()
		if diff > 0 {
			time.Sleep(time.Duration(diff) * time.Microsecond)
		}
	}
}

// Pull mode (web server)
func workServerMode(configFile string, start time.Time) {
	var index uint64 = 0

	// Default route
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Synchronize access to the global variables with the program’s state
		// We don’t need parallel executions here, so we can use a simple mutex
		syncMutex.Lock()
		defer syncMutex.Unlock()

		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		//w.Header().Add("Content-Type", "text/plain+influxdb+lineprotocol; charset=utf-8")
		w.Header().Add("Content-Disposition", "inline")

		// Other than root URL is 404
		if r.URL.Path != "/" {
			http.Error(w, "404 URL Not Found: "+r.URL.Path, http.StatusNotFound)
			return
		}

		switch r.Method {
		case "GET":

			// Authorization token check; 401
			token := config.Server.Authorization
			if token != "" && token != r.Header.Get("Authorization") {
				http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
				return
			}

			loop := time.Now()

			// Throttle requests, we don’t need to sample metrics so fast; 429
			//NOTE: Maybe, semantically correct way is to return previous results instead of 429
			// We’ll think about it later
			sinceLast := loop.Sub(currentStats.LastTime).Seconds()
			if sinceLast < WEB_SERVER_THROTTLE_SECONDS {
				// Too Fast
				retryAfter := WEB_SERVER_THROTTLE_SECONDS - sinceLast
				w.Header().Add("Retry-After", fmt.Sprintf("%.3f", retryAfter))
				http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
				return
			}

			// Normal execution; 200

			index++
			currentStats.Runtime = loop.Sub(start)

			//
			collectMetrics()

			// Response with metrics
			fmt.Fprintln(w, getMetricsText(index))

			// Print some basic information
			printBasicStats(index, true)

			// Print metrics if needed
			printMetrics(index)

			// Reload config if needed
			reloadConfig(configFile, loop)
		default:
			// Invalid method; 405
			w.Header().Add("Allow", "GET")
			http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// Favicon; 204 No Content
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			http.Error(w, "", http.StatusNoContent)
		default:
			w.Header().Add("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Run the server
	port := config.Server.Port
	println(fmt.Sprintf("# Listening on %s:%d...", config.Server.Host, port))
	var address string = config.Server.Host
	if address == "" {
		address = "0.0.0.0"
	}
	println(fmt.Sprintf("# Metrics will be available at http://%s:%d", address, port))
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", config.Server.Host, port), nil)
	if errors.Is(err, http.ErrServerClosed) {
		println("# Server closed")
	} else if err != nil {
		println(fmt.Sprintf("# Error listening for server: %s", err))
	}
}

func printBasicStats(index uint64, newLine bool) {
	// CPU usage is not known on the first iteration
	var cpuUsage string
	if index > 1 {
		cpuUsage = fmt.Sprintf("  CPU: %.1f%%", currentStats.CpuUsage)
	} else {
		cpuUsage = "  CPU: N/A%"
	}

	// Print some basic information
	print(
		"#",
		index,
		"    System:  uptime: ",
		formatDurationSeconds(currentStats.UptimeSeconds),
		cpuUsage,
		fmt.Sprintf("  memory: %.1f%%", currentStats.MemoryUsage),
		"    Monitor:  runtime: ",
		formatDurationSeconds(currentStats.Runtime.Abs().Seconds()),
		"  memory: ",
		formatBytes(currentStats.MemoryAlloc),
		//"  metrics: ",
		//influx.GetMetricsCount(),
	)

	if newLine {
		println()
	}
}

func loadConfig(configFile string, time time.Time) {
	contents, err := os.ReadFile(configFile)
	if err != nil {
		Exit(StatusInvalidConfig, err)
	}

	config = Config{}
	err = yaml.Unmarshal(contents, &config)
	if err != nil {
		Exit(StatusInvalidConfig, err)
	}

	if config.Hostname == "" {
		hostname, err := os.ReadFile("/etc/hostname")
		if err != nil {
			config.Hostname = "unknown"
		} else {
			config.Hostname = strings.TrimSpace(string(hostname))
		}
	}

	// Working mode
	switch strings.ToLower(string(config.Mode)) {
	case "", "push":
		config.Mode = Push
	case "pull", "server":
		config.Mode = Server
	default:
		println("Invalid working mode: `" + config.Mode + "`")
		println("Valid modes: `push` (send metrics, default), `server` (print metrics on HTTP requests)")
		Exit(StatusInvalidConfig)
	}

	// Metrics format
	switch strings.ToLower(string(config.Format)) {
	case "", "influx", "influxdb", "influx-db", "influx_db", "influx db":
		config.Format = Influx
	default:
		println("Invalid metrics format: `" + config.Format + "`")
		println("Valid formats: `influx` (InfluxDB line protocol)")
		Exit(StatusInvalidConfig)
	}

	// Web Server
	if config.Server.Port < 1 {
		config.Server.Port = 8000
	}

	// InfluxDB settings
	if config.Influx.Enabled {
		switch config.Influx.Precision {
		case "", "s", "sec":
			config.Influx.Precision = "s"
		case "m", "ms":
			config.Influx.Precision = "ms"
		case "u", "us", "µ", "µs":
			config.Influx.Precision = "us"
		case "n", "ns":
			config.Influx.Precision = "ns"
		default:
			println("Invalid InfluxDB precision: " + config.Influx.Precision)
			Exit(StatusInvalidConfig)
		}

		if config.Influx.Host == "" || config.Influx.Host == "HOST REQUIRED" {
			println(fieldRequiredForInflux("host"))
			Exit(StatusInvalidConfig)
		}
		if config.Influx.Token == "" || config.Influx.Token == "TOKEN REQUIRED" {
			println(fieldRequiredForInflux("token"))
			Exit(StatusInvalidConfig)
		}
		if config.Influx.Org == "" || config.Influx.Org == "ORG REQUIRED" {
			println(fieldRequiredForInflux("org"))
			Exit(StatusInvalidConfig)
		}
		if config.Influx.Bucket == "" || config.Influx.Bucket == "BUCKET REQUIRED" {
			println(fieldRequiredForInflux("bucket"))
			Exit(StatusInvalidConfig)
		}
	}

	for _, name := range config.Network.Interfaces {
		neededInterfaces[name] = true
	}
	for _, name := range config.Io.Devices {
		neededDevices[name] = true
	}
	for _, name := range config.Disk.Mounts {
		neededMounts[name] = true
	}
	for _, regex := range DEVICES_REQUIRED_EXPLICITLY {
		devicesRequiredExplicitly = append(devicesRequiredExplicitly, *regexp.MustCompile(regex))
	}
	for _, regex := range NETWORKS_REQUIRED_EXPLICITLY {
		networksRequiredExplicitly = append(networksRequiredExplicitly, *regexp.MustCompile(regex))
	}

	configLoadedAt = time
}

func reloadConfig(configFile string, time time.Time) {
	if uint64(time.Sub(configLoadedAt).Abs().Minutes()) >= RELOAD_CONFIG_EVERY {
		println("Auto-reloading config:", configFile)
		loadConfig(configFile, time)
	}
}

// Configuration reloading via catching `SIGHUP` OS signal
func createConfigReloadingSignalChannel(configFile string) {
	// Configuration reloading via catching `SIGHUP` OS signal
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGHUP)
	go func() {
		for {
			sig := <-signalChannel
			if sig == syscall.SIGHUP {
				println("SIGHUP signal received, reloading config...")
				loadConfig(configFile, time.Now())
			}
		}
	}()
}

func collectMetrics() {
	now := time.Now()

	influx.ResetMetrics()
	if config.Influx.Tags == nil {
		config.Influx.Tags = map[string]string{}
	}
	config.Influx.Tags["host"] = config.Hostname
	influx.SetCommonTags(config.Influx.Tags)

	collectUptime()
	collectCpu()
	collectMemory()

	if config.Disk.Enabled {
		collectDisk()
	}
	if config.Io.Enabled {
		collectIo()
	}
	if config.Network.Enabled {
		collectNetwork()
	}
	collectMonitor()
	if config.Monitor && config.Influx.Enabled && currentStats.LastSendMs > 0 {
		addMetricFloat(
			"monitor_metrics_sent_time",
			float64(currentStats.LastSendMs)/1_000,
		)
	}
	currentStats.LastTime = now
	currentStats.CollectTime = time.Since(now).Seconds()
}

// Collect uptime metrics
func collectUptime() {
	contents, err := os.ReadFile("/proc/uptime")
	if err != nil {
		Exit(StatusGeneralError, err)
	}
	uptime, err := strconv.ParseFloat(strings.Split(string(contents), " ")[0], 64)
	if err != nil {
		Exit(StatusGeneralError, err)
	}
	currentStats.UptimeSeconds = uptime
	if config.Uptime {
		addMetricFloat("uptime", uptime)
	}
}

// Collect CPU metrics
func collectCpu() {
	loadAvgContents, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		Exit(StatusGeneralError, err)
	}
	loadAvgFields := strings.Fields(string(loadAvgContents))
	loadAvg1min, _ := strconv.ParseFloat(loadAvgFields[0], 64)
	//currentTasks, _ := strconv.ParseUint(loadAvgFields[3], 10, 64)

	procStatContents, err := os.ReadFile("/proc/stat")
	if err != nil {
		Exit(StatusGeneralError, err)
	}

	var processes uint64 = 0
	var running uint64 = 0
	var blocked uint64 = 0
	var cpuCount uint64 = 0

	// file: /proc/stat
	//   0       1     2       3       4       5    6         7      8      9          10
	//        user  nice  system    idle  ioWait  irq   softIrq  steal  guest  guest_nice
	// cpu  151882  1102   40189  891653    2676    0      1368      0      0           0

	for _, line := range strings.Split(string(procStatContents), "\n") {
		items := strings.Fields(line)

		len := len(items)
		if len == 0 {
			continue
		}

		if items[0] == "processes" {
			processes, _ = strconv.ParseUint(items[1], 10, 64)
			continue
		}

		if items[0] == "procs_blocked" {
			blocked, _ = strconv.ParseUint(items[1], 10, 64)
			continue
		}

		if items[0] == "procs_running" {
			running, _ = strconv.ParseUint(items[1], 10, 64)
			continue
		}

		if len != 11 {
			continue
		}

		cpuName := items[0]

		if cpuName != "cpu" && strings.HasPrefix(items[0], "cpu") {
			cpuCount++
		}

		if cpuName != "cpu" {
			continue
		}

		// non-idle = user + nice + system + irq + softIrq + steal
		user, _ := strconv.ParseUint(items[1], 10, 64)
		nice, _ := strconv.ParseUint(items[2], 10, 64)
		system, _ := strconv.ParseUint(items[3], 10, 64)
		irq, _ := strconv.ParseUint(items[6], 10, 64)
		softIrq, _ := strconv.ParseUint(items[7], 10, 64)
		steal, _ := strconv.ParseUint(items[8], 10, 64)
		working := user + nice + system + irq + softIrq + steal

		// idle = idle + ioWait
		idle, _ := strconv.ParseUint(items[4], 10, 64)
		ioWait, _ := strconv.ParseUint(items[5], 10, 64)
		idleTotal := idle + ioWait
		//println("\tworking:", working, "idle:", idleTotal)

		if cpuInfo, exists := cpus[cpuName]; exists {
			diffWorking := float64(working - cpuInfo.LastWorking)
			// usage = 100% * diffWorking / (diffWorking + diffIdle)
			// https://github.com/htop-dev/htop/blob/40104588f38250afde9f71b6204d789039bbfe3e/linux/LinuxProcessList.c#L2075
			// https://stackoverflow.com/questions/23367857/accurate-calculation-of-cpu-usage-given-in-percentage-in-linux
			// https://stackoverflow.com/questions/1420426/how-to-calculate-the-cpu-usage-of-a-process-by-pid-in-linux-from-c/1424556#1424556
			usage := 100.0 *
				diffWorking /
				(diffWorking + float64(idleTotal-cpuInfo.LastIdle))

			if cpuName == "cpu" {
				// Add measurement for aggregate CPU usage only
				currentStats.CpuUsage = usage
				if config.Cpu {
					addMetricFloat("cpu_usage", usage)
				}
			}
		}
		cpus[cpuName] = CpuInfo{LastWorking: working, LastIdle: idleTotal}
	}

	if config.Cpu {
		addMetricFloat("load_avg", loadAvg1min)
		if cpuCount > 0 {
			// On first iteration have load_average / cpu_count as CPU usage
			loadUsage := 100.0 * loadAvg1min / float64(cpuCount)
			addMetricUint("cpu_count", cpuCount)
			addMetricFloat("load_usage", loadUsage)
		}
		addMetricUint("cpu_processes", processes)
		addMetricUint("cpu_processes_running", running)
		addMetricUint("cpu_processes_blocked", blocked)
	}
}

// Collect Memory metrics
func collectMemory() {

	contents, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		Exit(StatusGeneralError, err)
	}

	var total uint64 = 0
	var free uint64 = 0
	var available uint64 = 0
	var buffers uint64 = 0
	var cached uint64 = 0
	var swapTotal uint64 = 0
	var swapFree uint64 = 0
	var swapCached uint64 = 0

	for _, line := range strings.Split(string(contents), "\n") {
		items := strings.Fields(line)
		//println(items)
		if len(items) != 3 {
			continue
		}

		var multiplier uint64 = 1
		switch strings.ToLower(items[2]) {
		case "kb", "kib":
			multiplier = 1024
		case "mb", "mib":
			multiplier = 1024 * 1024
		case "gb", "gib":
			multiplier = 1024 * 1024 * 1024
		case "tb", "tib":
			multiplier = 1024 * 1024 * 1024 * 1024
		case "pb", "pib":
			multiplier = 1024 * 1024 * 1024 * 1024 * 1024
		default:
			multiplier = 1
		}

		value, _ := strconv.ParseUint(items[1], 10, 64)

		switch items[0] {
		case "MemTotal:":
			total = value * multiplier
		case "MemFree:":
			free = value * multiplier
		case "MemAvailable:":
			available = value * multiplier
		case "Buffers:":
			buffers = value * multiplier
		case "Cached:":
			cached = value * multiplier
		case "SwapTotal:":
			swapTotal = value * multiplier
		case "SwapFree:":
			swapFree = value * multiplier
		case "SwapCached:":
			swapCached = value * multiplier
		default:
			continue
		}
	}

	if total > 0 {
		// https://github.com/aristocratos/btop/issues/161#issuecomment-974869016
		// Modern calculation of memoru usage with `available`
		used := total - available
		usage := 100.0 * float64(used) / float64(total)
		currentStats.MemoryUsage = usage
		// Classic way without `available`, like `free` or `htop`
		usedClassic := total - free - buffers - cached - swapCached
		usageClassic := 100.0 * float64(usedClassic) / float64(total)

		if config.Memory {
			// Memory usage metrics
			addMetricUint("memory_total", total)
			addMetricUint("memory_free", free)
			addMetricUint("memory_available", available)
			addMetricUint("memory_buffers", buffers)
			addMetricUint("memory_cached", cached)
			addMetricUint("memory_used", used)
			addMetricFloat("memory_usage", usage)
			addMetricUint("memory_used_classic", usedClassic)
			addMetricFloat("memory_usage_classic", usageClassic)
			// Swap metrics
			addMetricUint("memory_swap_total", swapTotal)
			addMetricUint("memory_swap_free", swapFree)
			addMetricUint("memory_swap_cached", swapCached)
		}
	}

	if swapTotal > 0 {
		swapUsed := swapTotal - swapFree
		swapUsage := 100.0 * float64(swapUsed) / float64(swapTotal)
		currentStats.SwapUsage = swapUsage
		if config.Memory {
			// Swap usage metrics
			addMetricUint("memory_swap_used", swapUsed)
			addMetricFloat("memory_swap_usage", swapUsage)
		}
	}
}

// Collect Disk metrics
func collectDisk() {
	all := len(config.Disk.Mounts) == 0

	// 1KiB, 1024 B blocks
	cmd := exec.Command("df", "-kP")
	stdout, err := cmd.Output()
	if err != nil {
		Exit(StatusGeneralError, err)
	}

	for _, line := range strings.Split(string(stdout), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 6 {
			continue
		}

		// df -kP
		// 0                     1         2          3     4  5
		// Filesystem  1024-blocks      Used  Available  Use%  Mounted on
		// /dev/vda2     206319536  36351928  161510716   19%  /
		// tmpfs           1637216      1276    1635940    1%  /run
		// tmpfs           8186080         0    8186080    0%  /var/tmp

		mount := parts[5]

		// This mount needed?
		_, mountNeeded := neededMounts[mount]
		if !mountNeeded && !all {
			continue
		}

		filesystem := parts[0]
		//NOTE: Better `disk` or `device` to be similar with I/O metrics?
		tags := map[string]string{"disk": filesystem, "mount": mount}

		total, _ := strconv.ParseUint(parts[1], 10, 64)
		used, _ := strconv.ParseUint(parts[2], 10, 64)
		available, _ := strconv.ParseUint(parts[3], 10, 64)
		usage := 100.0 * float64(used) / float64(total)

		addMetricUint("disk_total", 1024*total, tags)
		addMetricUint("disk_used", 1024*used, tags)
		addMetricUint("disk_available", 1024*available, tags)
		addMetricFloat("disk_usage", usage, tags)
	}
}

// Collect I/O metrics
func collectIo() {
	time := time.Now()
	contents, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		Exit(StatusGeneralError, err)
	}
	all := len(config.Io.Devices) == 0

	var maxUsage float64 = 0
	var maxUsageDevice string = ""

	for _, line := range strings.Split(string(contents), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 14 {
			continue
		}
		device := parts[2]

		// This device explicitly needed?
		_, explicitlyNeeded := neededDevices[device]
		if !explicitlyNeeded {
			if !all {
				continue
			}
			if matchesAnyPattern(device, devicesRequiredExplicitly) {
				// Skip `loop<n>` and other not interesting disks
				// if they are not required explicitly
				continue
			}
		}

		// Skip devices with all "zeroes", nothing to calculate
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

		tags := map[string]string{"device": device}

		// Device         Reads                          Writes                          I/Os
		//   0 1      2       3      4       5       6        7    8       9      10       11        12
		//       device  r_cmpl        r_sects  r_time   w_cmpl      w_sects  w_time  ios_cur  ios_time ...
		// 254 1   vda1   16105  23375  417066    3150     6783 9359  476456   15425        0      9834 ...

		readsCompleted, _ := strconv.ParseUint(parts[3], 10, 64)
		readsSectors, _ := strconv.ParseUint(parts[5], 10, 64)
		readsTime, _ := strconv.ParseUint(parts[6], 10, 64)
		writesCompleted, _ := strconv.ParseUint(parts[7], 10, 64)
		writesSectors, _ := strconv.ParseUint(parts[9], 10, 64)
		writesTime, _ := strconv.ParseUint(parts[10], 10, 64)
		iosCurrent, _ := strconv.ParseUint(parts[11], 10, 64)
		iosTime, _ := strconv.ParseUint(parts[12], 10, 64)

		addMetricUint("io_current_ops", iosCurrent, tags)

		if last, exists := ioDevices[device]; exists {
			//println(last)

			// If previous values for this disk are known, calculate IO speed and usage
			timeDiff := time.Sub(last.Time).Abs().Seconds()

			// Operations per second
			writeOpsSpeed := (float64(writesCompleted) - float64(last.WritesCompleted)) / timeDiff
			readOpsSpeed := (float64(readsCompleted) - float64(last.ReadsCompleted)) / timeDiff

			// Bytes per second; sector size = 512 B = 1/2 KiB
			writeSpeed := 512 * (float64(writesSectors) - float64(last.WritesSectors)) / timeDiff
			readSpeed := 512 * (float64(readsSectors) - float64(last.ReadsSectors)) / timeDiff

			// IO utilization / device saturation
			writeUsage := 100.0 * (float64(writesTime) - float64(last.WritesTime)) / timeDiff / 1_000
			readUsage := 100.0 * (float64(readsTime) - float64(last.ReadsTime)) / timeDiff / 1_000
			ioUsage := 100.0 * (float64(iosTime) - float64(last.IosTime)) / timeDiff / 1_000

			//NOTE: Should be weighted by CPU count?
			// https://unix.stackexchange.com/questions/581778/proc-diskstats-disk-read-time-increasing-more-than-second-per-second/581790
			// https://www.kernel.org/doc/Documentation/iostats.txt
			// https://docs.percona.com/percona-toolkit/pt-diskstats.html
			//if ($this->cpuCount > 0) {
			//$writeUsage /= $this->cpuCount;
			//$readUsage /= $this->cpuCount;
			//$ioUsage /= $this->cpuCount;
			//}
			//echo $disk, ' : ', $ioUsage, PHP_EOL;

			addMetricFloat("io_write_ops_speed", writeOpsSpeed, tags)
			addMetricFloat("io_write_speed", writeSpeed, tags)
			addMetricFloat("io_write_usage", writeUsage, tags)
			addMetricFloat("io_read_ops_speed", readOpsSpeed, tags)
			addMetricFloat("io_read_speed", readSpeed, tags)
			addMetricFloat("io_read_usage", readUsage, tags)
			addMetricFloat("io_usage", ioUsage, tags)

			// Find the most used device
			if ioUsage > maxUsage || maxUsageDevice == "" {
				maxUsage = ioUsage
				maxUsageDevice = device
			}
		}

		ioDevices[device] = IoInfo{
			ReadsCompleted:  readsCompleted,
			ReadsSectors:    readsSectors,
			ReadsTime:       readsTime,
			WritesCompleted: writesCompleted,
			WritesSectors:   writesSectors,
			WritesTime:      writesTime,
			IosTime:         iosTime,
			Time:            time,
		}
	}

	if maxUsageDevice != "" {
		influx.AddMetric(
			"io_usage_max",
			[]influx.Field{
				{Name: "value", Type: influx.TypeFloat, FloatValue: maxUsage},
				{Name: "device", Type: influx.TypeString, StringValue: maxUsageDevice},
			},
			map[string]string{"device": maxUsageDevice},
			METRICS_DESCRIPTIONS["io_usage_max"],
		)
	}
}

// Collect Network metrics
func collectNetwork() {
	time := time.Now()
	contents, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		Exit(StatusGeneralError, err)
	}
	all := len(config.Network.Interfaces) == 0

	var maxLoadIn uint64 = 0
	var maxLoadInInterface string = ""
	var maxLoadOut uint64 = 0
	var maxLoadOutInterface string = ""

	// 0            1       2    3    4    5     6          7         8        9      10   11   12   13    14      15         16
	// Inter-|  Receive                                                |  Transmit
	// face  |  bytes packets errs drop fifo frame compressed multicast|   bytes packets errs drop fifo colls carrier compressed
	// lo:    2776770   11307    0    0    0     0          0         0  2776770   11307    0    0    0     0       0          0
	// eth0:  1215645    2751    0    0    0     0          0         0  1782404    4324    0    0    0   427       0          0
	// ppp0:  1622270    5552    1    0    0     0          0         0   354130    5669    0    0    0     0       0          0

	for _, line := range strings.Split(string(contents), "\n") {
		items := strings.Fields(line)
		if len(items) != 17 {
			continue
		}

		interfaceName := strings.TrimRight(items[0], ":")
		// This interface explicitly needed?
		_, explicitlyNeeded := neededInterfaces[interfaceName]
		if !explicitlyNeeded {
			if !all {
				continue
			}
			if matchesAnyPattern(interfaceName, networksRequiredExplicitly) {
				// Skip Docker’s and some other non-interesting interfaces
				// if they are not required explicitly
				continue
			}
		}

		tags := map[string]string{"interface": interfaceName}

		// Traffic (bytes)
		inBytes, _ := strconv.ParseUint(items[1], 10, 64)
		outBytes, _ := strconv.ParseUint(items[9], 10, 64)

		addMetricUint("network_in", inBytes, tags)
		addMetricUint("network_out", outBytes, tags)

		if interfaceInfo, exists := interfaces[interfaceName]; exists {
			// Load (bits per second)
			inDiff := math.Abs(float64(inBytes) - float64(interfaceInfo.ReceivedBytes))
			outDiff := math.Abs(float64(outBytes) - float64(interfaceInfo.TransmittedBytes))
			seconds := time.Sub(interfaceInfo.Time).Abs().Seconds()
			inLoad := uint64(math.Round(8 * inDiff / seconds))
			outLoad := uint64(math.Round(8 * outDiff / seconds))

			addMetricUint("network_load_in", inLoad, tags)
			addMetricUint("network_load_out", outLoad, tags)

			// Find the most loaded interfaces
			if inLoad > maxLoadIn || maxLoadInInterface == "" {
				maxLoadIn = inLoad
				maxLoadInInterface = interfaceName
			}
			if outLoad > maxLoadOut || maxLoadOutInterface == "" {
				maxLoadOut = outLoad
				maxLoadOutInterface = interfaceName
			}
		}

		interfaces[interfaceName] = NetworkInfo{
			ReceivedBytes:    inBytes,
			TransmittedBytes: outBytes,
			Time:             time,
		}
	}

	if maxLoadInInterface != "" {
		influx.AddMetric(
			"network_load_in_max",
			[]influx.Field{
				{Name: "value", Type: influx.TypeUint, UintValue: maxLoadIn},
				{Name: "interface", Type: influx.TypeString, StringValue: maxLoadInInterface},
			},
			map[string]string{"interface": maxLoadInInterface},
			METRICS_DESCRIPTIONS["network_load_in_max"],
		)
	}
	if maxLoadOutInterface != "" {
		influx.AddMetric(
			"network_load_out_max",
			[]influx.Field{
				{Name: "value", Type: influx.TypeUint, UintValue: maxLoadOut},
				{Name: "interface", Type: influx.TypeString, StringValue: maxLoadOutInterface},
			},
			map[string]string{"interface": maxLoadOutInterface},
			METRICS_DESCRIPTIONS["network_load_out_max"],
		)
	}
}

// Collect monitor’s own metrics
func collectMonitor() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	currentStats.MemoryAlloc = memStats.Alloc

	if config.Monitor {
		addMetricFloat("monitor_runtime", currentStats.Runtime.Seconds())
		addMetricUint("monitor_memory_alloc", currentStats.MemoryAlloc)
		//addMetricUint("monitor_memory_alloc_total", memStats.TotalAlloc)
		addMetricUint("monitor_memory_system", memStats.Sys)
		addMetricUint("monitor_memory_gc_count", uint64(memStats.NumGC))
	}
}

func sendMetrics(index uint64) bool {
	beforeSend := time.Now()
	result := influx.SendText(getMetricsText(index), config.Influx)
	currentStats.LastSendMs = uint64(time.Since(beforeSend).Milliseconds())
	return result
}

func printMetrics(index uint64) {
	if config.Print {
		PrintColorful(getMetricsText(index))
	}
}

func checkFile(filename string) bool {
	_, err := os.OpenFile(filename, os.O_RDONLY, 0644)
	if errors.Is(err, os.ErrNotExist) {
		println("No `" + filename + "` file.")
		return false
	}
	return true
}

func validateRunning() {
	// Is `/proc` filesystem present?
	isProc := checkFile("/proc/uptime") &&
		checkFile("/proc/loadavg") &&
		checkFile("/proc/stat") &&
		checkFile("/proc/meminfo") &&
		checkFile("/proc/net/dev") &&
		checkFile("/proc/diskstats")
	if !isProc {
		println(APP_NAME + " needs `/proc` filesystem to run.")
		Exit(StatusUnsupportedPlatform)
	}

	// Already running?
	if false {
		Exit(StatusAlreadyRunning)
	}

	//TODO: Logic

	//currentPath, _ := os.Executable()
	//currentPid := os.Getpid()
	//println("PID:", currentPid)
	//println("currentPath:", currentPath)
	//cmd := exec.Command("ps", "-eo", "user,pid,args")
	//stdout, _ := cmd.Output()
	//println("stdout:", string(stdout))
	//for _, line := range strings.Split(string(stdout), "\n") {
	//	slice := strings.Fields(line)
	//	println("`" + strings.Join(slice, "`, `") + "`")
	//}

	/*
		// Already running?
		$filename = pathinfo(__FILE__, PATHINFO_BASENAME);
		$output = $code = null;
		exec(
			'ps -eo pid,user,args | grep ' . escapeshellarg($filename),
			$output,
			$code
		);
		if ($code !== 0) {
			return;
		}
		$myPid = getmypid();
		foreach ($output as $line) {
			$match = null;
			if (!preg_match('/\s*(\d+)\s+(\S+)\s+(.*)/', $line, $match)) {
				continue;
			}
			$pid = (int)$match[0];
			$user = trim($match[2]);
			$command = trim($match[3]);
			if ($pid === $myPid) {
				continue;
			}
			if (str_starts_with($command, 'sudo ')) {
				continue;
			}
			if (str_contains($line, ' grep ')) {
				continue;
			}
			self::writeStderrLine(
				self::NAME . ' already running under `' . $user . '` user with PID ' . $pid . '.'
			);
			Exit(ExitAlreadyRunning)
		}
	*/
}

func tagsMapArrayToMap(tags []map[string]string) map[string]string {
	tagsMap := map[string]string{}
	for _, values := range tags {
		for key, tag := range values {
			tagsMap[key] = tag
		}
	}
	return tagsMap
}

func addMetricFloat(name string, value float64, tags ...map[string]string) {
	var comment string
	if knownDescription, exists := METRICS_DESCRIPTIONS[name]; exists {
		comment = knownDescription
	} else {
		comment = ""
	}
	influx.AddFloatMetric(name, value, tagsMapArrayToMap(tags), comment)
}

func addMetricUint(name string, value uint64, tags ...map[string]string) {
	var comment string
	if knownDescription, exists := METRICS_DESCRIPTIONS[name]; exists {
		comment = knownDescription
	} else {
		comment = ""
	}
	influx.AddUintMetric(name, value, tagsMapArrayToMap(tags), comment)
}

func getMetricsText(index uint64) string {
	prepend := []string{
		"####",
		"# " + APP_NAME + " metrics",
		"# #" + strconv.FormatUint(index, 10),
		"#",
		//"# @time " + time.Now().Format(time.RFC3339),
		"# @time " + time.Now().Format("2006-01-02T15:04:05.000Z07:00"),
		"# @link " + REPOSITORY_URL,
		"# @link " + INFLUX_PROTOCOL_URL,
		"####",
	}
	return strings.Join(prepend, "\n") + "\n" + influx.GetMetricsText()
}

func fieldRequiredForInflux(field string) string {
	return "InfluxDB `" + field + "` is required if InfluxDB is enabled"
}

func matchesAnyPattern(string string, patterns []regexp.Regexp) bool {
	for _, pattert := range patterns {
		if pattert.MatchString(string) {
			return true
		}
	}
	return false
}
