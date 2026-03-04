"""
文件监控模块
"""
import os
import time
import logging
import mimetypes
from pathlib import Path
from typing import List, Set
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler, FileCreatedEvent, FileModifiedEvent, FileDeletedEvent

from .config import settings
from .database import db, FileType
from .queue import task_queue, Task

logger = logging.getLogger(__name__)


def get_file_type(path: str) -> str:
    """根据扩展名判断文件类型"""
    ext = Path(path).suffix.lower()
    
    if ext in settings.image_extensions:
        return FileType.IMAGE
    elif ext in settings.document_extensions:
        return FileType.DOCUMENT
    else:
        return None


def is_supported_file(path: str) -> bool:
    """检查是否为支持的文件类型"""
    return get_file_type(path) is not None


class IndexerEventHandler(FileSystemEventHandler):
    """文件系统事件处理器"""
    
    def __init__(self):
        super().__init__()
        self._debounce: dict = {}  # 防抖
        self._debounce_interval = 1.0  # 秒
    
    def _should_process(self, path: str) -> bool:
        """检查是否应该处理（防抖）"""
        now = time.time()
        last_time = self._debounce.get(path, 0)
        
        if now - last_time < self._debounce_interval:
            return False
        
        self._debounce[path] = now
        return True
    
    def on_created(self, event):
        """文件创建事件"""
        if event.is_directory:
            return
        
        path = event.src_path
        if not is_supported_file(path):
            return
        
        if not self._should_process(path):
            return
        
        logger.info(f"File created: {path}")
        self._add_file(path)
    
    def on_modified(self, event):
        """文件修改事件"""
        if event.is_directory:
            return
        
        path = event.src_path
        if not is_supported_file(path):
            return
        
        if not self._should_process(path):
            return
        
        logger.info(f"File modified: {path}")
        self._add_file(path)
    
    def on_deleted(self, event):
        """文件删除事件"""
        if event.is_directory:
            return
        
        path = event.src_path
        logger.info(f"File deleted: {path}")
        db.delete_file(path)
    
    def _add_file(self, path: str):
        """添加文件到索引队列"""
        try:
            stat = os.stat(path)
            file_type = get_file_type(path)
            mime_type, _ = mimetypes.guess_type(path)
            
            # 添加到数据库
            file_id = db.add_file(
                path=path,
                file_type=file_type,
                mime_type=mime_type,
                size=stat.st_size,
                mtime=int(stat.st_mtime)
            )
            
            # 添加到任务队列
            task = Task(file_id=file_id, path=path, file_type=file_type)
            task_queue.add(task)
            
        except Exception as e:
            logger.error(f"Failed to add file {path}: {e}")


class FileWatcher:
    """文件监控器"""
    
    def __init__(self):
        self.observer = Observer()
        self.handler = IndexerEventHandler()
        self._running = False
    
    def start(self):
        """启动监控"""
        for watch_dir in settings.watch_dirs_list:
            if os.path.isdir(watch_dir):
                self.observer.schedule(self.handler, watch_dir, recursive=True)
                logger.info(f"Watching directory: {watch_dir}")
            else:
                logger.warning(f"Directory not found: {watch_dir}")
        
        self.observer.start()
        self._running = True
        logger.info("File watcher started")
    
    def stop(self):
        """停止监控"""
        self.observer.stop()
        self.observer.join()
        self._running = False
        logger.info("File watcher stopped")
    
    @property
    def is_running(self) -> bool:
        return self._running


def scan_directory(directory: str) -> int:
    """扫描目录，返回发现的文件数"""
    count = 0
    
    for root, dirs, files in os.walk(directory):
        # 跳过隐藏目录
        dirs[:] = [d for d in dirs if not d.startswith('.')]
        
        for filename in files:
            if filename.startswith('.'):
                continue
            
            path = os.path.join(root, filename)
            
            if not is_supported_file(path):
                continue
            
            try:
                # 检查是否已存在
                existing = db.get_file_by_path(path)
                stat = os.stat(path)
                
                if existing:
                    # 检查是否需要重新索引（修改时间不同）
                    if existing['mtime'] == int(stat.st_mtime) and existing['status'] == 'done':
                        continue
                
                # 添加到数据库和队列
                file_type = get_file_type(path)
                mime_type, _ = mimetypes.guess_type(path)
                
                file_id = db.add_file(
                    path=path,
                    file_type=file_type,
                    mime_type=mime_type,
                    size=stat.st_size,
                    mtime=int(stat.st_mtime)
                )
                
                task = Task(file_id=file_id, path=path, file_type=file_type)
                task_queue.add(task)
                count += 1
                
            except Exception as e:
                logger.error(f"Failed to scan file {path}: {e}")
    
    return count


def full_scan():
    """全量扫描所有监控目录"""
    total = 0
    for watch_dir in settings.watch_dirs_list:
        if os.path.isdir(watch_dir):
            logger.info(f"Scanning directory: {watch_dir}")
            count = scan_directory(watch_dir)
            total += count
            logger.info(f"Found {count} files in {watch_dir}")
    
    logger.info(f"Full scan completed, total {total} new/updated files")
    return total


# 全局文件监控器
watcher = FileWatcher()
