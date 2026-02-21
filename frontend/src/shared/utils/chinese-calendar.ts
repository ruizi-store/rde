/**
 * 中国日历工具库
 * 包含农历转换、节假日、调休日期等
 */

// 农历数据 (1900-2100)
// 每年用一个16位数表示：
// - 第1-12位：表示12个月的大小月，1为大月(30天)，0为小月(29天)
// - 第13-16位：表示闰月月份，0表示无闰月
const LUNAR_INFO = [
  0x04bd8, 0x04ae0, 0x0a570, 0x054d5, 0x0d260, 0x0d950, 0x16554, 0x056a0, 0x09ad0, 0x055d2, 0x04ae0,
  0x0a5b6, 0x0a4d0, 0x0d250, 0x1d255, 0x0b540, 0x0d6a0, 0x0ada2, 0x095b0, 0x14977, 0x04970, 0x0a4b0,
  0x0b4b5, 0x06a50, 0x06d40, 0x1ab54, 0x02b60, 0x09570, 0x052f2, 0x04970, 0x06566, 0x0d4a0, 0x0ea50,
  0x06e95, 0x05ad0, 0x02b60, 0x186e3, 0x092e0, 0x1c8d7, 0x0c950, 0x0d4a0, 0x1d8a6, 0x0b550, 0x056a0,
  0x1a5b4, 0x025d0, 0x092d0, 0x0d2b2, 0x0a950, 0x0b557, 0x06ca0, 0x0b550, 0x15355, 0x04da0, 0x0a5b0,
  0x14573, 0x052b0, 0x0a9a8, 0x0e950, 0x06aa0, 0x0aea6, 0x0ab50, 0x04b60, 0x0aae4, 0x0a570, 0x05260,
  0x0f263, 0x0d950, 0x05b57, 0x056a0, 0x096d0, 0x04dd5, 0x04ad0, 0x0a4d0, 0x0d4d4, 0x0d250, 0x0d558,
  0x0b540, 0x0b6a0, 0x195a6, 0x095b0, 0x049b0, 0x0a974, 0x0a4b0, 0x0b27a, 0x06a50, 0x06d40, 0x0af46,
  0x0ab60, 0x09570, 0x04af5, 0x04970, 0x064b0, 0x074a3, 0x0ea50, 0x06b58, 0x05ac0, 0x0ab60, 0x096d5,
  0x092e0, 0x0c960, 0x0d954, 0x0d4a0, 0x0da50, 0x07552, 0x056a0, 0x0abb7, 0x025d0, 0x092d0, 0x0cab5,
  0x0a950, 0x0b4a0, 0x0baa4, 0x0ad50, 0x055d9, 0x04ba0, 0x0a5b0, 0x15176, 0x052b0, 0x0a930, 0x07954,
  0x06aa0, 0x0ad50, 0x05b52, 0x04b60, 0x0a6e6, 0x0a4e0, 0x0d260, 0x0ea65, 0x0d530, 0x05aa0, 0x076a3,
  0x096d0, 0x04afb, 0x04ad0, 0x0a4d0, 0x1d0b6, 0x0d250, 0x0d520, 0x0dd45, 0x0b5a0, 0x056d0, 0x055b2,
  0x049b0, 0x0a577, 0x0a4b0, 0x0aa50, 0x1b255, 0x06d20, 0x0ada0, 0x14b63, 0x09370, 0x049f8, 0x04970,
  0x064b0, 0x168a6, 0x0ea50, 0x06b20, 0x1a6c4, 0x0aae0, 0x0a2e0, 0x0d2e3, 0x0c960, 0x0d557, 0x0d4a0,
  0x0da50, 0x05d55, 0x056a0, 0x0a6d0, 0x055d4, 0x052d0, 0x0a9b8, 0x0a950, 0x0b4a0, 0x0b6a6, 0x0ad50,
  0x055a0, 0x0aba4, 0x0a5b0, 0x052b0, 0x0b273, 0x06930, 0x07337, 0x06aa0, 0x0ad50, 0x14b55, 0x04b60,
  0x0a570, 0x054e4, 0x0d160, 0x0e968, 0x0d520, 0x0daa0, 0x16aa6, 0x056d0, 0x04ae0, 0x0a9d4, 0x0a2d0,
  0x0d150, 0x0f252, 0x0d520,
];

