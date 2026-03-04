<script lang="ts">
  import Icon from "@iconify/svelte";
  import { t } from "./i18n";
  import { Button, Modal } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import { fileService, type FileInfo } from "$shared/services/files";
  import {
    vmService,
    type ISOFile,
    type CreateVMRequest,
    type PortForward,
    type USBDevice,
    type NetworkInterface,
    type StorageInfo,
  } from "./service";

  // ==================== Props ====================

  interface Props {
    open: boolean;
    isos: ISOFile[];
    onclose: () => void;
    oncreate: () => void;
  }

  let { open = $bindable(), isos, onclose, oncreate }: Props = $props();

  // ==================== 状态 ====================

  const STEPS = $derived([
    { id: 1, name: $t("vm.steps.general"), icon: "mdi:cog" },
    { id: 2, name: $t("vm.steps.system"), icon: "mdi:disc" },
    { id: 3, name: $t("vm.steps.disk"), icon: "mdi:harddisk" },
    { id: 4, name: $t("vm.steps.cpuMemory"), icon: "mdi:cpu-64-bit" },
    { id: 5, name: $t("vm.steps.network"), icon: "mdi:lan" },
    { id: 6, name: $t("vm.steps.confirm"), icon: "mdi:check-circle" },
  ]);

  let step = $state(1);
  let creating = $state(false);

  // 设备列表
  let usbDevices = $state<USBDevice[]>([]);
  let networkInterfaces = $state<NetworkInterface[]>([]);
  let selectedUSBDevices = $state<USBDevice[]>([]);
  let loadingDevices = $state(false);

  // 存储信息
  let storageInfo = $state<StorageInfo | null>(null);
  let loadingStorage = $state(false);

  // ISO 文件浏览器
  let showIsoBrowser = $state(false);
  let browserPath = $state("/");
  let browserFiles = $state<FileInfo[]>([]);
  let loadingBrowser = $state(false);

  // 表单数据
  let form = $state({
    // 常规
    name: "",
    description: "",
    auto_start: false,
    // 系统
    iso_path: "",
    os_type: "linux" as "linux" | "windows" | "other",
    bios: "seabios" as "seabios" | "ovmf",
    machine: "q35" as "q35" | "i440fx",
    // 磁盘
    disk_gb: 32,
    disk_format: "qcow2" as "qcow2" | "raw",
    disk_cache: "writeback" as "writeback" | "none" | "writethrough",
    io_thread: true,
    discard: true,
    // CPU/内存
    cpu_cores: 2,
    cpu_model: "host" as "host" | "host-passthrough" | "qemu64" | "max",
    memory_mb: 2048,
    ballooning: true,
    use_kvm: true,
    enable_huge: false,
    // 网络
    network_mode: "user" as "user" | "bridge" | "none",
    bridge_iface: "",
    nic_model: "virtio" as "virtio" | "e1000" | "rtl8139",
    port_forwards: [{ name: "SSH", protocol: "tcp", host_port: 0, guest_port: 22 }] as PortForward[],
    // USB
    usb_devices: [] as USBDevice[],
  });

  // ==================== 方法 ====================

  async function loadDevices() {
    if (usbDevices.length > 0 && networkInterfaces.length > 0) return;
    
    loadingDevices = true;
    try {
      const [usb, net] = await Promise.all([
        vmService.listUSBDevices(),
        vmService.listNetworkInterfaces(),
      ]);
      usbDevices = usb;
      networkInterfaces = net;
    } catch (e: any) {
      console.error("加载设备列表失败:", e);
    } finally {
      loadingDevices = false;
    }
  }

  async function loadStorage() {
    if (storageInfo) return;
    loadingStorage = true;
    try {
      storageInfo = await vmService.getStorageInfo();
    } catch (e: any) {
      console.error("加载存储信息失败:", e);
    } finally {
      loadingStorage = false;
    }
  }

  // ISO 文件浏览器功能
  async function openIsoBrowser() {
    showIsoBrowser = true;
    await loadBrowserFiles(browserPath);
  }

  async function loadBrowserFiles(path: string) {
    loadingBrowser = true;
    try {
      const response: any = await fileService.list(path, false);
      if (response.data?.content) {
        // 只显示目录和 ISO 文件
        browserFiles = response.data.content.filter(
          (f: FileInfo) => f.is_dir || f.name.toLowerCase().endsWith(".iso")
        );
        browserPath = path;
      }
    } catch (e: any) {
      console.error("加载文件列表失败:", e);
    } finally {
      loadingBrowser = false;
    }
  }

  function navigateBrowser(path: string) {
    loadBrowserFiles(path);
  }

  function selectIsoFile(file: FileInfo) {
    if (file.is_dir) {
      navigateBrowser(file.path);
    } else {
      form.iso_path = file.path;
      showIsoBrowser = false;
    }
  }

  function getBrowserBreadcrumbs() {
    const parts = browserPath.split("/").filter(Boolean);
    const crumbs: { label: string; path: string }[] = [{ label: "/", path: "/" }];
    let accPath = "";
    for (const part of parts) {
      accPath += "/" + part;
      crumbs.push({ label: part, path: accPath });
    }
    return crumbs;
  }

  function reset() {
    step = 1;
    creating = false;
    selectedUSBDevices = [];
    form = {
      name: "",
      description: "",
      auto_start: false,
      iso_path: "",
      os_type: "linux",
      bios: "seabios",
      machine: "q35",
      disk_gb: 32,
      disk_format: "qcow2",
      disk_cache: "writeback",
      io_thread: true,
      discard: true,
      cpu_cores: 2,
      cpu_model: "host",
      memory_mb: 2048,
      ballooning: true,
      use_kvm: true,
      enable_huge: false,
      network_mode: "user",
      bridge_iface: "",
      nic_model: "virtio",
      port_forwards: [{ name: "SSH", protocol: "tcp", host_port: 0, guest_port: 22 }],
      usb_devices: [],
    };
  }

  function validateStep(s: number): string | null {
    switch (s) {
      case 1:
        if (!form.name.trim()) return $t("vm.validation.enterVMName");
        break;
      case 2:
        if (!form.iso_path) return $t("vm.validation.selectISO");
        break;
      case 3:
        if (form.disk_gb < 1) return $t("vm.validation.diskMinSize");
        break;
      case 4:
        if (form.cpu_cores < 1) return $t("vm.validation.cpuMinCores");
        if (form.memory_mb < 256) return $t("vm.validation.memoryMin");
        break;
      case 5:
        if (form.network_mode === "bridge" && !form.bridge_iface) {
          return $t("vm.validation.selectBridge");
        }
        break;
    }
    return null;
  }

  function nextStep() {
    const error = validateStep(step);
    if (error) {
      showToast(error, "error");
      return;
    }
    
    // 进入网络步骤前加载设备
    if (step === 4) {
      loadDevices();
    }
    // 进入磁盘步骤前加载存储信息
    if (step === 2) {
      loadStorage();
    }
    
    step++;
  }

  function prevStep() {
    step--;
  }

  function goToStep(s: number) {
    // 只能跳转到已完成的步骤
    if (s < step) {
      step = s;
    }
  }

  function toggleUSBDevice(device: USBDevice) {
    const key = `${device.vendor_id}:${device.product_id}`;
    const exists = selectedUSBDevices.find(
      d => `${d.vendor_id}:${d.product_id}` === key
    );
    if (exists) {
      selectedUSBDevices = selectedUSBDevices.filter(
        d => `${d.vendor_id}:${d.product_id}` !== key
      );
    } else {
      selectedUSBDevices = [...selectedUSBDevices, device];
    }
    form.usb_devices = selectedUSBDevices;
  }

  function isUSBSelected(device: USBDevice): boolean {
    return selectedUSBDevices.some(
      d => d.vendor_id === device.vendor_id && d.product_id === device.product_id
    );
  }

  function addPortForward() {
    form.port_forwards = [
      ...form.port_forwards,
      { name: "", protocol: "tcp", host_port: 0, guest_port: 80 },
    ];
  }

  function removePortForward(index: number) {
    form.port_forwards = form.port_forwards.filter((_, i) => i !== index);
  }

  async function create() {
    const error = validateStep(step);
    if (error) {
      showToast(error, "error");
      return;
    }

    creating = true;
    try {
      const req: CreateVMRequest = {
        name: form.name,
        description: form.description,
        memory_mb: form.memory_mb,
        cpu_cores: form.cpu_cores,
        disk_gb: form.disk_gb,
        iso_path: form.iso_path,
        os_type: form.os_type,
        use_kvm: form.use_kvm,
        port_forwards: form.network_mode === "user" ? form.port_forwards : [],
        usb_devices: form.usb_devices,
        network_mode: form.network_mode,
        bridge_iface: form.bridge_iface,
        cpu_model: form.cpu_model,
        enable_huge: form.enable_huge,
        io_thread: form.io_thread,
      };
      
      await vmService.createVM(req);
      showToast($t("vm.messages.createSuccess"), "success");
      oncreate();
      open = false;
      reset();
    } catch (e: any) {
      showToast($t("vm.messages.createFailed") + " " + e.message, "error");
    } finally {
      creating = false;
    }
  }

  function handleClose() {
    reset();
    onclose();
  }

  function formatBytes(bytes: number): string {
    if (bytes >= 1073741824) return `${(bytes / 1073741824).toFixed(1)} GB`;
    if (bytes >= 1048576) return `${(bytes / 1048576).toFixed(1)} MB`;
    return `${bytes} B`;
  }

  function formatMemory(mb: number): string {
    return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${mb} MB`;
  }

  function getOSIcon(type: string): string {
    switch (type) {
      case "windows": return "logos:microsoft-windows-icon";
      case "linux": return "logos:linux-tux";
      default: return "mdi:desktop-classic";
    }
  }
</script>

<Modal bind:open title={$t("vm.createVM")} size="lg" onclose={handleClose}>
  <div class="wizard">
    <!-- 步骤导航 -->
    <div class="steps-nav">
      {#each STEPS as s (s.id)}
        <button
          type="button"
          class="step-tab"
          class:active={step === s.id}
          class:done={step > s.id}
          class:disabled={s.id > step}
          onclick={() => goToStep(s.id)}
          disabled={s.id > step}
        >
          <span class="step-num">{s.id}</span>
          <Icon icon={s.icon} width="16" />
          <span class="step-name">{s.name}</span>
        </button>
      {/each}
    </div>

    <!-- 步骤内容 -->
    <div class="step-content">
      {#if step === 1}
        <!-- 步骤 1: 常规设置 -->
        <div class="step-panel">
          <div class="section-header">
            <Icon icon="mdi:cog" width="20" />
            <span>{$t("vm.general.title")}</span>
          </div>

          <div class="form-group">
            <label for="vm-name">{$t("vm.general.vmName")} <span class="required">*</span></label>
            <input
              id="vm-name"
              type="text"
              bind:value={form.name}
              placeholder={$t("vm.general.namePlaceholder")}
              required
            />
          </div>

          <div class="form-group">
            <label for="vm-desc">{$t("vm.general.description")}</label>
            <textarea
              id="vm-desc"
              bind:value={form.description}
              placeholder={$t("vm.general.descPlaceholder")}
              rows="3"
            ></textarea>
          </div>

          <div class="form-group">
            <label class="checkbox-label">
              <input type="checkbox" bind:checked={form.auto_start} />
              <Icon icon="mdi:power" width="18" />
              {$t("vm.general.autoStart")}
            </label>
            <p class="hint">{$t("vm.general.autoStartHint")}</p>
          </div>
        </div>

      {:else if step === 2}
        <!-- 步骤 2: 系统 -->
        <div class="step-panel">
          <div class="section-header">
            <Icon icon="mdi:disc" width="20" />
            <span>{$t("vm.system.title")}</span>
          </div>

          <div class="form-group">
            <label for="vm-iso">{$t("vm.system.isoImage")} <span class="required">*</span></label>
            <div class="iso-input-row">
              <input
                type="text"
                id="vm-iso"
                bind:value={form.iso_path}
                placeholder={$t("vm.system.isoPlaceholder")}
                readonly
                class="iso-input"
              />
              <Button variant="outline" onclick={openIsoBrowser}>
                <Icon icon="mdi:folder-open" width="16" />
                {$t("vm.system.browse")}
              </Button>
            </div>
          </div>

          <div class="form-row">
            <div class="form-group">
              <label for="os-type">{$t("vm.system.guestType")}</label>
              <select id="os-type" bind:value={form.os_type}>
                <option value="linux">{$t("vm.system.linux")}</option>
                <option value="windows">{$t("vm.system.windows")}</option>
                <option value="other">{$t("vm.system.other")}</option>
              </select>
            </div>
          </div>

          <details class="advanced-section">
            <summary>
              <Icon icon="mdi:cog" width="16" />
              高级选项
            </summary>
            <div class="advanced-content">
              <div class="form-row">
                <div class="form-group">
                  <label for="bios">{$t("vm.system.bios")}</label>
                  <select id="bios" bind:value={form.bios}>
                    <option value="seabios">SeaBIOS (传统)</option>
                    <option value="ovmf">OVMF (UEFI)</option>
                  </select>
                  <p class="hint">{$t("vm.system.uefiRequired")}</p>
                </div>
                <div class="form-group">
                  <label for="machine">{$t("vm.system.machineType")}</label>
                  <select id="machine" bind:value={form.machine}>
                    <option value="q35">q35 (推荐)</option>
                    <option value="i440fx">i440fx (传统)</option>
                  </select>
                </div>
              </div>
            </div>
          </details>
        </div>

      {:else if step === 3}
        <!-- 步骤 3: 磁盘 -->
        <div class="step-panel">
          <div class="section-header">
            <Icon icon="mdi:harddisk" width="20" />
            <span>{$t("vm.disk.title")}</span>
          </div>

          {#if storageInfo}
            <div class="storage-info">
              <div class="storage-bar">
                <div
                  class="storage-used"
                  style="width: {(storageInfo.used_space / storageInfo.total_space) * 100}%"
                ></div>
              </div>
              <div class="storage-stats">
                <span>{$t("vm.disk.available")} {formatBytes(storageInfo.free_space)}</span>
                <span>{$t("vm.disk.used")} {formatBytes(storageInfo.used_space)}</span>
                <span>{$t("vm.disk.total")} {formatBytes(storageInfo.total_space)}</span>
              </div>
            </div>
          {/if}

          <div class="form-group">
            <label for="disk-size">{$t("vm.disk.diskSize")} <span class="required">*</span></label>
            <div class="input-with-unit">
              <input
                id="disk-size"
                type="number"
                bind:value={form.disk_gb}
                min="1"
                max="2000"
              />
              <span class="unit">GB</span>
            </div>
            <div class="presets">
              <button type="button" class:active={form.disk_gb === 20} onclick={() => form.disk_gb = 20}>20 GB</button>
              <button type="button" class:active={form.disk_gb === 32} onclick={() => form.disk_gb = 32}>32 GB</button>
              <button type="button" class:active={form.disk_gb === 50} onclick={() => form.disk_gb = 50}>50 GB</button>
              <button type="button" class:active={form.disk_gb === 100} onclick={() => form.disk_gb = 100}>100 GB</button>
              <button type="button" class:active={form.disk_gb === 200} onclick={() => form.disk_gb = 200}>200 GB</button>
            </div>
          </div>

          <details class="advanced-section">
            <summary>
              <Icon icon="mdi:cog" width="16" />
              高级选项
            </summary>
            <div class="advanced-content">
              <div class="form-row">
                <div class="form-group">
                  <label for="disk-format">{$t("vm.disk.diskFormat")}</label>
                  <select id="disk-format" bind:value={form.disk_format}>
                    <option value="qcow2">qcow2 (推荐)</option>
                    <option value="raw">raw</option>
                  </select>
                  <p class="hint">{$t("vm.disk.qcow2Hint")}</p>
                </div>
                <div class="form-group">
                  <label for="disk-cache">{$t("vm.disk.cacheMode")}</label>
                  <select id="disk-cache" bind:value={form.disk_cache}>
                    <option value="writeback">writeback (推荐)</option>
                    <option value="none">none (最佳性能)</option>
                    <option value="writethrough">writethrough (最安全)</option>
                  </select>
                </div>
              </div>

              <div class="form-group">
                <label class="checkbox-label">
                  <input type="checkbox" bind:checked={form.io_thread} />
                  {$t("vm.disk.ioThread")}
                </label>
                <p class="hint">{$t("vm.disk.ioThreadHint")}</p>
              </div>

              <div class="form-group">
                <label class="checkbox-label">
                  <input type="checkbox" bind:checked={form.discard} />
                  {$t("vm.disk.discard")}
                </label>
                <p class="hint">{$t("vm.disk.discardHint")}</p>
              </div>
            </div>
          </details>
        </div>

      {:else if step === 4}
        <!-- 步骤 4: CPU / 内存 -->
        <div class="step-panel">
          <div class="section-header">
            <Icon icon="mdi:cpu-64-bit" width="20" />
            <span>{$t("vm.cpuMemory.title")}</span>
          </div>

          <div class="resource-config">
            <div class="resource-item">
              <label>
                <Icon icon="mdi:cpu-64-bit" width="18" />
                {$t("vm.cpuMemory.cpuCores")}
              </label>
              <div class="slider-group">
                <input
                  type="range"
                  bind:value={form.cpu_cores}
                  min="1"
                  max="16"
                  step="1"
                />
                <span class="value">{form.cpu_cores} {$t("vm.cpuMemory.cores")}</span>
              </div>
              <div class="presets">
                <button type="button" class:active={form.cpu_cores === 1} onclick={() => form.cpu_cores = 1}>1</button>
                <button type="button" class:active={form.cpu_cores === 2} onclick={() => form.cpu_cores = 2}>2</button>
                <button type="button" class:active={form.cpu_cores === 4} onclick={() => form.cpu_cores = 4}>4</button>
                <button type="button" class:active={form.cpu_cores === 8} onclick={() => form.cpu_cores = 8}>8</button>
              </div>
            </div>

            <div class="resource-item">
              <label>
                <Icon icon="mdi:memory" width="18" />
                {$t("vm.cpuMemory.memory")}
              </label>
              <div class="slider-group">
                <input
                  type="range"
                  bind:value={form.memory_mb}
                  min="512"
                  max="32768"
                  step="512"
                />
                <span class="value">{formatMemory(form.memory_mb)}</span>
              </div>
              <div class="presets">
                <button type="button" class:active={form.memory_mb === 1024} onclick={() => form.memory_mb = 1024}>1 GB</button>
                <button type="button" class:active={form.memory_mb === 2048} onclick={() => form.memory_mb = 2048}>2 GB</button>
                <button type="button" class:active={form.memory_mb === 4096} onclick={() => form.memory_mb = 4096}>4 GB</button>
                <button type="button" class:active={form.memory_mb === 8192} onclick={() => form.memory_mb = 8192}>8 GB</button>
              </div>
            </div>
          </div>

          <div class="form-group">
            <label class="checkbox-label">
              <input type="checkbox" bind:checked={form.use_kvm} />
              <Icon icon="mdi:chip" width="18" />
              {$t("vm.cpuMemory.enableKVM")}
            </label>
            <p class="hint">{$t("vm.cpuMemory.kvmHint")}</p>
          </div>

          <details class="advanced-section">
            <summary>
              <Icon icon="mdi:rocket-launch" width="16" />
              性能优化
            </summary>
            <div class="advanced-content">
              <div class="form-group">
                <label for="cpu-model">{$t("vm.cpuMemory.cpuMode")}</label>
                <select id="cpu-model" bind:value={form.cpu_model}>
                  <option value="host">host (推荐)</option>
                  <option value="host-passthrough">host-passthrough (最佳性能)</option>
                  <option value="qemu64">qemu64 (最佳兼容)</option>
                  <option value="max">max (最多特性)</option>
                </select>
              </div>

              <div class="form-group">
                <label class="checkbox-label">
                  <input type="checkbox" bind:checked={form.ballooning} />
                  {$t("vm.cpuMemory.ballooning")}
                </label>
                <p class="hint">{$t("vm.cpuMemory.ballooningHint")}</p>
              </div>

              <div class="form-group">
                <label class="checkbox-label">
                  <input type="checkbox" bind:checked={form.enable_huge} />
                  {$t("vm.cpuMemory.hugepages")}
                </label>
                <p class="hint">{$t("vm.cpuMemory.hugepagesHint")}</p>
              </div>
            </div>
          </details>
        </div>

      {:else if step === 5}
        <!-- 步骤 5: 网络 -->
        <div class="step-panel">
          <div class="section-header">
            <Icon icon="mdi:lan" width="20" />
            <span>{$t("vm.network.title")}</span>
          </div>

          <div class="network-modes">
            <label class="mode-card" class:selected={form.network_mode === 'user'}>
              <input type="radio" name="network_mode" value="user" bind:group={form.network_mode} />
              <Icon icon="mdi:home-network" width="28" />
              <div class="mode-info">
                <span class="mode-name">{$t("vm.network.userMode")}</span>
                <span class="mode-desc">{$t("vm.network.userModeDesc")}</span>
              </div>
            </label>
            <label class="mode-card" class:selected={form.network_mode === 'bridge'}>
              <input type="radio" name="network_mode" value="bridge" bind:group={form.network_mode} />
              <Icon icon="mdi:bridge" width="28" />
              <div class="mode-info">
                <span class="mode-name">{$t("vm.network.bridgeMode")}</span>
                <span class="mode-desc">{$t("vm.network.bridgeModeDesc")}</span>
              </div>
            </label>
            <label class="mode-card" class:selected={form.network_mode === 'none'}>
              <input type="radio" name="network_mode" value="none" bind:group={form.network_mode} />
              <Icon icon="mdi:lan-disconnect" width="28" />
              <div class="mode-info">
                <span class="mode-name">{$t("vm.network.noNetwork")}</span>
                <span class="mode-desc">{$t("vm.network.noNetworkDesc")}</span>
              </div>
            </label>
          </div>

          {#if form.network_mode === 'bridge'}
            <div class="form-group">
              <label for="bridge-iface">{$t("vm.network.selectBridge")} <span class="required">*</span></label>
              <select id="bridge-iface" bind:value={form.bridge_iface}>
                <option value="">{$t("vm.network.selectNetworkInterface")}</option>
                {#each networkInterfaces.filter(n => n.type === 'bridge' || n.type === 'ethernet') as iface (iface.name)}
                  <option value={iface.name}>
                    {iface.name} ({iface.type}) {iface.address ? `- ${iface.address}` : ''}
                  </option>
                {/each}
              </select>
            </div>
          {/if}

          {#if form.network_mode === 'user'}
            <div class="port-forwards-section">
              <h4>{$t("vm.network.portForward")}</h4>
              <p class="hint">{$t("vm.network.portForwardHint")}</p>

              <div class="port-forwards">
                {#each form.port_forwards as pf, i (i)}
                  <div class="port-forward-row">
                    <input
                      type="text"
                      placeholder={$t("vm.network.name")}
                      bind:value={pf.name}
                      class="pf-name"
                    />
                    <select bind:value={pf.protocol} class="pf-proto">
                      <option value="tcp">TCP</option>
                      <option value="udp">UDP</option>
                    </select>
                    <input
                      type="number"
                      placeholder={$t("vm.network.hostPort")}
                      bind:value={pf.host_port}
                      min="0"
                      max="65535"
                      class="pf-port"
                    />
                    <span class="arrow">→</span>
                    <input
                      type="number"
                      placeholder={$t("vm.network.guestPort")}
                      bind:value={pf.guest_port}
                      min="1"
                      max="65535"
                      class="pf-port"
                    />
                    <button type="button" class="remove-btn" onclick={() => removePortForward(i)}>
                      <Icon icon="mdi:close" width="16" />
                    </button>
                  </div>
                {/each}
                <button type="button" class="add-btn" onclick={addPortForward}>
                  <Icon icon="mdi:plus" width="16" /> {$t("vm.network.addRule")}
                </button>
              </div>
            </div>
          {/if}

          <details class="advanced-section">
            <summary>
              <Icon icon="mdi:cog" width="16" />
              高级选项
            </summary>
            <div class="advanced-content">
              <div class="form-group">
                <label for="nic-model">{$t("vm.network.nicModel")}</label>
                <select id="nic-model" bind:value={form.nic_model}>
                  <option value="virtio">virtio (推荐)</option>
                  <option value="e1000">e1000 (Intel)</option>
                  <option value="rtl8139">rtl8139 (Realtek)</option>
                </select>
              </div>

              <!-- USB 设备直通 -->
              <h4>{$t("vm.network.usbPassthrough")}</h4>
              {#if loadingDevices}
                <div class="loading">{$t("vm.network.loadingDevices")}</div>
              {:else if usbDevices.length === 0}
                <p class="hint">{$t("vm.network.noUSBDevices")}</p>
              {:else}
                <div class="usb-device-list">
                  {#each usbDevices as device (device.vendor_id + ':' + device.product_id)}
                    <label class="usb-device-item" class:selected={isUSBSelected(device)}>
                      <input 
                        type="checkbox"
                        checked={isUSBSelected(device)}
                        onchange={() => toggleUSBDevice(device)}
                      />
                      <Icon icon="mdi:usb" width="18" />
                      <div class="device-info">
                        <span class="device-name">{device.name || $t("vm.network.usbDevice")}</span>
                        <span class="device-id">{device.vendor_id}:{device.product_id}</span>
                      </div>
                    </label>
                  {/each}
                </div>
              {/if}
            </div>
          </details>
        </div>

      {:else if step === 6}
        <!-- 步骤 6: 确认 -->
        <div class="step-panel">
          <div class="section-header">
            <Icon icon="mdi:check-circle" width="20" />
            <span>{$t("vm.summary.title")}</span>
          </div>

          <div class="summary-card">
            <div class="summary-header">
              <Icon icon={getOSIcon(form.os_type)} width="32" />
              <div>
                <h3>{form.name}</h3>
                {#if form.description}
                  <p class="desc">{form.description}</p>
                {/if}
              </div>
            </div>

            <div class="summary-grid">
              <div class="summary-section">
                <h4><Icon icon="mdi:disc" width="16" /> 系统</h4>
                <div class="summary-row">
                  <span class="label">ISO 镜像</span>
                  <span class="value">{form.iso_path ? form.iso_path.split('/').pop() : '-'}</span>
                </div>
                <div class="summary-row">
                  <span class="label">{$t("vm.summary.guestType")}</span>
                  <span class="value">{form.os_type === 'linux' ? $t("vm.system.linux") : form.os_type === 'windows' ? $t("vm.system.windows") : $t("vm.system.other")}</span>
                </div>
                <div class="summary-row">
                  <span class="label">BIOS</span>
                  <span class="value">{form.bios === 'seabios' ? 'SeaBIOS' : 'OVMF (UEFI)'}</span>
                </div>
              </div>

              <div class="summary-section">
                <h4><Icon icon="mdi:harddisk" width="16" /> {$t("vm.steps.disk")}</h4>
                <div class="summary-row">
                  <span class="label">{$t("vm.summary.size")}</span>
                  <span class="value">{form.disk_gb} GB</span>
                </div>
                <div class="summary-row">
                  <span class="label">{$t("vm.summary.format")}</span>
                  <span class="value">{form.disk_format}</span>
                </div>
              </div>

              <div class="summary-section">
                <h4><Icon icon="mdi:cpu-64-bit" width="16" /> {$t("vm.steps.cpuMemory")}</h4>
                <div class="summary-row">
                  <span class="label">{$t("vm.summary.cpu")}</span>
                  <span class="value">{form.cpu_cores} {$t("vm.cpuMemory.cores")} ({form.cpu_model})</span>
                </div>
                <div class="summary-row">
                  <span class="label">{$t("vm.summary.memory")}</span>
                  <span class="value">{formatMemory(form.memory_mb)}</span>
                </div>
                <div class="summary-row">
                  <span class="label">{$t("vm.summary.kvmAccel")}</span>
                  <span class="value">{form.use_kvm ? $t("vm.summary.yes") : $t("vm.summary.no")}</span>
                </div>
              </div>

              <div class="summary-section">
                <h4><Icon icon="mdi:lan" width="16" /> {$t("vm.steps.network")}</h4>
                <div class="summary-row">
                  <span class="label">{$t("vm.summary.mode")}</span>
                  <span class="value">
                    {form.network_mode === 'user' ? $t("vm.summary.nat") : form.network_mode === 'bridge' ? $t("vm.summary.bridge") : $t("vm.summary.noNetwork")}
                  </span>
                </div>
                {#if form.network_mode === 'user' && form.port_forwards.length > 0}
                  <div class="summary-row">
                    <span class="label">{$t("vm.summary.portForward")}</span>
                    <span class="value">
                      {form.port_forwards.map(p => `${p.name || p.guest_port}:${p.host_port || 'auto'}→${p.guest_port}`).join(', ')}
                    </span>
                  </div>
                {/if}
                {#if form.network_mode === 'bridge'}
                  <div class="summary-row">
                    <span class="label">{$t("vm.summary.bridgeInterface")}</span>
                    <span class="value">{form.bridge_iface}</span>
                  </div>
                {/if}
              </div>
            </div>

            <div class="summary-footer">
              <label class="checkbox-label">
                <input type="checkbox" bind:checked={form.auto_start} />
                <Icon icon="mdi:rocket-launch" width="18" />
                {$t("vm.summary.launchAfterCreate")}
              </label>
            </div>
          </div>
        </div>
      {/if}
    </div>

    <!-- 底部按钮 -->
    <div class="wizard-footer">
      {#if step > 1}
        <Button variant="ghost" onclick={prevStep}>
          <Icon icon="mdi:arrow-left" width="16" />
          {$t("vm.actions.prevStep")}
        </Button>
      {:else}
        <div></div>
      {/if}

      <div class="footer-right">
        {#if step < 6}
          <Button variant="primary" onclick={nextStep}>
            {$t("vm.actions.nextStep")}
            <Icon icon="mdi:arrow-right" width="16" />
          </Button>
        {:else}
          <Button variant="success" onclick={create} disabled={creating}>
            {#if creating}
              <Icon icon="mdi:loading" width="16" class="spin" />
              {$t("vm.actions.creating")}
            {:else}
              <Icon icon="mdi:rocket-launch" width="16" />
              {$t("vm.actions.create")}
            {/if}
          </Button>
        {/if}
      </div>
    </div>
  </div>
</Modal>

<!-- ISO 文件浏览器 -->
<Modal bind:open={showIsoBrowser} title={$t("vm.isoBrowser.title")} width="600px">
  <div class="iso-browser">
    <!-- 面包屑导航 -->
    <div class="browser-nav">
      <button class="nav-btn" onclick={() => navigateBrowser("/")} title={$t("vm.isoBrowser.rootDir")}>
        <Icon icon="mdi:home" width="18" />
      </button>
      <button
        class="nav-btn"
        onclick={() => {
          const parent = browserPath.substring(0, browserPath.lastIndexOf("/")) || "/";
          navigateBrowser(parent);
        }}
        disabled={browserPath === "/"}
        title={$t("vm.isoBrowser.goUp")}
      >
        <Icon icon="mdi:arrow-up" width="18" />
      </button>
      <div class="breadcrumbs">
        {#each getBrowserBreadcrumbs() as crumb, i}
          {#if i > 0}<span class="sep">/</span>{/if}
          <button class="crumb" onclick={() => navigateBrowser(crumb.path)}>
            {crumb.label}
          </button>
        {/each}
      </div>
    </div>

    <!-- 文件列表 -->
    <div class="browser-list">
      {#if loadingBrowser}
        <div class="browser-loading">
          <Icon icon="mdi:loading" width="24" class="spin" />
          <span>{$t("vm.isoBrowser.loading")}</span>
        </div>
      {:else if browserFiles.length === 0}
        <div class="browser-empty">
          <Icon icon="mdi:folder-open-outline" width="48" />
          <span>{$t("vm.isoBrowser.noISOFiles")}</span>
        </div>
      {:else}
        {#each browserFiles as file (file.path)}
          <button
            class="browser-item"
            class:is-dir={file.is_dir}
            ondblclick={() => selectIsoFile(file)}
            onclick={() => !file.is_dir && (form.iso_path = file.path)}
          >
            <Icon
              icon={file.is_dir ? "mdi:folder" : "mdi:disc"}
              width="20"
              class={file.is_dir ? "folder-icon" : "iso-icon"}
            />
            <span class="file-name">{file.name}</span>
            {#if !file.is_dir}
              <span class="file-size">{formatBytes(file.size)}</span>
            {/if}
          </button>
        {/each}
      {/if}
    </div>

    <!-- 底部操作栏 -->
    <div class="browser-footer">
      <div class="selected-path">
        {#if form.iso_path}
          <Icon icon="mdi:check-circle" width="16" class="selected-icon" />
          <span>{form.iso_path}</span>
        {:else}
          <span class="hint">{$t("vm.isoBrowser.selectHint")}</span>
        {/if}
      </div>
      <div class="browser-actions">
        <Button variant="ghost" onclick={() => (showIsoBrowser = false)}>{$t("common.cancel")}</Button>
        <Button variant="primary" onclick={() => (showIsoBrowser = false)} disabled={!form.iso_path}>
          {$t("common.confirm")}
        </Button>
      </div>
    </div>
  </div>
</Modal>

<style>
  .wizard {
    display: flex;
    flex-direction: column;
    min-height: 500px;
  }

  /* 步骤导航 */
  .steps-nav {
    display: flex;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
    overflow-x: auto;
  }

  .step-tab {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 12px 16px;
    border: none;
    background: none;
    font-size: 13px;
    color: var(--text-muted, #999);
    cursor: pointer;
    white-space: nowrap;
    border-bottom: 2px solid transparent;
    transition: all 0.2s;
  }

  .step-tab:hover:not(:disabled) {
    color: var(--text-primary, #333);
    background: var(--bg-hover, #f5f5f5);
  }

  .step-tab.active {
    color: var(--color-primary, #4a90d9);
    border-bottom-color: var(--color-primary, #4a90d9);
  }

  .step-tab.done {
    color: var(--color-success, #4caf50);
  }

  .step-tab:disabled {
    cursor: not-allowed;
  }

  .step-num {
    width: 20px;
    height: 20px;
    border-radius: 50%;
    background: var(--bg-tertiary, #e0e0e0);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 11px;
    font-weight: 600;
  }

  .step-tab.active .step-num {
    background: var(--color-primary, #4a90d9);
    color: white;
  }

  .step-tab.done .step-num {
    background: var(--color-success, #4caf50);
    color: white;
  }

  /* 步骤内容 */
  .step-content {
    flex: 1;
    padding: 20px;
    overflow-y: auto;
  }

  .step-panel {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .section-header {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary, #333);
    padding-bottom: 8px;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
  }

  /* 表单样式 */
  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .form-group label {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-secondary, #666);
  }

  .form-group input[type="text"],
  .form-group input[type="number"],
  .form-group select,
  .form-group textarea {
    padding: 10px 12px;
    border: 1px solid var(--border-color, #ddd);
    border-radius: 6px;
    font-size: 14px;
    transition: border-color 0.2s;
  }

  .form-group input:focus,
  .form-group select:focus,
  .form-group textarea:focus {
    outline: none;
    border-color: var(--color-primary, #4a90d9);
  }

  .required {
    color: var(--color-error, #f44336);
  }

  .hint {
    font-size: 12px;
    color: var(--text-muted, #999);
    margin: 0;
  }

  .form-row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 16px;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-weight: normal !important;
  }

  .checkbox-label input {
    width: 16px;
    height: 16px;
  }

  /* 输入框带单位 */
  .input-with-unit {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .input-with-unit input {
    flex: 1;
    max-width: 120px;
  }

  .unit {
    font-size: 14px;
    color: var(--text-muted, #999);
  }

  /* 预设按钮 */
  .presets {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    margin-top: 4px;
  }

  .presets button {
    padding: 4px 12px;
    border: 1px solid var(--border-color, #ddd);
    border-radius: 4px;
    background: var(--bg-secondary, #f5f5f5);
    font-size: 12px;
    cursor: pointer;
    transition: all 0.2s;
  }

  .presets button:hover {
    background: var(--bg-hover, #e8e8e8);
  }

  .presets button.active {
    background: var(--color-primary, #4a90d9);
    color: white;
    border-color: var(--color-primary, #4a90d9);
  }

  /* 高级选项 */
  .advanced-section {
    margin-top: 8px;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
  }

  .advanced-section summary {
    padding: 10px 12px;
    cursor: pointer;
    font-size: 13px;
    color: var(--text-secondary, #666);
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .advanced-section summary:hover {
    background: var(--bg-hover, #f5f5f5);
  }

  .advanced-content {
    padding: 16px;
    border-top: 1px solid var(--border-color, #e0e0e0);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  /* 存储信息 */
  .storage-info {
    background: var(--bg-tertiary, #f5f5f5);
    padding: 12px;
    border-radius: 6px;
  }

  .storage-bar {
    height: 8px;
    background: var(--bg-secondary, #e0e0e0);
    border-radius: 4px;
    overflow: hidden;
  }

  .storage-used {
    height: 100%;
    background: var(--color-primary, #4a90d9);
    transition: width 0.3s;
  }

  .storage-stats {
    display: flex;
    justify-content: space-between;
    margin-top: 8px;
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  /* 资源配置 */
  .resource-config {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .resource-item {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .resource-item > label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 500;
  }

  .slider-group {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .slider-group input[type="range"] {
    flex: 1;
    height: 6px;
    -webkit-appearance: none;
    background: var(--bg-tertiary, #e0e0e0);
    border-radius: 3px;
  }

  .slider-group input[type="range"]::-webkit-slider-thumb {
    -webkit-appearance: none;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--color-primary, #4a90d9);
    cursor: pointer;
  }

  .slider-group .value {
    min-width: 70px;
    text-align: right;
    font-weight: 600;
    color: var(--color-primary, #4a90d9);
  }

  /* 网络模式 */
  .network-modes {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
  }

  .mode-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 16px;
    border: 2px solid var(--border-color, #ddd);
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.2s;
    text-align: center;
  }

  .mode-card:hover {
    border-color: var(--color-primary, #4a90d9);
  }

  .mode-card.selected {
    border-color: var(--color-primary, #4a90d9);
    background: var(--color-primary-light, #e3f2fd);
  }

  .mode-card input {
    display: none;
  }

  .mode-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .mode-name {
    font-weight: 600;
    font-size: 14px;
  }

  .mode-desc {
    font-size: 11px;
    color: var(--text-muted, #999);
  }

  /* 端口转发 */
  .port-forwards-section {
    margin-top: 16px;
  }

  .port-forwards-section h4 {
    margin: 0 0 8px 0;
    font-size: 14px;
  }

  .port-forwards {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .port-forward-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .pf-name {
    width: 80px;
  }

  .pf-proto {
    width: 70px;
  }

  .pf-port {
    width: 90px;
  }

  .arrow {
    color: var(--text-muted, #999);
  }

  .remove-btn {
    padding: 6px;
    border: none;
    background: none;
    color: var(--color-error, #f44336);
    cursor: pointer;
    border-radius: 4px;
  }

  .remove-btn:hover {
    background: var(--bg-hover, #f5f5f5);
  }

  .add-btn {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 8px 12px;
    border: 1px dashed var(--border-color, #ddd);
    border-radius: 4px;
    background: none;
    font-size: 13px;
    color: var(--text-secondary, #666);
    cursor: pointer;
  }

  .add-btn:hover {
    border-color: var(--color-primary, #4a90d9);
    color: var(--color-primary, #4a90d9);
  }

  /* USB 设备 */
  .usb-device-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 200px;
    overflow-y: auto;
  }

  .usb-device-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border: 1px solid var(--border-color, #ddd);
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s;
  }

  .usb-device-item:hover {
    background: var(--bg-hover, #f5f5f5);
  }

  .usb-device-item.selected {
    border-color: var(--color-primary, #4a90d9);
    background: var(--color-primary-light, #e3f2fd);
  }

  .device-info {
    display: flex;
    flex-direction: column;
  }

  .device-name {
    font-size: 13px;
    font-weight: 500;
  }

  .device-id {
    font-size: 11px;
    color: var(--text-muted, #999);
    font-family: monospace;
  }

  /* 确认页 */
  .summary-card {
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 8px;
    overflow: hidden;
  }

  .summary-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px;
    background: var(--bg-tertiary, #f5f5f5);
    border-bottom: 1px solid var(--border-color, #e0e0e0);
  }

  .summary-header h3 {
    margin: 0;
    font-size: 18px;
  }

  .summary-header .desc {
    margin: 4px 0 0 0;
    font-size: 13px;
    color: var(--text-muted, #999);
  }

  .summary-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
    padding: 16px;
  }

  .summary-section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .summary-section h4 {
    margin: 0;
    font-size: 13px;
    color: var(--text-secondary, #666);
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .summary-row {
    display: flex;
    justify-content: space-between;
    font-size: 13px;
  }

  .summary-row .label {
    color: var(--text-muted, #999);
  }

  .summary-row .value {
    font-weight: 500;
    text-align: right;
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .summary-footer {
    padding: 12px 16px;
    border-top: 1px solid var(--border-color, #e0e0e0);
    background: var(--bg-secondary, #fafafa);
  }

  /* 底部按钮 */
  .wizard-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-top: 1px solid var(--border-color, #e0e0e0);
    background: var(--bg-secondary, #fafafa);
  }

  .footer-right {
    display: flex;
    gap: 8px;
  }

  /* 动画 */
  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .loading {
    padding: 20px;
    text-align: center;
    color: var(--text-muted, #999);
  }

  /* ISO 输入行 */
  .iso-input-row {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .iso-input {
    flex: 1;
    padding: 10px 12px;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
    font-size: 14px;
    background: var(--bg-primary, #fff);
    color: var(--text-primary, #333);
    cursor: pointer;
  }

  .iso-input:focus {
    outline: none;
    border-color: var(--color-primary, #4a90d9);
  }

  /* ISO 浏览器 */
  .iso-browser {
    display: flex;
    flex-direction: column;
    height: 450px;
  }

  .browser-nav {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
    background: var(--bg-secondary, #fafafa);
  }

  .nav-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--text-primary, #333);
    cursor: pointer;
  }

  .nav-btn:hover:not(:disabled) {
    background: var(--bg-hover, rgba(0, 0, 0, 0.05));
  }

  .nav-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .breadcrumbs {
    display: flex;
    align-items: center;
    flex: 1;
    overflow-x: auto;
    padding: 0 8px;
    font-size: 13px;
  }

  .breadcrumbs .sep {
    margin: 0 4px;
    color: var(--text-muted, #999);
  }

  .breadcrumbs .crumb {
    border: none;
    background: none;
    color: var(--color-primary, #4a90d9);
    cursor: pointer;
    padding: 2px 4px;
    border-radius: 3px;
  }

  .breadcrumbs .crumb:hover {
    background: var(--bg-hover, rgba(0, 0, 0, 0.05));
  }

  .browser-list {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
  }

  .browser-loading,
  .browser-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 12px;
    color: var(--text-muted, #999);
  }

  .browser-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 10px 12px;
    border: none;
    border-radius: 6px;
    background: transparent;
    text-align: left;
    cursor: pointer;
    transition: background 0.15s;
  }

  .browser-item:hover {
    background: var(--bg-hover, rgba(0, 0, 0, 0.05));
  }

  .browser-item.is-dir {
    font-weight: 500;
  }

  .browser-item :global(.folder-icon) {
    color: #f5a623;
  }

  .browser-item :global(.iso-icon) {
    color: #4a90d9;
  }

  .browser-item .file-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .browser-item .file-size {
    font-size: 12px;
    color: var(--text-muted, #999);
  }

  .browser-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-top: 1px solid var(--border-color, #e0e0e0);
    background: var(--bg-secondary, #fafafa);
  }

  .selected-path {
    display: flex;
    align-items: center;
    gap: 6px;
    flex: 1;
    font-size: 13px;
    overflow: hidden;
  }

  .selected-path span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .selected-path :global(.selected-icon) {
    color: #03ae00;
    flex-shrink: 0;
  }

  .selected-path .hint {
    color: var(--text-muted, #999);
  }

  .browser-actions {
    display: flex;
    gap: 8px;
    flex-shrink: 0;
  }
</style>
