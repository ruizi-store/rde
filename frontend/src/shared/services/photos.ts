// 相册服务
// 处理照片、相册、图库等操作

import { api } from "./api";

// ============ 类型定义 ============

export interface Photo {
  id: string;
  library_id: string;
  path: string;
  filename: string;
  hash: string;
  size: number;
  width: number;
  height: number;
  mime_type: string;
  type: "photo" | "video";
  taken_at: string | null;
  timezone: string;
  duration: number;
  // EXIF
  camera_make: string;
  camera_model: string;
  lens_make: string;
  lens_model: string;
  f_number: number;
  exposure_time: string;
  iso: number;
  focal_length: number;
  orientation: number;
  // GPS
  latitude: number | null;
  longitude: number | null;
  altitude: number | null;
  city: string;
  country: string;
  // 状态
  status: "pending" | "indexed" | "failed";
  is_favorite: boolean;
  is_archived: boolean;
  is_deleted: boolean;
  deleted_at: string | null;
  // 时间戳
  created_at: string;
  updated_at: string;
  indexed_at: string | null;
  // URLs
  thumbnail_url: string;
  preview_url: string;
  original_url: string;
}

export interface Album {
  id: string;
  name: string;
  description: string;
  cover_id: string;
  cover_url?: string;
  type: "manual" | "auto" | "shared";
  sort_order: string;
  user_id: string;
  is_public: boolean;
  share_token?: string;
  photo_count: number;
  created_at: string;
  updated_at: string;
}

export interface Library {
  id: string;
  name: string;
  path: string;
  user_id: string;
  scan_enabled: boolean;
  last_scan_at: string | null;
  photo_count: number;
  video_count: number;
  total_size: number;
  created_at: string;
  updated_at: string;
  scanning?: boolean;
}

export interface TimelineGroup {
  date: string;
  count: number;
  photos: Photo[];
}

export interface CalendarDay {
  date: string;
  count: number;
}

export interface ScanProgress {
  library_id: string;
  status: "idle" | "scanning" | "indexing" | "completed" | "failed";
  total_files: number;
  scanned_files: number;
  indexed_files: number;
  failed_files: number;
  started_at?: string;
  completed_at?: string;
  error?: string;
}

export interface PhotoStats {
  total_photos: number;
  total_videos: number;
  total_size: number;
  total_albums: number;
  favorite_count: number;
  archived_count: number;
  trash_count: number;
}

export interface ListPhotosResponse {
  photos: Photo[];
  total: number;
  offset: number;
  limit: number;
}

export interface TimelineResponse {
  groups: TimelineGroup[];
  total: number;
}

export interface CalendarResponse {
  days: CalendarDay[];
}

// ============ 服务类 ============

class PhotoService {
  // ============ 图库管理 ============

  async createLibrary(name: string, path: string, scanEnabled: boolean = true): Promise<Library> {
    return api.post<Library>("/photos/libraries", { name, path, scan_enabled: scanEnabled });
  }

  async listLibraries(): Promise<Library[]> {
    return api.get<Library[]>("/photos/libraries");
  }

  async getLibrary(id: string): Promise<Library> {
    return api.get<Library>(`/photos/libraries/${id}`);
  }

  async updateLibrary(id: string, data: Partial<{ name: string; scan_enabled: boolean }>): Promise<Library> {
    return api.put<Library>(`/photos/libraries/${id}`, data);
  }

  async deleteLibrary(id: string): Promise<void> {
    return api.delete(`/photos/libraries/${id}`);
  }

  async scanLibrary(id: string): Promise<void> {
    return api.post(`/photos/libraries/${id}/scan`);
  }

  async getScanProgress(id: string): Promise<ScanProgress> {
    return api.get<ScanProgress>(`/photos/libraries/${id}/progress`);
  }

  // ============ 照片 ============

  async listPhotos(params: {
    library_id?: string;
    album_id?: string;
    type?: "photo" | "video" | "all";
    favorite?: boolean;
    archived?: boolean;
    start_date?: string;
    end_date?: string;
    offset?: number;
    limit?: number;
    sort?: "date_asc" | "date_desc" | "name_asc" | "name_desc";
  } = {}): Promise<ListPhotosResponse> {
    return api.get<ListPhotosResponse>("/photos", params);
  }

  async getPhoto(id: string): Promise<Photo> {
    return api.get<Photo>(`/photos/${id}`);
  }

  async updatePhoto(id: string, data: Partial<{ is_favorite: boolean; is_archived: boolean; taken_at: string }>): Promise<Photo> {
    return api.put<Photo>(`/photos/${id}`, data);
  }

  async deletePhoto(id: string, force: boolean = false): Promise<void> {
    return api.delete(`/photos/${id}`, { force });
  }

  getThumbnailUrl(id: string): string {
    return `/api/v1/photos/${id}/thumbnail`;
  }

  getPreviewUrl(id: string): string {
    return `/api/v1/photos/${id}/preview`;
  }

  getOriginalUrl(id: string): string {
    return `/api/v1/photos/${id}/original`;
  }

