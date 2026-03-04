"""
管理 API - 支持多用户
"""
import asyncio
from typing import Optional, List
from fastapi import APIRouter, Query, Header, HTTPException, BackgroundTasks
from pydantic import BaseModel

from ..config import settings

router = APIRouter(prefix="/api", tags=["admin"])


class UserStatusResponse(BaseModel):
    """用户状态响应"""
    user: str
    status: str
    database: dict
    queue: dict
    vectors: dict


class GlobalStatusResponse(BaseModel):
    """全局状态响应"""
    status: str
    multi_user: bool
    users: Optional[dict] = None
    workers: dict
    watcher: dict


class ScanResponse(BaseModel):
    """扫描响应"""
    message: str
    files_found: int
    user: Optional[str] = None


class UserListResponse(BaseModel):
    """用户列表响应"""
    total: int
    users: List[dict]


def get_user_resources(username: str):
    """获取用户资源"""
    if settings.multi_user:
        from ..user_manager import user_manager
        
        if not user_manager.validate_user(username):
            raise HTTPException(status_code=404, detail=f"User not found: {username}")
        
        db = user_manager.get_database(username)
        vector_index = user_manager.get_vector_index(username)
        task_queue = user_manager.get_task_queue(username)
        
        return db, vector_index, task_queue
    else:
        from ..database import db
        from ..vector_index import vector_index
        from ..queue import task_queue
        return db, vector_index, task_queue


@router.get("/health")
async def health_check():
    """健康检查"""
    return {"status": "ok", "multi_user": settings.multi_user}


@router.get("/admin/status", response_model=GlobalStatusResponse)
async def get_global_status():
    """
    获取全局索引服务状态
    """
    from ..worker import worker_pool
    from ..watcher import watcher
    
    result = GlobalStatusResponse(
        status="running",
        multi_user=settings.multi_user,
        workers=worker_pool.stats,
        watcher={"running": watcher.is_running}
    )
    
    if settings.multi_user:
        from ..user_manager import user_manager
        result.users = user_manager.get_stats()
    
    return result


@router.get("/admin/users", response_model=UserListResponse)
async def list_users():
    """
    列出所有用户
    """
    if not settings.multi_user:
        return UserListResponse(total=1, users=[{"username": "default", "status": "active"}])
    
    from ..user_manager import user_manager
    stats = user_manager.get_stats()
    
    return UserListResponse(
        total=stats["total_users"],
        users=stats["users"]
    )


@router.get("/status", response_model=UserStatusResponse)
async def get_user_status(
    user: Optional[str] = Query(None, description="用户名"),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID")
):
    """
    获取用户索引状态
    """
    username = user or x_user_id
    
    if settings.multi_user and not username:
        raise HTTPException(
            status_code=400,
            detail="User parameter required. Use ?user=username or X-User-ID header"
        )
    
    username = username or "default"
    db, vector_index, task_queue = get_user_resources(username)
    
    # 确保向量索引已初始化
    vector_index.initialize()
    
    return UserStatusResponse(
        user=username,
        status="running" if not task_queue.is_paused else "paused",
        database=db.get_stats(),
        queue={
            "pending": task_queue.size,
            "processing": task_queue.processing_count,
            "paused": task_queue.is_paused
        },
        vectors=vector_index.stats
    )


@router.post("/scan", response_model=ScanResponse)
async def trigger_scan(
    background_tasks: BackgroundTasks,
    user: Optional[str] = Query(None, description="用户名（留空扫描所有用户）"),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID")
):
    """
    触发扫描
    多用户模式下可指定用户，留空则扫描所有用户
    """
    username = user or x_user_id
    
    if settings.multi_user:
        from ..user_manager import user_manager
        
        if username:
            # 扫描指定用户
            if not user_manager.validate_user(username):
                raise HTTPException(status_code=404, detail=f"User not found: {username}")
            
            # TODO: 实现单用户扫描
            return ScanResponse(message="User scan started", files_found=0, user=username)
        else:
            # 扫描所有用户
            user_manager.sync_users()
            return ScanResponse(message="All users scan started", files_found=0)
    else:
        from ..watcher import full_scan
        
        loop = asyncio.get_event_loop()
        files_found = await loop.run_in_executor(None, full_scan)
        
        return ScanResponse(message="Scan completed", files_found=files_found)


@router.post("/pause")
async def pause_indexing(
    user: Optional[str] = Query(None, description="用户名"),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID")
):
    """
    暂停索引
    """
    username = user or x_user_id
    
    if settings.multi_user and username:
        _, _, task_queue = get_user_resources(username)
        task_queue.pause()
        return {"message": f"Indexing paused for user: {username}"}
    else:
        from ..queue import task_queue
        task_queue.pause()
        return {"message": "Indexing paused"}


@router.post("/resume")
async def resume_indexing(
    user: Optional[str] = Query(None, description="用户名"),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID")
):
    """
    恢复索引
    """
    username = user or x_user_id
    
    if settings.multi_user and username:
        _, _, task_queue = get_user_resources(username)
        task_queue.resume()
        return {"message": f"Indexing resumed for user: {username}"}
    else:
        from ..queue import task_queue
        task_queue.resume()
        return {"message": "Indexing resumed"}


@router.post("/rebuild")
async def rebuild_index(
    background_tasks: BackgroundTasks,
    user: Optional[str] = Query(None, description="用户名"),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID")
):
    """
    重建向量索引
    """
    username = user or x_user_id
    
    if settings.multi_user and username:
        _, vector_index, _ = get_user_resources(username)
        
        def do_rebuild():
            vector_index.rebuild()
        
        background_tasks.add_task(do_rebuild)
        return {"message": f"Index rebuild started for user: {username}"}
    else:
        from ..vector_index import vector_index
        
        def do_rebuild():
            vector_index.rebuild()
        
        background_tasks.add_task(do_rebuild)
        return {"message": "Index rebuild started"}


@router.delete("/index/{file_path:path}")
async def delete_file_index(
    file_path: str,
    user: Optional[str] = Query(None, description="用户名"),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID")
):
    """
    删除单个文件的索引
    """
    username = user or x_user_id
    
    if settings.multi_user and not username:
        raise HTTPException(
            status_code=400,
            detail="User parameter required"
        )
    
    username = username or "default"
    db, _, _ = get_user_resources(username)
    
    # 添加前导斜杠
    if not file_path.startswith('/'):
        file_path = '/' + file_path
    
    file_info = db.get_file_by_path(file_path)
    if not file_info:
        raise HTTPException(status_code=404, detail="File not found in index")
    
    db.delete_file(file_path)
    
    return {"message": f"Index deleted for {file_path}", "user": username}


@router.delete("/index")
async def clear_user_index(
    user: Optional[str] = Query(None, description="用户名"),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID")
):
    """
    清除用户所有索引
    警告：此操作不可恢复
    """
    username = user or x_user_id
    
    if settings.multi_user and not username:
        raise HTTPException(
            status_code=400,
            detail="User parameter required for clearing index"
        )
    
    username = username or "default"
    db, vector_index, _ = get_user_resources(username)
    
    # 清除数据库
    with db.get_conn() as conn:
        cursor = conn.cursor()
        cursor.execute("DELETE FROM vectors")
        cursor.execute("DELETE FROM face_attributes")
        cursor.execute("DELETE FROM ocr_text")
        cursor.execute("DELETE FROM files")
    
    # 重建空索引
    vector_index.rebuild()
    
    return {"message": f"All index cleared for user: {username}"}
