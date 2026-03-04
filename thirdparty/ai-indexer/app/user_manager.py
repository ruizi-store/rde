"""
用户管理模块
负责多用户环境下的用户生命周期管理
"""
import os
import json
import time
import logging
import threading
from typing import Dict, List, Optional, Set
from dataclasses import dataclass, asdict
from pathlib import Path
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler, DirCreatedEvent, DirDeletedEvent

from .config import settings

logger = logging.getLogger(__name__)


@dataclass
class UserInfo:
    """用户信息"""
    username: str
    created_at: int
    home_dir: str
    index_dir: str
    status: str = "active"  # active, inactive, deleted
    last_scan: Optional[int] = None
    file_count: int = 0


class UserRegistry:
    """用户注册表 - JSON 持久化"""
    
    def __init__(self, registry_path: str = None):
        self.registry_path = registry_path or os.path.join(
            settings.index_dir, "_system", "users.json"
        )
        self._users: Dict[str, UserInfo] = {}
        self._lock = threading.Lock()
        self._ensure_dir()
        self._load()
    
    def _ensure_dir(self):
        """确保目录存在"""
        os.makedirs(os.path.dirname(self.registry_path), exist_ok=True)
    
    def _load(self):
        """从文件加载"""
        if os.path.exists(self.registry_path):
            try:
                with open(self.registry_path, 'r') as f:
                    data = json.load(f)
                    for username, info in data.items():
                        self._users[username] = UserInfo(**info)
                logger.info(f"Loaded {len(self._users)} users from registry")
            except Exception as e:
                logger.error(f"Failed to load user registry: {e}")
    
    def _save(self):
        """保存到文件"""
        try:
            with open(self.registry_path, 'w') as f:
                data = {u: asdict(info) for u, info in self._users.items()}
                json.dump(data, f, indent=2)
        except Exception as e:
            logger.error(f"Failed to save user registry: {e}")
    
    def register(self, username: str) -> UserInfo:
        """注册用户"""
        with self._lock:
            if username in self._users:
                return self._users[username]
            
            user_info = UserInfo(
                username=username,
                created_at=int(time.time()),
                home_dir=os.path.join(settings.homes_dir, username),
                index_dir=os.path.join(settings.index_dir, username)
            )
            
            # 创建索引目录
            os.makedirs(user_info.index_dir, exist_ok=True)
            
            self._users[username] = user_info
            self._save()
            
            logger.info(f"Registered user: {username}")
            return user_info
    
    def unregister(self, username: str):
        """注销用户"""
        with self._lock:
            if username not in self._users:
                return
            
            user_info = self._users[username]
            user_info.status = "deleted"
            
            # 删除索引目录
            import shutil
            if os.path.exists(user_info.index_dir):
                shutil.rmtree(user_info.index_dir)
            
            del self._users[username]
            self._save()
            
            logger.info(f"Unregistered user: {username}")
    
    def get(self, username: str) -> Optional[UserInfo]:
        """获取用户信息"""
        return self._users.get(username)
    
    def list_users(self) -> List[str]:
        """列出所有用户"""
        return list(self._users.keys())
    
    def list_user_infos(self) -> List[UserInfo]:
        """列出所有用户信息"""
        return list(self._users.values())
    
    def update(self, username: str, **kwargs):
        """更新用户信息"""
        with self._lock:
            if username in self._users:
                for key, value in kwargs.items():
                    if hasattr(self._users[username], key):
                        setattr(self._users[username], key, value)
                self._save()
    
    def exists(self, username: str) -> bool:
        """检查用户是否存在"""
        return username in self._users


class UserWatcher(FileSystemEventHandler):
    """用户目录监控器 - 监控 homes 目录变化"""
    
    def __init__(self, on_user_added=None, on_user_removed=None):
        super().__init__()
        self.on_user_added = on_user_added
        self.on_user_removed = on_user_removed
    
    def on_created(self, event):
        """目录创建事件"""
        if not event.is_directory:
            return
        
        # 只处理 homes 直接子目录
        parent = os.path.dirname(event.src_path)
        if parent != settings.homes_dir:
            return
        
        username = os.path.basename(event.src_path)
        if username.startswith('.') or username.startswith('_'):
            return
        
        logger.info(f"User directory created: {username}")
        if self.on_user_added:
            self.on_user_added(username)
    
    def on_deleted(self, event):
        """目录删除事件"""
        if not event.is_directory:
            return
        
        parent = os.path.dirname(event.src_path)
        if parent != settings.homes_dir:
            return
        
        username = os.path.basename(event.src_path)
        logger.info(f"User directory deleted: {username}")
        if self.on_user_removed:
            self.on_user_removed(username)