// 天干
const HEAVENLY_STEMS = ["甲", "乙", "丙", "丁", "戊", "己", "庚", "辛", "壬", "癸"];

// 地支
const EARTHLY_BRANCHES = ["子", "丑", "寅", "卯", "辰", "巳", "午", "未", "申", "酉", "戌", "亥"];

// 生肖
const ZODIAC_ANIMALS = ["鼠", "牛", "虎", "兔", "龙", "蛇", "马", "羊", "猴", "鸡", "狗", "猪"];

// 农历月份
const LUNAR_MONTHS = ["正", "二", "三", "四", "五", "六", "七", "八", "九", "十", "冬", "腊"];

// 农历日期
const LUNAR_DAYS = [
  "初一",
  "初二",
  "初三",
  "初四",
  "初五",
  "初六",
  "初七",
  "初八",
  "初九",
  "初十",
  "十一",
  "十二",
  "十三",
  "十四",
  "十五",
  "十六",
  "十七",
  "十八",
  "十九",
  "二十",
  "廿一",
  "廿二",
  "廿三",
  "廿四",
  "廿五",
  "廿六",
  "廿七",
  "廿八",
  "廿九",
  "三十",
];

/**
 * 中国法定节假日数据 (2025-2027)
 * 数据来源：国务院办公厅发布的节假日安排
 * holiday: 放假日期
 * workday: 调休上班日期
 */
export const HOLIDAYS: Record<string, { name: string; holiday: string[]; workday: string[] }> = {
  // 2025年节假日
  "2025": {
    name: "2025年",
    holiday: [
      // 元旦 1.1
      "2025-01-01",
      // 春节 1.28-2.4 (农历除夕至初七)
      "2025-01-28",
      "2025-01-29",
      "2025-01-30",
      "2025-01-31",
      "2025-02-01",
      "2025-02-02",
      "2025-02-03",
      "2025-02-04",
      // 清明节 4.4-4.6
      "2025-04-04",
      "2025-04-05",
      "2025-04-06",
      // 劳动节 5.1-5.5
      "2025-05-01",
      "2025-05-02",
      "2025-05-03",
      "2025-05-04",
      "2025-05-05",
      // 端午节 5.31-6.2
      "2025-05-31",
      "2025-06-01",
      "2025-06-02",
      // 中秋节+国庆节 10.1-10.8
      "2025-10-01",
      "2025-10-02",
      "2025-10-03",
      "2025-10-04",
      "2025-10-05",
      "2025-10-06",
      "2025-10-07",
      "2025-10-08",
    ],
    workday: [
      // 春节调休
      "2025-01-26", // 周日
      "2025-02-08", // 周六
      // 劳动节调休
      "2025-04-27", // 周日
      // 国庆调休
      "2025-09-28", // 周日
      "2025-10-11", // 周六
    ],
  },
  // 2026年节假日 (预估，实际以国务院公布为准)
  "2026": {
    name: "2026年",
    holiday: [
      // 元旦 1.1-1.3
      "2026-01-01",
      "2026-01-02",
      "2026-01-03",
      // 春节 2.16-2.22 (农历除夕至初六)
      "2026-02-16",
      "2026-02-17",
      "2026-02-18",
      "2026-02-19",
      "2026-02-20",
      "2026-02-21",
      "2026-02-22",
      // 清明节 4.4-4.6
      "2026-04-04",
      "2026-04-05",
      "2026-04-06",
      // 劳动节 5.1-5.5
      "2026-05-01",
      "2026-05-02",
      "2026-05-03",
      "2026-05-04",
      "2026-05-05",
      // 端午节 5.30-6.1
      "2026-05-30",
      "2026-05-31",
      "2026-06-01",
      // 中秋节 9.25-9.27
      "2026-09-25",
      "2026-09-26",
      "2026-09-27",
      // 国庆节 10.1-10.7
      "2026-10-01",
      "2026-10-02",
      "2026-10-03",
      "2026-10-04",
      "2026-10-05",
      "2026-10-06",
      "2026-10-07",
      "2026-10-08",
    ],
    workday: [
      // 春节调休
      "2026-02-14", // 周六
      "2026-02-28", // 周六
      // 国庆调休
      "2026-09-27", // 周日
      "2026-10-10", // 周六
    ],
  },
  // 2027年节假日 (预估)
  "2027": {
    name: "2027年",
    holiday: [
      // 元旦 1.1-1.3
      "2027-01-01",
      "2027-01-02",
      "2027-01-03",
      // 春节 2.5-2.11 (农历除夕至初六)
      "2027-02-05",
      "2027-02-06",
      "2027-02-07",
      "2027-02-08",
      "2027-02-09",
      "2027-02-10",
      "2027-02-11",
      // 清明节 4.3-4.5
      "2027-04-03",
      "2027-04-04",
      "2027-04-05",
      // 劳动节 5.1-5.5
      "2027-05-01",
      "2027-05-02",
      "2027-05-03",
      "2027-05-04",
      "2027-05-05",
      // 端午节 6.18-6.20
      "2027-06-18",
      "2027-06-19",
      "2027-06-20",
      // 中秋节 9.14-9.16
      "2027-09-14",
      "2027-09-15",
      "2027-09-16",
      // 国庆节 10.1-10.7
      "2027-10-01",
      "2027-10-02",
      "2027-10-03",
      "2027-10-04",
      "2027-10-05",
      "2027-10-06",
      "2027-10-07",
    ],
    workday: [
      // 春节调休
      "2027-02-07", // 周日
      "2027-02-20", // 周六
      // 国庆调休
      "2027-09-26", // 周日
      "2027-10-09", // 周六
    ],
  },
};

