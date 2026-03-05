// Package vm 虚拟机服务
package vm

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service 虚拟机服务
type Service struct {
	logger       *zap.Logger
	dataDir      string
	vms          map[string]*VM
	processes    map[string]*exec.Cmd
	mu           sync.RWMutex
	vncBasePort  int
	sshBasePort  int
	vncProxy     *VNCProxy
	vncTokenMgr  *VNCTokenManager
	qmpManager   *QMPManager
}

// NewService 创建服务实例
func NewService(logger *zap.Logger, dataDir string) *Service {
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(filepath.Join(dataDir, "disks"), 0755)
	os.MkdirAll(filepath.Join(dataDir, "iso"), 0755)
	os.MkdirAll(filepath.Join(dataDir, "snapshots"), 0755)

	s := &Service{
		logger:      logger,
		dataDir:     dataDir,
		vms:         make(map[string]*VM),
		processes:   make(map[string]*exec.Cmd),
		vncBasePort: 5900,
		sshBasePort: 2222,
		vncProxy:    NewVNCProxy(logger),
		vncTokenMgr: NewVNCTokenManager(),
		qmpManager:  NewQMPManager(),
	}

	s.loadVMs()
	return s
}

// Start 启动服务
func (s *Service) Start() error {
	return nil
}

// Stop 停止服务
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, cmd := range s.processes {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
		if vm, ok := s.vms[id]; ok {
			vm.Status = VMStatusStopped
			vm.PID = 0
		}
	}
	s.processes = make(map[string]*exec.Cmd)
	s.saveVMs()
}

// GetVMs 获取虚拟机列表
func (s *Service) GetVMs() []*VM {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vms := make([]*VM, 0, len(s.vms))
	for _, vm := range s.vms {
		s.updateVMStatus(vm)
		vms = append(vms, vm)
	}
	return vms
}

// GetVM 获取虚拟机
func (s *Service) GetVM(id string) (*VM, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vm, ok := s.vms[id]
	if !ok {
		return nil, fmt.Errorf("vm not found: %s", id)
	}
	s.updateVMStatus(vm)
	return vm, nil
}

// CreateVM 创建虚拟机
func (s *Service) CreateVM(req CreateVMRequest) (*VM, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()[:8]

	// 检查是否使用模板
	var template *VMTemplate
	if req.Template != "" {
		var err error
		template, err = s.GetTemplate(req.Template)
		if err != nil {
			return nil, fmt.Errorf("get template: %w", err)
		}
	}

	// 默认配置（模板优先）
	cpu := req.CPU
	if cpu <= 0 {
		if template != nil {
			cpu = template.CPU
		}
		if cpu <= 0 {
			cpu = 2
		}
	}
	memory := req.Memory
	if memory <= 0 {
		if template != nil {
			memory = int64(template.Memory)
		}
		if memory <= 0 {
			memory = 2048
		}
	}
	diskSize := req.DiskSize
	if diskSize <= 0 {
		if template != nil {
			diskSize = int64(template.DiskSize)
		}
		if diskSize <= 0 {
			diskSize = 20
		}
	}

	arch := req.Arch
	if arch == "" {
		arch = "x86_64"
	}

	osType := req.OSType
	if osType == "" {
		if template != nil {
			osType = template.OSType
		}
		if osType == "" {
			osType = "linux"
		}
	}

	os := ""
	if template != nil {
		os = template.OS
	}

	diskPath := filepath.Join(s.dataDir, "disks", id+".qcow2")

	vm := &VM{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Status:      VMStatusCreating,
		CPU:         cpu,
		Memory:      memory,
		DiskSize:    diskSize,
		DiskPath:    diskPath,
		ISOPath:     req.ISOPath,
		OS:          os,
		OSType:      osType,
		Arch:        arch,
		Accelerator: s.detectAccelerator(),
		Network:     "user",
		Display:     "vnc",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置端口转发，默认添加 SSH
	if len(req.PortForwards) > 0 {
		vm.PortForwards = req.PortForwards
	} else {
		// 默认 SSH 端口转发
		vm.PortForwards = []PortForward{
			{Name: "SSH", Protocol: "tcp", HostPort: 0, GuestPort: 22},
		}
	}

	// 创建磁盘：如果模板有 BaseDisk，使用增量磁盘
	if template != nil && template.BaseDisk != "" {
		// 使用增量磁盘（基于模板）
		if err := s.createIncrementalDisk(diskPath, template.BaseDisk, int(diskSize)); err != nil {
			return nil, fmt.Errorf("create incremental disk: %w", err)
		}
		s.logger.Info("created incremental disk from template",
			zap.String("vmID", id),
			zap.String("template", template.ID),
			zap.String("baseDisk", template.BaseDisk))
	} else {
		// 创建完整磁盘
		if err := s.createDisk(diskPath, diskSize); err != nil {
			return nil, fmt.Errorf("create disk: %w", err)
		}
	}

	vm.Status = VMStatusStopped
	s.vms[id] = vm
	s.saveVMs()

	return vm, nil
}

// UpdateVM 更新虚拟机
func (s *Service) UpdateVM(id string, req UpdateVMRequest) (*VM, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	vm, ok := s.vms[id]
	if !ok {
		return nil, fmt.Errorf("vm not found: %s", id)
	}

	if vm.Status == VMStatusRunning {
		return nil, fmt.Errorf("cannot update running vm")
	}

	if req.Name != "" {
		vm.Name = req.Name
	}
	if req.Description != "" {
		vm.Description = req.Description
	}
	if req.CPU > 0 {
		vm.CPU = req.CPU
	}
	if req.Memory > 0 {
		vm.Memory = req.Memory
	}
	if req.PortForwards != nil {
		vm.PortForwards = req.PortForwards
	}
	if req.USBDevices != nil {
		vm.USBDevices = req.USBDevices
	}
	if req.NetworkMode != "" {
		vm.NetworkMode = req.NetworkMode
	}
	if req.BridgeIface != "" {
		vm.BridgeIface = req.BridgeIface
	}
	if req.CPUModel != "" {
		vm.CPUModel = req.CPUModel
	}
	if req.EnableHuge != nil {
		vm.EnableHuge = *req.EnableHuge
	}
	if req.IOThread != nil {
		vm.IOThread = *req.IOThread
	}
	if req.CPUPinning != nil {
		vm.CPUPinning = req.CPUPinning
	}

	vm.UpdatedAt = time.Now()
	s.saveVMs()

	return vm, nil
}

// DeleteVM 删除虚拟机
func (s *Service) DeleteVM(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	vm, ok := s.vms[id]
	if !ok {
		return fmt.Errorf("vm not found: %s", id)
	}

	if vm.Status == VMStatusRunning {
		return fmt.Errorf("cannot delete running vm")
	}

	// 删除磁盘文件
	if vm.DiskPath != "" {
		os.Remove(vm.DiskPath)
	}

	delete(s.vms, id)
	s.saveVMs()

	return nil
}

// StartVM 启动虚拟机
func (s *Service) StartVM(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	vm, ok := s.vms[id]
	if !ok {
		return fmt.Errorf("vm not found: %s", id)
	}

	if vm.Status == VMStatusRunning {
		return nil
	}

	// 检查是否已有 QEMU 进程在运行（防止重复启动）
	if pid, vncPort := s.findRunningQEMU(id); pid > 0 {
		vm.Status = VMStatusRunning
		vm.PID = pid
		vm.VNCPort = vncPort
		vm.QMPSocket = filepath.Join(s.dataDir, "qmp", id+".sock")
		s.saveVMs()
		s.logger.Info("vm already running, recovered state", zap.String("id", id), zap.Int("pid", pid))
		return nil
	}

	// 分配端口
	vm.VNCPort = s.allocateVNCPort()

	// 为端口转发分配端口 (HostPort=0 表示自动分配)
	basePort := s.sshBasePort
	for i := range vm.PortForwards {
		if vm.PortForwards[i].HostPort == 0 {
			vm.PortForwards[i].HostPort = s.allocatePort(basePort)
			basePort = vm.PortForwards[i].HostPort + 1
		}
	}

	// 更新 SSHPort (向后兼容)
	for _, pf := range vm.PortForwards {
		if pf.GuestPort == 22 {
			vm.SSHPort = pf.HostPort
			break
		}
	}

	// 设置 QMP socket 路径
	vm.QMPSocket = filepath.Join(s.dataDir, "qmp", id+".sock")
	os.MkdirAll(filepath.Dir(vm.QMPSocket), 0755)
	os.Remove(vm.QMPSocket) // 确保旧 socket 被清理

	args := s.buildQEMUArgs(vm)
	cmd := exec.Command("qemu-system-"+vm.Arch, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start qemu: %w", err)
	}

	vm.PID = cmd.Process.Pid
	vm.Status = VMStatusRunning
	vm.UpdatedAt = time.Now()

	s.processes[id] = cmd
	s.saveVMs()

	// 等待 QEMU 启动并连接 QMP (异步)
	go func() {
		// 等待 socket 文件出现
		for i := 0; i < 50; i++ {
			if _, err := os.Stat(vm.QMPSocket); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// 尝试连接 QMP
		if _, err := s.qmpManager.GetOrCreate(id, vm.QMPSocket); err != nil {
			s.logger.Warn("failed to connect QMP", zap.String("vm", id), zap.Error(err))
		}
	}()

	// 等待进程结束
	go func() {
		cmd.Wait()
		// -daemonize 模式下原进程会立即退出，需要检查实际 QEMU 守护进程是否还在运行
		time.Sleep(500 * time.Millisecond)
		if pid, _ := s.findRunningQEMU(id); pid > 0 {
			// QEMU 守护进程仍在运行，不更新状态
			s.logger.Debug("qemu daemon still running", zap.String("vm", id), zap.Int("pid", pid))
			return
		}
		s.mu.Lock()
		if v, ok := s.vms[id]; ok {
			v.Status = VMStatusStopped
			v.PID = 0
		}
		delete(s.processes, id)
		s.qmpManager.Remove(id)
		s.saveVMs()
		s.mu.Unlock()
	}()

	s.logger.Info("vm started", zap.String("id", id), zap.Int("vnc_port", vm.VNCPort))
	return nil
}

// StopVM 停止虚拟机
func (s *Service) StopVM(id string, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	vm, ok := s.vms[id]
	if !ok {
		return fmt.Errorf("vm not found: %s", id)
	}

	if vm.Status != VMStatusRunning && vm.Status != VMStatusPaused {
		return nil
	}

	if force {
		// 强制停止: 使用 QMP quit 或 kill 进程
		if client, err := s.qmpManager.GetOrCreate(id, vm.QMPSocket); err == nil {
			client.Quit()
		} else if cmd, ok := s.processes[id]; ok && cmd.Process != nil {
			cmd.Process.Kill()
		}
	} else {
		// 优雅关机: 发送 ACPI powerdown
		if client, err := s.qmpManager.GetOrCreate(id, vm.QMPSocket); err == nil {
			if err := client.SystemPowerdown(); err != nil {
				s.logger.Warn("QMP powerdown failed, using signal", zap.Error(err))
				if cmd, ok := s.processes[id]; ok && cmd.Process != nil {
					cmd.Process.Signal(os.Interrupt)
				}
			}
		} else if cmd, ok := s.processes[id]; ok && cmd.Process != nil {
			cmd.Process.Signal(os.Interrupt)
		}
	}

	// 等待进程结束
	if cmd, ok := s.processes[id]; ok {
		done := make(chan struct{})
		go func() {
			cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			// 超时强制 kill
			if cmd.Process != nil {
				cmd.Process.Kill()
				cmd.Wait()
			}
		}
	}

	vm.Status = VMStatusStopped
	vm.PID = 0
	vm.UpdatedAt = time.Now()

	delete(s.processes, id)
	s.qmpManager.Remove(id)
	s.saveVMs()

	s.logger.Info("vm stopped", zap.String("id", id))
	return nil
}

// PauseVM 暂停虚拟机
func (s *Service) PauseVM(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	vm, ok := s.vms[id]
	if !ok {
		return fmt.Errorf("vm not found: %s", id)
	}

	if vm.Status != VMStatusRunning {
		return fmt.Errorf("vm is not running")
	}

	// 使用 QMP stop 命令暂停
	client, err := s.qmpManager.GetOrCreate(id, vm.QMPSocket)
	if err != nil {
		return fmt.Errorf("connect QMP: %w", err)
	}

	if err := client.Stop(); err != nil {
		return fmt.Errorf("QMP stop: %w", err)
	}

	vm.Status = VMStatusPaused
	vm.UpdatedAt = time.Now()
	s.saveVMs()

	s.logger.Info("vm paused", zap.String("id", id))
	return nil
}

// ResumeVM 恢复虚拟机
func (s *Service) ResumeVM(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	vm, ok := s.vms[id]
	if !ok {
		return fmt.Errorf("vm not found: %s", id)
	}

	if vm.Status != VMStatusPaused {
		return fmt.Errorf("vm is not paused")
	}

	// 使用 QMP cont 命令恢复
	client, err := s.qmpManager.GetOrCreate(id, vm.QMPSocket)
	if err != nil {
		return fmt.Errorf("connect QMP: %w", err)
	}

	if err := client.Cont(); err != nil {
		return fmt.Errorf("QMP cont: %w", err)
	}

	vm.Status = VMStatusRunning
	vm.UpdatedAt = time.Now()
	s.saveVMs()

	s.logger.Info("vm resumed", zap.String("id", id))
	return nil
}

// GetSnapshots 获取快照列表
func (s *Service) GetSnapshots(vmID string) ([]Snapshot, error) {
	s.mu.RLock()
	vm, ok := s.vms[vmID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("vm not found: %s", vmID)
	}

	snapshots := make([]Snapshot, 0)

	// 使用 qemu-img 获取快照列表
	cmd := exec.Command("qemu-img", "snapshot", "-l", vm.DiskPath)
	output, err := cmd.Output()
	if err != nil {
		return snapshots, nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Snapshot") || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			snapshots = append(snapshots, Snapshot{
				ID:        fields[0],
				VMID:      vmID,
				Name:      fields[1],
				CreatedAt: time.Now(),
			})
		}
	}

	return snapshots, nil
}

