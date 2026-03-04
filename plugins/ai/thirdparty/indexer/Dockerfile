# RDE Indexer Service
# 智能文件索引服务 Docker 镜像
# 支持多用户模式

FROM python:3.11-slim

LABEL maintainer="RDE Team"
LABEL description="Smart file indexer with AI-powered search (multi-user)"

# 设置工作目录
WORKDIR /app

# 使用清华 apt 源
RUN sed -i 's|deb.debian.org|mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list.d/debian.sources

# 安装系统依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
    # 编译工具
    build-essential \
    g++ \
    # OpenCV 依赖
    libgl1 \
    libglib2.0-0 \
    libsm6 \
    libxext6 \
    libxrender1 \
    # 其他依赖
    libgomp1 \
    curl \
    && rm -rf /var/lib/apt/lists/*

# 复制依赖文件
COPY requirements.txt .

# 安装 Python 依赖（使用清华 pip 源）
RUN pip install --no-cache-dir -i https://pypi.tuna.tsinghua.edu.cn/simple -r requirements.txt

# 复制应用代码
COPY app/ ./app/

# 创建数据目录
RUN mkdir -p /data/homes /data/index

# 环境变量
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

# 多用户模式配置
ENV MULTI_USER=true
ENV HOMES_DIR=/data/homes
ENV INDEX_DIR=/data/index
ENV WATCH_SUBDIRS=Pictures,Documents
ENV USER_SCAN_INTERVAL=3600

# 通用配置
ENV CONCURRENT_WORKERS=2
ENV SCAN_INTERVAL=3600
ENV CPU_THRESHOLD=70
ENV DEVICE=cpu
ENV LOG_LEVEL=info
ENV PORT=8081

# 数据卷
VOLUME ["/data/homes", "/data/index"]

# 暴露端口
EXPOSE 8081

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
    CMD curl -f http://localhost:8081/api/health || exit 1

# 启动命令
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8081"]
