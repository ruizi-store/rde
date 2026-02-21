<script lang="ts">
  import Icon from "@iconify/svelte";
  import { t, locale } from "svelte-i18n";
  import {
    getMonthDays,
    getDayInfo,
    getNextFestival,
    type DayInfo,
  } from "$shared/utils/chinese-calendar";

  // 是否显示中国农历和节日（仅中文区域显示）
  let showChineseCalendar = $derived($locale?.startsWith("zh") ?? false);

  let { visible = $bindable(false) }: { visible: boolean } = $props();

  // 当前日期
  const today = new Date();
  let currentYear = $state(today.getFullYear());
  let currentMonth = $state(today.getMonth() + 1);

  // 日历数据
  let calendarDays = $derived(getMonthDays(currentYear, currentMonth));
  let todayInfo = $derived(getDayInfo(new Date()));
  let nextFestival = $derived(getNextFestival());

  // 星期标题（根据语言返回对应数组）
  const WEEKDAYS: Record<string, string[]> = {
    "zh-CN": ["一", "二", "三", "四", "五", "六", "日"],
    "en-US": ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"],
  };
  let weekDays = $derived(WEEKDAYS[$locale || "zh-CN"] || WEEKDAYS["en-US"]);

  function close() {
    visible = false;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      close();
    }
  }

  function prevMonth() {
    if (currentMonth === 1) {
      currentMonth = 12;
      currentYear--;
    } else {
      currentMonth--;
    }
  }

  function nextMonth() {
    if (currentMonth === 12) {
      currentMonth = 1;
      currentYear++;
    } else {
      currentMonth++;
    }
  }

  function goToday() {
    currentYear = today.getFullYear();
    currentMonth = today.getMonth() + 1;
  }

  // 获取日期显示的副文本（农历或节日）
  function getDaySubText(day: DayInfo): string {
    if (day.lunarFestival) return day.lunarFestival;
    if (day.festival) return day.festival;
    return day.lunar.dayStr;
  }

  // 判断是否是当前月
  function isCurrentMonth(day: DayInfo): boolean {
    return day.month === currentMonth;
  }

  // 格式化月份标题（根据语言设置）
  let monthTitle = $derived(() => {
    const date = new Date(currentYear, currentMonth - 1, 1);
    const localeCode = $locale || "zh-CN";
    return date.toLocaleDateString(localeCode, { year: "numeric", month: "long" });
  });
</script>