// CreateSnapshot 创建快照
func (s *Service) CreateSnapshot(vmID string, req CreateSnapshotRequest) (*Snapshot, error) {
	s.mu.RLock()
	vm, ok := s.vms[vmID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("vm not found: %s", vmID)
	}

	tag := req.Tag
	if tag == "" {
		tag = fmt.Sprintf("snap-%d", time.Now().Unix())
	}

	cmd := exec.Command("qemu-img", "snapshot", "-c", tag, vm.DiskPath)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}

	snapshot := &Snapshot{
		ID:        uuid.New().String()[:8],
		VMID:      vmID,
		Name:      req.Name,
		Tag:       tag,
		CreatedAt: time.Now(),
	}

	return snapshot, nil
}

// DeleteSnapshot 删除快照
func (s *Service) DeleteSnapshot(vmID, tag string) error {
	s.mu.RLock()
	vm, ok := s.vms[vmID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("vm not found: %s", vmID)
	}

	cmd := exec.Command("qemu-img", "snapshot", "-d", tag, vm.DiskPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("delete snapshot: %w", err)
	}

	return nil
}

// RevertSnapshot 恢复快照
func (s *Service) RevertSnapshot(vmID, tag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	vm, ok := s.vms[vmID]
	if !ok {
		return fmt.Errorf("vm not found: %s", vmID)
	}

	if vm.Status == VMStatusRunning {
		return fmt.Errorf("cannot revert snapshot while vm is running")
	}

	cmd := exec.Command("qemu-img", "snapshot", "-a", tag, vm.DiskPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("revert snapshot: %w", err)
	}

	return nil
}

