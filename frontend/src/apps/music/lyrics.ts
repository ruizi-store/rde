// 歌词服务
// 支持内嵌歌词、在线API、本地LRC文件

import { api } from "$shared/services/api";

// 歌词行
export interface LyricLine {
  time: number;      // 时间（秒）
  text: string;      // 歌词文本
  translation?: string; // 翻译（可选）
}

// 歌词数据
export interface LyricsData {
  source: "embedded" | "online" | "lrc_file" | "none";
  format: "lrc" | "plain" | "";
  lines: LyricLine[];
  raw: string;       // 原始歌词文本
}

// 后端响应
interface LyricsResponse {
  success: number;
  data: {
    source: string;
    lyrics: string;
    format: string;
  };
}

// 在线歌词API响应（以网易云为例）
interface OnlineLyricsResult {
  lyrics: string;
  translation?: string;
}

class LyricsService {
  private cache = new Map<string, LyricsData>();
  
  // 获取歌词（优先级：内嵌 > 在线API > 本地LRC）
  // 所有来源都由后端统一处理
  async getLyrics(
    filePath: string,
    title?: string,
    artist?: string
  ): Promise<LyricsData> {
    const cacheKey = filePath;
    
    // 检查缓存
    if (this.cache.has(cacheKey)) {
      return this.cache.get(cacheKey)!;
    }
    
    // 从后端获取（内嵌歌词、本地LRC或在线搜索）
    try {
      const result = await this.fetchFromBackend(filePath);
      this.cache.set(cacheKey, result);
      return result;
    } catch (e) {
      console.warn("Failed to fetch lyrics:", e);
    }
    
    // 没有找到歌词
    const emptyResult: LyricsData = {
      source: "none",
      format: "",
      lines: [],
      raw: "",
    };
    this.cache.set(cacheKey, emptyResult);
    return emptyResult;
  }
  
  // 从后端获取歌词
  private async fetchFromBackend(filePath: string): Promise<LyricsData> {
    const response = await api.get<LyricsResponse>("/files/audio/lyrics", { path: filePath });
    
    if (response.success !== 200 || !response.data) {
      return { source: "none", format: "", lines: [], raw: "" };
    }
    
    const { source, lyrics, format } = response.data;
    
    if (!lyrics || source === "none") {
      return { source: "none", format: "", lines: [], raw: "" };
    }
    
    const lines = format === "lrc" ? this.parseLRC(lyrics) : this.parsePlain(lyrics);
    
    return {
      source: source as LyricsData["source"],
      format: format as LyricsData["format"],
      lines,
      raw: lyrics,
    };
  }
  
  // 解析LRC格式歌词
  parseLRC(lrc: string): LyricLine[] {
    const lines: LyricLine[] = [];
    const lrcLines = lrc.split(/\r?\n/);
    
    // LRC时间戳正则: [mm:ss.xx] 或 [mm:ss:xx] 或 [mm:ss]
    const timeRegex = /\[(\d{1,2}):(\d{2})(?:[.:](\d{1,3}))?\]/g;
    
    for (const line of lrcLines) {
      const trimmed = line.trim();
      if (!trimmed) continue;
      
      // 跳过元数据标签 [ti:xxx] [ar:xxx] 等
      if (/^\[[a-z]{2}:/.test(trimmed)) continue;
      
      // 提取所有时间戳
      const times: number[] = [];
      let match: RegExpExecArray | null;
      let lastIndex = 0;
      
      while ((match = timeRegex.exec(trimmed)) !== null) {
        const minutes = parseInt(match[1], 10);
        const seconds = parseInt(match[2], 10);
        const ms = match[3] ? parseInt(match[3].padEnd(3, "0"), 10) : 0;
        times.push(minutes * 60 + seconds + ms / 1000);
        lastIndex = match.index + match[0].length;
      }
      
      // 提取歌词文本
      const text = trimmed.slice(lastIndex).trim();
      if (!text || times.length === 0) continue;
      
      // 为每个时间戳创建歌词行
      for (const time of times) {
        lines.push({ time, text });
      }
    }
    
    // 按时间排序
    lines.sort((a, b) => a.time - b.time);
    
    return lines;
  }
  
  // 解析纯文本歌词（无时间戳）
  parsePlain(text: string): LyricLine[] {
    return text
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter((line) => line.length > 0)
      .map((text, index) => ({
        time: -1, // 无时间戳
        text,
      }));
  }
  
  // 根据当前时间获取当前歌词行索引
  getCurrentLineIndex(lines: LyricLine[], currentTime: number): number {
    if (lines.length === 0) return -1;
    
    // 如果是纯文本歌词（无时间戳），返回-1
    if (lines[0].time < 0) return -1;
    
    // 二分查找
    let left = 0;
    let right = lines.length - 1;
    
    while (left <= right) {
      const mid = Math.floor((left + right) / 2);
      if (lines[mid].time <= currentTime) {
        left = mid + 1;
      } else {
        right = mid - 1;
      }
    }
    
    return right;
  }
  
  // 清除缓存
  clearCache(filePath?: string) {
    if (filePath) {
      this.cache.delete(filePath);
    } else {
      this.cache.clear();
    }
  }
}

export const lyricsService = new LyricsService();
