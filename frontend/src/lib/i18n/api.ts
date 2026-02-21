// i18n API 服务
import { api } from "$shared/services/api";

// i18n 设置响应
export interface I18nSettings {
  region: string;
  detected_region: string;
  mirrors: Record<string, string>;
  lyric_source: string;
}

// i18n 设置请求
export interface I18nSettingsRequest {
  region?: string;
  mirrors?: Record<string, string>;
  lyric_source?: string;
}

// 镜像选项
export interface MirrorOption {
  name: string;
  url: string;
  priority?: number;
}

// 歌词源
export interface LyricSource {
  id: string;
  name: Record<string, string>;
}

// 语言选项
export interface LanguageOption {
  code: string;
  name: string;
  native_name: string;
}

// 区域选项
export interface RegionOption {
  code: string;
  name: Record<string, string>;
  description: Record<string, string>;
}

// 镜像服务信息
export interface MirrorServiceInfo {
  id: string;
  name: Record<string, string>;
}

// i18n 选项响应
export interface I18nOptionsResponse {
  languages: LanguageOption[];
  regions: RegionOption[];
  mirrors: Record<string, Record<string, MirrorOption[]>>;
  lyric_sources: Record<string, LyricSource[]>;
  services: MirrorServiceInfo[];
}

// 区域切换预览项
export interface RegionSwitchItem {
  service: string;
  service_name: Record<string, string>;
  current_url: string;
  new_url: string;
  enabled: boolean;
}

// 区域切换预览响应
export interface RegionSwitchPreviewResponse {
  items: RegionSwitchItem[];
}

// 区域检测响应
export interface RegionDetectResponse {
  region: string;
  language: string;
  suggested_region: string;
  suggested_language: string;
}

// 获取 i18n 设置
export async function getI18nSettings(): Promise<I18nSettings> {
  const res = await api.get<{ data: I18nSettings }>("/system/i18n");
  return res.data;
}

// 更新 i18n 设置
export async function updateI18nSettings(
  settings: I18nSettingsRequest
): Promise<I18nSettings> {
  const res = await api.put<{ data: I18nSettings }>(
    "/system/i18n",
    settings
  );
  return res.data;
}

// 获取 i18n 选项
export async function getI18nOptions(): Promise<I18nOptionsResponse> {
  const res = await api.get<{ data: I18nOptionsResponse }>(
    "/system/i18n/options"
  );
  return res.data;
}

// 检测区域
export async function detectRegion(): Promise<RegionDetectResponse> {
  const res = await api.get<{ data: RegionDetectResponse }>(
    "/system/i18n/detect"
  );
  return res.data;
}

// 预览区域切换
export async function previewRegionSwitch(
  fromRegion: string,
  toRegion: string
): Promise<RegionSwitchPreviewResponse> {
  const res = await api.post<{ data: RegionSwitchPreviewResponse }>(
    "/system/i18n/preview-switch",
    { from_region: fromRegion, to_region: toRegion }
  );
  return res.data;
}