// GetISOs 获取 ISO 列表
func (s *Service) GetISOs() ([]ISOFile, error) {
	isoDir := filepath.Join(s.dataDir, "iso")
	entries, err := os.ReadDir(isoDir)
	if err != nil {
		return nil, err
	}

	isos := make([]ISOFile, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".iso") {
			continue
		}
		info, _ := entry.Info()
		isos = append(isos, ISOFile{
			Name:    name,
			Path:    filepath.Join(isoDir, name),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	return isos, nil
}

// ResizeDisk 调整磁盘大小
func (s *Service) ResizeDisk(vmID string, size int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	vm, ok := s.vms[vmID]
	if !ok {
		return fmt.Errorf("vm not found: %s", vmID)
	}

	if vm.Status == VMStatusRunning {
		return fmt.Errorf("cannot resize disk while vm is running")
	}

	sizeStr := fmt.Sprintf("%dG", size)
	cmd := exec.Command("qemu-img", "resize", vm.DiskPath, sizeStr)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("resize disk: %w", err)
	}

	vm.DiskSize = size
	vm.UpdatedAt = time.Now()
	s.saveVMs()

	return nil
}

func (s *Service) createDisk(path string, sizeGB int64) error {
	sizeStr := fmt.Sprintf("%dG", sizeGB)
	cmd := exec.Command("qemu-img", "create", "-f", "qcow2", path, sizeStr)
	return cmd.Run()
}

func (s *Service) buildQEMUArgs(vm *VM) []string {
	args := []string{
		"-name", vm.Name,
		"-m", fmt.Sprintf("%d", vm.Memory),
		"-smp", fmt.Sprintf("%d", vm.CPU),
		"-vnc", fmt.Sprintf(":%d", vm.VNCPort-5900),
		"-qmp", fmt.Sprintf("unix:%s,server,nowait", vm.QMPSocket),
		"-daemonize",
	}

	// 磁盘配置（支持 I/O 线程）
	driveOpts := fmt.Sprintf("file=%s,format=qcow2", vm.DiskPath)
	if vm.IOThread {
		args = append(args, "-object", "iothread,id=io1")
		driveOpts += ",aio=native,cache=none"
		args = append(args, "-drive", driveOpts+",if=none,id=drive0")
		args = append(args, "-device", "virtio-blk-pci,drive=drive0,iothread=io1")
	} else {
		args = append(args, "-drive", driveOpts+",if=virtio")
	}

	if vm.Accelerator == "kvm" {
		args = append(args, "-enable-kvm")

		// CPU 模型配置
		cpuModel := vm.CPUModel
		if cpuModel == "" {
			cpuModel = "host" // 默认使用 host CPU
		}
		args = append(args, "-cpu", cpuModel)

		// CPU 绑定
		if len(vm.CPUPinning) > 0 {
			for i, core := range vm.CPUPinning {
				if i < vm.CPU {
					args = append(args, "-vcpu", fmt.Sprintf("%d,affinity=%d", i, core))
				}
			}
		}

		// 大页内存
		if vm.EnableHuge {
			args = append(args, "-mem-path", "/dev/hugepages")
			args = append(args, "-mem-prealloc")
		}
	}

	// 网络配置
	networkMode := vm.NetworkMode
	if networkMode == "" {
		networkMode = "user" // 默认 user 模式
	}

	switch networkMode {
	case "bridge":
		// 桥接网络模式
		bridgeIface := vm.BridgeIface
		if bridgeIface == "" {
			bridgeIface = "virbr0" // 默认桥接接口
		}
		// 使用 tap 设备桥接
		args = append(args, "-netdev", fmt.Sprintf("bridge,id=net0,br=%s", bridgeIface))
		args = append(args, "-device", "virtio-net-pci,netdev=net0")
	case "none":
		// 无网络
		args = append(args, "-nic", "none")
	default:
		// user 模式（默认）
		netdev := "user,id=net0"
		for _, pf := range vm.PortForwards {
			proto := pf.Protocol
			if proto == "" {
				proto = "tcp"
			}
			netdev += fmt.Sprintf(",hostfwd=%s::%d-:%d", proto, pf.HostPort, pf.GuestPort)
		}
		args = append(args, "-netdev", netdev)
		args = append(args, "-device", "virtio-net-pci,netdev=net0")
	}

	// USB 控制器和设备直通
	if len(vm.USBDevices) > 0 {
		// 添加 USB 控制器
		args = append(args, "-usb")
		args = append(args, "-device", "usb-ehci,id=ehci")

		// 添加 USB 设备
		for _, usbDev := range vm.USBDevices {
			if usbDev.VendorID != "" && usbDev.ProductID != "" {
				// 通过 vendor:product 直通
				args = append(args, "-device",
					fmt.Sprintf("usb-host,vendorid=0x%s,productid=0x%s", usbDev.VendorID, usbDev.ProductID))
			} else if usbDev.Bus > 0 && usbDev.Device > 0 {
				// 通过 bus.device 直通
				args = append(args, "-device",
					fmt.Sprintf("usb-host,hostbus=%d,hostaddr=%d", usbDev.Bus, usbDev.Device))
			}
		}
	}

	// ISO
	if vm.ISOPath != "" {
		args = append(args, "-cdrom", vm.ISOPath)
		args = append(args, "-boot", "d")
	}

	return args
}

func (s *Service) detectAccelerator() string {
	if _, err := os.Stat("/dev/kvm"); err == nil {
		return "kvm"
	}
	return "tcg"
}

func (s *Service) allocateVNCPort() int {
	port := s.vncBasePort
	for _, vm := range s.vms {
		if vm.VNCPort >= port {
			port = vm.VNCPort + 1
		}
	}
	return port
}

func (s *Service) allocateSSHPort() int {
	return s.allocatePort(s.sshBasePort)
}

// allocatePort 分配一个未使用的端口
func (s *Service) allocatePort(basePort int) int {
	usedPorts := make(map[int]bool)
	for _, vm := range s.vms {
		usedPorts[vm.VNCPort] = true
		usedPorts[vm.SSHPort] = true
		for _, pf := range vm.PortForwards {
			usedPorts[pf.HostPort] = true
		}
	}

	port := basePort
	for usedPorts[port] {
		port++
	}
	return port
}

func (s *Service) updateVMStatus(vm *VM) {
	if vm.PID <= 0 {
		return
	}
	if _, err := os.FindProcess(vm.PID); err != nil {
		vm.Status = VMStatusStopped
		vm.PID = 0
	}
}

func (s *Service) loadVMs() {
	file := filepath.Join(s.dataDir, "vms.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return
	}

	var vms map[string]*VM
	if json.Unmarshal(data, &vms) == nil {
		for _, vm := range vms {
			// 检测是否有正在运行的 QEMU 进程
			if pid, vncPort := s.findRunningQEMU(vm.ID); pid > 0 {
				vm.Status = VMStatusRunning
				vm.PID = pid
				vm.VNCPort = vncPort
				vm.QMPSocket = filepath.Join(s.dataDir, "qmp", vm.ID+".sock")
				// 异步连接 QMP
				go func(vmID, qmpSock string) {
					if _, err := s.qmpManager.GetOrCreate(vmID, qmpSock); err != nil {
						s.logger.Warn("reconnect QMP failed", zap.String("vm", vmID), zap.Error(err))
					}
				}(vm.ID, vm.QMPSocket)
				s.logger.Info("recovered running vm", zap.String("id", vm.ID), zap.Int("pid", pid), zap.Int("vnc_port", vncPort))
			} else {
				vm.Status = VMStatusStopped
				vm.PID = 0
			}
		}
		s.vms = vms
	}
}

// findRunningQEMU 查找正在运行的 QEMU 进程
func (s *Service) findRunningQEMU(vmID string) (pid int, vncPort int) {
	// 查找 QMP socket 文件
	qmpSock := filepath.Join(s.dataDir, "qmp", vmID+".sock")
	if _, err := os.Stat(qmpSock); os.IsNotExist(err) {
		return 0, 0
	}

	// 读取 /proc 查找 QEMU 进程
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, 0
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		p, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}

		cmdStr := string(cmdline)
		// 检查是否是目标 VM 的 QEMU 进程
		if strings.Contains(cmdStr, "qemu-system") && strings.Contains(cmdStr, vmID+".sock") {
			pid = p
			// 解析 VNC 端口: 从 -vnc :N 参数
			if idx := strings.Index(cmdStr, "-vnc"); idx >= 0 {
				// cmdline 用 \x00 分隔，找到 :N
				parts := strings.Split(cmdStr[idx:], "\x00")
				if len(parts) >= 2 {
					vncArg := parts[1] // :0 或 :1 等
					if strings.HasPrefix(vncArg, ":") {
						if n, err := strconv.Atoi(vncArg[1:]); err == nil {
							vncPort = 5900 + n
						}
					}
				}
			}
			return pid, vncPort
		}
	}
	return 0, 0
}

func (s *Service) saveVMs() {
	file := filepath.Join(s.dataDir, "vms.json")
	data, _ := json.MarshalIndent(s.vms, "", "  ")
	os.WriteFile(file, data, 0644)
}

// GetTemplates 获取模板列表（包含内置和自定义模板）
func (s *Service) GetTemplates() []VMTemplate {
	// 内置模板
	builtinTemplates := []VMTemplate{
		{ID: "ubuntu-22.04", Name: "Ubuntu 22.04 LTS", OS: "Ubuntu 22.04", OSType: "linux", Memory: 2048, CPU: 2, DiskSize: 20},
		{ID: "ubuntu-24.04", Name: "Ubuntu 24.04 LTS", OS: "Ubuntu 24.04", OSType: "linux", Memory: 2048, CPU: 2, DiskSize: 20},
		{ID: "debian-12", Name: "Debian 12", OS: "Debian 12", OSType: "linux", Memory: 2048, CPU: 2, DiskSize: 20},
		{ID: "fedora-40", Name: "Fedora 40", OS: "Fedora 40", OSType: "linux", Memory: 2048, CPU: 2, DiskSize: 20},
		{ID: "windows-11", Name: "Windows 11", OS: "Windows 11", OSType: "windows", Memory: 8192, CPU: 4, DiskSize: 64},
	}

	// 加载自定义模板
	customTemplates, err := s.loadCustomTemplates()
	if err != nil {
		s.logger.Warn("load custom templates failed", zap.Error(err))
		return builtinTemplates
	}

	// 合并：自定义模板在前
	return append(customTemplates, builtinTemplates...)
}

