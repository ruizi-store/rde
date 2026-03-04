"""
文档处理器
支持 PDF、Word、TXT 等文档的文本提取和向量化
"""
import os
import logging
import numpy as np
from typing import Dict, Any, Optional, List
from pathlib import Path

from .base import BaseProcessor
from ..database import db, VectorType
from ..config import settings

logger = logging.getLogger(__name__)


class DocumentProcessor(BaseProcessor):
    """文档处理器"""
    
    def __init__(self):
        super().__init__()
        self.text_model = None
    
    def initialize(self):
        """初始化文本嵌入模型"""
        logger.info("Initializing document processor...")
        
        try:
            from sentence_transformers import SentenceTransformer
            
            # 使用中文文本嵌入模型
            model_name = "BAAI/bge-small-zh-v1.5"
            
            logger.info(f"Loading text embedding model: {model_name}")
            self.text_model = SentenceTransformer(model_name)
            logger.info("Text embedding model loaded")
            
        except Exception as e:
            logger.error(f"Failed to load text model: {e}")
            self.text_model = None
        
        self._initialized = True
        logger.info("Document processor initialized")
    
    def process(self, file_id: int, path: str) -> Dict[str, Any]:
        """处理文档文件"""
        self.ensure_initialized()
        
        result = {
            'text_extracted': False,
            'vectorized': False,
            'errors': []
        }
        
        try:
            # 1. 提取文本
            ext = Path(path).suffix.lower()
            text = self._extract_text(path, ext)
            
            if not text:
                result['errors'].append("No text extracted")
                return result
            
            result['text_extracted'] = True
            result['text_length'] = len(text)
            
            # 2. 向量化
            if self.text_model:
                try:
                    # 分块处理长文本
                    chunks = self._split_text(text, max_length=500)
                    
                    for i, chunk in enumerate(chunks):
                        vector = self._encode_text(chunk)
                        if vector is not None:
                            db.add_vector(
                                file_id=file_id,
                                vector_type=VectorType.TEXT,
                                vector=vector.tobytes(),
                                metadata={
                                    'chunk_index': i,
                                    'chunk_count': len(chunks),
                                    'text_preview': chunk[:200]
                                }
                            )
                    
                    result['vectorized'] = True
                    result['chunk_count'] = len(chunks)
                    
                except Exception as e:
                    result['errors'].append(f"Vectorization: {e}")
                    logger.error(f"Vectorization failed for {path}: {e}")
            
            # 3. 添加全文到 OCR 表（复用全文搜索）
            db.add_ocr_text(file_id, text[:10000], 1.0)  # 限制长度
            
        except Exception as e:
            result['errors'].append(f"Document processing: {e}")
            logger.error(f"Failed to process document {path}: {e}")
        
        return result
    
    def _extract_text(self, path: str, ext: str) -> str:
        """根据文件类型提取文本"""
        try:
            if ext == '.pdf':
                return self._extract_pdf(path)
            elif ext in ['.docx', '.doc']:
                return self._extract_docx(path)
            elif ext in ['.txt', '.md']:
                return self._extract_txt(path)
            else:
                logger.warning(f"Unsupported document type: {ext}")
                return ''
        except Exception as e:
            logger.error(f"Text extraction failed for {path}: {e}")
            return ''
    
    def _extract_pdf(self, path: str) -> str:
        """提取 PDF 文本"""
        try:
            from pypdf import PdfReader
            
            reader = PdfReader(path)
            texts = []
            
            for page in reader.pages:
                text = page.extract_text()
                if text:
                    texts.append(text)
            
            return '\n'.join(texts)
            
        except Exception as e:
            logger.error(f"PDF extraction failed: {e}")
            return ''
    
    def _extract_docx(self, path: str) -> str:
        """提取 Word 文档文本"""
        try:
            from docx import Document
            
            doc = Document(path)
            texts = []
            
            for para in doc.paragraphs:
                if para.text:
                    texts.append(para.text)
            
            return '\n'.join(texts)
            
        except Exception as e:
            logger.error(f"DOCX extraction failed: {e}")
            return ''
    
    def _extract_txt(self, path: str) -> str:
        """提取纯文本文件"""
        try:
            # 尝试多种编码
            encodings = ['utf-8', 'gbk', 'gb2312', 'latin-1']
            
            for encoding in encodings:
                try:
                    with open(path, 'r', encoding=encoding) as f:
                        return f.read()
                except UnicodeDecodeError:
                    continue
            
            return ''
            
        except Exception as e:
            logger.error(f"TXT extraction failed: {e}")
            return ''
    
    def _split_text(self, text: str, max_length: int = 500) -> List[str]:
        """分割长文本为多个块"""
        if len(text) <= max_length:
            return [text]
        
        chunks = []
        
        # 按段落分割
        paragraphs = text.split('\n')
        current_chunk = ''
        
        for para in paragraphs:
            if len(current_chunk) + len(para) + 1 <= max_length:
                current_chunk += para + '\n'
            else:
                if current_chunk:
                    chunks.append(current_chunk.strip())
                current_chunk = para + '\n'
        
        if current_chunk:
            chunks.append(current_chunk.strip())
        
        # 如果单个段落太长，进一步分割
        final_chunks = []
        for chunk in chunks:
            if len(chunk) > max_length:
                # 按句子分割
                sentences = chunk.replace('。', '。\n').replace('！', '！\n').replace('？', '？\n').split('\n')
                sub_chunk = ''
                for sent in sentences:
                    if len(sub_chunk) + len(sent) <= max_length:
                        sub_chunk += sent
                    else:
                        if sub_chunk:
                            final_chunks.append(sub_chunk)
                        sub_chunk = sent
                if sub_chunk:
                    final_chunks.append(sub_chunk)
            else:
                final_chunks.append(chunk)
        
        return final_chunks
    
    def _encode_text(self, text: str) -> Optional[np.ndarray]:
        """编码文本为向量"""
        if not self.text_model:
            return None
        
        vector = self.text_model.encode(text)
        return np.array(vector, dtype=np.float32)
    
    def encode_query(self, query: str) -> Optional[np.ndarray]:
        """编码查询文本（用于搜索）"""
        if not self.text_model:
            return None
        
        self.ensure_initialized()
        vector = self.text_model.encode(query)
        return np.array(vector, dtype=np.float32)
    
    def cleanup(self):
        """清理资源"""
        self.text_model = None
        self._initialized = False
        logger.info("Document processor cleaned up")