/**
 * 农历节日
 */
const LUNAR_FESTIVALS: Record<string, string> = {
  "1-1": "春节",
  "1-15": "元宵",
  "2-2": "龙抬头",
  "5-5": "端午",
  "7-7": "七夕",
  "7-15": "中元",
  "8-15": "中秋",
  "9-9": "重阳",
  "12-8": "腊八",
  "12-23": "小年",
  "12-30": "除夕",
};

/**
 * 公历节日
 */
const SOLAR_FESTIVALS: Record<string, string> = {
  "1-1": "元旦",
  "2-14": "情人节",
  "3-8": "妇女节",
  "3-12": "植树节",
  "4-1": "愚人节",
  "5-1": "劳动节",
  "5-4": "青年节",
  "6-1": "儿童节",
  "7-1": "建党节",
  "8-1": "建军节",
  "9-10": "教师节",
  "10-1": "国庆节",
  "12-25": "圣诞节",
};

/**
 * 获取某年的农历数据
 */
function getLunarYearInfo(year: number): number {
  return LUNAR_INFO[year - 1900] || 0;
}

/**
 * 获取某年农历的总天数
 */
function getLunarYearDays(year: number): number {
  let sum = 348; // 12个月，每月29天
  const info = getLunarYearInfo(year);
  for (let i = 0x8000; i > 0x8; i >>= 1) {
    sum += info & i ? 1 : 0;
  }
  return sum + getLeapMonthDays(year);
}

/**
 * 获取闰月天数
 */
function getLeapMonthDays(year: number): number {
  const leapMonth = getLeapMonth(year);
  if (leapMonth === 0) return 0;
  return getLunarYearInfo(year) & 0x10000 ? 30 : 29;
}

/**
 * 获取闰月月份，0表示无闰月
 */
function getLeapMonth(year: number): number {
  return getLunarYearInfo(year) & 0xf;
}

/**
 * 获取某年某月的农历天数
 */
function getLunarMonthDays(year: number, month: number): number {
  return getLunarYearInfo(year) & (0x10000 >> month) ? 30 : 29;
}

/**
 * 公历转农历
 */