// GetTemplate 根据ID获取单个模板
func (s *Service) GetTemplate(templateID string) (*VMTemplate, error) {
	templates := s.GetTemplates()
	for _, t := range templates {
		if t.ID == templateID {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("template not found: %s", templateID)
}

// GetVNCWebSocket 获取 VNC WebSocket 地址
func (s *Service) GetVNCWebSocket(vmID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vm, ok := s.vms[vmID]
	if !ok {
		return "", fmt.Errorf("vm not found: %s", vmID)
	}

	if vm.Status != VMStatusRunning {
		return "", fmt.Errorf("vm is not running")
	}

	return "localhost:" + strconv.Itoa(vm.VNCPort), nil
}

// RestartVM 重启虚拟机
func (s *Service) RestartVM(id string) error {
	if err := s.StopVM(id, false); err != nil {
		return fmt.Errorf("stop for restart: %w", err)
	}
	// 等待进程退出
	time.Sleep(time.Second)
	if err := s.StartVM(id); err != nil {
		return fmt.Errorf("start after restart: %w", err)
	}
	return nil
}

// StopAllVMs 停止所有虚拟机
func (s *Service) StopAllVMs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, cmd := range s.processes {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
		if vm, ok := s.vms[id]; ok {
			vm.Status = VMStatusStopped
			vm.PID = 0
		}
	}
	s.processes = make(map[string]*exec.Cmd)
	s.saveVMs()
}

// UploadISO 上传 ISO 文件
func (s *Service) UploadISO(filename string, data []byte) (string, error) {
	isoDir := filepath.Join(s.dataDir, "iso")
	os.MkdirAll(isoDir, 0755)

	path := filepath.Join(isoDir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write iso: %w", err)
	}

	s.logger.Info("ISO uploaded", zap.String("path", path), zap.Int("size", len(data)))
	return path, nil
}

// DeleteISO 删除 ISO 文件
func (s *Service) DeleteISO(name string) error {
	path := filepath.Join(s.dataDir, "iso", name)

	// 检查是否有 VM 正在使用此 ISO
	s.mu.RLock()
	for _, vm := range s.vms {
		if vm.ISOPath == path && vm.Status == VMStatusRunning {
			s.mu.RUnlock()
			return fmt.Errorf("ISO is in use by VM %s", vm.Name)
		}
	}
	s.mu.RUnlock()

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete iso: %w", err)
	}

	s.logger.Info("ISO deleted", zap.String("name", name))
	return nil
}

// GetVNCToken 获取 VNC 连接令牌
func (s *Service) GetVNCToken(vmID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vm, ok := s.vms[vmID]
	if !ok {
		return "", fmt.Errorf("vm not found: %s", vmID)
	}

	if vm.Status != VMStatusRunning {
		return "", fmt.Errorf("vm is not running")
	}

	token := s.vncTokenMgr.GenerateToken(vmID, vm.VNCPort, 30*time.Minute)
	return token, nil
}

// ValidateVNCToken 验证 VNC 令牌
func (s *Service) ValidateVNCToken(token string) (*VNCToken, bool) {
	return s.vncTokenMgr.ValidateToken(token)
}

// GetVNCProxy 获取 VNC 代理
func (s *Service) GetVNCProxy() *VNCProxy {
	return s.vncProxy
}

// SendKey 发送按键到虚拟机
func (s *Service) SendKey(vmID string, keys []string) error {
	s.mu.RLock()
	vm, ok := s.vms[vmID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("vm not found: %s", vmID)
	}

	if vm.Status != VMStatusRunning {
		return fmt.Errorf("vm is not running")
	}

	client, err := s.qmpManager.GetOrCreate(vmID, vm.QMPSocket)
	if err != nil {
		return fmt.Errorf("connect QMP: %w", err)
	}

	return client.SendKey(keys)
}

// SendCtrlAltDel 发送 Ctrl+Alt+Del 到虚拟机
func (s *Service) SendCtrlAltDel(vmID string) error {
	s.mu.RLock()
	vm, ok := s.vms[vmID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("vm not found: %s", vmID)
	}

	if vm.Status != VMStatusRunning {
		return fmt.Errorf("vm is not running")
	}

	client, err := s.qmpManager.GetOrCreate(vmID, vm.QMPSocket)
	if err != nil {
		return fmt.Errorf("connect QMP: %w", err)
	}

	return client.SendCtrlAltDel()
}

// Screendump 截取虚拟机屏幕
func (s *Service) Screendump(vmID string) (string, error) {
	s.mu.RLock()
	vm, ok := s.vms[vmID]
	s.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("vm not found: %s", vmID)
	}

	if vm.Status != VMStatusRunning {
		return "", fmt.Errorf("vm is not running")
	}

	client, err := s.qmpManager.GetOrCreate(vmID, vm.QMPSocket)
	if err != nil {
		return "", fmt.Errorf("connect QMP: %w", err)
	}

	screenshotDir := filepath.Join(s.dataDir, "screenshots")
	os.MkdirAll(screenshotDir, 0755)
	filename := filepath.Join(screenshotDir, fmt.Sprintf("%s_%d.ppm", vmID, time.Now().Unix()))

	if err := client.Screendump(filename); err != nil {
		return "", fmt.Errorf("screendump: %w", err)
	}

	return filename, nil
}

// ==================== P2: 模板系统增强 ====================

// customTemplatesFile 返回自定义模板配置文件路径
func (s *Service) customTemplatesFile() string {
	return filepath.Join(s.dataDir, "custom_templates.json")
}

// loadCustomTemplates 加载自定义模板
func (s *Service) loadCustomTemplates() ([]VMTemplate, error) {
	data, err := os.ReadFile(s.customTemplatesFile())
	if err != nil {
		if os.IsNotExist(err) {
			return []VMTemplate{}, nil
		}
		return nil, err
	}

	var templates []VMTemplate
	if err := json.Unmarshal(data, &templates); err != nil {
		return nil, err
	}
	return templates, nil
}

// saveCustomTemplates 保存自定义模板
func (s *Service) saveCustomTemplates(templates []VMTemplate) error {
	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.customTemplatesFile(), data, 0644)
}

// CreateTemplateFromVM 从现有VM创建模板
func (s *Service) CreateTemplateFromVM(req CreateTemplateRequest) (*VMTemplate, error) {
	s.mu.RLock()
	vm, ok := s.vms[req.VMID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("vm not found: %s", req.VMID)
	}

	if vm.Status == VMStatusRunning {
		return nil, fmt.Errorf("请先关闭虚拟机再创建模板")
	}

	// 创建模板目录
	templateDir := filepath.Join(s.dataDir, "templates")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return nil, fmt.Errorf("create template dir: %w", err)
	}

	// 生成模板ID
	templateID := fmt.Sprintf("custom-%s-%d", req.VMID[:8], time.Now().Unix())

	// 复制磁盘文件作为模板基础镜像
	baseDiskPath := filepath.Join(templateDir, fmt.Sprintf("%s.qcow2", templateID))

	// 使用 qemu-img convert 压缩磁盘
	cmd := exec.Command("qemu-img", "convert", "-c", "-O", "qcow2", vm.DiskPath, baseDiskPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("copy disk: %s, %w", string(output), err)
	}

	// 获取模板磁盘大小
	diskInfo, _ := os.Stat(baseDiskPath)
	diskSize := diskInfo.Size() / (1024 * 1024 * 1024) // GB

	// 创建模板
	template := VMTemplate{
		ID:          templateID,
		Name:        req.Name,
		Description: req.Description,
		OS:          vm.OS,
		OSType:      vm.OSType,
		Memory:      vm.Memory,
		CPU:         vm.CPU,
		DiskSize:    diskSize,
		BaseDisk:    baseDiskPath,
		IsCustom:    true,
		CreatedAt:   time.Now(),
	}

	// 保存到自定义模板列表
	templates, err := s.loadCustomTemplates()
	if err != nil {
		return nil, fmt.Errorf("load templates: %w", err)
	}
	templates = append(templates, template)
	if err := s.saveCustomTemplates(templates); err != nil {
		return nil, fmt.Errorf("save templates: %w", err)
	}

	return &template, nil
}

// DeleteTemplate 删除自定义模板
func (s *Service) DeleteTemplate(templateID string) error {
	templates, err := s.loadCustomTemplates()
	if err != nil {
		return fmt.Errorf("load templates: %w", err)
	}

	var found *VMTemplate
	var newTemplates []VMTemplate
	for _, t := range templates {
		if t.ID == templateID {
			found = &t
		} else {
			newTemplates = append(newTemplates, t)
		}
	}

	if found == nil {
		return fmt.Errorf("template not found: %s", templateID)
	}

	// 删除模板磁盘文件
	if found.BaseDisk != "" {
		os.Remove(found.BaseDisk)
	}

	return s.saveCustomTemplates(newTemplates)
}

// ==================== P2: 增量磁盘支持 ====================

// createIncrementalDisk 创建基于backing file的增量磁盘
func (s *Service) createIncrementalDisk(diskPath, backingFile string, sizeGB int) error {
	args := []string{"create", "-f", "qcow2"}

	if backingFile != "" {
		// 增量磁盘，基于模板
		args = append(args, "-b", backingFile, "-F", "qcow2", diskPath)
	} else {
		// 完整磁盘
		args = append(args, diskPath, fmt.Sprintf("%dG", sizeGB))
	}

	cmd := exec.Command("qemu-img", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create disk: %s, %w", string(output), err)
	}
	return nil
}

// ==================== P2: 备份还原功能 ====================

