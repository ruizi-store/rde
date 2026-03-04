"""
处理器模块 - 初始化
"""
from .base import BaseProcessor
from .image import ImageProcessor
from .document import DocumentProcessor

__all__ = ['BaseProcessor', 'ImageProcessor', 'DocumentProcessor']