export function solarToLunar(year: number, month: number, day: number): LunarDate {
  // 1900年1月31日是农历正月初一
  const baseDate = new Date(1900, 0, 31);
  const targetDate = new Date(year, month - 1, day);
  let offset = Math.floor((targetDate.getTime() - baseDate.getTime()) / 86400000);

  let lunarYear = 1900;
  let lunarMonth = 1;
  let lunarDay = 1;
  let isLeap = false;

  // 计算年
  for (let i = 1900; i < 2100 && offset > 0; i++) {
    const yearDays = getLunarYearDays(i);
    if (offset < yearDays) {
      lunarYear = i;
      break;
    }
    offset -= yearDays;
  }

  // 计算月
  const leapMonth = getLeapMonth(lunarYear);
  for (let i = 1; i <= 12; i++) {
    let monthDays: number;
    if (leapMonth > 0 && i === leapMonth + 1 && !isLeap) {
      monthDays = getLeapMonthDays(lunarYear);
      isLeap = true;
      i--;
    } else {
      monthDays = getLunarMonthDays(lunarYear, i);
    }

    if (offset < monthDays) {
      lunarMonth = i;
      lunarDay = offset + 1;
      break;
    }
    offset -= monthDays;
    if (isLeap && i === leapMonth + 1) {
      isLeap = false;
    }
  }

  // 计算干支年
  const ganzhiYear = getGanzhiYear(lunarYear);
  const zodiac = getZodiac(lunarYear);

  return {
    year: lunarYear,
    month: lunarMonth,
    day: lunarDay,
    isLeap,
    monthStr: (isLeap ? "闰" : "") + LUNAR_MONTHS[lunarMonth - 1] + "月",
    dayStr: LUNAR_DAYS[lunarDay - 1],
    ganzhiYear,
    zodiac,
  };
}

/**
 * 获取干支年
 */
function getGanzhiYear(year: number): string {
  const gan = HEAVENLY_STEMS[(year - 4) % 10];
  const zhi = EARTHLY_BRANCHES[(year - 4) % 12];
  return gan + zhi;
}

/**
 * 获取生肖
 */
function getZodiac(year: number): string {
  return ZODIAC_ANIMALS[(year - 4) % 12];
}

/**
 * 农历日期接口
 */
export interface LunarDate {
  year: number;
  month: number;
  day: number;
  isLeap: boolean;
  monthStr: string;
  dayStr: string;
  ganzhiYear: string;
  zodiac: string;
}

/**
 * 日期信息接口
 */
export interface DayInfo {
  date: Date;
  year: number;
  month: number;
  day: number;
  dayOfWeek: number;
  lunar: LunarDate;
  isToday: boolean;
  isWeekend: boolean;
  isHoliday: boolean;
  isWorkday: boolean; // 调休上班
  festival?: string; // 节日名称
  lunarFestival?: string; // 农历节日
}

/**
 * 格式化日期为 YYYY-MM-DD
 */
