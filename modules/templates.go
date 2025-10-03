package simon

const APP_NAME string = "SiMon"
const APP_VERSION string = "1.0.0+source"
const APP_COMMAND string = "simon"
const REPOSITORY_URL string = "https://github.com/maximal/simon"
const REPOSITORY_PHP_URL string = "https://github.com/maximal/simon-php"
const INFLUX_PROTOCOL_URL string = "https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol/"

const CONFIG_TEMPLATE string = `####
# %s config.
#
# @date %s
# @time %s
#
# @link %s
# @link https://maximals.ru
# @link https://sijeko.ru
####

# This machine name, for measurements tag ` + "`host`" + `
# It will be added to every measurement
# If ` + "`null`" + `, will be taken from ` + "`/etc/hostname`" + ` file
hostname: %s

# Working mode: ` + "`push`" + ` (send metrics, default), ` + "`server`" + ` (print metrics on HTTP requests)
mode: push

# Web server settings (for ` + "`server`" + ` mode only)
server:
  # Port to listen to
  port: 8000
  # Host to listen to; default, empty string, means all addresses on all interfaces
  #host: ""
  # ` + "`Aurhorization`" + ` HTTP header to check before showing metrics;
  # Any random string; if empty, metrics will be shown without authorization
  #authorization: ""

# InfluxDB settings
influx:
  # Whether to send data to Influx; disabled by default, console output only
  #enabled: true
  # Influx host name, required if Influx enabled
  # HTTP installation example, with port: influx.host.org:8086
  # HTTPS installation example: https://influx.host.org
  host: HOST REQUIRED
  # Influx API token, required if Influx enabled
  token: TOKEN REQUIRED
  # Influx org name, required if Influx enabled
  org: ORG REQUIRED
  # Influx bucket name, required if Influx enabled
  bucket: BUCKET REQUIRED
  # Timestamps precision: ` + "`s` (default), " + "`ms`, " + "`μs`/`us`, " + "`ns`" + `
  # It is recommended not to use ` + "`ms`, " + "`μs`, or " + "`ns`" + `
  # if the monitoring interval is greater than ` + "`1`" + `
  precision: s
  # Additional tags to be added to every measurement; key:value pairs
  tags:
    #custom1: value1
    #custom2: value2

# Whether to track system uptime; enabled by default
uptime: true

# Whether to track CPU usage; enabled by default
cpu: true

# Whether to track memory usage; enabled by default
memory: true

disk:
  # Whether to track disks usage; enabled by default
  enabled: true
  # Mount points to track; default, empty array, means all
  mounts:
    %s

io:
  # Whether to track I/O usage; enabled by default
  enabled: true
  # I/O devices to track; default, empty array, means all
  devices:
    %s

network:
  # Whether to track network interfaces usage; enabled by default
  enabled: true
  # Network interfaces to track; default, empty array, means all
  interfaces:
    %s

# Whether to track monitor’s own metrics; enabled by default
monitor: true

# Whether to print metrics to stdout; disabled by default, set true to enable
#print: true`

const SERVICE_TEMPLATE string = `####
# @link %s
####
[Unit]
Description=%s, Linux system monitor
Documentation=%s
StartLimitIntervalSec=60
StartLimitBurst=4

[Service]
#ExecStart=/path/to/simon/simon -c /path/to/simon/config.yml -i 15
ExecStart=%s -c '%s' -i 15
ExecReload=/bin/kill -HUP "$MAINPID"
Restart=on-failure
RestartSec=1
# Do not restart on the following known exit codes (won’t be successful):
RestartPreventExitStatus=10 11 12 13
# 1 (fatal), 2 (panic) — unknown codes, service should be restarted

# Hardening
SystemCallArchitectures=native
MemoryDenyWriteExecute=true
NoNewPrivileges=true

[Install]
WantedBy=default.target`