// backupsDir 返回备份目录路径
func (s *Service) backupsDir() string {
	return filepath.Join(s.dataDir, "backups")
}

// backupIndexFile 返回备份索引文件路径
func (s *Service) backupIndexFile() string {
	return filepath.Join(s.backupsDir(), "index.json")
}

// loadBackupIndex 加载备份索引
func (s *Service) loadBackupIndex() ([]BackupInfo, error) {
	data, err := os.ReadFile(s.backupIndexFile())
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, err
	}

	var backups []BackupInfo
	if err := json.Unmarshal(data, &backups); err != nil {
		return nil, err
	}
	return backups, nil
}

// saveBackupIndex 保存备份索引
func (s *Service) saveBackupIndex(backups []BackupInfo) error {
	if err := os.MkdirAll(s.backupsDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(backups, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.backupIndexFile(), data, 0644)
}

// CreateBackup 创建VM备份
func (s *Service) CreateBackup(req CreateBackupRequest) (*BackupInfo, error) {
	s.mu.RLock()
	vm, ok := s.vms[req.VMID]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("vm not found: %s", req.VMID)
	}

	if vm.Status == VMStatusRunning {
		return nil, fmt.Errorf("请先关闭虚拟机再创建备份")
	}

	// 创建备份目录
	backupDir := s.backupsDir()
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}

	// 生成备份ID
	backupID := fmt.Sprintf("backup-%s-%d", req.VMID[:8], time.Now().Unix())
	backupPath := filepath.Join(backupDir, backupID)
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return nil, fmt.Errorf("create backup path: %w", err)
	}

	// 复制磁盘文件（压缩）
	diskBackup := filepath.Join(backupPath, "disk.qcow2")
	cmd := exec.Command("qemu-img", "convert", "-c", "-O", "qcow2", vm.DiskPath, diskBackup)
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(backupPath)
		return nil, fmt.Errorf("backup disk: %s, %w", string(output), err)
	}

	// 保存VM配置
	vmConfig := *vm
	vmConfig.DiskPath = "" // 清除原路径
	configBackup := filepath.Join(backupPath, "config.json")
	configData, _ := json.MarshalIndent(vmConfig, "", "  ")
	if err := os.WriteFile(configBackup, configData, 0644); err != nil {
		os.RemoveAll(backupPath)
		return nil, fmt.Errorf("backup config: %w", err)
	}

	// 获取备份大小
	diskInfo, _ := os.Stat(diskBackup)
	backupSize := diskInfo.Size()

	// 创建备份信息
	backup := BackupInfo{
		ID:          backupID,
		VMID:        req.VMID,
		VMName:      vm.Name,
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   time.Now(),
		Size:        backupSize,
		Path:        backupPath,
	}

	// 保存到备份索引
	backups, err := s.loadBackupIndex()
	if err != nil {
		return nil, fmt.Errorf("load backup index: %w", err)
	}
	backups = append(backups, backup)
	if err := s.saveBackupIndex(backups); err != nil {
		return nil, fmt.Errorf("save backup index: %w", err)
	}

	return &backup, nil
}

// ListBackups 列出所有备份
func (s *Service) ListBackups(vmID string) ([]BackupInfo, error) {
	backups, err := s.loadBackupIndex()
	if err != nil {
		return nil, err
	}

	if vmID == "" {
		return backups, nil
	}

	// 过滤指定VM的备份
	var filtered []BackupInfo
	for _, b := range backups {
		if b.VMID == vmID {
			filtered = append(filtered, b)
		}
	}
	return filtered, nil
}

// RestoreBackup 从备份恢复VM
func (s *Service) RestoreBackup(req RestoreBackupRequest) (*VM, error) {
	// 加载备份索引
	backups, err := s.loadBackupIndex()
	if err != nil {
		return nil, fmt.Errorf("load backup index: %w", err)
	}

	// 查找备份
	var backup *BackupInfo
	for _, b := range backups {
		if b.ID == req.BackupID {
			backup = &b
			break
		}
	}
	if backup == nil {
		return nil, fmt.Errorf("backup not found: %s", req.BackupID)
	}

	// 读取备份配置
	configFile := filepath.Join(backup.Path, "config.json")
	configData, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("read backup config: %w", err)
	}

	var vmConfig VM
	if err := json.Unmarshal(configData, &vmConfig); err != nil {
		return nil, fmt.Errorf("parse backup config: %w", err)
	}

	// 生成新VM ID
	newVMID := uuid.New().String()

	// 设置新VM名称
	newName := req.NewName
	if newName == "" {
		newName = fmt.Sprintf("%s (恢复)", backup.VMName)
	}

	// 创建新VM目录
	vmDir := filepath.Join(s.dataDir, "vms", newVMID)
	if err := os.MkdirAll(vmDir, 0755); err != nil {
		return nil, fmt.Errorf("create vm dir: %w", err)
	}

	// 复制磁盘文件
	diskBackup := filepath.Join(backup.Path, "disk.qcow2")
	newDiskPath := filepath.Join(vmDir, "disk.qcow2")
	cmd := exec.Command("cp", diskBackup, newDiskPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(vmDir)
		return nil, fmt.Errorf("restore disk: %s, %w", string(output), err)
	}

	// 创建新VM
	newVM := &VM{
		ID:           newVMID,
		Name:         newName,
		Status:       VMStatusStopped,
		CPU:          vmConfig.CPU,
		Memory:       vmConfig.Memory,
		DiskPath:     newDiskPath,
		DiskSize:     vmConfig.DiskSize,
		OS:           vmConfig.OS,
		OSType:       vmConfig.OSType,
		VNCPort:      0,
		PortForwards: vmConfig.PortForwards,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 保存新VM
	s.mu.Lock()
	s.vms[newVMID] = newVM
	s.mu.Unlock()

	s.saveVMs()

	return newVM, nil
}

