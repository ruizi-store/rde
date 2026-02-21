<script lang="ts">
  import { goto } from "$app/navigation";
  import { onMount } from "svelte";
  import { setupApi, type LocaleSettings } from "$shared/services/setup";
  import { Button, Select, Radio, type SelectOption } from "$shared/ui";
  import { locale, _ as t } from "svelte-i18n";
  import { saveLanguagePreference } from "$lib/i18n";

  import SetupCard from "../SetupCard.svelte";
  import FormField from "../FormField.svelte";

  let settings = $state<LocaleSettings>({
    language: "zh-CN",
    timezone: "Asia/Shanghai",
    time_format: "24h",
    date_format: "YYYY-MM-DD",
  });

  let loading = $state(false);
  let error = $state("");

  // 支持的语言列表
  const languages = $derived<SelectOption[]>([
    { value: "zh-CN", label: $t("setup.langZhCN") },
    { value: "en-US", label: $t("setup.langEnUS") },
  ]);

  // 常用时区列表
  const timezones = $derived<SelectOption[]>([
    { value: "Asia/Shanghai", label: $t("setup.tzShanghai") },
    { value: "Asia/Hong_Kong", label: $t("setup.tzHongKong") },
    { value: "Asia/Taipei", label: $t("setup.tzTaipei") },
    { value: "Asia/Tokyo", label: $t("setup.tzTokyo") },
    { value: "Asia/Seoul", label: $t("setup.tzSeoul") },
    { value: "Asia/Singapore", label: $t("setup.tzSingapore") },
    { value: "America/New_York", label: $t("setup.tzNewYork") },
    { value: "America/Los_Angeles", label: $t("setup.tzLosAngeles") },
    { value: "America/Chicago", label: $t("setup.tzChicago") },
    { value: "Europe/London", label: $t("setup.tzLondon") },
    { value: "Europe/Paris", label: $t("setup.tzParis") },
    { value: "Europe/Berlin", label: $t("setup.tzBerlin") },
    { value: "Australia/Sydney", label: $t("setup.tzSydney") },
    { value: "Pacific/Auckland", label: $t("setup.tzAuckland") },
  ]);

  // 时间格式
  const timeFormats = $derived<SelectOption[]>([
    { value: "24h", label: $t("setup.time24h") },
    { value: "12h", label: $t("setup.time12h") },
  ]);

  // 日期格式
  const dateFormats: SelectOption[] = [
    { value: "YYYY-MM-DD", label: "2024-01-15" },
    { value: "DD/MM/YYYY", label: "15/01/2024" },
    { value: "MM/DD/YYYY", label: "01/15/2024" },
    { value: "YYYY年MM月DD日", label: "2024年01月15日" },
  ];

  // 语言切换时立即更新界面
  function onLanguageChange(lang: string) {
    locale.set(lang);
    saveLanguagePreference(lang as "zh-CN" | "en-US");
    document.documentElement.lang = lang;
  }

  async function handleSubmit() {
    loading = true;
    error = "";

    try {
      // 保存语言和时区设置到后端
      await setupApi.setLocale(settings);

      goto("/setup/step2");
    } catch (e) {
      error = e instanceof Error ? e.message : $t("setup.saveSettingsFailed");
    } finally {
      loading = false;
    }
  }

  // 尝试自动检测语言和时区
  onMount(async () => {
    // 自动检测浏览器语言
    const browserLang = navigator.language;
    if (browserLang.startsWith("zh")) {
      settings.language = "zh-CN";
    } else {
      settings.language = "en-US";
    }

    // 尝试获取时区
    try {
      const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
      const matchedTz = timezones.find((t) => t.value === tz);
      if (matchedTz) {
        settings.timezone = matchedTz.value;
      }
    } catch {
      // 忽略错误，使用默认值
    }
  });
</script>

<SetupCard
  header={{ icon: "🌐", title: $t("setup.langAndTimezone"), description: $t("setup.langAndTimezoneDesc") }}
  {error}
>
  <form
    class="flex flex-col gap-4"
    onsubmit={(e) => {
      e.preventDefault();
      handleSubmit();
    }}
  >
    <FormField icon="🌐" label={$t("setup.displayLanguage")} labelFor="language">
      <Select 
        id="language" 
        options={languages} 
        bind:value={settings.language}
        onchange={(val) => onLanguageChange(val)}
      />
    </FormField>

    <FormField icon="" label={$t("setup.timezoneSelect")} labelFor="timezone">
      <Select id="timezone" options={timezones} bind:value={settings.timezone} />
    </FormField>

    <FormField icon="⏰" label={$t("setup.timeFormat")} labelFor="time-format">
      <Radio name="time-format" options={timeFormats} bind:value={settings.time_format} />
    </FormField>

    <FormField icon="📅" label={$t("setup.dateFormat")} labelFor="date-format">
      <Radio name="date-format" options={dateFormats} bind:value={settings.date_format} />
    </FormField>
  </form>

  {#snippet footer()}
    <Button variant="ghost" onclick={() => goto("/setup/restore")}>
      ☁️ {$t("setup.cloudRestore.restoreFromCloud")}
    </Button>
    <Button variant="primary" onclick={handleSubmit} {loading}>
      {#if loading}
        {$t("setup.saving")}
      {:else}
        {$t("setup.nextStep")}
      {/if}
    </Button>
  {/snippet}
</SetupCard>
