// Package vm 虚拟机资源统计
package vm

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// cpuHistory 用于计算 CPU 使用率的历史数据
type cpuHistory struct {
	lastTotal    int64
	lastCPUTotal int64
	lastTime     time.Time
}

var (
	cpuHistoryMap  = make(map[int]*cpuHistory)
	cpuHistoryLock sync.Mutex
)

// GetVMStats 获取虚拟机资源统计
func (s *Service) GetVMStats(id string) (*VMStatsDetail, error) {
	s.mu.RLock()
	vm, ok := s.vms[id]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("vm not found: %s", id)
	}

	if vm.Status != VMStatusRunning || vm.PID <= 0 {
		return nil, fmt.Errorf("vm not running")
	}

	pid := vm.PID
	stats := &VMStatsDetail{
		VMID:        id,
		MemoryTotal: vm.Memory * 1024 * 1024, // MB to bytes
		Timestamp:   time.Now(),
	}

	// CPU 使用率
	if cpuPercent, err := getProcessCPUPercent(pid); err == nil {
		stats.CPUPercent = cpuPercent
	}

	// 内存使用
	if memUsed, err := getProcessMemory(pid); err == nil {
		stats.MemoryUsed = memUsed
	}

	// 磁盘 IO
	if diskRead, diskWrite, err := getProcessDiskIO(pid); err == nil {
		stats.DiskRead = diskRead
		stats.DiskWrite = diskWrite
	}

	// 运行时间
	if startTime, err := getProcessStartTime(pid); err == nil {
		stats.Uptime = int64(time.Since(startTime).Seconds())
	}

	return stats, nil
}

// GetAllVMStats 获取所有运行中 VM 的统计
func (s *Service) GetAllVMStats() []*VMStatsDetail {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stats []*VMStatsDetail
	for id, vm := range s.vms {
		if vm.Status == VMStatusRunning && vm.PID > 0 {
			s.mu.RUnlock()
			if vmStats, err := s.GetVMStats(id); err == nil {
				stats = append(stats, vmStats)
			}
			s.mu.RLock()
		}
	}
	return stats
}

// getProcessCPUPercent 获取进程 CPU 使用率
func getProcessCPUPercent(pid int) (float64, error) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 17 {
		return 0, fmt.Errorf("invalid stat format")
	}

	utime, _ := strconv.ParseInt(fields[13], 10, 64)
	stime, _ := strconv.ParseInt(fields[14], 10, 64)
	totalTime := utime + stime

	cpuTotal, _, err := getSystemCPUTime()
	if err != nil {
		return 0, err
	}

	cpuHistoryLock.Lock()
	defer cpuHistoryLock.Unlock()

	history, exists := cpuHistoryMap[pid]
	if !exists || time.Since(history.lastTime) > 5*time.Second {
		cpuHistoryMap[pid] = &cpuHistory{
			lastTotal:    totalTime,
			lastCPUTotal: cpuTotal,
			lastTime:     time.Now(),
		}
		return 0, nil
	}

	totalDelta := float64(totalTime - history.lastTotal)
	cpuDelta := float64(cpuTotal - history.lastCPUTotal)
	if cpuDelta == 0 {
		return 0, nil
	}

	percent := (totalDelta / cpuDelta) * 100

	cpuHistoryMap[pid] = &cpuHistory{
		lastTotal:    totalTime,
		lastCPUTotal: cpuTotal,
		lastTime:     time.Now(),
	}

	return percent, nil
}

// getSystemCPUTime 获取系统 CPU 时间
func getSystemCPUTime() (total int64, idle int64, err error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				return 0, 0, fmt.Errorf("invalid cpu stat format")
			}
			for i := 1; i < len(fields); i++ {
				val, _ := strconv.ParseInt(fields[i], 10, 64)
				total += val
				if i == 4 {
					idle = val
				}
			}
			return total, idle, nil
		}
	}
	return 0, 0, fmt.Errorf("cpu stat not found")
}

// getProcessMemory 获取进程内存使用（bytes）
func getProcessMemory(pid int) (int64, error) {
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	file, err := os.Open(statusPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				return val * 1024, nil // KB to bytes
			}
		}
	}
	return 0, fmt.Errorf("VmRSS not found")
}

// getProcessDiskIO 获取进程磁盘 IO
func getProcessDiskIO(pid int) (read int64, write int64, err error) {
	ioPath := fmt.Sprintf("/proc/%d/io", pid)
	file, err := os.Open(ioPath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "read_bytes:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				read, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		} else if strings.HasPrefix(line, "write_bytes:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				write, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		}
	}
	return read, write, nil
}

// getProcessStartTime 获取进程启动时间
func getProcessStartTime(pid int) (time.Time, error) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return time.Time{}, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 22 {
		return time.Time{}, fmt.Errorf("invalid stat format")
	}

	// 系统运行时间
	uptimeData, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return time.Time{}, err
	}
	uptimeFields := strings.Fields(string(uptimeData))
	uptime, _ := strconv.ParseFloat(uptimeFields[0], 64)

	// 进程启动时间（自系统启动以来的 clock ticks）
	starttime, _ := strconv.ParseInt(fields[21], 10, 64)
	clkTck := int64(100) // sysconf(_SC_CLK_TCK) 默认值

	now := time.Now()
	bootTime := now.Add(-time.Duration(uptime * float64(time.Second)))
	processStart := bootTime.Add(time.Duration(starttime/clkTck) * time.Second)

	return processStart, nil
}