// DeleteBackup 删除备份
func (s *Service) DeleteBackup(backupID string) error {
	backups, err := s.loadBackupIndex()
	if err != nil {
		return fmt.Errorf("load backup index: %w", err)
	}

	var found *BackupInfo
	var newBackups []BackupInfo
	for _, b := range backups {
		if b.ID == backupID {
			found = &b
		} else {
			newBackups = append(newBackups, b)
		}
	}

	if found == nil {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	// 删除备份文件
	if err := os.RemoveAll(found.Path); err != nil {
		return fmt.Errorf("remove backup files: %w", err)
	}

	return s.saveBackupIndex(newBackups)
}

// ==================== P3: 批量操作 ====================

// BatchStart 批量启动虚拟机
func (s *Service) BatchStart(ids []string) []BatchResult {
	results := make([]BatchResult, len(ids))
	for i, id := range ids {
		results[i] = BatchResult{ID: id}
		if err := s.StartVM(id); err != nil {
			results[i].Error = err.Error()
		} else {
			results[i].Success = true
		}
	}
	return results
}

// BatchStop 批量停止虚拟机
func (s *Service) BatchStop(ids []string, force bool) []BatchResult {
	results := make([]BatchResult, len(ids))
	for i, id := range ids {
		results[i] = BatchResult{ID: id}
		if err := s.StopVM(id, force); err != nil {
			results[i].Error = err.Error()
		} else {
			results[i].Success = true
		}
	}
	return results
}

// BatchDelete 批量删除虚拟机
func (s *Service) BatchDelete(ids []string) []BatchResult {
	results := make([]BatchResult, len(ids))
	for i, id := range ids {
		results[i] = BatchResult{ID: id}
		if err := s.DeleteVM(id); err != nil {
			results[i].Error = err.Error()
		} else {
			results[i].Success = true
		}
	}
	return results
}

// ==================== P3: 克隆 ====================

// CloneVM 克隆虚拟机
func (s *Service) CloneVM(id string, newName string) (*VM, error) {
	s.mu.RLock()
	srcVM, ok := s.vms[id]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("vm not found: %s", id)
	}

	if srcVM.Status == VMStatusRunning {
		return nil, fmt.Errorf("请先关闭虚拟机再进行克隆")
	}

	// 生成新 VM ID
	newID := uuid.New().String()[:8]

	// 创建新磁盘目录
	newDiskPath := filepath.Join(s.dataDir, "disks", newID+".qcow2")

	// 复制磁盘文件
	cmd := exec.Command("cp", srcVM.DiskPath, newDiskPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("copy disk: %s, %w", string(output), err)
	}

	// 创建新 VM
	newVM := &VM{
		ID:           newID,
		Name:         newName,
		Description:  srcVM.Description,
		Status:       VMStatusStopped,
		CPU:          srcVM.CPU,
		Memory:       srcVM.Memory,
		DiskSize:     srcVM.DiskSize,
		DiskPath:     newDiskPath,
		OS:           srcVM.OS,
		OSType:       srcVM.OSType,
		Arch:         srcVM.Arch,
		Accelerator:  srcVM.Accelerator,
		Network:      srcVM.Network,
		Display:      srcVM.Display,
		PortForwards: srcVM.PortForwards,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	s.mu.Lock()
	s.vms[newID] = newVM
	s.mu.Unlock()

	s.saveVMs()

	s.logger.Info("VM cloned", zap.String("srcID", id), zap.String("newID", newID))
	return newVM, nil
}

// ==================== P3: 存储信息 ====================

// GetStorageInfo 获取存储使用信息
func (s *Service) GetStorageInfo() (*StorageInfo, error) {
	info := &StorageInfo{}

	// 获取数据目录所在分区的空间信息
	var stat syscall.Statfs_t
	if err := syscall.Statfs(s.dataDir, &stat); err != nil {
		return nil, fmt.Errorf("statfs: %w", err)
	}
	info.TotalSpace = int64(stat.Blocks) * int64(stat.Bsize)
	info.FreeSpace = int64(stat.Bavail) * int64(stat.Bsize)
	info.UsedSpace = info.TotalSpace - info.FreeSpace

	// 统计 VM 磁盘占用
	disksDir := filepath.Join(s.dataDir, "disks")
	info.VMDiskUsage = s.getDirSize(disksDir)

	// 统计 ISO 占用
	isoDir := filepath.Join(s.dataDir, "iso")
	info.ISOUsage = s.getDirSize(isoDir)

	// 统计备份占用
	backupsDir := filepath.Join(s.dataDir, "backups")
	info.BackupUsage = s.getDirSize(backupsDir)

	// 统计模板占用
	templatesDir := filepath.Join(s.dataDir, "templates")
	info.TemplateUsage = s.getDirSize(templatesDir)

	// 统计快照占用（快照通常嵌入在 qcow2 文件中，这里仅统计独立快照目录）
	snapshotsDir := filepath.Join(s.dataDir, "snapshots")
	info.SnapshotUsage = s.getDirSize(snapshotsDir)

	// 统计 VM 数量
	s.mu.RLock()
	info.VMCount = len(s.vms)
	for _, vm := range s.vms {
		if vm.Status == VMStatusRunning {
			info.RunningVMCount++
		}
	}
	s.mu.RUnlock()

	return info, nil
}

// getDirSize 获取目录大小
func (s *Service) getDirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// ListUSBDevices 列出主机上的USB设备
func (s *Service) ListUSBDevices() ([]USBDevice, error) {
	var devices []USBDevice

	// 读取 /sys/bus/usb/devices
	devicesPath := "/sys/bus/usb/devices"
	entries, err := os.ReadDir(devicesPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取USB设备: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		// 过滤掉非设备目录（如 usb1, 1-0:1.0 等）
		if strings.Contains(name, ":") || strings.HasPrefix(name, "usb") {
			continue
		}

		devicePath := filepath.Join(devicesPath, name)

		// 读取 vendor ID
		vendorData, err := os.ReadFile(filepath.Join(devicePath, "idVendor"))
		if err != nil {
			continue
		}
		vendorID := strings.TrimSpace(string(vendorData))

		// 读取 product ID
		productData, err := os.ReadFile(filepath.Join(devicePath, "idProduct"))
		if err != nil {
			continue
		}
		productID := strings.TrimSpace(string(productData))

		// 读取设备名称（可选）
		var deviceName string
		if productNameData, err := os.ReadFile(filepath.Join(devicePath, "product")); err == nil {
			deviceName = strings.TrimSpace(string(productNameData))
		}
		if deviceName == "" {
			if manufacturerData, err := os.ReadFile(filepath.Join(devicePath, "manufacturer")); err == nil {
				deviceName = strings.TrimSpace(string(manufacturerData))
			}
		}

		// 解析 bus 和 device 号
		var bus, device int
		parts := strings.Split(name, "-")
		if len(parts) >= 1 {
			fmt.Sscanf(parts[0], "%d", &bus)
		}
		if devnumData, err := os.ReadFile(filepath.Join(devicePath, "devnum")); err == nil {
			fmt.Sscanf(strings.TrimSpace(string(devnumData)), "%d", &device)
		}

		devices = append(devices, USBDevice{
			VendorID:  vendorID,
			ProductID: productID,
			Name:      deviceName,
			Bus:       bus,
			Device:    device,
		})
	}

	return devices, nil
}

// NetworkInterface 网络接口信息
type NetworkInterface struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // bridge, ethernet, wireless
	Status  string `json:"status"`
	Address string `json:"address,omitempty"`
}

// ListNetworkInterfaces 列出可用网络接口（用于桥接）
func (s *Service) ListNetworkInterfaces() ([]NetworkInterface, error) {
	var interfaces []NetworkInterface

	// 读取 /sys/class/net
	netPath := "/sys/class/net"
	entries, err := os.ReadDir(netPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取网络接口: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" {
			continue // 跳过回环接口
		}

		iface := NetworkInterface{Name: name}

		// 判断接口类型
		bridgePath := filepath.Join(netPath, name, "bridge")
		if _, err := os.Stat(bridgePath); err == nil {
			iface.Type = "bridge"
		} else if strings.HasPrefix(name, "wl") {
			iface.Type = "wireless"
		} else if strings.HasPrefix(name, "en") || strings.HasPrefix(name, "eth") {
			iface.Type = "ethernet"
		} else if strings.HasPrefix(name, "virbr") || strings.HasPrefix(name, "docker") {
			iface.Type = "virtual"
		} else {
			iface.Type = "other"
		}

		// 读取状态
		operstatePath := filepath.Join(netPath, name, "operstate")
		if data, err := os.ReadFile(operstatePath); err == nil {
			iface.Status = strings.TrimSpace(string(data))
		}

		// 读取 IP 地址
		iface.Address = s.getInterfaceIP(name)

		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

// getInterfaceIP 获取接口的IP地址
func (s *Service) getInterfaceIP(name string) string {
	cmd := exec.Command("ip", "-4", "addr", "show", name)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "inet ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// 返回 IP，去掉 CIDR
				addr := parts[1]
				if idx := strings.Index(addr, "/"); idx > 0 {
					return addr[:idx]
				}
				return addr
			}
		}
	}
	return ""
}

// ==================== P5: 自动启动 ====================

// SetAutoStart 设置 VM 自动启动
func (s *Service) SetAutoStart(vmID string, enabled bool, order int, delay int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	vm, exists := s.vms[vmID]
	if !exists {
		return fmt.Errorf("虚拟机不存在")
	}

	vm.AutoStart = enabled
	vm.StartOrder = order
	vm.StartDelay = delay
	vm.UpdatedAt = time.Now()
	s.saveVMs()

	return nil
}

// GetAutoStartVMs 获取自动启动的 VM 列表（按顺序）
func (s *Service) GetAutoStartVMs() []*VM {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var autoStartVMs []*VM
	for _, vm := range s.vms {
		if vm.AutoStart {
			autoStartVMs = append(autoStartVMs, vm)
		}
	}

	// 按启动顺序排序
	for i := 0; i < len(autoStartVMs)-1; i++ {
		for j := i + 1; j < len(autoStartVMs); j++ {
			if autoStartVMs[i].StartOrder > autoStartVMs[j].StartOrder {
				autoStartVMs[i], autoStartVMs[j] = autoStartVMs[j], autoStartVMs[i]
			}
		}
	}

	return autoStartVMs
}

// StartAutoStartVMs 启动所有自动启动的 VM
func (s *Service) StartAutoStartVMs() []BatchResult {
	vms := s.GetAutoStartVMs()
	results := make([]BatchResult, 0, len(vms))

	for _, vm := range vms {
		// 等待延迟
		if vm.StartDelay > 0 {
			time.Sleep(time.Duration(vm.StartDelay) * time.Second)
		}

		err := s.StartVM(vm.ID)
		result := BatchResult{ID: vm.ID, Success: err == nil}
		if err != nil {
			result.Error = err.Error()
			s.logger.Warn("自动启动 VM 失败", zap.String("vm", vm.Name), zap.Error(err))
		} else {
			s.logger.Info("自动启动 VM 成功", zap.String("vm", vm.Name))
		}
		results = append(results, result)
	}

	return results
}

// ==================== P5: VM 导入导出 ====================

