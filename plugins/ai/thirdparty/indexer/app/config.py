"""
配置管理模块
"""
import os
from typing import List
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """应用配置"""
    
    # ========== 多用户模式 ==========
    # 是否启用多用户模式
    multi_user: bool = True
    
    # 用户 home 目录根路径
    homes_dir: str = "/data/homes"
    
    # 监控的子目录（相对于用户 home）
    watch_subdirs: str = "Pictures,Documents"
    
    # 用户同步间隔（秒）
    user_scan_interval: int = 3600
    
    # ========== 单用户模式（兼容） ==========
    # 监控目录（逗号分隔）- 单用户模式使用
    watch_dirs: str = "/data/images,/data/documents"
    
    # ========== 通用配置 ==========
    # 索引存储目录
    index_dir: str = "/data/index"
    
    # 并发处理数
    concurrent_workers: int = 2
    
    # 全量扫描间隔（秒）
    scan_interval: int = 3600
    
    # CPU 阈值（超过则暂停索引）
    cpu_threshold: int = 70
    
    # 推理设备
    device: str = "cpu"
    
    # 日志级别
    log_level: str = "info"
    
    # API 端口
    port: int = 8081
    
    # 批次处理
    batch_size: int = 50
    batch_delay: int = 5  # 秒
    
    # 支持的图片格式
    image_extensions: List[str] = [".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp"]
    
    # 支持的文档格式
    document_extensions: List[str] = [".pdf", ".docx", ".doc", ".txt", ".md"]
    
    class Config:
        env_prefix = ""
        case_sensitive = False
    
    @property
    def watch_dirs_list(self) -> List[str]:
        """获取监控目录列表（单用户模式）"""
        return [d.strip() for d in self.watch_dirs.split(",") if d.strip()]
    
    @property
    def watch_subdirs_list(self) -> List[str]:
        """获取监控子目录列表（多用户模式）"""
        return [d.strip() for d in self.watch_subdirs.split(",") if d.strip()]
    
    @property
    def db_path(self) -> str:
        """数据库路径（单用户模式）"""
        return os.path.join(self.index_dir, "index.db")
    
    @property
    def faiss_path(self) -> str:
        """Faiss 索引路径（单用户模式）"""
        return os.path.join(self.index_dir, "vectors.faiss")
    
    def get_user_db_path(self, username: str) -> str:
        """获取用户数据库路径"""
        return os.path.join(self.index_dir, username, "index.db")
    
    def get_user_index_dir(self, username: str) -> str:
        """获取用户索引目录"""
        return os.path.join(self.index_dir, username)
    
    def get_user_home_dir(self, username: str) -> str:
        """获取用户 home 目录"""
        return os.path.join(self.homes_dir, username)


# 全局配置实例
settings = Settings()
