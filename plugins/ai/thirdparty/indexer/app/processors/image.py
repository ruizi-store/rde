"""
图片处理器
包含 CLIP 向量提取、OCR 文字识别、人脸属性检测
"""
import os
import io
import logging
import numpy as np
from typing import Dict, Any, List, Optional
from PIL import Image

from .base import BaseProcessor
from ..database import db, VectorType
from ..config import settings

logger = logging.getLogger(__name__)


class ImageProcessor(BaseProcessor):
    """图片处理器"""
    
    def __init__(self):
        super().__init__()
        self.clip_model = None
        self.ocr_engine = None
        self.face_analyzer = None
    
    def initialize(self):
        """初始化 AI 模型"""
        logger.info("Initializing image processor...")
        
        # 加载 CLIP 模型
        self._load_clip()
        
        # 加载 OCR 引擎
        self._load_ocr()
        
        # 加载人脸分析器
        self._load_face_analyzer()
        
        self._initialized = True
        logger.info("Image processor initialized")
    
    def _load_clip(self):
        """加载 CLIP 模型"""
        try:
            from sentence_transformers import SentenceTransformer
            
            # 使用中文 CLIP 模型
            model_name = "OFA-Sys/chinese-clip-vit-base-patch16"
            
            logger.info(f"Loading CLIP model: {model_name}")
            self.clip_model = SentenceTransformer(model_name)
            logger.info("CLIP model loaded")
            
        except Exception as e:
            logger.error(f"Failed to load CLIP model: {e}")
            self.clip_model = None
    
    def _load_ocr(self):
        """加载 OCR 引擎"""
        try:
            from paddleocr import PaddleOCR
            
            logger.info("Loading PaddleOCR...")
            self.ocr_engine = PaddleOCR(
                use_angle_cls=True,
                lang='ch',
                use_gpu=settings.device == 'cuda',
                show_log=False
            )
            logger.info("PaddleOCR loaded")
            
        except Exception as e:
            logger.error(f"Failed to load PaddleOCR: {e}")
            self.ocr_engine = None
    
    def _load_face_analyzer(self):
        """加载人脸分析器"""
        try:
            from insightface.app import FaceAnalysis
            
            logger.info("Loading InsightFace...")
            self.face_analyzer = FaceAnalysis(
                name='buffalo_l',
                providers=['CPUExecutionProvider']
            )
            self.face_analyzer.prepare(ctx_id=0, det_size=(640, 640))
            logger.info("InsightFace loaded")
            
        except Exception as e:
            logger.error(f"Failed to load InsightFace: {e}")
            self.face_analyzer = None
    
    def process(self, file_id: int, path: str) -> Dict[str, Any]:
        """处理图片文件"""
        self.ensure_initialized()
        
        result = {
            'clip': False,
            'ocr': False,
            'face': False,
            'errors': []
        }
        
        try:
            # 打开图片
            image = Image.open(path)
            if image.mode != 'RGB':
                image = image.convert('RGB')
            
            # 1. 提取 CLIP 向量
            if self.clip_model:
                try:
                    clip_vector = self._extract_clip_vector(image)
                    if clip_vector is not None:
                        db.add_vector(
                            file_id=file_id,
                            vector_type=VectorType.CLIP,
                            vector=clip_vector.tobytes(),
                            metadata={'dim': len(clip_vector)}
                        )
                        result['clip'] = True
                except Exception as e:
                    result['errors'].append(f"CLIP: {e}")
                    logger.error(f"CLIP extraction failed for {path}: {e}")
            
            # 2. OCR 文字识别
            if self.ocr_engine:
                try:
                    ocr_text, confidence = self._extract_ocr_text(path)
                    if ocr_text:
                        db.add_ocr_text(file_id, ocr_text, confidence)
                        result['ocr'] = True
                except Exception as e:
                    result['errors'].append(f"OCR: {e}")
                    logger.error(f"OCR failed for {path}: {e}")
            
            # 3. 人脸属性检测
            if self.face_analyzer:
                try:
                    faces = self._detect_faces(path)
                    for face in faces:
                        db.add_face(
                            file_id=file_id,
                            age=face['age'],
                            gender=face['gender'],
                            bbox=face['bbox'],
                            confidence=face['confidence']
                        )
                    if faces:
                        result['face'] = True
                        result['face_count'] = len(faces)
                except Exception as e:
                    result['errors'].append(f"Face: {e}")
                    logger.error(f"Face detection failed for {path}: {e}")
            
        except Exception as e:
            result['errors'].append(f"Image open: {e}")
            logger.error(f"Failed to process image {path}: {e}")
        
        return result
    
    def _extract_clip_vector(self, image: Image.Image) -> Optional[np.ndarray]:
        """提取 CLIP 图像向量"""
        vector = self.clip_model.encode(image)
        return np.array(vector, dtype=np.float32)
    
    def _extract_ocr_text(self, path: str) -> tuple:
        """提取 OCR 文字"""
        result = self.ocr_engine.ocr(path, cls=True)
        
        if not result or not result[0]:
            return '', 0.0
        
        texts = []
        total_conf = 0.0
        count = 0
        
        for line in result[0]:
            if line and len(line) >= 2:
                text, conf = line[1]
                texts.append(text)
                total_conf += conf
                count += 1
        
        full_text = '\n'.join(texts)
        avg_conf = total_conf / count if count > 0 else 0.0
        
        return full_text, avg_conf
    
    def _detect_faces(self, path: str) -> List[Dict]:
        """检测人脸并获取属性"""
        import cv2
        
        img = cv2.imread(path)
        if img is None:
            return []
        
        faces = self.face_analyzer.get(img)
        
        results = []
        for face in faces:
            results.append({
                'age': int(face.age),
                'gender': 'male' if face.gender == 1 else 'female',
                'bbox': face.bbox.tolist(),
                'confidence': float(face.det_score)
            })
        
        return results
    
    def encode_text(self, text: str) -> Optional[np.ndarray]:
        """编码文本为 CLIP 向量（用于搜索）"""
        if not self.clip_model:
            return None
        
        self.ensure_initialized()
        vector = self.clip_model.encode(text)
        return np.array(vector, dtype=np.float32)
    
    def cleanup(self):
        """清理资源"""
        self.clip_model = None
        self.ocr_engine = None
        self.face_analyzer = None
        self._initialized = False
        logger.info("Image processor cleaned up")