// ExportVM 导出虚拟机
func (s *Service) ExportVM(req ExportVMRequest) (*ExportResult, error) {
	s.mu.RLock()
	vm, exists := s.vms[req.VMID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("虚拟机不存在")
	}

	if vm.Status == VMStatusRunning {
		return nil, fmt.Errorf("请先停止虚拟机再导出")
	}

	// 创建导出目录
	exportDir := filepath.Join(s.dataDir, "exports")
	os.MkdirAll(exportDir, 0755)

	format := req.Format
	if format == "" {
		format = ExportFormatOVA
	}

	timestamp := time.Now().Format("20060102-150405")
	baseName := fmt.Sprintf("%s-%s", vm.Name, timestamp)

	var exportPath string
	var exportSize int64

	switch format {
	case ExportFormatOVA:
		// 创建 OVA 包（简化版：tar 打包）
		exportPath = filepath.Join(exportDir, baseName+".ova")
		
		// 创建 OVF 描述文件
		ovfPath := filepath.Join(exportDir, baseName+".ovf")
		ovfContent := s.generateOVF(vm, req.IncludeISO)
		if err := os.WriteFile(ovfPath, []byte(ovfContent), 0644); err != nil {
			return nil, fmt.Errorf("创建 OVF 文件失败: %v", err)
		}
		defer os.Remove(ovfPath)

		// 复制磁盘文件
		diskCopy := filepath.Join(exportDir, baseName+"-disk.qcow2")
		if err := s.copyFile(vm.DiskPath, diskCopy); err != nil {
			return nil, fmt.Errorf("复制磁盘失败: %v", err)
		}
		defer os.Remove(diskCopy)

		// 创建 tar 包
		tarArgs := []string{"-cvf", exportPath, "-C", exportDir, 
			filepath.Base(ovfPath), filepath.Base(diskCopy)}
		
		if req.IncludeISO && vm.ISOPath != "" {
			isoCopy := filepath.Join(exportDir, filepath.Base(vm.ISOPath))
			if err := s.copyFile(vm.ISOPath, isoCopy); err == nil {
				tarArgs = append(tarArgs, filepath.Base(isoCopy))
				defer os.Remove(isoCopy)
			}
		}

		cmd := exec.Command("tar", tarArgs...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("创建 OVA 失败: %v, %s", err, string(output))
		}

	case ExportFormatQCOW2:
		// 直接复制 qcow2 文件
		exportPath = filepath.Join(exportDir, baseName+".qcow2")
		if req.Compress {
			// 使用 qemu-img 压缩转换
			cmd := exec.Command("qemu-img", "convert", "-c", "-O", "qcow2", 
				vm.DiskPath, exportPath)
			if output, err := cmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("压缩导出失败: %v, %s", err, string(output))
			}
		} else {
			if err := s.copyFile(vm.DiskPath, exportPath); err != nil {
				return nil, fmt.Errorf("复制磁盘失败: %v", err)
			}
		}

	case ExportFormatRAW:
		// 转换为 raw 格式
		exportPath = filepath.Join(exportDir, baseName+".raw")
		cmd := exec.Command("qemu-img", "convert", "-O", "raw", vm.DiskPath, exportPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("转换为 RAW 失败: %v, %s", err, string(output))
		}

	default:
		return nil, fmt.Errorf("不支持的导出格式: %s", format)
	}

	// 获取文件大小
	if info, err := os.Stat(exportPath); err == nil {
		exportSize = info.Size()
	}

	return &ExportResult{
		Path:   exportPath,
		Size:   exportSize,
		Format: string(format),
	}, nil
}

// generateOVF 生成 OVF 描述文件
func (s *Service) generateOVF(vm *VM, includeISO bool) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Envelope xmlns="http://schemas.dmtf.org/ovf/envelope/1">
  <VirtualSystem ovf:id="%s">
    <Name>%s</Name>
    <Info>%s</Info>
    <VirtualHardwareSection>
      <Info>Virtual hardware requirements</Info>
      <Item>
        <rasd:Caption>%d virtual CPU(s)</rasd:Caption>
        <rasd:VirtualQuantity>%d</rasd:VirtualQuantity>
        <rasd:ResourceType>3</rasd:ResourceType>
      </Item>
      <Item>
        <rasd:Caption>%d MB of memory</rasd:Caption>
        <rasd:VirtualQuantity>%d</rasd:VirtualQuantity>
        <rasd:ResourceType>4</rasd:ResourceType>
      </Item>
    </VirtualHardwareSection>
  </VirtualSystem>
