"""
数据库操作模块
"""
import os
import json
import sqlite3
import asyncio
from typing import Optional, List, Dict, Any
from contextlib import contextmanager
from dataclasses import dataclass
from enum import Enum

from .config import settings


class FileStatus(str, Enum):
    """文件状态"""
    PENDING = "pending"
    PROCESSING = "processing"
    DONE = "done"
    ERROR = "error"


class FileType(str, Enum):
    """文件类型"""
    IMAGE = "image"
    DOCUMENT = "document"
    AUDIO = "audio"


class VectorType(str, Enum):
    """向量类型"""
    CLIP = "clip"
    OCR = "ocr"
    FACE = "face"
    TEXT = "text"


@dataclass
class FileRecord:
    """文件记录"""
    id: int
    path: str
    type: str
    mime_type: Optional[str]
    size: int
    mtime: int
    status: str
    indexed_at: Optional[int]
    error_msg: Optional[str]
    created_at: int


class Database:
    """SQLite 数据库管理"""
    
    def __init__(self, db_path: str = None):
        self.db_path = db_path or settings.db_path
        self._ensure_dir()
        self._init_db()
    
    def _ensure_dir(self):
        """确保目录存在"""
        os.makedirs(os.path.dirname(self.db_path), exist_ok=True)
    
    @contextmanager
    def get_conn(self):
        """获取数据库连接"""
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        try:
            yield conn
            conn.commit()
        except Exception:
            conn.rollback()
            raise
        finally:
            conn.close()
    
    def _init_db(self):
        """初始化数据库表"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            
            # 文件表
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS files (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    path TEXT UNIQUE NOT NULL,
                    type TEXT NOT NULL,
                    mime_type TEXT,
                    size INTEGER,
                    mtime INTEGER,
                    status TEXT DEFAULT 'pending',
                    indexed_at INTEGER,
                    error_msg TEXT,
                    created_at INTEGER DEFAULT (strftime('%s', 'now'))
                )
            """)
            
            cursor.execute("CREATE INDEX IF NOT EXISTS idx_files_status ON files(status)")
            cursor.execute("CREATE INDEX IF NOT EXISTS idx_files_type ON files(type)")
            cursor.execute("CREATE INDEX IF NOT EXISTS idx_files_path ON files(path)")
            
            # 向量表
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS vectors (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    file_id INTEGER NOT NULL,
                    vector_type TEXT NOT NULL,
                    vector BLOB NOT NULL,
                    metadata TEXT,
                    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
                )
            """)
            
            cursor.execute("CREATE INDEX IF NOT EXISTS idx_vectors_file ON vectors(file_id)")
            cursor.execute("CREATE INDEX IF NOT EXISTS idx_vectors_type ON vectors(vector_type)")
            
            # 人脸属性表
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS face_attributes (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    file_id INTEGER NOT NULL,
                    age INTEGER,
                    gender TEXT,
                    bbox TEXT,
                    confidence REAL,
                    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
                )
            """)
            
            cursor.execute("CREATE INDEX IF NOT EXISTS idx_face_file ON face_attributes(file_id)")
            cursor.execute("CREATE INDEX IF NOT EXISTS idx_face_age ON face_attributes(age)")
            cursor.execute("CREATE INDEX IF NOT EXISTS idx_face_gender ON face_attributes(gender)")
            
            # OCR 文本表
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS ocr_text (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    file_id INTEGER NOT NULL,
                    text TEXT NOT NULL,
                    confidence REAL,
                    FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
                )
            """)
            
            cursor.execute("CREATE INDEX IF NOT EXISTS idx_ocr_file ON ocr_text(file_id)")
            
            # FTS 全文搜索（如果不存在）
            cursor.execute("""
                CREATE VIRTUAL TABLE IF NOT EXISTS ocr_text_fts 
                USING fts5(text, content='ocr_text', content_rowid='id')
            """)
    
    # ========== 文件操作 ==========
    
    def add_file(self, path: str, file_type: str, mime_type: str = None,
                 size: int = 0, mtime: int = 0) -> int:
        """添加文件记录"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                INSERT OR REPLACE INTO files (path, type, mime_type, size, mtime, status)
                VALUES (?, ?, ?, ?, ?, 'pending')
            """, (path, file_type, mime_type, size, mtime))
            return cursor.lastrowid
    
    def get_file_by_path(self, path: str) -> Optional[Dict]:
        """通过路径获取文件"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            cursor.execute("SELECT * FROM files WHERE path = ?", (path,))
            row = cursor.fetchone()
            return dict(row) if row else None
    
    def get_file_by_id(self, file_id: int) -> Optional[Dict]:
        """通过ID获取文件"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            cursor.execute("SELECT * FROM files WHERE id = ?", (file_id,))
            row = cursor.fetchone()
            return dict(row) if row else None
    
    def update_file_status(self, file_id: int, status: str, error_msg: str = None):
        """更新文件状态"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            if status == FileStatus.DONE:
                cursor.execute("""
                    UPDATE files SET status = ?, indexed_at = strftime('%s', 'now'), error_msg = NULL
                    WHERE id = ?
                """, (status, file_id))
            else:
                cursor.execute("""
                    UPDATE files SET status = ?, error_msg = ?
                    WHERE id = ?
                """, (status, error_msg, file_id))
    
    def delete_file(self, path: str):
        """删除文件记录"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            cursor.execute("DELETE FROM files WHERE path = ?", (path,))
    
    def get_pending_files(self, limit: int = 100) -> List[Dict]:
        """获取待处理文件"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                SELECT * FROM files WHERE status = 'pending'
                ORDER BY created_at ASC LIMIT ?
            """, (limit,))
            return [dict(row) for row in cursor.fetchall()]
    
    def get_stats(self) -> Dict[str, int]:
        """获取统计信息"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            
            stats = {}
            
            # 按状态统计
            cursor.execute("""
                SELECT status, COUNT(*) as count FROM files GROUP BY status
            """)
            for row in cursor.fetchall():
                stats[f"status_{row['status']}"] = row['count']
            
            # 按类型统计
            cursor.execute("""
                SELECT type, COUNT(*) as count FROM files WHERE status = 'done' GROUP BY type
            """)
            for row in cursor.fetchall():
                stats[f"type_{row['type']}"] = row['count']
            
            # 总数
            cursor.execute("SELECT COUNT(*) as count FROM files")
            stats['total'] = cursor.fetchone()['count']
            
            return stats
    
    # ========== 向量操作 ==========
    
    def add_vector(self, file_id: int, vector_type: str, vector: bytes, metadata: dict = None):
        """添加向量"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            meta_json = json.dumps(metadata) if metadata else None
            cursor.execute("""
                INSERT INTO vectors (file_id, vector_type, vector, metadata)
                VALUES (?, ?, ?, ?)
            """, (file_id, vector_type, vector, meta_json))
    
    def get_vectors_by_file(self, file_id: int) -> List[Dict]:
        """获取文件的所有向量"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            cursor.execute("SELECT * FROM vectors WHERE file_id = ?", (file_id,))
            return [dict(row) for row in cursor.fetchall()]
    
    def delete_vectors_by_file(self, file_id: int):
        """删除文件的所有向量"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            cursor.execute("DELETE FROM vectors WHERE file_id = ?", (file_id,))
    
    # ========== 人脸属性操作 ==========
    
    def add_face(self, file_id: int, age: int, gender: str, bbox: list, confidence: float):
        """添加人脸属性"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                INSERT INTO face_attributes (file_id, age, gender, bbox, confidence)
                VALUES (?, ?, ?, ?, ?)
            """, (file_id, age, gender, json.dumps(bbox), confidence))
    
    def search_faces(self, age_min: int = None, age_max: int = None,
                     gender: str = None, limit: int = 20) -> List[Dict]:
        """搜索人脸"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            
            query = """
                SELECT f.*, fa.age, fa.gender, fa.confidence
                FROM files f
                JOIN face_attributes fa ON f.id = fa.file_id
                WHERE f.status = 'done'
            """
            params = []
            
            if age_min is not None:
                query += " AND fa.age >= ?"
                params.append(age_min)
            
            if age_max is not None:
                query += " AND fa.age <= ?"
                params.append(age_max)
            
            if gender:
                query += " AND fa.gender = ?"
                params.append(gender)
            
            query += " ORDER BY fa.confidence DESC LIMIT ?"
            params.append(limit)
            
            cursor.execute(query, params)
            return [dict(row) for row in cursor.fetchall()]
    
    # ========== OCR 操作 ==========
    
    def add_ocr_text(self, file_id: int, text: str, confidence: float):
        """添加 OCR 文本"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                INSERT INTO ocr_text (file_id, text, confidence)
                VALUES (?, ?, ?)
            """, (file_id, text, confidence))
            
            # 更新 FTS 索引
            cursor.execute("""
                INSERT INTO ocr_text_fts (rowid, text) VALUES (last_insert_rowid(), ?)
            """, (text,))
    
    def search_ocr_text(self, query: str, limit: int = 20) -> List[Dict]:
        """搜索 OCR 文本"""
        with self.get_conn() as conn:
            cursor = conn.cursor()
            cursor.execute("""
                SELECT f.*, ot.text, ot.confidence
                FROM files f
                JOIN ocr_text ot ON f.id = ot.file_id
                JOIN ocr_text_fts fts ON ot.id = fts.rowid
                WHERE ocr_text_fts MATCH ?
                ORDER BY rank LIMIT ?
            """, (query, limit))
            return [dict(row) for row in cursor.fetchall()]


# 全局数据库实例
db = Database()
