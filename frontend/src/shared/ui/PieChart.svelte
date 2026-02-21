<script lang="ts">
  import { t } from "$lib/i18n";
  /**
   * 存储空间饼图组件
   * 显示各存储池/磁盘的使用情况
   */

  interface Segment {
    label: string;
    value: number;
    color: string;
  }

  interface Props {
    segments: Segment[];
    total: number;
    size?: number;
    showLegend?: boolean;
  }

  let { segments, total, size = 200, showLegend = true }: Props = $props();

  // 计算每个段的角度
  function calculateSegments() {
    let currentAngle = -90; // 从顶部开始
    return segments.map((segment) => {
      const percentage = total > 0 ? (segment.value / total) * 100 : 0;
      const angle = (percentage / 100) * 360;
      const startAngle = currentAngle;
      currentAngle += angle;
      return {
        ...segment,
        percentage,
        startAngle,
        endAngle: currentAngle,
      };
    });
  }

  // 生成SVG扇形路径
  function describeArc(
    cx: number,
    cy: number,
    radius: number,
    startAngle: number,
    endAngle: number,
  ) {
    const start = polarToCartesian(cx, cy, radius, endAngle);
    const end = polarToCartesian(cx, cy, radius, startAngle);
    const largeArcFlag = endAngle - startAngle <= 180 ? 0 : 1;
    return [
      "M",
      cx,
      cy,
      "L",
      start.x,
      start.y,
      "A",
      radius,
      radius,
      0,
      largeArcFlag,
      0,
      end.x,
      end.y,
      "Z",
    ].join(" ");
  }

  function polarToCartesian(cx: number, cy: number, radius: number, angle: number) {
    const radians = (angle * Math.PI) / 180;
    return {
      x: cx + radius * Math.cos(radians),
      y: cy + radius * Math.sin(radians),
    };
  }

  // 格式化字节
  function formatBytes(bytes: number): string {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  }

  let calculatedSegments = $derived(calculateSegments());
  let centerX = $derived(size / 2);
  let centerY = $derived(size / 2);
  let radius = $derived(size / 2 - 10);
  let innerRadius = $derived(radius * 0.6); // 环形图中间空白
</script>

<div class="pie-chart-container">
  <svg width={size} height={size} class="pie-chart">
    <!-- 背景圆（如果没有数据） -->
    {#if total === 0}
      <circle cx={centerX} cy={centerY} r={radius} fill="#e5e7eb" />
    {:else}
      <!-- 扇形 -->
      {#each calculatedSegments as segment, i}
        {#if segment.percentage > 0}
          <path
            d={describeArc(centerX, centerY, radius, segment.startAngle, segment.endAngle)}
            fill={segment.color}
            class="pie-segment"
          >
            <title
              >{segment.label}: {formatBytes(segment.value)} ({segment.percentage.toFixed(
                1,
              )}%)</title
            >
          </path>
        {/if}
      {/each}

      <!-- 中心白色圆（环形效果） -->
      <circle cx={centerX} cy={centerY} r={innerRadius} fill="white" />
    {/if}

    <!-- 中心文字 -->
    <text x={centerX} y={centerY - 8} text-anchor="middle" class="center-label">{$t('common.total')}</text>
    <text x={centerX} y={centerY + 12} text-anchor="middle" class="center-value"
      >{formatBytes(total)}</text
    >
  </svg>

  {#if showLegend}
    <div class="legend">
      {#each calculatedSegments as segment}
        <div class="legend-item">
          <span class="legend-color" style="background: {segment.color}"></span>
          <span class="legend-label">{segment.label}</span>
          <span class="legend-value">{formatBytes(segment.value)}</span>
          <span class="legend-percent">{segment.percentage.toFixed(1)}%</span>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .pie-chart-container {
    display: flex;
    align-items: center;
    gap: 24px;
  }

  .pie-chart {
    flex-shrink: 0;
  }

  .pie-segment {
    transition: opacity 0.2s;
    cursor: pointer;

    &:hover {
      opacity: 0.85;
    }
  }

  .center-label {
    font-size: 12px;
    fill: #6b7280;
  }

  .center-value {
    font-size: 16px;
    font-weight: 600;
    fill: #1f2937;
  }

  .legend {
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-width: 180px;
  }

  .legend-item {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
  }

  .legend-color {
    width: 12px;
    height: 12px;
    border-radius: 3px;
    flex-shrink: 0;
  }

  .legend-label {
    flex: 1;
    color: #374151;
  }

  .legend-value {
    color: #6b7280;
    font-family: monospace;
    font-size: 12px;
  }

  .legend-percent {
    width: 48px;
    text-align: right;
    color: #9ca3af;
    font-size: 12px;
  }
</style>