</Envelope>`, vm.ID, vm.Name, vm.Description, vm.CPU, vm.CPU, vm.Memory, vm.Memory)
}

// ImportVM 导入虚拟机
func (s *Service) ImportVM(req ImportVMRequest) (*VM, error) {
	// 检查文件是否存在
	if _, err := os.Stat(req.Path); err != nil {
		return nil, fmt.Errorf("导入文件不存在: %v", err)
	}

	ext := strings.ToLower(filepath.Ext(req.Path))
	
	// 生成新 VM
	vmID := uuid.New().String()
	vmName := req.Name
	if vmName == "" {
		vmName = strings.TrimSuffix(filepath.Base(req.Path), ext)
	}

	// 目标磁盘路径
	diskPath := filepath.Join(s.dataDir, "disks", vmID+".qcow2")

	switch ext {
	case ".ova":
		// 解压 OVA
		tmpDir := filepath.Join(s.dataDir, "tmp", vmID)
		os.MkdirAll(tmpDir, 0755)
		defer os.RemoveAll(tmpDir)

		cmd := exec.Command("tar", "-xvf", req.Path, "-C", tmpDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("解压 OVA 失败: %v, %s", err, string(output))
		}

		// 查找磁盘文件
		entries, _ := os.ReadDir(tmpDir)
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasSuffix(name, ".qcow2") || strings.HasSuffix(name, ".vmdk") {
				srcDisk := filepath.Join(tmpDir, name)
				// 转换为 qcow2
				cmd := exec.Command("qemu-img", "convert", "-O", "qcow2", srcDisk, diskPath)
				if output, err := cmd.CombinedOutput(); err != nil {
					return nil, fmt.Errorf("转换磁盘失败: %v, %s", err, string(output))
				}
				break
			}
		}

	case ".qcow2":
		// 直接复制
		if err := s.copyFile(req.Path, diskPath); err != nil {
			return nil, fmt.Errorf("复制磁盘失败: %v", err)
		}

	case ".vmdk", ".vdi", ".raw":
		// 转换格式
		cmd := exec.Command("qemu-img", "convert", "-O", "qcow2", req.Path, diskPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("转换磁盘失败: %v, %s", err, string(output))
		}

	default:
		return nil, fmt.Errorf("不支持的文件格式: %s", ext)
	}

	// 获取磁盘信息
	diskSize := s.getDiskSize(diskPath)

	// 创建 VM 记录
	vm := &VM{
		ID:          vmID,
		Name:        vmName,
		Description: req.Description,
		Status:      VMStatusStopped,
		CPU:         2,
		Memory:      2048,
		DiskSize:    diskSize,
		DiskPath:    diskPath,
		VNCPort:     s.allocateVNCPort(),
		SSHPort:     s.allocateSSHPort(),
		QMPSocket:   filepath.Join(s.dataDir, "sockets", vmID+".qmp"),
		Accelerator: s.detectAccelerator(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	s.mu.Lock()
	s.vms[vmID] = vm
	s.saveVMs()
	s.mu.Unlock()

	return vm, nil
}

// getDiskSize 获取磁盘虚拟大小 (GB)
func (s *Service) getDiskSize(path string) int64 {
	cmd := exec.Command("qemu-img", "info", "--output=json", path)
	output, err := cmd.Output()
	if err != nil {
		return 20 // 默认值
	}

	var info struct {
		VirtualSize int64 `json:"virtual-size"`
	}
	if json.Unmarshal(output, &info) == nil {
		return info.VirtualSize / (1024 * 1024 * 1024)
	}
	return 20
}

// copyFile 复制文件
func (s *Service) copyFile(src, dst string) error {
	cmd := exec.Command("cp", "-f", src, dst)
	return cmd.Run()
}

// ==================== P5: 实时快照 (QMP) ====================

// CreateLiveSnapshot 创建实时快照（VM 运行时）
func (s *Service) CreateLiveSnapshot(vmID string, req LiveSnapshotRequest) (*Snapshot, error) {
	s.mu.RLock()
	vm, exists := s.vms[vmID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("虚拟机不存在")
	}

	if vm.Status != VMStatusRunning {
		// 非运行状态使用 qemu-img
		return s.CreateSnapshot(vmID, CreateSnapshotRequest{Name: req.Name})
	}

	// 运行状态使用 QMP
	client := NewQMPClient(vm.QMPSocket)
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("连接 QMP 失败: %v", err)
	}
	defer client.Close()

	tag := fmt.Sprintf("snap-%d", time.Now().Unix())

	if req.IncludeRAM {
		// 包含内存状态的快照 (savevm)
		resp, err := client.Execute("human-monitor-command", map[string]interface{}{
			"command-line": fmt.Sprintf("savevm %s", tag),
		})
		if err != nil {
			return nil, fmt.Errorf("创建快照失败: %v", err)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("创建快照失败: %s", resp.Error.Desc)
		}
	} else {
		// 仅磁盘快照
		resp, err := client.Execute("blockdev-snapshot-sync", map[string]interface{}{
			"device":        "drive0",
			"snapshot-file": filepath.Join(s.dataDir, "snapshots", fmt.Sprintf("%s-%s.qcow2", vmID, tag)),
			"format":        "qcow2",
		})
		if err != nil {
			return nil, fmt.Errorf("创建快照失败: %v", err)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("创建快照失败: %s", resp.Error.Desc)
		}
	}

	snapshot := &Snapshot{
		ID:        tag,
		VMID:      vmID,
		Name:      req.Name,
		Tag:       tag,
		CreatedAt: time.Now(),
	}

	return snapshot, nil
}

// RevertLiveSnapshot 回滚到实时快照
func (s *Service) RevertLiveSnapshot(vmID, snapshotTag string) error {
	s.mu.RLock()
	vm, exists := s.vms[vmID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("虚拟机不存在")
	}

	if vm.Status == VMStatusRunning {
		// 运行时使用 QMP loadvm
		client := NewQMPClient(vm.QMPSocket)
		if err := client.Connect(); err != nil {
			return fmt.Errorf("连接 QMP 失败: %v", err)
		}
		defer client.Close()

		resp, err := client.Execute("human-monitor-command", map[string]interface{}{
			"command-line": fmt.Sprintf("loadvm %s", snapshotTag),
		})
		if err != nil {
			return fmt.Errorf("回滚快照失败: %v", err)
		}
		if resp.Error != nil {
			return fmt.Errorf("回滚快照失败: %s", resp.Error.Desc)
		}
	} else {
		// 停止状态使用 qemu-img
		cmd := exec.Command("qemu-img", "snapshot", "-a", snapshotTag, vm.DiskPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("回滚快照失败: %v, %s", err, string(output))
		}
	}

	return nil
}

// ==================== P6: SSH 终端集成 ====================

// GetVMSSHInfo 获取 VM SSH 连接信息
// 返回可用于 SSH 连接的地址和端口（基于端口转发规则）
func (s *Service) GetVMSSHInfo(vmID string) (*VMSSHSession, error) {
	s.mu.RLock()
	vm, exists := s.vms[vmID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("虚拟机不存在")
	}

	if vm.Status != VMStatusRunning {
		return nil, fmt.Errorf("虚拟机未运行，无法获取 SSH 信息")
	}

	// 查找 SSH 端口转发规则（guest 端口 22）
	var sshPort int
	for _, pf := range vm.PortForwards {
		if pf.GuestPort == 22 {
			sshPort = pf.HostPort
			break
		}
	}

	if sshPort == 0 {
		return nil, fmt.Errorf("未配置 SSH 端口转发，请添加 22 端口转发规则")
	}

	return &VMSSHSession{
		ID:        uuid.New().String(),
		VMID:      vmID,
		VMName:    vm.Name,
		Host:      "127.0.0.1",
		Port:      sshPort,
		CreatedAt: time.Now().Unix(),
	}, nil
}

// ==================== P6: 资源监控 ====================

// GetVMResourceStats 获取 VM 实时资源统计
func (s *Service) GetVMResourceStats(vmID string) (*VMResourceStats, error) {
	s.mu.RLock()
	vm, exists := s.vms[vmID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("虚拟机不存在")
	}

	if vm.Status != VMStatusRunning {
		return &VMResourceStats{
			VMID:      vmID,
			Timestamp: time.Now().Unix(),
			CPU:       VMCPUStats{VCPUs: vm.CPU},
			Memory:    VMMemoryStats{Total: uint64(vm.Memory)},
		}, nil
	}

	stats := &VMResourceStats{
		VMID:      vmID,
		Timestamp: time.Now().Unix(),
		CPU:       VMCPUStats{VCPUs: vm.CPU},
		Memory:    VMMemoryStats{Total: uint64(vm.Memory)},
	}

	// 连接 QMP 获取详细统计
	client := NewQMPClient(vm.QMPSocket)
	if err := client.Connect(); err != nil {
		s.logger.Warn("无法连接 QMP 获取资源统计", zap.Error(err))
		return stats, nil
	}
	defer client.Close()

	// 获取块设备统计
	s.getBlockStats(client, stats)

	// 获取网络统计
	s.getNetworkStats(client, stats)

	// 获取内存统计
	s.getMemoryStats(client, stats)

	// 获取 CPU 统计
	s.getCPUStats(client, stats)

	return stats, nil
}

// getBlockStats 获取块设备统计
func (s *Service) getBlockStats(client *QMPClient, stats *VMResourceStats) {
	resp, err := client.Execute("query-blockstats", nil)
	if err != nil {
		return
	}

	// 解析 blockstats 响应
	type BlockStatsDevice struct {
		Device string `json:"device"`
		Stats  struct {
			RdBytes      uint64 `json:"rd_bytes"`
			WrBytes      uint64 `json:"wr_bytes"`
			RdOperations uint64 `json:"rd_operations"`
			WrOperations uint64 `json:"wr_operations"`
		} `json:"stats"`
	}

	var devices []BlockStatsDevice
	data, _ := json.Marshal(resp.Return)
	json.Unmarshal(data, &devices)

	for _, dev := range devices {
		stats.Disks = append(stats.Disks, VMDiskStats{
			Device:     dev.Device,
			BytesRead:  dev.Stats.RdBytes,
			BytesWrite: dev.Stats.WrBytes,
			OpsRead:    dev.Stats.RdOperations,
			OpsWrite:   dev.Stats.WrOperations,
		})
	}
}

// getNetworkStats 获取网络统计
func (s *Service) getNetworkStats(client *QMPClient, stats *VMResourceStats) {
	resp, err := client.Execute("query-rx-filter", nil)
	if err != nil {
		return
	}

	// query-rx-filter 主要是过滤信息，我们使用 hmp 命令获取更详细的网络统计
	resp, err = client.Execute("human-monitor-command", map[string]interface{}{
		"command-line": "info network",
	})
	if err != nil {
		return
	}

	// 网络统计通过 QEMU monitor 获取
	var result string
	if err := json.Unmarshal(resp.Return, &result); err == nil {
		lines := strings.Split(result, "\n")
		var currentNet *VMNetworkStats
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "net") || strings.HasPrefix(line, "user") {
				if currentNet != nil {
					stats.Networks = append(stats.Networks, *currentNet)
				}
				currentNet = &VMNetworkStats{
					Device: strings.Split(line, ":")[0],
				}
			}
		}
		if currentNet != nil {
			stats.Networks = append(stats.Networks, *currentNet)
		}
	}
}

// getMemoryStats 获取内存统计
func (s *Service) getMemoryStats(client *QMPClient, stats *VMResourceStats) {
	// 尝试获取 balloon 统计
	resp, err := client.Execute("query-balloon", nil)
	if err == nil && resp.Return != nil {
		type BalloonInfo struct {
			Actual uint64 `json:"actual"`
		}
		var balloon BalloonInfo
		if json.Unmarshal(resp.Return, &balloon) == nil {
			stats.Memory.Balloon = balloon.Actual / (1024 * 1024) // 转换为 MB
			stats.Memory.Used = stats.Memory.Balloon
			stats.Memory.Available = stats.Memory.Total - stats.Memory.Used
			if stats.Memory.Total > 0 {
				stats.Memory.UsedPercent = float64(stats.Memory.Used) / float64(stats.Memory.Total) * 100
			}
		}
	}

	// 如果 balloon 不可用，使用估算值
	if stats.Memory.Used == 0 {
		// 假设使用了 70% 内存（无法精确获取时的估算）
		stats.Memory.UsedPercent = 70
		stats.Memory.Used = stats.Memory.Total * 70 / 100
		stats.Memory.Available = stats.Memory.Total - stats.Memory.Used
	}
}

// getCPUStats 获取 CPU 统计
func (s *Service) getCPUStats(client *QMPClient, stats *VMResourceStats) {
	resp, err := client.Execute("query-cpus-fast", nil)
	if err != nil {
		return
	}

	// 解析 CPU 信息
	type CPUInfo struct {
		CPUIndex  int    `json:"cpu-index"`
		ThreadID  int    `json:"thread-id"`
		Target    string `json:"target"`
	}

	var cpus []CPUInfo
	json.Unmarshal(resp.Return, &cpus)

	stats.CPU.VCPUs = len(cpus)

	// CPU 使用率需要通过采样计算，这里返回进程级别的估算
	// 实际生产环境应该使用 guest agent 或者持续采样
	resp, err = client.Execute("human-monitor-command", map[string]interface{}{
		"command-line": "info cpus",
	})
	if err == nil {
		var result string
		if json.Unmarshal(resp.Return, &result) == nil {
			// 解析 CPU halt 状态来估算使用率
			haltCount := 0
			lines := strings.Split(result, "\n")
			for _, line := range lines {
				if strings.Contains(line, "halted") {
					haltCount++
				}
			}
			if len(cpus) > 0 {
				// 估算：非 halted 的 CPU 视为活跃
				activeCount := len(cpus) - haltCount
				stats.CPU.UsagePercent = float64(activeCount) / float64(len(cpus)) * 100
			}
		}
	}
}

// GetAllVMsStats 获取所有运行中 VM 的资源统计
func (s *Service) GetAllVMsStats() ([]VMResourceStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var allStats []VMResourceStats
	for id, vm := range s.vms {
		if vm.Status == VMStatusRunning {
			if stats, err := s.GetVMResourceStats(id); err == nil {
				allStats = append(allStats, *stats)
			}
		}
	}

	return allStats, nil
}

