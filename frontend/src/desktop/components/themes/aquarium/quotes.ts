// 谚语库 - 水族馆泡泡中显示的文字
export const quotes: string[] = [
  "千里之行，始于足下",
  "学无止境",
  "知识就是力量",
  "时间就是金钱",
  "有志者事竟成",
  "滴水穿石",
  "熟能生巧",
  "一寸光阴一寸金",
  "书山有路勤为径",
  "天道酬勤",
  "厚积薄发",
  "宁静致远",
  "海纳百川",
  "自强不息",
  "勿以善小而不为",
  "三人行必有我师",
  "温故而知新",
  "业精于勤",
  "锲而不舍",
  "见贤思齐",
];

// 获取随机谚语
export function getRandomQuote(): string {
  return quotes[Math.floor(Math.random() * quotes.length)];
}

// 获取多个不重复的谚语
export function getRandomQuotes(count: number): string[] {
  const shuffled = [...quotes].sort(() => Math.random() - 0.5);
  return shuffled.slice(0, Math.min(count, quotes.length));
}