export function formatDate(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

/**
 * 检查是否为节假日
 */
export function isHoliday(date: Date): boolean {
  const year = date.getFullYear().toString();
  const dateStr = formatDate(date);
  return HOLIDAYS[year]?.holiday.includes(dateStr) ?? false;
}

/**
 * 检查是否为调休工作日
 */
export function isWorkday(date: Date): boolean {
  const year = date.getFullYear().toString();
  const dateStr = formatDate(date);
  return HOLIDAYS[year]?.workday.includes(dateStr) ?? false;
}

/**
 * 获取公历节日
 */
export function getSolarFestival(month: number, day: number): string | undefined {
  return SOLAR_FESTIVALS[`${month}-${day}`];
}

/**
 * 获取农历节日
 */
export function getLunarFestival(
  month: number,
  day: number,
  isLastDayOfYear: boolean = false,
): string | undefined {
  // 除夕特殊处理（腊月二十九也可能是除夕）
  if (month === 12 && (day === 30 || (day === 29 && isLastDayOfYear))) {
    return "除夕";
  }
  return LUNAR_FESTIVALS[`${month}-${day}`];
}

/**
 * 获取某一天的详细信息
 */
export function getDayInfo(date: Date): DayInfo {
  const year = date.getFullYear();
  const month = date.getMonth() + 1;
  const day = date.getDate();
  const dayOfWeek = date.getDay();

  const lunar = solarToLunar(year, month, day);
  const today = new Date();
  const isToday =
    year === today.getFullYear() && month === today.getMonth() + 1 && day === today.getDate();

  const isWeekend = dayOfWeek === 0 || dayOfWeek === 6;
  const holiday = isHoliday(date);
  const workday = isWorkday(date);

  const festival = getSolarFestival(month, day);
  const lunarFestival = getLunarFestival(lunar.month, lunar.day);

  return {
    date,
    year,
    month,
    day,
    dayOfWeek,
    lunar,
    isToday,
    isWeekend,
    isHoliday: holiday,
    isWorkday: workday,
    festival,
    lunarFestival,
  };
}

/**
 * 获取某月的所有日期信息
 */
export function getMonthDays(year: number, month: number): DayInfo[] {
  const days: DayInfo[] = [];

  // 获取这个月的第一天
  const firstDay = new Date(year, month - 1, 1);
  const firstDayOfWeek = firstDay.getDay();

  // 获取这个月的天数
  const daysInMonth = new Date(year, month, 0).getDate();

  // 填充上个月的日期
  const prevMonthDays = firstDayOfWeek === 0 ? 6 : firstDayOfWeek - 1;
  const prevMonth = month === 1 ? 12 : month - 1;
  const prevMonthYear = month === 1 ? year - 1 : year;
  const prevMonthTotalDays = new Date(prevMonthYear, prevMonth, 0).getDate();

  for (let i = prevMonthDays - 1; i >= 0; i--) {
    const date = new Date(prevMonthYear, prevMonth - 1, prevMonthTotalDays - i);
    days.push(getDayInfo(date));
  }

  // 填充本月日期
  for (let i = 1; i <= daysInMonth; i++) {
    const date = new Date(year, month - 1, i);
    days.push(getDayInfo(date));
  }

  // 填充下个月的日期（补齐到6行 = 42天）
  const remaining = 42 - days.length;
  const nextMonth = month === 12 ? 1 : month + 1;
  const nextMonthYear = month === 12 ? year + 1 : year;

  for (let i = 1; i <= remaining; i++) {
    const date = new Date(nextMonthYear, nextMonth - 1, i);
    days.push(getDayInfo(date));
  }

  return days;
}

/**
 * 获取下一个重要节日
 */
export function getNextFestival(from: Date = new Date()): { name: string; days: number } | null {
  const year = from.getFullYear();
  const holidays = HOLIDAYS[year.toString()];
  if (!holidays) return null;

  // 主要节日及其大致日期
  const festivals = [
    { name: "春节", pattern: /^(\d{4})-0[12]-/ },
    { name: "清明节", pattern: /^(\d{4})-04-0[3-6]$/ },
    { name: "劳动节", pattern: /^(\d{4})-05-0[1-5]$/ },
    { name: "端午节", pattern: /^(\d{4})-0[56]-/ },
    { name: "中秋节", pattern: /^(\d{4})-09-/ },
    { name: "国庆节", pattern: /^(\d{4})-10-0[1-8]$/ },
  ];

  // 简化版：计算到春节的天数
  const springFestivalDates: Record<string, string> = {
    "2025": "2025-01-29",
    "2026": "2026-02-17",
    "2027": "2027-02-06",
  };

  const springDate = springFestivalDates[year.toString()];
  if (springDate) {
    const spring = new Date(springDate);
    const diff = Math.ceil((spring.getTime() - from.getTime()) / 86400000);
    if (diff > 0) {
      return { name: "春节", days: diff };
    }
  }

  // 如果春节已过，计算到国庆
  const nationalDay = new Date(year, 9, 1);
  const diffNational = Math.ceil((nationalDay.getTime() - from.getTime()) / 86400000);
  if (diffNational > 0) {
    return { name: "国庆节", days: diffNational };
  }

  // 计算到明年春节
  const nextYear = (year + 1).toString();
  const nextSpringDate = springFestivalDates[nextYear];
  if (nextSpringDate) {
    const nextSpring = new Date(nextSpringDate);
    const diff = Math.ceil((nextSpring.getTime() - from.getTime()) / 86400000);
    return { name: "春节", days: diff };
  }

  return null;
}
