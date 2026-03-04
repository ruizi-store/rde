"""
向量搜索模块
基于 Faiss 的向量索引和搜索
"""
import os
import logging
import numpy as np
from typing import List, Dict, Optional, Tuple, TYPE_CHECKING
import threading

from .config import settings

if TYPE_CHECKING:
    from .database import Database

logger = logging.getLogger(__name__)


class VectorIndex:
    """向量索引管理 - 支持多用户"""
    
    def __init__(self, index_dir: str = None, database: 'Database' = None):
        """
        初始化向量索引
        
        Args:
            index_dir: 索引目录（多用户模式）
            database: 数据库实例（多用户模式）
        """
        self.index_dir = index_dir or settings.index_dir
        self._database = database
        self.clip_index = None
        self.text_index = None
        self.clip_id_map: Dict[int, int] = {}  # faiss_id -> file_id
        self.text_id_map: Dict[int, int] = {}
        self._lock = threading.Lock()
        self._dimension = 512  # CLIP 向量维度
        self._text_dimension = 512  # BGE 向量维度
        self._initialized = False
    
    @property
    def database(self):
        """获取数据库实例"""
        if self._database:
            return self._database
        # 兼容单用户模式
        from .database import db
        return db
    
    def initialize(self):
        """初始化索引"""
        if self._initialized:
            return
        
        try:
            import faiss
            
            # 创建 CLIP 索引
            self.clip_index = faiss.IndexFlatIP(self._dimension)  # 内积（余弦相似度）
            
            # 创建文本索引
            self.text_index = faiss.IndexFlatIP(self._text_dimension)
            
            # 从数据库加载已有向量
            self._load_from_database()
            
            self._initialized = True
            logger.info(f"Vector index initialized: {self.index_dir}")
            
        except ImportError:
            logger.error("Faiss not installed")
        except Exception as e:
            logger.error(f"Failed to initialize vector index: {e}")
    
    def _load_from_database(self):
        """从数据库加载向量到索引"""
        from .database import VectorType
        
        with self.database.get_conn() as conn:
            cursor = conn.cursor()
            
            # 加载 CLIP 向量
            cursor.execute("""
                SELECT v.file_id, v.vector 
                FROM vectors v
                JOIN files f ON v.file_id = f.id
                WHERE v.vector_type = ? AND f.status = 'done'
            """, (VectorType.CLIP,))
            
            clip_vectors = []
            clip_ids = []
            
            for row in cursor.fetchall():
                vector = np.frombuffer(row['vector'], dtype=np.float32)
                if len(vector) == self._dimension:
                    clip_vectors.append(vector)
                    clip_ids.append(row['file_id'])
            
            if clip_vectors:
                vectors_array = np.vstack(clip_vectors)
                # 归一化（用于余弦相似度）
                faiss_module = __import__('faiss')
                faiss_module.normalize_L2(vectors_array)
                self.clip_index.add(vectors_array)
                
                for i, file_id in enumerate(clip_ids):
                    self.clip_id_map[i] = file_id
            
            logger.info(f"Loaded {len(clip_vectors)} CLIP vectors")
            
            # 加载文本向量
            cursor.execute("""
                SELECT v.file_id, v.vector 
                FROM vectors v
                JOIN files f ON v.file_id = f.id
                WHERE v.vector_type = ? AND f.status = 'done'
            """, (VectorType.TEXT,))
            
            text_vectors = []
            text_ids = []
            
            for row in cursor.fetchall():
                vector = np.frombuffer(row['vector'], dtype=np.float32)
                if len(vector) == self._text_dimension:
                    text_vectors.append(vector)
                    text_ids.append(row['file_id'])
            
            if text_vectors:
                vectors_array = np.vstack(text_vectors)
                faiss_module = __import__('faiss')
                faiss_module.normalize_L2(vectors_array)
                self.text_index.add(vectors_array)
                
                for i, file_id in enumerate(text_ids):
                    self.text_id_map[i] = file_id
            
            logger.info(f"Loaded {len(text_vectors)} text vectors")
    
    def add_clip_vector(self, file_id: int, vector: np.ndarray):
        """添加 CLIP 向量到索引"""
        with self._lock:
            if self.clip_index is None:
                return
            
            vector = vector.reshape(1, -1).astype(np.float32)
            import faiss
            faiss.normalize_L2(vector)
            
            idx = self.clip_index.ntotal
            self.clip_index.add(vector)
            self.clip_id_map[idx] = file_id
    
    def add_text_vector(self, file_id: int, vector: np.ndarray):
        """添加文本向量到索引"""
        with self._lock:
            if self.text_index is None:
                return
            
            vector = vector.reshape(1, -1).astype(np.float32)
            import faiss
            faiss.normalize_L2(vector)
            
            idx = self.text_index.ntotal
            self.text_index.add(vector)
            self.text_id_map[idx] = file_id
    
    def search_clip(self, query_vector: np.ndarray, k: int = 20) -> List[Tuple[int, float]]:
        """搜索相似图片"""
        if self.clip_index is None or self.clip_index.ntotal == 0:
            return []
        
        query = query_vector.reshape(1, -1).astype(np.float32)
        import faiss
        faiss.normalize_L2(query)
        
        k = min(k, self.clip_index.ntotal)
        distances, indices = self.clip_index.search(query, k)
        
        results = []
        for i, (dist, idx) in enumerate(zip(distances[0], indices[0])):
            if idx >= 0 and idx in self.clip_id_map:
                file_id = self.clip_id_map[idx]
                results.append((file_id, float(dist)))
        
        return results
    
    def search_text(self, query_vector: np.ndarray, k: int = 20) -> List[Tuple[int, float]]:
        """搜索相似文档"""
        if self.text_index is None or self.text_index.ntotal == 0:
            return []
        
        query = query_vector.reshape(1, -1).astype(np.float32)
        import faiss
        faiss.normalize_L2(query)
        
        k = min(k, self.text_index.ntotal)
        distances, indices = self.text_index.search(query, k)
        
        results = []
        for i, (dist, idx) in enumerate(zip(distances[0], indices[0])):
            if idx >= 0 and idx in self.text_id_map:
                file_id = self.text_id_map[idx]
                results.append((file_id, float(dist)))
        
        return results
    
    def rebuild(self):
        """重建索引"""
        with self._lock:
            import faiss
            
            self.clip_index = faiss.IndexFlatIP(self._dimension)
            self.text_index = faiss.IndexFlatIP(self._text_dimension)
            self.clip_id_map.clear()
            self.text_id_map.clear()
            
            self._load_from_database()
            logger.info(f"Vector index rebuilt: {self.index_dir}")
    
    @property
    def stats(self) -> Dict:
        """获取统计信息"""
        return {
            'clip_vectors': self.clip_index.ntotal if self.clip_index else 0,
            'text_vectors': self.text_index.ntotal if self.text_index else 0,
            'index_dir': self.index_dir
        }


# 全局向量索引（单用户模式兼容）
vector_index = VectorIndex()