const METRICS_TEMPLATE string = `####
# %s metrics
# #1
#
# @time %s
# @link %s
# @link %s
####
# metrics start
#### Uptime ####
# System uptime, seconds
uptime,host=hostname                                   value=[float]

#### CPU metrics ####
# System load average for the last minute
load_avg,host=hostname                                 value=[float]
# 100%% * load_average / cpu_count
load_usage,host=hostname                               value=[float]
# CPU count
cpu_count,host=hostname                                value=[uint]
# CPU usage, %%
cpu_usage,host=hostname                                value=[float]
# Number of created processes and threads
cpu_processes,host=unknown                             value=[uint]
# Number of processes currently running on CPUs
cpu_processes_running,host=unknown                     value=[uint]
# Number of blocked processes, waiting for I/O to complete
cpu_processes_blocked,host=unknown                     value=[uint]

#### Memory metrics ####
# RAM memory, bytes
memory_total,host=hostname                             value=[uint]
memory_free,host=hostname                              value=[uint]
memory_available,host=hostname                         value=[uint]
memory_buffers,host=hostname                           value=[uint]
memory_cached,host=hostname                            value=[uint]
# Used memory, bytes; total − available
memory_used,host=hostname                              value=[uint]
# Memory usage; 100%% * memory_used / memory_total
memory_usage,host=hostname                             value=[float]
# Used memory (like htop); total − free − buffers − cached − swap_cached
memory_used_classic,host=hostname                      value=[uint]
# Memory usage (like htop); 100%% * memory_used_classic / memory_total
memory_usage_classic,host=hostname                     value=[float]

# Swap memory, bytes
memory_swap_total,host=hostname                        value=[uint]
memory_swap_free,host=hostname                         value=[uint]
memory_swap_cached,host=hostname                       value=[uint]
# Used swap memory, bytes; swap_total − swap_free
memory_swap_used,host=hostname                         value=[uint]
# Swap usage; 100%% * swap_used / swap_total
memory_swap_usage,host=hostname                        value=[float]

#### Disk metrics, for each monitored mount point ####
# Disk total, bytes
disk_total,host=hostname,disk=/dev/vda1,mount=/        value=[uint]
# Disk used, bytes
disk_used,host=hostname,disk=/dev/vda1,mount=/         value=[uint]
# Disk available, bytes
disk_available,host=hostname,disk=/dev/vda1,mount=/    value=[uint]
# Disk usage; 100%% * disk_used / (disk_used + disk_available)
disk_usage,host=hostname,disk=/dev/vda1,mount=/        value=[float]

#### IO metrics, for each monitored device ####
# Number of I/O operations currently in progress
io_current_ops,host=hostname,device=vda1               value=[uint]
# Write operations completed per second
io_write_ops_speed,host=hostname,device=vda1           value=[float]
# Write speed, bytes per second
io_write_speed,host=hostname,device=vda1               value=[float]
# Write usage / utilization / saturation, %%
io_write_usage,host=hostname,device=vda1               value=[float]
# Read operations completed per second
io_read_ops_speed,host=hostname,device=vda1            value=[float]
# Read speed, bytes per second
io_read_speed,host=hostname,device=vda1                value=[float]
# Read usage / utilization / saturation, %%
io_read_usage,host=hostname,device=vda1                value=[float]
# IO usage / utilization / saturation, %%
io_usage,host=hostname,device=vda1                     value=[float]

# Most used I/O device (with maximum I/O usage)
io_usage_max,host=hostname,device=vda                  value=[float],device_name="vda"

#### Network metrics, for each monitored interface ####
# Received traffic, bytes
network_in,host=hostname,interface=eth0                value=[uint]
# Transmitted traffic, bytes
network_out,host=hostname,interface=eth0               value=[uint]
# Receiving load / speed, bits per second
network_load_in,host=hostname,interface=eth0           value=[uint]
# Transmitting load / speed, bits per second
network_load_out,host=hostname,interface=eth0          value=[uint]

# Most loaded device (with maximum receiving load)
network_load_in_max,host=hostname,interface=eth0       value=[uint],interface_name="eth0"
# Most loaded device (with maximum transmitting load)
network_load_out_max,host=hostname,interface=eth0      value=[uint],interface_name="eth0"

#### Monitor’s own metrics ####
# Monitor runtime, seconds
monitor_runtime,host=hostname                          value=[float]
# Memory allocated in heap, bytes
monitor_memory_alloc,host=hostname                     value=[uint]
# Total memory obtained from the OS, bytes
monitor_memory_system,host=hostname                    value=[uint]
# Number of completed garbage collection cycles
monitor_memory_gc_count,host=hostname                  value=[uint]
# Metrics send time, seconds
monitor_metrics_sent_time,host=hostname                value=[float]
## metrics end`

