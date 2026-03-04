<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "./i18n";
  import { Button, Input, Modal, Spinner, EmptyState, Select, Switch, Tabs, Alert } from "$shared/ui";
  import {
    aiService,
    type ProviderConfig,
    type AIModel,
    type Conversation,
    type ChatMessage,
    type StreamChunk,
    type AIConfig,
  } from "./service";
  import { Marked } from "marked";
  import hljs from "highlight.js";
  import SetupWizard from "./SetupWizard.svelte";
  import GatewaySettings from "./GatewaySettings.svelte";
  import SkillsPanel from "./SkillsPanel.svelte";
  import { generateUUID } from "$shared/utils/uuid";

  // ==================== Markdown ====================

  const marked = new Marked({
    breaks: true,
    gfm: true,
  });

  function renderMarkdown(text: string): string {
    if (!text) return "";
    try {
      return marked.parse(text) as string;
    } catch {
      return escapeHtml(text);
    }
  }

  function escapeHtml(text: string): string {
    return text
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function highlightCode(node: HTMLElement) {
    node.querySelectorAll("pre code").forEach((block) => {
      hljs.highlightElement(block as HTMLElement);
    });
  }

  function markdownAction(node: HTMLElement) {
    highlightCode(node);
    return {
      update() {
        highlightCode(node);
      },
    };
  }

  // ==================== State ====================

  let providers = $state<ProviderConfig[]>([]);
  let models = $state<AIModel[]>([]);
  let conversations = $state<Conversation[]>([]);

  let selectedProviderId = $state("");
  let selectedModel = $state("");
  let currentConversation = $state<Conversation | null>(null);

  let chatMessages = $state<ChatMessage[]>([]);
  let inputText = $state("");
  let isStreaming = $state(false);
  let streamingContent = $state("");
  let abortController: AbortController | null = null;

  let loading = $state(true);
  let error = $state("");

  // Settings modal
  let showSettings = $state(false);
  let showNewProvider = $state(false);
  let newProvider = $state({ provider: "ollama" as const, name: "", base_url: "", api_key: "" });

  // Sub-panels
  let showSetupWizard = $state(false);
  let showGateway = $state(false);
  let showSkills = $state(false);
  let settingsTab = $state("providers");
  let needsSetup = $state(false);

  // Global AI config
  let aiConfig = $state<AIConfig | null>(null);
  let configSaving = $state(false);

  // Conversation rename
  let renamingConvId = $state<string | null>(null);
  let renameTitle = $state("");

  // Sidebar
  let showSidebar = $state(true);

  // Voice
  let isRecording = $state(false);
  let mediaRecorder: MediaRecorder | null = null;
  let ttsPlaying = $state<string | null>(null);
  let ttsAudio: HTMLAudioElement | null = null;

  let messagesContainer: HTMLDivElement;

  // ==================== Lifecycle ====================

  onMount(async () => {
    await loadData();
  });

  async function loadData() {
    loading = true;
    error = "";
    needsSetup = false;
    try {
      // 并行加载所有数据
      const [setupStatus, status, p, c] = await Promise.all([
        aiService.getSetupStatus(),
        aiService.getStatus(),
        aiService.getProviders(),
        aiService.getConversations(),
      ]);

      // 检查初始设置状态
      if (setupStatus.status !== "complete") {
        needsSetup = true;
        loading = false;
        return;
      }

      providers = p || [];
      conversations = c || [];

      // 检查是否需要设置（没有已启用的提供商）
      const enabledProviders = providers.filter((p) => p.enabled);
      if (enabledProviders.length === 0) {
        needsSetup = true;
        return;
      }

      if (!selectedProviderId) {
        // 优先使用后端配置的默认提供商
        const defaultProvider = status.default_provider 
          ? providers.find((p) => p.id === status.default_provider && p.enabled)
          : null;
        const selected = defaultProvider || enabledProviders[0];
        selectedProviderId = selected.id;
        
        // 先结束 loading，再后台加载模型
        loading = false;
        await loadModels(selected.id);
        
        // 使用后端配置的默认模型
        if (status.default_model && models.some(m => m.id === status.default_model || m.name === status.default_model)) {
          selectedModel = status.default_model;
        }
        return;
      }
    } catch (e: any) {
      error = e.message || $t("ai.loadFailed");
    } finally {
      loading = false;
    }
  }

  async function loadModels(providerId: string) {
    if (!providerId) return;
    try {
      models = await aiService.getModels(providerId);
      if (models.length > 0 && !selectedModel) {
        selectedModel = models[0].id || models[0].name;
      }
    } catch {
      models = [];
    }
  }

  // ==================== Provider Management ====================

  async function handleProviderChange() {
    selectedModel = "";
    models = [];
    if (selectedProviderId) {
      await loadModels(selectedProviderId);
    }
  }

  async function addProvider() {
    try {
      const created = await aiService.createProvider({
        provider: newProvider.provider,
        name: newProvider.name,
        base_url: newProvider.base_url,
        api_key: newProvider.api_key || undefined,
        enabled: true,
      });
      providers = [...providers, created];
      if (!selectedProviderId) {
        selectedProviderId = created.id;
        await loadModels(created.id);
      }
      showNewProvider = false;
      newProvider = { provider: "ollama", name: "", base_url: "", api_key: "" };
    } catch (e: any) {
      error = e.message;
    }
  }

  async function deleteProvider(id: string) {
    try {
      await aiService.deleteProvider(id);
      providers = providers.filter((p) => p.id !== id);
      if (selectedProviderId === id) {
        selectedProviderId = providers[0]?.id || "";
        models = [];
        selectedModel = "";
      }
    } catch (e: any) {
      error = e.message;
    }
  }

  // ==================== Conversation Management ====================

  async function newConversation() {
    if (!selectedModel) return;
    try {
      const conv = await aiService.createConversation($t("ai.newConversation"), selectedModel);
      conversations = [conv, ...conversations];
      selectConversation(conv);
    } catch (e: any) {
      error = e.message;
    }
  }

  function selectConversation(conv: Conversation) {
    currentConversation = conv;
    chatMessages = conv.messages || [];
    scrollToBottom();
  }

  async function loadConversationDetail(id: string) {
    try {
      const conv = await aiService.getConversation(id);
      selectConversation(conv);
    } catch (e: any) {
      error = e.message;
    }
  }

  async function deleteConversation(id: string) {
    try {
      await aiService.deleteConversation(id);
      conversations = conversations.filter((c) => c.id !== id);
      if (currentConversation?.id === id) {
        currentConversation = null;
        chatMessages = [];
      }
    } catch (e: any) {
      error = e.message;
    }
  }

  function startRename(conv: Conversation) {
    renamingConvId = conv.id;
    renameTitle = conv.title;
  }

  async function finishRename(id: string) {
    const title = renameTitle.trim();
    if (!title || !renamingConvId) {
      renamingConvId = null;
      return;
    }
    try {
      const updated = await aiService.updateConversation(id, { title });
      conversations = conversations.map((c) => (c.id === id ? { ...c, title: updated.title || title } : c));
      if (currentConversation?.id === id) {
        currentConversation = { ...currentConversation, title: updated.title || title };
      }
    } catch (e: any) {
      error = e.message;
    } finally {
      renamingConvId = null;
    }
  }

  // ==================== Global Config ====================

  async function loadConfig() {
    try {
      aiConfig = await aiService.getConfig();
    } catch {
      aiConfig = null;
    }
  }

  async function saveConfig() {
    if (!aiConfig) return;
    configSaving = true;
    try {
      aiConfig = await aiService.updateConfig(aiConfig);
    } catch (e: any) {
      error = e.message;
    } finally {
      configSaving = false;
    }
  }

  // ==================== Voice ====================

  async function toggleRecording() {
    if (isRecording) {
      mediaRecorder?.stop();
      return;
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const recorder = new MediaRecorder(stream);
      const chunks: Blob[] = [];
      recorder.ondataavailable = (e) => chunks.push(e.data);
      recorder.onstop = async () => {
        isRecording = false;
        stream.getTracks().forEach((t) => t.stop());
        const blob = new Blob(chunks, { type: "audio/webm" });
        const file = new File([blob], "recording.webm", { type: "audio/webm" });
        try {
          const result = await aiService.transcribeAudio(file);
          if (result.text) {
            inputText += (inputText ? " " : "") + result.text;
          }
        } catch (e: any) {
          error = $t("ai.voiceRecognitionFailed") + ": " + (e.message || $t("ai.unknownError"));
        }
      };
      recorder.start();
      mediaRecorder = recorder;
      isRecording = true;
    } catch {
      error = $t("ai.micAccessDenied");
    }
  }

  async function playTTS(msgId: string, text: string) {
    if (ttsPlaying === msgId) {
      ttsAudio?.pause();
      ttsAudio = null;
      ttsPlaying = null;
      return;
    }
    try {
      ttsPlaying = msgId;
      const blob = await aiService.textToSpeech(text);
      const url = URL.createObjectURL(blob);
      ttsAudio = new Audio(url);
      ttsAudio.onended = () => {
        ttsPlaying = null;
        URL.revokeObjectURL(url);
      };
      ttsAudio.onerror = () => {
        ttsPlaying = null;
        URL.revokeObjectURL(url);
      };
      ttsAudio.play();
    } catch {
      ttsPlaying = null;
    }
  }

  // ==================== Chat ====================

  async function sendMessage() {
    const text = inputText.trim();
    if (!text || !selectedProviderId || !selectedModel || isStreaming) return;

    // If no conversation, create one
    if (!currentConversation) {
      try {
        const conv = await aiService.createConversation(
          text.slice(0, 30) + (text.length > 30 ? "..." : ""),
          selectedModel,
        );
        conversations = [conv, ...conversations];
        currentConversation = conv;
        chatMessages = [];
      } catch (e: any) {
        error = e.message;
        return;
      }
    }

    // Add user message to UI
    const userMsg: ChatMessage = {
      id: generateUUID(),
      role: "user",
      content: text,
      timestamp: new Date().toISOString(),
    };
    chatMessages = [...chatMessages, userMsg];
    inputText = "";
    scrollToBottom();

    // Build request messages
    const reqMessages = chatMessages.map((m) => ({ role: m.role, content: m.content }));

    // Start streaming
    isStreaming = true;
    streamingContent = "";
    abortController = new AbortController();

    try {
      const stream = aiService.streamChat(
        {
          provider_id: selectedProviderId,
          model: selectedModel,
          messages: reqMessages,
          conversation_id: currentConversation?.id,
        },
        abortController.signal,
      );

      for await (const chunk of stream) {
        if (chunk.error) {
          throw new Error(chunk.error);
        }
        if (chunk.delta) {
          streamingContent += chunk.delta;
          scrollToBottom();
        }
        if (chunk.finish_reason) break;
      }

      // Add assistant message
      const assistantMsg: ChatMessage = {
        id: generateUUID(),
        role: "assistant",
        content: streamingContent,
        timestamp: new Date().toISOString(),
      };
      chatMessages = [...chatMessages, assistantMsg];
      streamingContent = "";
    } catch (e: any) {
      if (e.name !== "AbortError") {
        // Show error as system message
        const errMsg: ChatMessage = {
          id: generateUUID(),
          role: "assistant",
          content: `⚠️ 错误: ${e.message}`,
          timestamp: new Date().toISOString(),
        };
        chatMessages = [...chatMessages, errMsg];
        if (streamingContent) {
          streamingContent = "";
        }
      }
    } finally {
      isStreaming = false;
      abortController = null;
      scrollToBottom();
    }
  }

  function stopStreaming() {
    abortController?.abort();
  }

  function scrollToBottom() {
    requestAnimationFrame(() => {
      if (messagesContainer) {
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
      }
    });
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  }

  function formatTime(ts: string): string {
    try {
      return new Date(ts).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
    } catch {
      return "";
    }
  }

  const providerTypeOptions = [
    { value: "ollama", label: "Ollama" },
    { value: "openai", label: "OpenAI" },
    { value: "claude", label: "Claude" },
    { value: "deepseek", label: "Deepseek" },
    { value: "gemini", label: "Gemini" },
    { value: "zhipu", label: $t("ai.zhipu") },
    { value: "qwen", label: $t("ai.qwen") },
    { value: "moonshot", label: "Moonshot" },
    { value: "groq", label: "Groq" },
    { value: "openrouter", label: "OpenRouter" },
  ];

  const settingsTabs = $derived([
    { id: "providers", label: $t("ai.providers") },
    { id: "config", label: $t("ai.globalConfig") },
    { id: "gateway", label: $t("ai.gatewayAndAlerts") },
  ]);
</script>

<div class="flex h-full bg-[var(--bg-primary)]">
  <!-- Sidebar: Conversations -->
  {#if showSidebar}
    <div class="w-64 flex flex-col border-r border-[var(--border-primary)] bg-[var(--bg-secondary)]">
      <div class="p-3 border-b border-[var(--border-primary)] flex items-center justify-between">
        <span class="text-sm font-medium text-[var(--text-primary)]">{$t("ai.conversationList")}</span>
        <div class="flex gap-1">
          <Button size="sm" variant="ghost" onclick={newConversation}>+</Button>
          <Button size="sm" variant="ghost" onclick={() => (showSettings = true)}>⚙</Button>
        </div>
      </div>

      <div class="flex-1 overflow-y-auto">
        {#if conversations.length === 0}
          <div class="p-4 text-center text-xs text-[var(--text-tertiary)]">{$t("ai.noConversations")}</div>
        {:else}
          {#each conversations as conv}
            <div
              class="w-full text-left px-3 py-2 text-sm hover:bg-[var(--bg-hover)] border-b border-[var(--border-secondary)] flex items-center justify-between group transition-colors cursor-pointer"
              class:bg-[var(--bg-active)]={currentConversation?.id === conv.id}
              role="button"
              tabindex="0"
              onclick={() => loadConversationDetail(conv.id)}
              onkeydown={(e) => e.key === 'Enter' && loadConversationDetail(conv.id)}
            >
              <div class="flex-1 min-w-0">
                {#if renamingConvId === conv.id}
                  <input
                    class="w-full text-sm bg-[var(--bg-primary)] text-[var(--text-primary)] border border-blue-500 rounded px-1 py-0.5"
                    bind:value={renameTitle}
                    onblur={() => finishRename(conv.id)}
                    onkeydown={(e) => { if (e.key === 'Enter') finishRename(conv.id); if (e.key === 'Escape') renamingConvId = null; }}
                    onclick={(e) => e.stopPropagation()}
                  />
                {:else}
                  <div class="truncate text-[var(--text-primary)]" ondblclick={(e) => { e.stopPropagation(); startRename(conv); }}>{conv.title}</div>
                {/if}
                <div class="text-xs text-[var(--text-tertiary)]">{formatTime(conv.updated_at || conv.created_at)}</div>
              </div>
              <button
                class="opacity-0 group-hover:opacity-100 text-[var(--text-tertiary)] hover:text-red-500 ml-1 text-xs transition-opacity"
                onclick={(e) => { e.stopPropagation(); deleteConversation(conv.id); }}
                title={$t("ai.delete")}
              >✕</button>
            </div>
          {/each}
        {/if}
      </div>
    </div>
  {/if}

  <!-- Main Chat Area -->
  <div class="flex-1 flex flex-col min-w-0">
    <!-- Toolbar -->
    <div class="px-4 py-2 border-b border-[var(--border-primary)] flex items-center gap-3 bg-[var(--bg-secondary)]">
      <Button size="sm" variant="ghost" onclick={() => (showSidebar = !showSidebar)}>
        {showSidebar ? "◀" : "▶"}
      </Button>

      {#if providers.length > 0}
        <Select
          size="sm"
          options={providers.map(p => ({ value: p.id, label: `${p.name} (${p.provider})` }))}
          bind:value={selectedProviderId}
          onchange={() => handleProviderChange()}
        />

        {#if models.length > 0}
          <Select
            size="sm"
            options={models.map(m => ({ value: m.id || m.name, label: m.name }))}
            bind:value={selectedModel}
          />
        {/if}
      {:else}
        <Button size="sm" onclick={() => (showSettings = true)}>{$t("ai.addProvider")}</Button>
      {/if}

      {#if currentConversation}
        <span class="text-xs text-[var(--text-tertiary)] ml-auto truncate">{currentConversation.title}</span>
      {/if}
      <div class="ml-auto flex gap-1">
        <span title={$t("ai.skillsPanel")}><Button size="sm" variant="ghost" onclick={() => (showSkills = !showSkills)}>🛠️</Button></span>
        <span title={$t("ai.setupWizard")}><Button size="sm" variant="ghost" onclick={() => (showSetupWizard = true)}>📋</Button></span>
      </div>
    </div>

    <!-- Messages -->
    <div class="flex-1 overflow-y-auto p-4 space-y-4" bind:this={messagesContainer}>
      {#if loading}
        <div class="flex justify-center p-8"><Spinner /></div>
      {:else if error && chatMessages.length === 0}
        <EmptyState icon="⚠️" title={$t("ai.loadFailed")} description={error} />
      {:else if needsSetup}
        <div class="flex flex-col items-center justify-center h-full text-[var(--text-tertiary)]">
          <div class="text-5xl mb-4">🤖</div>
          <div class="text-lg font-medium mb-2">{$t("ai.aiAssistant")}</div>
          <div class="text-sm mb-4">{$t("ai.firstTimeSetup")}</div>
          <Button variant="primary" onclick={() => (showSetupWizard = true)}>{$t("ai.startSetup")}</Button>
        </div>
      {:else if chatMessages.length === 0 && !isStreaming}
        <div class="flex flex-col items-center justify-center h-full text-[var(--text-tertiary)]">
          <div class="text-5xl mb-4">🤖</div>
          <div class="text-lg font-medium mb-2">{$t("ai.aiAssistant")}</div>
          <div class="text-sm">{$t("ai.selectModelToStart")}</div>
          {#if providers.length === 0}
            <Button variant="primary" onclick={() => (showSettings = true)}>{$t("ai.configureProvider")}</Button>
          {/if}
        </div>
      {:else}
        {#each chatMessages as msg}
          <div class="flex {msg.role === 'user' ? 'justify-end' : 'justify-start'}">
            <div
              class="max-w-[80%] rounded-lg px-4 py-2 text-sm leading-relaxed {msg.role === 'user'
                ? 'bg-blue-600 text-white'
                : 'bg-[var(--bg-secondary)] text-[var(--text-primary)] border border-[var(--border-primary)]'}"
            >
              {#if msg.role === 'user'}
                <div class="whitespace-pre-wrap break-words">{msg.content}</div>
              {:else}
                <div class="markdown-body break-words" use:markdownAction>{@html renderMarkdown(msg.content)}</div>
                <button
                  class="inline-flex items-center gap-1 text-xs mt-1 px-1.5 py-0.5 rounded hover:bg-[var(--bg-hover)] text-[var(--text-tertiary)] hover:text-[var(--text-primary)] transition-colors"
                  onclick={() => playTTS(msg.id, msg.content)}
                  title={ttsPlaying === msg.id ? $t("ai.stopPlayback") : $t("ai.readAloud")}
                >{ttsPlaying === msg.id ? "⏹" : "🔊"}</button>
              {/if}
              <div class="text-xs mt-1 {msg.role === 'user' ? 'text-blue-200' : 'text-[var(--text-tertiary)]'}">{formatTime(msg.timestamp)}</div>
            </div>
          </div>
        {/each}

        <!-- Streaming indicator -->
        {#if isStreaming}
          <div class="flex justify-start">
            <div class="max-w-[80%] rounded-lg px-4 py-2 text-sm leading-relaxed bg-[var(--bg-secondary)] text-[var(--text-primary)] border border-[var(--border-primary)]">
              {#if streamingContent}
                <div class="markdown-body break-words" use:markdownAction>{@html renderMarkdown(streamingContent)}</div>
              {:else}
                <div class="flex items-center gap-2 text-[var(--text-tertiary)]">
                  <Spinner size="sm" />
                  <span>{$t("ai.thinking")}</span>
                </div>
              {/if}
            </div>
          </div>
        {/if}
      {/if}
    </div>

    <!-- Input -->
    <div class="p-4 border-t border-[var(--border-primary)] bg-[var(--bg-secondary)]">
      <div class="flex gap-2 items-end">
        <textarea
          class="flex-1 resize-none rounded-lg border border-[var(--border-primary)] bg-[var(--bg-primary)] text-[var(--text-primary)] px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          rows="2"
          placeholder={providers.length === 0 ? $t("ai.addProviderFirst") : $t("ai.inputPlaceholder")}
          disabled={providers.length === 0 || isStreaming}
          bind:value={inputText}
          onkeydown={handleKeydown}
        ></textarea>
        <button
          class="p-2 rounded-lg transition-colors {isRecording ? 'bg-red-500 text-white animate-pulse' : 'bg-[var(--bg-primary)] text-[var(--text-tertiary)] hover:text-[var(--text-primary)] border border-[var(--border-primary)]'}"
          onclick={toggleRecording}
          title={isRecording ? $t("ai.stopRecording") : $t("ai.voiceInput")}
          disabled={isStreaming}
        >{isRecording ? "⏹" : "🎤"}</button>
        {#if isStreaming}
          <Button variant="danger" onclick={stopStreaming}>{$t("ai.stop")}</Button>
        {:else}
          <Button variant="primary" onclick={sendMessage} disabled={!inputText.trim() || !selectedModel}>{$t("ai.send")}</Button>
        {/if}
      </div>
    </div>
  </div>

  <!-- Skills Panel (right sidebar) -->
  {#if showSkills}
    <div class="w-80 flex flex-col border-l border-[var(--border-primary)] bg-[var(--bg-secondary)]">
      <div class="p-3 border-b border-[var(--border-primary)] flex items-center justify-between">
        <span class="text-sm font-medium text-[var(--text-primary)]">{$t("ai.skillsPanel")}</span>
        <Button size="sm" variant="ghost" onclick={() => (showSkills = false)}>✕</Button>
      </div>
      <div class="flex-1 overflow-hidden">
        <SkillsPanel />
      </div>
    </div>
  {/if}
</div>

<!-- Setup Wizard Modal -->
<Modal open={showSetupWizard} title={$t("ai.setupWizard")} onclose={() => (showSetupWizard = false)} size="lg">
  <SetupWizard onclose={() => { showSetupWizard = false; loadData(); }} />
</Modal>

<!-- Settings Modal -->
<Modal open={showSettings} title={$t("ai.aiSettings")} onclose={() => (showSettings = false)} size="lg">
  <Tabs tabs={settingsTabs} bind:activeTab={settingsTab} variant="underline" size="sm"
    onchange={(id) => { if (id === "config") loadConfig(); }}>
    {#snippet children(tab)}
    <div class="space-y-4 pt-2">
    {#if tab === "providers"}
    <div class="flex items-center justify-between">
      <h3 class="font-medium text-[var(--text-primary)]">{$t("ai.aiProviders")}</h3>
      <Button size="sm" onclick={() => (showNewProvider = true)}>{$t("ai.add")}</Button>
    </div>

    {#if providers.length === 0}
      <div class="text-sm text-[var(--text-tertiary)] text-center py-4">
        {$t("ai.noProviderConfigured")}
      </div>
    {:else}
      <div class="space-y-2">
        {#each providers as p}
          <div class="flex items-center justify-between p-3 rounded-lg border border-[var(--border-primary)] bg-[var(--bg-primary)]">
            <div>
              <div class="text-sm font-medium text-[var(--text-primary)]">{p.name}</div>
              <div class="text-xs text-[var(--text-tertiary)]">{p.provider} · {p.base_url}</div>
            </div>
            <div class="flex gap-2">
              <span class="text-xs px-2 py-0.5 rounded {p.enabled ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300' : 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400'}">
                {p.enabled ? $t("ai.enabled") : $t("ai.disabled")}
              </span>
              <Button size="sm" variant="danger" onclick={() => deleteProvider(p.id)}>{$t("ai.delete")}</Button>
            </div>
          </div>
        {/each}
      </div>
    {/if}
    {:else if tab === "config"}
      <!-- Global AI Config -->
      {#if aiConfig}
        <div class="space-y-4">
          <h3 class="font-medium text-[var(--text-primary)]">{$t("ai.globalAiConfig")}</h3>
          <div>
            <label class="block text-sm font-medium text-[var(--text-primary)] mb-1">{$t("ai.defaultProviderId")}</label>
            <Input bind:value={aiConfig.default_provider} placeholder={$t("ai.defaultProviderIdPlaceholder")} />
          </div>
          <div>
            <label class="block text-sm font-medium text-[var(--text-primary)] mb-1">{$t("ai.defaultModel")}</label>
            <Input bind:value={aiConfig.default_model} placeholder={$t("ai.defaultModelPlaceholder")} />
          </div>
          <div>
            <label class="block text-sm font-medium text-[var(--text-primary)] mb-1">{$t("ai.systemPrompt")}</label>
            <textarea
              class="w-full text-sm bg-[var(--bg-primary)] text-[var(--text-primary)] border border-[var(--border-primary)] rounded px-3 py-2 min-h-[80px]"
              bind:value={aiConfig.system_prompt}
              rows="3"
            ></textarea>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium text-[var(--text-primary)] mb-1">{$t("ai.temperature")} ({aiConfig.temperature})</label>
              <input type="range" min="0" max="2" step="0.1" bind:value={aiConfig.temperature} class="w-full" />
            </div>
            <div>
              <label class="block text-sm font-medium text-[var(--text-primary)] mb-1">{$t("ai.maxTokens")}</label>
              <input type="number" class="num-input w-full text-sm bg-[var(--bg-primary)] text-[var(--text-primary)] border border-[var(--border-primary)] rounded px-3 py-2" bind:value={aiConfig.max_tokens} min="256" max="32768" step="256" />
            </div>
          </div>
          <Switch bind:checked={aiConfig.enable_tools} label={$t("ai.enableTools")} />
          <div class="flex justify-end">
            <Button variant="primary" onclick={saveConfig} disabled={configSaving}>
              {configSaving ? $t("ai.saving") : $t("ai.saveConfig")}
            </Button>
          </div>
        </div>
      {:else}
        <div class="flex justify-center p-8"><Spinner /></div>
      {/if}
    {:else if tab === "gateway"}
      <GatewaySettings onclose={() => (showSettings = false)} />
    {/if}
    </div>
    {/snippet}
  </Tabs>
</Modal>

<!-- New Provider Modal -->
<Modal open={showNewProvider} title={$t("ai.addProviderTitle")} onclose={() => (showNewProvider = false)} size="md">
  <div class="space-y-4">
    <div>
      <label class="block text-sm font-medium text-[var(--text-primary)] mb-1">{$t("ai.type")}</label>
      <Select options={providerTypeOptions} bind:value={newProvider.provider} />
    </div>
    <div>
      <label class="block text-sm font-medium text-[var(--text-primary)] mb-1">{$t("ai.name")}</label>
      <Input bind:value={newProvider.name} placeholder={$t("ai.namePlaceholder")} />
    </div>
    <div>
      <label class="block text-sm font-medium text-[var(--text-primary)] mb-1">Base URL</label>
      <Input bind:value={newProvider.base_url} placeholder={$t("ai.baseUrlPlaceholder")} />
    </div>
    <div>
      <label class="block text-sm font-medium text-[var(--text-primary)] mb-1">{$t("ai.apiKeyOptional")}</label>
      <Input type="password" bind:value={newProvider.api_key} placeholder="sk-..." />
    </div>
    <div class="flex justify-end gap-2">
      <Button variant="ghost" onclick={() => (showNewProvider = false)}>{$t("common.cancel")}</Button>
      <Button variant="primary" onclick={addProvider} disabled={!newProvider.name || !newProvider.base_url}>{$t("common.ok")}</Button>
    </div>
  </div>
</Modal>

<style>
  :global(.markdown-body) {
    line-height: 1.6;
    word-wrap: break-word;
  }
  :global(.markdown-body p) {
    margin: 0.4em 0;
  }
  :global(.markdown-body p:first-child) {
    margin-top: 0;
  }
  :global(.markdown-body p:last-child) {
    margin-bottom: 0;
  }
  :global(.markdown-body pre) {
    background: var(--bg-primary);
    border: 1px solid var(--border-primary);
    border-radius: 6px;
    padding: 0.75em 1em;
    margin: 0.5em 0;
    overflow-x: auto;
    font-size: 0.85em;
  }
  :global(.markdown-body pre code) {
    background: none;
    padding: 0;
    border-radius: 0;
    font-size: inherit;
  }
  :global(.markdown-body code) {
    background: var(--bg-primary);
    padding: 0.15em 0.4em;
    border-radius: 4px;
    font-size: 0.9em;
    font-family: 'Fira Code', 'Cascadia Code', 'JetBrains Mono', monospace;
  }
  :global(.markdown-body ul, .markdown-body ol) {
    margin: 0.4em 0;
    padding-left: 1.5em;
  }
  :global(.markdown-body li) {
    margin: 0.15em 0;
  }
  :global(.markdown-body blockquote) {
    border-left: 3px solid var(--border-primary);
    padding: 0.25em 0.75em;
    margin: 0.4em 0;
    color: var(--text-secondary);
  }
  :global(.markdown-body h1, .markdown-body h2, .markdown-body h3, .markdown-body h4) {
    margin: 0.6em 0 0.3em;
    font-weight: 600;
  }
  :global(.markdown-body h1) { font-size: 1.3em; }
  :global(.markdown-body h2) { font-size: 1.15em; }
  :global(.markdown-body h3) { font-size: 1.05em; }
  :global(.markdown-body table) {
    border-collapse: collapse;
    margin: 0.5em 0;
    width: 100%;
  }
  :global(.markdown-body th, .markdown-body td) {
    border: 1px solid var(--border-primary);
    padding: 0.35em 0.6em;
    text-align: left;
  }
  :global(.markdown-body th) {
    background: var(--bg-primary);
    font-weight: 600;
  }
  :global(.markdown-body hr) {
    border: none;
    border-top: 1px solid var(--border-primary);
    margin: 0.6em 0;
  }
  :global(.markdown-body a) {
    color: #3b82f6;
    text-decoration: underline;
  }
  :global(.markdown-body img) {
    max-width: 100%;
    border-radius: 6px;
  }
</style>
