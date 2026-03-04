"""
任务队列模块
"""
import time
import threading
from typing import Optional, Callable, List
from dataclasses import dataclass
from queue import Queue, Empty
import logging

from .database import db, FileStatus

logger = logging.getLogger(__name__)


@dataclass
class Task:
    """索引任务"""
    file_id: int
    path: str
    file_type: str
    priority: int = 0  # 数字越大优先级越高


class TaskQueue:
    """任务队列管理"""
    
    def __init__(self):
        self._queue: Queue[Task] = Queue()
        self._processing: set = set()
        self._lock = threading.Lock()
        self._paused = False
    
    def add(self, task: Task):
        """添加任务"""
        with self._lock:
            if task.file_id not in self._processing:
                self._queue.put(task)
                logger.debug(f"Task added: {task.path}")
    
    def get(self, timeout: float = 1.0) -> Optional[Task]:
        """获取任务"""
        if self._paused:
            return None
        
        try:
            task = self._queue.get(timeout=timeout)
            with self._lock:
                self._processing.add(task.file_id)
            return task
        except Empty:
            return None
    
    def complete(self, task: Task):
        """完成任务"""
        with self._lock:
            self._processing.discard(task.file_id)
        self._queue.task_done()
    
    def pause(self):
        """暂停队列"""
        self._paused = True
        logger.info("Task queue paused")
    
    def resume(self):
        """恢复队列"""
        self._paused = False
        logger.info("Task queue resumed")
    
    @property
    def is_paused(self) -> bool:
        return self._paused
    
    @property
    def size(self) -> int:
        return self._queue.qsize()
    
    @property
    def processing_count(self) -> int:
        return len(self._processing)
    
    def load_pending_from_db(self):
        """从数据库加载待处理任务"""
        pending_files = db.get_pending_files(limit=1000)
        for f in pending_files:
            task = Task(
                file_id=f['id'],
                path=f['path'],
                file_type=f['type']
            )
            self.add(task)
        
        logger.info(f"Loaded {len(pending_files)} pending tasks from database")


# 全局任务队列
task_queue = TaskQueue()
