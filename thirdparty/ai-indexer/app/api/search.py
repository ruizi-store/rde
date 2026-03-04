"""
搜索 API - 支持多用户
"""
import os
import base64
from typing import Optional, List
from fastapi import APIRouter, Query, Header, HTTPException
from pydantic import BaseModel
from PIL import Image
import io

from ..config import settings
from ..processors import ImageProcessor, DocumentProcessor

router = APIRouter(prefix="/api", tags=["search"])

# 处理器实例（用于编码查询，所有用户共享）
image_processor = ImageProcessor()
document_processor = DocumentProcessor()


class SearchResult(BaseModel):
    """搜索结果"""
    path: str
    type: str
    score: float
    thumbnail: Optional[str] = None
    metadata: dict = {}


class SearchResponse(BaseModel):
    """搜索响应"""
    total: int
    items: List[SearchResult]
    user: Optional[str] = None


def get_thumbnail(path: str, size: tuple = (100, 100)) -> Optional[str]:
    """生成缩略图的 base64"""
    try:
        if not os.path.exists(path):
            return None
        
        img = Image.open(path)
        img.thumbnail(size)
        
        buffer = io.BytesIO()
        img.save(buffer, format='JPEG', quality=60)
        
        return base64.b64encode(buffer.getvalue()).decode()
    except Exception:
        return None


def get_user_resources(username: str):
    """获取用户的数据库和向量索引"""
    if settings.multi_user:
        from ..user_manager import user_manager
        
        if not user_manager.validate_user(username):
            raise HTTPException(status_code=404, detail=f"User not found: {username}")
        
        db = user_manager.get_database(username)
        vector_index = user_manager.get_vector_index(username)
        
        if not db or not vector_index:
            raise HTTPException(status_code=500, detail="Failed to initialize user resources")
        
        # 确保向量索引已初始化
        vector_index.initialize()
        
        return db, vector_index
    else:
        # 单用户模式
        from ..database import db
        from ..vector_index import vector_index
        return db, vector_index


def get_username(user: Optional[str], x_user_id: Optional[str]) -> str:
    """获取用户名"""
    username = user or x_user_id
    
    if settings.multi_user and not username:
        raise HTTPException(
            status_code=400, 
            detail="User parameter required. Use ?user=username or X-User-ID header"
        )
    
    return username or "default"


@router.get("/search", response_model=SearchResponse)
async def semantic_search(
    q: str = Query(..., description="搜索关键词"),
    type: str = Query("all", description="文件类型: image/document/all"),
    limit: int = Query(20, ge=1, le=100),
    offset: int = Query(0, ge=0),
    user: Optional[str] = Query(None, description="用户名（多用户模式）"),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID")
):
    """
    语义搜索
    支持自然语言描述搜索图片和文档
    """
    username = get_username(user, x_user_id)
    db, vector_index = get_user_resources(username)
    
    results = []
    seen_files = set()
    
    # 搜索图片
    if type in ["all", "image"]:
        try:
            # 使用 CLIP 编码查询
            query_vector = image_processor.encode_text(q)
            if query_vector is not None:
                clip_results = vector_index.search_clip(query_vector, k=limit * 2)
                
                for file_id, score in clip_results:
                    if file_id in seen_files:
                        continue
                    seen_files.add(file_id)
                    
                    file_info = db.get_file_by_id(file_id)
                    if file_info and file_info['type'] == 'image':
                        results.append(SearchResult(
                            path=file_info['path'],
                            type='image',
                            score=score,
                            thumbnail=get_thumbnail(file_info['path']),
                            metadata={}
                        ))
        except Exception as e:
            pass  # 忽略错误，继续其他搜索
    
    # 搜索文档
    if type in ["all", "document"]:
        try:
            # 使用文本模型编码查询
            query_vector = document_processor.encode_query(q)
            if query_vector is not None:
                text_results = vector_index.search_text(query_vector, k=limit * 2)
                
                for file_id, score in text_results:
                    if file_id in seen_files:
                        continue
                    seen_files.add(file_id)
                    
                    file_info = db.get_file_by_id(file_id)
                    if file_info and file_info['type'] == 'document':
                        results.append(SearchResult(
                            path=file_info['path'],
                            type='document',
                            score=score,
                            metadata={}
                        ))
        except Exception as e:
            pass
    
    # 排序并分页
    results.sort(key=lambda x: x.score, reverse=True)
    total = len(results)
    results = results[offset:offset + limit]
    
    return SearchResponse(total=total, items=results, user=username)


@router.get("/search/text", response_model=SearchResponse)
async def search_by_text(
    q: str = Query(..., description="搜索文字"),
    limit: int = Query(20, ge=1, le=100),
    user: Optional[str] = Query(None, description="用户名"),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID")
):
    """
    OCR 文字搜索
    搜索包含特定文字的图片
    """
    username = get_username(user, x_user_id)
    db, _ = get_user_resources(username)
    
    ocr_results = db.search_ocr_text(q, limit=limit)
    
    items = []
    for row in ocr_results:
        items.append(SearchResult(
            path=row['path'],
            type=row['type'],
            score=row.get('confidence', 1.0),
            thumbnail=get_thumbnail(row['path']) if row['type'] == 'image' else None,
            metadata={'text_preview': row.get('text', '')[:200]}
        ))
    
    return SearchResponse(total=len(items), items=items, user=username)


@router.get("/search/face", response_model=SearchResponse)
async def search_by_face(
    age_min: Optional[int] = Query(None, ge=0, le=100),
    age_max: Optional[int] = Query(None, ge=0, le=100),
    gender: Optional[str] = Query(None, regex="^(male|female)$"),
    limit: int = Query(20, ge=1, le=100),
    user: Optional[str] = Query(None, description="用户名"),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID")
):
    """
    人脸属性搜索
    按年龄、性别搜索人物照片
    """
    username = get_username(user, x_user_id)
    db, _ = get_user_resources(username)
    
    face_results = db.search_faces(
        age_min=age_min,
        age_max=age_max,
        gender=gender,
        limit=limit
    )
    
    items = []
    for row in face_results:
        items.append(SearchResult(
            path=row['path'],
            type='image',
            score=row.get('confidence', 1.0),
            thumbnail=get_thumbnail(row['path']),
            metadata={
                'age': row.get('age'),
                'gender': row.get('gender')
            }
        ))
    
    return SearchResponse(total=len(items), items=items, user=username)


@router.get("/search/doc", response_model=SearchResponse)
async def search_document_type(
    doc_type: str = Query(..., description="证件类型: id_card/bank_card/passport"),
    limit: int = Query(20, ge=1, le=100),
    user: Optional[str] = Query(None, description="用户名"),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID")
):
    """
    证件类型搜索
    搜索包含特定类型证件的图片
    
    注意：需要先进行证件检测训练，当前返回空结果
    """
    username = get_username(user, x_user_id)
    # TODO: 实现证件检测模型
    return SearchResponse(total=0, items=[], user=username)