  // ============ 批量操作 ============

  async batchDelete(photoIds: string[], force: boolean = false): Promise<{ deleted: number }> {
    return api.post<{ deleted: number }>("/photos/batch/delete", { photo_ids: photoIds, force });
  }

  async batchFavorite(photoIds: string[], isFavorite: boolean): Promise<{ updated: number }> {
    return api.post<{ updated: number }>("/photos/batch/favorite", { photo_ids: photoIds, is_favorite: isFavorite });
  }

  async batchArchive(photoIds: string[], isArchived: boolean): Promise<{ updated: number }> {
    return api.post<{ updated: number }>("/photos/batch/archive", { photo_ids: photoIds, is_archived: isArchived });
  }

  // ============ 相册 ============

  async createAlbum(name: string, description?: string, photoIds?: string[]): Promise<Album> {
    return api.post<Album>("/photos/albums", { name, description, photo_ids: photoIds });
  }

  async listAlbums(): Promise<Album[]> {
    return api.get<Album[]>("/photos/albums");
  }

  async getAlbum(id: string): Promise<Album> {
    return api.get<Album>(`/photos/albums/${id}`);
  }

  async updateAlbum(id: string, data: Partial<{ name: string; description: string; cover_id: string; sort_order: string }>): Promise<Album> {
    return api.put<Album>(`/photos/albums/${id}`, data);
  }

  async deleteAlbum(id: string): Promise<void> {
    return api.delete(`/photos/albums/${id}`);
  }

  async getAlbumPhotos(id: string, offset: number = 0, limit: number = 50): Promise<ListPhotosResponse> {
    return api.get<ListPhotosResponse>(`/photos/albums/${id}/photos`, { offset, limit });
  }

  async addPhotosToAlbum(albumId: string, photoIds: string[]): Promise<{ added: number }> {
    return api.post<{ added: number }>(`/photos/albums/${albumId}/photos`, { photo_ids: photoIds });
  }

  async removePhotoFromAlbum(albumId: string, photoId: string): Promise<void> {
    return api.delete(`/photos/albums/${albumId}/photos/${photoId}`);
  }

  // ============ 时间线 ============

  async getTimeline(params: {
    library_id?: string;
    group_by?: "day" | "month" | "year";
    start_date?: string;
    end_date?: string;
  } = {}): Promise<TimelineResponse> {
    return api.get<TimelineResponse>("/photos/timeline", params);
  }

  async getCalendar(year: number, month: number, libraryId?: string): Promise<CalendarResponse> {
    const params: { year: number; month: number; library_id?: string } = { year, month };
    if (libraryId) {
      params.library_id = libraryId;
    }
    return api.get<CalendarResponse>("/photos/calendar", params);
  }

  // ============ 回收站 ============

  async listTrash(offset: number = 0, limit: number = 50): Promise<ListPhotosResponse> {
    return api.get<ListPhotosResponse>("/photos/trash", { offset, limit });
  }

  async restorePhoto(id: string): Promise<void> {
    return api.post(`/photos/trash/${id}/restore`);
  }

  async emptyTrash(): Promise<void> {
    return api.delete("/photos/trash");
  }

  // ============ 统计 ============

  async getStats(): Promise<PhotoStats> {
    return api.get<PhotoStats>("/photos/stats");
  }

  // ============ AI 智能搜索 ============

  async getAIStatus(): Promise<AIStatusResponse> {
    return api.get<AIStatusResponse>("/photos/ai/status");
  }

  async aiSearch(query: string, limit: number = 50): Promise<AISearchResponse> {
    return api.get<AISearchResponse>("/photos/ai/search", { q: query, limit });
  }

  async aiSearchFace(params: FaceSearchParams): Promise<AISearchResponse> {
    const queryParams: Record<string, string | number | boolean> = {};
    if (params.age_min !== undefined) queryParams.age_min = params.age_min;
    if (params.age_max !== undefined) queryParams.age_max = params.age_max;
    if (params.gender) queryParams.gender = params.gender;
    if (params.limit) queryParams.limit = params.limit;
    return api.get<AISearchResponse>("/photos/ai/search/face", queryParams);
  }

  async aiSearchText(query: string, limit: number = 50): Promise<AISearchResponse> {
    return api.get<AISearchResponse>("/photos/ai/search/text", { q: query, limit });
  }

  async triggerAIIndex(dir?: string): Promise<{ message: string }> {
    const params = dir ? { dir } : {};
    return api.post("/photos/ai/index", params);
  }
}

// ============ AI 类型 ============

export interface AIStatusResponse {
  enabled: boolean;
  message?: string;
  error?: string;
  status?: {
    user: string;
    status: string;
    database: Record<string, unknown>;
    queue: Record<string, unknown>;
    vectors: Record<string, unknown>;
  };
}

export interface AISearchResponse {
  total: number;
  photos: Photo[];
}

export interface FaceSearchParams {
  age_min?: number;
  age_max?: number;
  gender?: "male" | "female";
  limit?: number;
}

export const photoService = new PhotoService();
