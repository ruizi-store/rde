"""
后台工作者模块
负责从队列获取任务并处理
"""
import os
import time
import logging
import threading
import psutil
from typing import Dict, Optional

from .config import settings
from .database import db, FileStatus, FileType
from .queue import task_queue, Task
from .processors import ImageProcessor, DocumentProcessor

logger = logging.getLogger(__name__)


class Worker:
    """后台工作者"""
    
    def __init__(self, worker_id: int):
        self.worker_id = worker_id
        self.image_processor = ImageProcessor()
        self.document_processor = DocumentProcessor()
        self._running = False
        self._thread: Optional[threading.Thread] = None
        self._processed_count = 0
        self._error_count = 0
    
    def start(self):
        """启动工作者线程"""
        if self._running:
            return
        
        self._running = True
        self._thread = threading.Thread(target=self._run, daemon=True)
        self._thread.start()
        logger.info(f"Worker {self.worker_id} started")
    
    def stop(self):
        """停止工作者"""
        self._running = False
        if self._thread:
            self._thread.join(timeout=5)
        logger.info(f"Worker {self.worker_id} stopped")
    
    def _run(self):
        """工作者主循环"""
        batch_count = 0
        
        while self._running:
            # 检查系统负载
            if self._should_throttle():
                time.sleep(5)
                continue
            
            # 获取任务
            task = task_queue.get(timeout=1.0)
            if task is None:
                continue
            
            try:
                self._process_task(task)
                self._processed_count += 1
                batch_count += 1
                
                # 批次处理后休息
                if batch_count >= settings.batch_size:
                    batch_count = 0
                    time.sleep(settings.batch_delay)
                    
            except Exception as e:
                logger.error(f"Worker {self.worker_id} error: {e}")
                self._error_count += 1
                
            finally:
                task_queue.complete(task)
    
    def _should_throttle(self) -> bool:
        """检查是否应该降速"""
        # CPU 负载过高
        cpu_percent = psutil.cpu_percent(interval=0.1)
        if cpu_percent > settings.cpu_threshold:
            logger.debug(f"CPU usage {cpu_percent}% > threshold, throttling")
            return True
        
        return False
    
    def _process_task(self, task: Task):
        """处理单个任务"""
        logger.debug(f"Processing: {task.path}")
        
        # 更新状态为处理中
        db.update_file_status(task.file_id, FileStatus.PROCESSING)
        
        try:
            # 检查文件是否存在
            if not os.path.exists(task.path):
                db.delete_file(task.path)
                return
            
            # 根据文件类型选择处理器
            if task.file_type == FileType.IMAGE:
                result = self.image_processor.process(task.file_id, task.path)
            elif task.file_type == FileType.DOCUMENT:
                result = self.document_processor.process(task.file_id, task.path)
            else:
                logger.warning(f"Unknown file type: {task.file_type}")
                return
            
            # 更新状态
            if result.get('errors'):
                db.update_file_status(
                    task.file_id,
                    FileStatus.ERROR,
                    error_msg='; '.join(result['errors'])
                )
            else:
                db.update_file_status(task.file_id, FileStatus.DONE)
            
            logger.info(f"Processed: {task.path} - {result}")
            
        except Exception as e:
            logger.error(f"Failed to process {task.path}: {e}")
            db.update_file_status(task.file_id, FileStatus.ERROR, error_msg=str(e))
    
    @property
    def stats(self) -> Dict:
        """获取工作者统计"""
        return {
            'worker_id': self.worker_id,
            'running': self._running,
            'processed': self._processed_count,
            'errors': self._error_count
        }


class WorkerPool:
    """工作者池"""
    
    def __init__(self, num_workers: int = None):
        self.num_workers = num_workers or settings.concurrent_workers
        self.workers: list[Worker] = []
    
    def start(self):
        """启动所有工作者"""
        for i in range(self.num_workers):
            worker = Worker(worker_id=i)
            worker.start()
            self.workers.append(worker)
        
        logger.info(f"Worker pool started with {self.num_workers} workers")
    
    def stop(self):
        """停止所有工作者"""
        for worker in self.workers:
            worker.stop()
        self.workers.clear()
        logger.info("Worker pool stopped")
    
    @property
    def stats(self) -> Dict:
        """获取工作者池统计"""
        worker_stats = [w.stats for w in self.workers]
        return {
            'num_workers': len(self.workers),
            'total_processed': sum(w['processed'] for w in worker_stats),
            'total_errors': sum(w['errors'] for w in worker_stats),
            'workers': worker_stats
        }


# 全局工作者池
worker_pool = WorkerPool()
