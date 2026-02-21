<!-- BubbleManager.svelte - 泡泡管理器 -->
<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import Bubble from "./Bubble.svelte";
  import FallingText from "./FallingText.svelte";
  import ParticleSystem from "../effects/ParticleSystem.svelte";
  import { getRandomQuotes } from "./quotes";

  interface BubbleData {
    id: string;
    text: string;
    x: number;
    y: number;
    size: number;
  }

  interface FallingTextData {
    id: string;
    text: string;
    x: number;
    y: number;
  }

  interface ParticleEffect {
    id: string;
    x: number;
    y: number;
    type: "pop" | "shatter";
    chars?: string[];
  }

  interface Props {
    maxBubbles?: number;
    taskbarHeight?: number;
  }

  let { maxBubbles = 6, taskbarHeight = 48 }: Props = $props();

  let bubbles = $state<BubbleData[]>([]);
  let fallingTexts = $state<FallingTextData[]>([]);
  let particleEffects = $state<ParticleEffect[]>([]);

  let bubbleIdCounter = 0;
  let mounted = $state(false);

  // 生成初始泡泡
  function generateInitialBubbles() {
    const quotes = getRandomQuotes(maxBubbles);
    const newBubbles: BubbleData[] = [];

    for (let i = 0; i < maxBubbles; i++) {
      newBubbles.push(createBubble(quotes[i], i));
    }

    bubbles = newBubbles;
  }

  // 创建单个泡泡
  function createBubble(text: string, index: number = 0): BubbleData {
    const size = 70 + Math.random() * 40; // 70-110px
    const margin = size;

    // 分散在屏幕各处
    const cols = 3;
    const rows = 2;
    const colWidth = (window.innerWidth - margin * 2) / cols;
    const rowHeight = (window.innerHeight - taskbarHeight - margin * 2) / rows;

    const col = index % cols;
    const row = Math.floor(index / cols) % rows;

    const x = margin + colWidth * col + Math.random() * (colWidth - size);
    const y = margin + rowHeight * row + Math.random() * (rowHeight - size) + size / 2;

    return {
      id: `bubble-${bubbleIdCounter++}`,
      text,
      x,
      y,
      size,
    };
  }

  // 泡泡被戳破时的处理
  function handleBubblePop(id: string, x: number, y: number, text: string) {
    // 添加破裂粒子效果
    particleEffects = [
      ...particleEffects,
      {
        id: `pop-${Date.now()}`,
        x,
        y,
        type: "pop",
      },
    ];

    // 移除泡泡
    bubbles = bubbles.filter((b) => b.id !== id);

    // 添加掉落的文字
    fallingTexts = [
      ...fallingTexts,
      {
        id: `falling-${Date.now()}`,
        text,
        x,
        y,
      },
    ];

    // 延迟生成新泡泡
    setTimeout(() => {
      if (bubbles.length < maxBubbles) {
        const quotes = getRandomQuotes(1);
        bubbles = [...bubbles, createBubble(quotes[0], bubbles.length)];
      }
    }, 2000);
  }

  // 文字碰到任务栏时的处理
  function handleTextHitTaskbar(x: number, y: number, chars: string[]) {
    // 添加文字碎裂粒子效果
    particleEffects = [
      ...particleEffects,
      {
        id: `shatter-${Date.now()}`,
        x,
        y,
        type: "shatter",
        chars,
      },
    ];

    // 移除掉落的文字
    fallingTexts = fallingTexts.filter(
      (ft) => !(Math.abs(ft.x - x) < 50 && Math.abs(ft.y - y) < 50),
    );
  }

  // 粒子效果完成时的处理
  function handleParticleComplete(id: string) {
    particleEffects = particleEffects.filter((p) => p.id !== id);
  }

  onMount(() => {
    mounted = true;
    generateInitialBubbles();
  });
</script>

<div class="bubble-manager">
  <!-- 泡泡 -->
  {#each bubbles as bubble (bubble.id)}
    <Bubble
      id={bubble.id}
      text={bubble.text}
      initialX={bubble.x}
      initialY={bubble.y}
      size={bubble.size}
      onPop={handleBubblePop}
    />
  {/each}

  <!-- 掉落的文字 -->
  {#each fallingTexts as ft (ft.id)}
    <FallingText
      text={ft.text}
      startX={ft.x}
      startY={ft.y}
      {taskbarHeight}
      onHitTaskbar={handleTextHitTaskbar}
    />
  {/each}

  <!-- 粒子效果 -->
  {#each particleEffects as effect (effect.id)}
    {#if effect.type === "pop"}
      <ParticleSystem
        x={effect.x}
        y={effect.y}
        particleCount={15}
        colors={["#fff", "#87CEEB", "#4FC3F7", "#B3E5FC", "#E1F5FE"]}
        spread={360}
        gravity={200}
        initialVelocity={150}
        lifetime={0.8}
        onComplete={() => handleParticleComplete(effect.id)}
      />
    {:else if effect.type === "shatter"}
      <ParticleSystem
        x={effect.x}
        y={effect.y}
        particleCount={effect.chars?.length || 10}
        chars={effect.chars}
        colors={["#fff", "#FFD700", "#FFA500", "#FF6B6B"]}
        spread={180}
        gravity={600}
        initialVelocity={250}
        lifetime={1.2}
        onComplete={() => handleParticleComplete(effect.id)}
      />
    {/if}
  {/each}
</div>

<style>
  .bubble-manager {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
    z-index: 2;
    overflow: hidden;
  }
</style>