{#if visible}
  <div
    class="calendar-overlay"
    onclick={close}
    onkeydown={handleKeydown}
    role="presentation"
    tabindex="-1"
  >
    <div class="calendar-popup" onclick={(e) => e.stopPropagation()}>
      <!-- 日历头部 -->
      <div class="calendar-header">
        <button class="nav-btn" onclick={prevMonth}>
          <Icon icon="mdi:chevron-left" width="18" />
        </button>
        <span class="calendar-title">{monthTitle()}</span>
        <button class="nav-btn" onclick={nextMonth}>
          <Icon icon="mdi:chevron-right" width="18" />
        </button>
        <button class="today-btn" onclick={goToday}>{$t("desktop.calendar.today")}</button>
      </div>

      <!-- 星期标题 -->
      <div class="calendar-weekdays">
        {#each weekDays as day, i}
          <span class:weekend={i >= 5}>{day}</span>
        {/each}
      </div>

      <!-- 日期网格 -->
      <div class="calendar-days">
        {#each calendarDays as day}
          <div
            class="calendar-day"
            class:other-month={!isCurrentMonth(day)}
            class:today={day.isToday}
            class:weekend={day.isWeekend && !day.isWorkday}
            class:holiday={day.isHoliday}
            class:workday={day.isWorkday}
          >
            <span class="day-number">{day.day}</span>
            {#if showChineseCalendar}
              <span class="day-lunar">{getDaySubText(day)}</span>
              {#if day.isHoliday}
                <span class="day-badge holiday">{$t("desktop.calendar.holiday")}</span>
              {:else if day.isWorkday}
                <span class="day-badge workday">{$t("desktop.calendar.workday")}</span>
              {/if}
            {/if}
          </div>
        {/each}
      </div>

      <!-- 农历信息（仅中文区域显示） -->
      {#if showChineseCalendar}
        <div class="lunar-info">
          <div class="lunar-date">
            <span class="lunar-month">{todayInfo.lunar.monthStr}{todayInfo.lunar.dayStr}</span>
            <span class="lunar-year">{todayInfo.lunar.ganzhiYear}年 {todayInfo.lunar.zodiac}</span>
          </div>
          {#if nextFestival}
            <div class="next-festival">
              <Icon icon="mdi:clock-outline" width="14" />
              <span>{$t("desktop.calendar.daysToFestival", { values: { name: nextFestival.name, days: nextFestival.days } })}</span>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .calendar-overlay {
    position: fixed;
    inset: 0;
    z-index: 10000;
  }

  .calendar-popup {
    position: absolute;
    bottom: 56px;
    right: 8px;
    width: 320px;
    background: rgba(30, 30, 34, 0.95);
    backdrop-filter: blur(24px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
    animation: slideUp 0.2s ease-out;
    padding: 12px;

    :global([data-theme="light"]) & {
      background: rgba(255, 255, 255, 0.95);
      border-color: rgba(0, 0, 0, 0.1);
      box-shadow: 0 12px 40px rgba(0, 0, 0, 0.15);
    }
  }

  .calendar-header {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 10px;
  }

  .nav-btn {
    width: 28px;
    height: 28px;
    border: none;
    border-radius: 6px;
    background: rgba(255, 255, 255, 0.05);
    color: rgba(255, 255, 255, 0.7);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.15s;

    &:hover {
      background: rgba(255, 255, 255, 0.1);
    }

    :global([data-theme="light"]) & {
      background: rgba(0, 0, 0, 0.05);
      color: rgba(0, 0, 0, 0.7);

      &:hover {
        background: rgba(0, 0, 0, 0.1);
      }
    }
  }

  .calendar-title {
    flex: 1;
    text-align: center;
    font-size: 14px;
    font-weight: 500;
    color: rgba(255, 255, 255, 0.9);

    :global([data-theme="light"]) & {
      color: rgba(0, 0, 0, 0.85);
    }
  }

  .today-btn {
    padding: 4px 10px;
    border: none;
    border-radius: 6px;
    background: rgba(74, 144, 217, 0.2);
    color: #4a90d9;
    font-size: 12px;
    cursor: pointer;
    transition: all 0.15s;

    &:hover {
      background: rgba(74, 144, 217, 0.3);
    }
  }

  .calendar-weekdays {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 2px;
    margin-bottom: 4px;

    span {
      text-align: center;
      font-size: 11px;
      color: rgba(255, 255, 255, 0.4);
      padding: 4px 0;

      &.weekend {
        color: rgba(255, 107, 122, 0.6);
      }

      :global([data-theme="light"]) & {
        color: rgba(0, 0, 0, 0.4);

        &.weekend {
          color: rgba(220, 53, 69, 0.6);
        }
      }
    }
  }

  .calendar-days {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 2px;
  }

  .calendar-day {
    aspect-ratio: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
    position: relative;
    cursor: default;
    transition: background 0.15s;

    &:hover {
      background: rgba(255, 255, 255, 0.05);

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.05);
      }
    }

    &.other-month {
      opacity: 0.3;
    }

    &.weekend:not(.holiday):not(.workday) {
      .day-number {
        color: rgba(255, 107, 122, 0.9);

        :global([data-theme="light"]) & {
          color: rgba(220, 53, 69, 0.9);
        }
      }
    }

    &.holiday {
      background: rgba(255, 107, 122, 0.1);
      .day-number {
        color: #ff6b7a;
      }
    }

    &.workday {
      .day-number {
        color: rgba(255, 255, 255, 0.9);

        :global([data-theme="light"]) & {
          color: rgba(0, 0, 0, 0.85);
        }
      }
    }

    &.today {
      background: rgba(74, 144, 217, 0.3);
      border: 1px solid rgba(74, 144, 217, 0.5);

      .day-number {
        color: #4a90d9;
        font-weight: 600;
      }
    }

    .day-number {
      font-size: 13px;
      color: rgba(255, 255, 255, 0.85);
      line-height: 1;

      :global([data-theme="light"]) & {
        color: rgba(0, 0, 0, 0.85);
      }
    }

    .day-lunar {
      font-size: 9px;
      color: rgba(255, 255, 255, 0.4);
      margin-top: 2px;
      max-width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;

      :global([data-theme="light"]) & {
        color: rgba(0, 0, 0, 0.4);
      }
    }

    .day-badge {
      position: absolute;
      top: 2px;
      right: 2px;
      font-size: 8px;
      padding: 1px 3px;
      border-radius: 3px;

      &.holiday {
        background: rgba(255, 107, 122, 0.3);
        color: #ff6b7a;
      }
      &.workday {
        background: rgba(250, 173, 20, 0.3);
        color: #faad14;
      }
    }
  }

  .lunar-info {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: 10px;
    padding-top: 10px;
    border-top: 1px solid rgba(255, 255, 255, 0.06);

    :global([data-theme="light"]) & {
      border-top-color: rgba(0, 0, 0, 0.06);
    }
  }

  .lunar-date {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .lunar-month {
    font-size: 14px;
    color: rgba(255, 255, 255, 0.9);
    font-weight: 500;

    :global([data-theme="light"]) & {
      color: rgba(0, 0, 0, 0.85);
    }
  }

  .lunar-year {
    font-size: 11px;
    color: rgba(255, 255, 255, 0.5);

    :global([data-theme="light"]) & {
      color: rgba(0, 0, 0, 0.5);
    }
  }

  .next-festival {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
    color: rgba(255, 255, 255, 0.5);
    background: rgba(255, 255, 255, 0.04);
    padding: 4px 8px;
    border-radius: 4px;

    :global([data-theme="light"]) & {
      color: rgba(0, 0, 0, 0.5);
      background: rgba(0, 0, 0, 0.04);
    }
  }

  @keyframes slideUp {
    from {
      opacity: 0;
      transform: translateY(10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