class UserManager:
    """用户管理器 - 管理用户生命周期"""
    
    def __init__(self):
        self.registry = UserRegistry()
        self.observer: Optional[Observer] = None
        self._file_watchers: Dict[str, any] = {}  # 每用户的文件监控
        self._databases: Dict[str, any] = {}       # 每用户的数据库
        self._vector_indices: Dict[str, any] = {}  # 每用户的向量索引
        self._task_queues: Dict[str, any] = {}     # 每用户的任务队列
    
    def start(self):
        """启动用户管理器"""
        logger.info("Starting user manager...")
        
        # 初始同步
        self.sync_users()
        
        # 启动用户目录监控
        if os.path.isdir(settings.homes_dir):
            handler = UserWatcher(
                on_user_added=self._on_user_added,
                on_user_removed=self._on_user_removed
            )
            self.observer = Observer()
            self.observer.schedule(handler, settings.homes_dir, recursive=False)
            self.observer.start()
            logger.info(f"Watching for user changes in: {settings.homes_dir}")
        
        logger.info("User manager started")
    
    def stop(self):
        """停止用户管理器"""
        if self.observer:
            self.observer.stop()
            self.observer.join()
        
        # 停止所有用户的文件监控
        for username, watcher in self._file_watchers.items():
            if hasattr(watcher, 'stop'):
                watcher.stop()
        
        self._file_watchers.clear()
        self._databases.clear()
        self._vector_indices.clear()
        
        logger.info("User manager stopped")
    
    def sync_users(self):
        """同步用户列表"""
        if not os.path.isdir(settings.homes_dir):
            logger.warning(f"Homes directory not found: {settings.homes_dir}")
            return
        
        # 扫描 homes 目录
        current_users = set()
        for name in os.listdir(settings.homes_dir):
            if name.startswith('.') or name.startswith('_'):
                continue
            full_path = os.path.join(settings.homes_dir, name)
            if os.path.isdir(full_path):
                current_users.add(name)
        
        # 获取已注册用户
        registered_users = set(self.registry.list_users())
        
        # 新增用户
        for username in current_users - registered_users:
            self._on_user_added(username)
        
        # 删除用户
        for username in registered_users - current_users:
            self._on_user_removed(username)
        
        logger.info(f"User sync completed: {len(current_users)} users")
    
    def _on_user_added(self, username: str):
        """用户添加回调"""
        logger.info(f"Adding user: {username}")
        
        # 注册用户
        user_info = self.registry.register(username)
        
        # 初始化用户资源（延迟到首次请求时）
        # self._init_user_resources(username)
    
    def _on_user_removed(self, username: str):
        """用户删除回调"""
        logger.info(f"Removing user: {username}")
        
        # 停止用户资源
        self._cleanup_user_resources(username)
        
        # 注销用户
        self.registry.unregister(username)
    
    def _init_user_resources(self, username: str):
        """初始化用户资源（数据库、索引等）"""
        from .database import Database
        from .vector_index import VectorIndex
        from .queue import TaskQueue
        
        user_info = self.registry.get(username)
        if not user_info:
            return
        
        # 创建用户数据库
        db_path = os.path.join(user_info.index_dir, "index.db")
        self._databases[username] = Database(db_path)
        
        # 创建用户向量索引
        self._vector_indices[username] = VectorIndex(user_info.index_dir)
        
        # 创建用户任务队列
        self._task_queues[username] = TaskQueue()
        
        logger.info(f"Initialized resources for user: {username}")
    
    def _cleanup_user_resources(self, username: str):
        """清理用户资源"""
        # 停止文件监控
        if username in self._file_watchers:
            watcher = self._file_watchers.pop(username)
            if hasattr(watcher, 'stop'):
                watcher.stop()
        
        # 清理数据库连接
        if username in self._databases:
            del self._databases[username]
        
        # 清理向量索引
        if username in self._vector_indices:
            del self._vector_indices[username]
        
        # 清理任务队列
        if username in self._task_queues:
            del self._task_queues[username]
        
        logger.info(f"Cleaned up resources for user: {username}")
    
    def get_database(self, username: str):
        """获取用户数据库"""
        if username not in self._databases:
            self._init_user_resources(username)
        return self._databases.get(username)
    
    def get_vector_index(self, username: str):
        """获取用户向量索引"""
        if username not in self._vector_indices:
            self._init_user_resources(username)
        return self._vector_indices.get(username)
    
    def get_task_queue(self, username: str):
        """获取用户任务队列"""
        if username not in self._task_queues:
            self._init_user_resources(username)
        return self._task_queues.get(username)
    
    def get_user_home(self, username: str) -> Optional[str]:
        """获取用户 home 目录"""
        user_info = self.registry.get(username)
        return user_info.home_dir if user_info else None
    
    def get_user_watch_dirs(self, username: str) -> List[str]:
        """获取用户需要监控的目录"""
        user_info = self.registry.get(username)
        if not user_info:
            return []
        
        watch_dirs = []
        for subdir in settings.watch_subdirs_list:
            full_path = os.path.join(user_info.home_dir, subdir)
            if os.path.isdir(full_path):
                watch_dirs.append(full_path)
        
        return watch_dirs
    
    def validate_user(self, username: str) -> bool:
        """验证用户是否有效"""
        if not username:
            return False
        
        # 检查注册表
        if self.registry.exists(username):
            return True
        
        # 检查目录是否存在（可能刚创建还未同步）
        home_dir = os.path.join(settings.homes_dir, username)
        if os.path.isdir(home_dir):
            # 自动注册
            self._on_user_added(username)
            return True
        
        return False
    
    def get_stats(self) -> Dict:
        """获取统计信息"""
        users = self.registry.list_user_infos()
        return {
            "total_users": len(users),
            "active_users": len([u for u in users if u.status == "active"]),
            "users": [
                {
                    "username": u.username,
                    "status": u.status,
                    "file_count": u.file_count,
                    "last_scan": u.last_scan
                }
                for u in users
            ]
        }


# 全局用户管理器实例
user_manager = UserManager()