var METRICS_DESCRIPTIONS map[string]string = map[string]string{
	// #### Uptime ####
	"uptime": "System uptime, seconds",

	// #### CPU metrics ####
	"load_avg":              "System load average for the last minute",
	"load_usage":            "100% * load_average / cpu_count",
	"cpu_count":             "CPU count",
	"cpu_usage":             "CPU usage",
	"cpu_processes":         "Number of created processes and threads",
	"cpu_processes_running": "Number of processes currently running on CPUs",
	"cpu_processes_blocked": "Number of blocked processes, waiting for I/O to complete",

	// #### Memory metrics ####
	"memory_total":         "RAM memory, bytes",
	"memory_used":          "Used memory, bytes; total − available",
	"memory_usage":         "Memory usage; 100% * memory_used / memory_total",
	"memory_used_classic":  "Used memory (like htop); total − free − buffers − cached − swap_cached",
	"memory_usage_classic": "Memory usage (like htop); 100% * memory_used_classic / memory_total",
	"memory_swap_total":    "Swap memory, bytes",
	"memory_swap_used":     "Used swap memory, bytes; swap_total − swap_free",
	"memory_swap_usage":    "Swap usage; 100% * swap_used / swap_total",

	// #### Disk metrics, for each monitored mount point ####
	"disk_total":     "Disk total, bytes",
	"disk_used":      "Disk used, bytes",
	"disk_available": "Disk available, bytes",
	"disk_usage":     "Disk usage; 100% * disk_used / (disk_used + disk_available)",

	// #### IO metrics, for each monitored device ####
	"io_current_ops":     "Number of I/O operations currently in progress",
	"io_write_ops_speed": "Write operations completed per second",
	"io_write_speed":     "Write speed, bytes per second",
	"io_write_usage":     "Write usage / utilization / saturation, %",
	"io_read_ops_speed":  "Read operations completed per second",
	"io_read_speed":      "Read speed, bytes per second",
	"io_read_usage":      "Read usage / utilization / saturation, %",
	"io_usage":           "IO usage / utilization / saturation, %",

	"io_usage_max": "Most used I/O device (with maximum I/O usage)",

	// #### Network metrics, for each monitored interface ####
	"network_in":           "Received traffic, bytes",
	"network_out":          "Transmitted traffic, bytes",
	"network_load_in":      "Receiving load / speed, bits per second",
	"network_load_out":     "Transmitting load / speed, bits per second",
	"network_load_in_max":  "Most loaded device (with maximum receiving load)",
	"network_load_out_max": "Most loaded device (with maximum transmitting load)",

	// #### Monitor’s own metrics ####
	"monitor_runtime":            "Monitor runtime, seconds",
	"monitor_metrics_sent_time":  "Metrics send time, seconds",
	"monitor_memory_alloc":       "Memory allocated in heap, bytes",
	"monitor_memory_alloc_total": "Total memory allocated in heap, bytes",
	"monitor_memory_system":      "Total memory obtained from the OS, bytes",
	"monitor_memory_gc_count":    "Number of completed garbage collection cycles",
}
