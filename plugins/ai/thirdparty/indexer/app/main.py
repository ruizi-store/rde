"""
RuiziOS Indexer Service
智能文件索引服务 - 主入口
支持多用户模式
"""
import os
import logging
import asyncio
import threading
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from .config import settings
from .api import search_router, admin_router

# 配置日志
logging.basicConfig(
    level=getattr(logging, settings.log_level.upper()),
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """应用生命周期管理"""
    logger.info("Starting Indexer Service...")
    logger.info(f"Multi-user mode: {settings.multi_user}")
    
    # 确保索引目录存在
    os.makedirs(settings.index_dir, exist_ok=True)
    
    if settings.multi_user:
        # 多用户模式
        from .user_manager import user_manager
        from .worker import worker_pool
        from .watcher import watcher
        
        # 启动用户管理器
        user_manager.start()
        
        # 启动工作者池（共享）
        worker_pool.start()
        
        # 启动目录监控（监控 homes 目录变化）
        # 用户目录的文件监控在 user_manager 中处理
        
        # 定时同步用户
        async def periodic_user_sync():
            while True:
                await asyncio.sleep(settings.user_scan_interval)
                logger.info("Starting periodic user sync...")
                loop = asyncio.get_event_loop()
                await loop.run_in_executor(None, user_manager.sync_users)
        
        sync_task = asyncio.create_task(periodic_user_sync())
        
        logger.info("Indexer Service started (multi-user mode)")
        
        yield
        
        # 停止服务
        logger.info("Stopping Indexer Service...")
        sync_task.cancel()
        user_manager.stop()
        worker_pool.stop()
        
    else:
        # 单用户模式（兼容）
        from .database import db
        from .queue import task_queue
        from .watcher import watcher, full_scan
        from .worker import worker_pool
        from .vector_index import vector_index
        
        # 初始化向量索引
        vector_index.initialize()
        
        # 从数据库加载待处理任务
        task_queue.load_pending_from_db()
        
        # 启动工作者池
        worker_pool.start()
        
        # 启动文件监控
        watcher.start()
        
        # 首次全量扫描
        def initial_scan():
            logger.info("Starting initial scan...")
            count = full_scan()
            logger.info(f"Initial scan found {count} files")
        
        # 在后台线程执行初始扫描
        scan_thread = threading.Thread(target=initial_scan, daemon=True)
        scan_thread.start()
        
        # 定时全量扫描
        async def periodic_scan():
            while True:
                await asyncio.sleep(settings.scan_interval)
                logger.info("Starting periodic scan...")
                loop = asyncio.get_event_loop()
                await loop.run_in_executor(None, full_scan)
        
        scan_task = asyncio.create_task(periodic_scan())
        
        logger.info("Indexer Service started (single-user mode)")
        
        yield
        
        # 停止服务
        logger.info("Stopping Indexer Service...")
        scan_task.cancel()
        watcher.stop()
        worker_pool.stop()
    
    logger.info("Indexer Service stopped")


# 创建 FastAPI 应用
app = FastAPI(
    title="RuiziOS Indexer Service",
    description="智能文件索引服务 - 支持图片和文档的语义搜索",
    version="0.1.0",
    lifespan=lifespan
)

# CORS 配置
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 注册路由
app.include_router(search_router)
app.include_router(admin_router)


@app.get("/")
async def root():
    """根路径"""
    return {
        "name": "RuiziOS Indexer Service",
        "version": "0.1.0",
        "status": "running"
    }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(
        "app.main:app",
        host="0.0.0.0",
        port=settings.port,
        reload=False
    )
