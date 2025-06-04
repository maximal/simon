# SiMon — Linux System Monitor
SiMon is a simple Linux system monitor. It gathers various performance metrics from `/proc` filesystem and prints or sends them to [InfluxDB](https://www.influxdata.com/).

Used `/proc` data:
* `/proc/uptime` — system’s uptime information;
* `/proc/loadavg` — load average;
* `/proc/stat` — processor metrics and usage;
* `/proc/meminfo` — memory metrics and usage;
* `/proc/diskstats` — I/O utilization/saturation statistics;
* `/proc/net/dev` — network interfaces load.

SiMon works in two different modes:
* Send metrics to the external InfluxDB;
* Display metrics on the web page to be gathered by InfluxDB scraper.

Then, collected metrics can be displayed, for example, on a Grafana dashboard.

Tested on Debian and Ubuntu installations with `amd64` and `arm64` architectures.
Help and testing on other distributions and architectures is highly appreciated.

Ported from the original PHP version: https://github.com/maximal/simon-php


## Grafana Dashboard Example
![Grafana Dashboard](grafana-dashboard.png)


## Usage
Get the sources:
```shell
git clone https://github.com/maximal/simon
cd simon
```

Build the executable for production usage:
```shell
go build -ldflags "-s -w" -trimpath .
```

Build for specific platform (if you need to run SiMon on a different machine):
```shell
GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -trimpath .
```

Make test run:
```shell
./simon -c simon.example.yml -i 1
# Long flags version:
# ./simon --config=simon.example.yml --interval=1
```

You should see the program’s logs with the basic information:
```plain
#1    System:  uptime: 1:28:51.7  CPU: N/A%  memory: 11.8%    Monitor:  runtime: 0:00.0  memory: 344 KiB
#2    System:  uptime: 1:28:52.7  CPU: 0.8%  memory: 11.8%    Monitor:  runtime: 0:01.0  memory: 792 KiB
#3    System:  uptime: 1:28:53.7  CPU: 1.0%  memory: 11.8%    Monitor:  runtime: 0:02.0  memory: 958 KiB
#4    System:  uptime: 1:28:54.7  CPU: 0.3%  memory: 11.8%    Monitor:  runtime: 0:03.0  memory: 1.1 MiB
#5    System:  uptime: 1:28:55.7  CPU: 0.8%  memory: 11.8%    Monitor:  runtime: 0:04.0  memory: 456 KiB
... ... ...
```

You’re ready for the service installation.


## Setting Up a Linux Service
In order for SiMon to work in the background and start automatically with the system, you need to set up a Linux service (daemon).

For your convenience, there are `simon install -u <username>` and `simon install` command which print out shell commands needed for the service installation.

The service could be run under the specific user without administrative rights or under `root` user. You’ll need `sudo` rights during the installation in both cases to make the service start with the system.

### As a User’s Service
`sudo` needed on step 6 only.
1. Log in as user.

2. Copy SiMon to the user’s home directory:
   ```shell
   mkdir -p ~/.local/bin/
   cp  /path/to/simon  ~/.local/bin/simon
   ```

3. Create a config file in the user’s home directory:
   ```shell
   mkdir -p ~/.config/simon/
   ~/.local/bin/simon config  >  ~/.config/simon/config.yml
   ```

4. Edit the config file, set InfluxDB connection and other settings. See `Config File Reference` section.

5. Set up a user’s service:
   ```shell
   mkdir -p ~/.local/share/systemd/user
   ~/.local/bin/simon service  >  ~/.local/share/systemd/user/simon.service
   systemctl --user enable simon
   systemctl --user start simon
   ```

6. Extend service life for the user (`sudo` rights needed):
   ```shell
   sudo loginctl enable-linger <username>
   ```

7. Check the service’s logs:
   ```shell
   journalctl --user -fu simon
   ```


### As a System Service (For `root` User)
`sudo` needed on all the steps.

1. Copy SiMon to the binaries directory:
   ```shell
   sudo cp  /path/to/simon  /usr/local/bin/simon
   ```

2. Create a config file:
   ```shell
   sudo mkdir -p /etc/simon/
   /usr/local/bin/simon config  |  sudo tee /etc/simon/config.yml > /dev/null
   ```

3. Edit the config file, set InfluxDB connection and other settings. See `Config File Reference` section.

4. Set up a system service:
   ```shell
   /usr/local/bin/simon service  |  sudo tee /lib/systemd/system/simon.service > /dev/null
   sudo systemctl enable simon
   sudo systemctl start simon
   ```

5. Check the service’s logs:
   ```shell
   sudo journalctl -fu simon
   ```


## Config File Reference
The configuration file has the following structure:
```yaml
# User’s `~/.config/simon/config.yml` or system `/etc/simon/config.yml` file

# This machine name, for measurements tag `host`
# It will be added to every measurement
# If `null`, will be taken from `/etc/hostname` file
hostname: servername

# Working mode: `push` (send metrics, default), `server` (print metrics on HTTP requests)
mode: push

# Web server settings (for `server` mode only)
server:
  # Port to listen to
  port: 8000
  # Host to listen to; default, empty string, means all addresses on all interfaces
  #host: ""
  # `Aurhorization` HTTP header to check before showing metrics;
  # Any random string; if empty, metrics will be shown without authorization
  #authorization: ""

# InfluxDB settings
influx:
  # Whether to send data to Influx; disabled by default, console output only
  enabled: true
  # Influx host name, required if Influx enabled
  # HTTP installation example, with port: influx.host.org:8086
  # HTTPS installation example: https://influx.host.org
  host: influx.host.org:8086
  # Influx API token, required if Influx enabled
  token: Influx token with the bucket write permissions
  # Influx org name, required if Influx enabled
  org: Influx organization name
  # Influx bucket name, required if Influx enabled
  bucket: simon_metrics
  # Timestamps precision: s (default), ms, us, ns
  # It is recommended not to use `ms`, `us`, or `ns`
  # if the monitoring interval is greater than `1`
  precision: s
  # Additional tags to be added to every measurement; key:value pairs
  tags:
    custom_tag: custom_tag_value
    #custom_tag_2: custom_tag_value_2

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
    - /
    #- /home
    #- /tmp

io:
  # Whether to track I/O usage; enabled by default
  enabled: true
  # I/O devices to track; default, empty array, means all
  devices:
    - vda
    - vda1
    #- vdb

network:
  # Whether to track network interfaces usage; enabled by default
  enabled: true
  # Network interfaces to track; default, empty array, means all
  interfaces:
    - lo
    - eth0
    #- tunl0

# Whether to track monitor’s own metrics; enabled by default
monitor: true

# Whether to print metrics to stdout; disabled by default, set true to enable
#print: true
```


## Metrics Example
You could print Metrics example using `simon metrics` command:
```plain
##
# SiMon metrics
#
# @link https://github.com/maximal/simon
# @link https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol/
##
# metrics start
#### Uptime ####
# System uptime, seconds
uptime,host=hostname                                   value=[float]

#### CPU metrics ####
# System load average for the last minute
load_avg,host=hostname                                 value=[float]
# 100% * load_average / cpu_count
load_usage,host=hostname                               value=[float]
# CPU count
cpu_count,host=hostname                                value=[uint]
# CPU usage, %
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
# Memory usage; 100% * memory_used / memory_total
memory_usage,host=hostname                             value=[float]
# Used memory (like htop); total − free − buffers − cached − swap_cached
memory_used_classic,host=hostname                      value=[uint]
# Memory usage (like htop); 100% * memory_used_classic / memory_total
memory_usage_classic,host=hostname                     value=[float]

# Swap memory, bytes
memory_swap_total,host=hostname                        value=[uint]
memory_swap_free,host=hostname                         value=[uint]
memory_swap_cached,host=hostname                       value=[uint]
# Used swap memory, bytes; swap_total − swap_free
memory_swap_used,host=hostname                         value=[uint]
# Swap usage; 100% * swap_used / swap_total
memory_swap_usage,host=hostname                        value=[float]

#### Disk metrics, for each monitored mount point ####
# Disk total, bytes
disk_total,host=hostname,disk=/dev/vda1,mount=/        value=[uint]
# Disk used, bytes
disk_used,host=hostname,disk=/dev/vda1,mount=/         value=[uint]
# Disk available, bytes
disk_available,host=hostname,disk=/dev/vda1,mount=/    value=[uint]
# Disk usage; 100% * disk_used / disk_total
disk_usage,host=hostname,disk=/dev/vda1,mount=/        value=[float]

#### IO metrics, for each monitored device ####
# Number of I/O operations currently in progress
io_current_ops,host=hostname,device=vda1               value=[uint]
# Write operations completed per second
io_write_ops_speed,host=hostname,device=vda1           value=[float]
# Write speed, bytes per second
io_write_speed,host=hostname,device=vda1               value=[float]
# Write usage / utilization / saturation, %
io_write_usage,host=hostname,device=vda1               value=[float]
# Read operations completed per second
io_read_ops_speed,host=hostname,device=vda1            value=[float]
# Read speed, bytes per second
io_read_speed,host=hostname,device=vda1                value=[float]
# Read usage / utilization / saturation, %
io_read_usage,host=hostname,device=vda1                value=[float]
# IO usage / utilization / saturation, %
io_usage,host=hostname,device=vda1                     value=[float]

# Most used I/O device (with maximum I/O usage)
io_usage_max,host=hostname,device=vda                  value=[float],device="vda"

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
network_load_in_max,host=hostname,interface=eth0       value=[uint],interface="eth0"
# Most loaded device (with maximum transmitting load)
network_load_out_max,host=hostname,interface=eth0      value=[uint],interface="eth0"

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
## metrics end
```


## Author
* https://github.com/maximal
* https://maximals.ru/
* https://sijeko.ru/
