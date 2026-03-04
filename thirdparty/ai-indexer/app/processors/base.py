"""
处理器基类
"""
from abc import ABC, abstractmethod
from typing import Dict, Any
import logging

logger = logging.getLogger(__name__)


class BaseProcessor(ABC):
    """文件处理器基类"""
    
    def __init__(self):
        self._initialized = False
    
    @abstractmethod
    def initialize(self):
        """初始化处理器（加载模型等）"""
        pass
    
    @abstractmethod
    def process(self, file_id: int, path: str) -> Dict[str, Any]:
        """
        处理文件
        
        Args:
            file_id: 文件ID
            path: 文件路径
        
        Returns:
            处理结果字典
        """
        pass
    
    @abstractmethod
    def cleanup(self):
        """清理资源"""
        pass
    
    @property
    def is_initialized(self) -> bool:
        return self._initialized
    
    def ensure_initialized(self):
        """确保已初始化"""
        if not self._initialized:
            self.initialize()
            self._initialized = True
