package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// SystemInfo 机器基本信息
type SystemInfo struct {
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	Distro      string `json:"distro"`
	Kernel      string `json:"kernel"`
	Arch        string `json:"arch"`
	Uptime      string `json:"uptime"`
	CPUModel    string `json:"cpu_model"`
	CPUCores    int    `json:"cpu_cores"`
	MemoryTotal uint64 `json:"memory_total"`
	Disk        []Disk `json:"disks"`
	Net         []Net  `json:"networks"`
	LoadAvg     string `json:"load_avg"`
	Time        string `json:"time"`
}

type Disk struct {
	MountPoint string `json:"mount"`
	Total      uint64 `json:"total"`
	Used       uint64 `json:"used"`
	Free       uint64 `json:"free"`
	Percent    string `json:"percent"`
}

type Net struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// Stats CPU/内存实时占用
type Stats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemTotal      uint64  `json:"mem_total"`
	MemUsed       uint64  `json:"mem_used"`
	MemAvailable  uint64  `json:"mem_available"`
	MemPercent    float64 `json:"mem_percent"`
	SwapTotal     uint64  `json:"swap_total"`
	SwapUsed      uint64  `json:"swap_used"`
	ProcessCount  int     `json:"process_count"`
	SampledAt     int64   `json:"sampled_at"`
	SelfMemKB     uint64  `json:"self_mem_kb"`
}

var lastCPUTotal, lastCPUIdle uint64

func handleSystem(w http.ResponseWriter, r *http.Request) {
	info := collectSystemInfo()
	writeJSON(w, info)
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	stats := collectStats()
	writeJSON(w, stats)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func collectSystemInfo() SystemInfo {
	info := SystemInfo{
		Arch: runtime.GOARCH,
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}
	info.Hostname, _ = os.Hostname()
	info.OS = runtime.GOOS
	info.Distro = readDistro()
	info.Kernel = readFile("/proc/sys/kernel/osrelease")
	info.Uptime = readUptime()
	info.CPUModel, info.CPUCores = readCPUInfo()
	info.MemoryTotal = readMemTotal()
	info.LoadAvg = readFile("/proc/loadavg")
	info.Disk = readDisks()
	info.Net = readNetworks()
	return info
}

func readDistro() string {
	if v := readFile("/etc/os-release"); v != "" {
		for _, line := range strings.Split(v, "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
			}
		}
	}
	return "Unknown Linux"
}

func readUptime() string {
	data := readFile("/proc/uptime")
	fields := strings.Fields(data)
	if len(fields) == 0 {
		return "0s"
	}
	sec, _ := strconv.ParseFloat(fields[0], 64)
	return formatDuration(time.Duration(sec) * time.Second)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return strconv.Itoa(days) + "天" + strconv.Itoa(hours) + "小时" + strconv.Itoa(mins) + "分"
	}
	if hours > 0 {
		return strconv.Itoa(hours) + "小时" + strconv.Itoa(mins) + "分"
	}
	return strconv.Itoa(mins) + "分"
}

func readCPUInfo() (string, int) {
	data := readFile("/proc/cpuinfo")
	model := ""
	cores := 0
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "model name") && model == "" {
			model = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
		if strings.HasPrefix(line, "processor") {
			cores++
		}
	}
	if model == "" {
		// ARM 等架构可能没有 model name
		for _, line := range strings.Split(data, "\n") {
			if strings.HasPrefix(line, "Hardware") {
				model = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				break
			}
		}
	}
	if cores == 0 {
		cores = runtime.NumCPU()
	}
	return model, cores
}

func readMemTotal() uint64 {
	data := readFile("/proc/meminfo")
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseUint(fields[1], 10, 64)
				return v * 1024 // KB -> Bytes
			}
		}
	}
	return 0
}

func readDisks() []Disk {
	var disks []Disk
	out, err := exec.Command("df", "-B1", "-P").Output()
	if err != nil {
		return disks
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		// 只保留真实文件系统挂载点
		if !strings.HasPrefix(fields[0], "/dev/") {
			continue
		}
		total, _ := strconv.ParseUint(fields[1], 10, 64)
		used, _ := strconv.ParseUint(fields[2], 10, 64)
		free, _ := strconv.ParseUint(fields[3], 10, 64)
		disks = append(disks, Disk{
			MountPoint: fields[5],
			Total:      total,
			Used:       used,
			Free:       free,
			Percent:    fields[4],
		})
	}
	return disks
}

func readNetworks() []Net {
	var nets []Net
	out, err := exec.Command("ip", "-o", "addr", "show").Output()
	if err != nil {
		// 回退到 hostname -I
		out2, _ := exec.Command("hostname", "-I").Output()
		ips := strings.Fields(string(out2))
		if len(ips) > 0 {
			nets = append(nets, Net{Name: "default", IP: ips[0]})
		}
		return nets
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if fields[2] == "inet" {
			nets = append(nets, Net{Name: fields[1], IP: strings.Split(fields[3], "/")[0]})
		}
	}
	return nets
}

func collectStats() Stats {
	stats := Stats{
		SampledAt: time.Now().Unix(),
	}
	stats.CPUPercent = readCPUPercent()
	stats.MemTotal, stats.MemUsed, stats.MemAvailable, stats.MemPercent, stats.SwapTotal, stats.SwapUsed = readMemStats()
	stats.ProcessCount = countProcesses()
	stats.SelfMemKB = selfMemKB()
	return stats
}

// readCPUPercent 采样两次 /proc/stat 计算 CPU 占用率
func readCPUPercent() float64 {
	readCPUSample := func() (total, idle uint64) {
		data := readFile("/proc/stat")
		for _, line := range strings.Split(data, "\n") {
			if strings.HasPrefix(line, "cpu ") {
				fields := strings.Fields(line)
				for i := 1; i < len(fields); i++ {
					v, _ := strconv.ParseUint(fields[i], 10, 64)
					total += v
					if i == 4 { // idle
						idle = v
					}
				}
				return
			}
		}
		return
	}

	t1, i1 := readCPUSample()
	time.Sleep(200 * time.Millisecond)
	t2, i2 := readCPUSample()

	if t2 <= t1 {
		return 0
	}
	totalDelta := t2 - t1
	idleDelta := i2 - i1
	return (1 - float64(idleDelta)/float64(totalDelta)) * 100
}

func readMemStats() (total, used, available uint64, percent float64, swapTotal, swapUsed uint64) {
	data := readFile("/proc/meminfo")
	vals := map[string]uint64{}
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			key := strings.TrimSuffix(fields[0], ":")
			v, _ := strconv.ParseUint(fields[1], 10, 64)
			vals[key] = v * 1024
		}
	}
	total = vals["MemTotal"]
	available = vals["MemAvailable"]
	if available == 0 {
		// 老内核无 MemAvailable，用 MemFree + Buffers + Cached 近似
		available = vals["MemFree"] + vals["Buffers"] + vals["Cached"]
	}
	used = total - available
	if total > 0 {
		percent = float64(used) / float64(total) * 100
	}
	swapTotal = vals["SwapTotal"]
	swapUsed = swapTotal - vals["SwapFree"]
	return
}

func countProcesses() int {
	out, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range out {
		if _, err := strconv.Atoi(e.Name()); err == nil {
			count++
		}
	}
	return count
}

// selfMemKB 读取自身内存占用
func selfMemKB() uint64 {
	data := readFile("/proc/self/status")
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseUint(fields[1], 10, 64)
				return v
			}
		}
	}
	return 0
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// readLines 读取文件所有行
func readLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}
