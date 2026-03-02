#!/bin/bash

# ====================================================
# 微服务编译脚本
# 功能：从当前目录的源代码编译生成可执行文件
# ====================================================

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 脚本所在目录（即 microservice-deploy 目录）
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}    微服务音乐平台 - 编译脚本${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

# 步骤1: 检查 Go 环境
echo -e "${YELLOW}[1/4] 检查 Go 环境...${NC}"
if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ 未找到 Go 命令${NC}"
    echo -e "${YELLOW}请先安装 Go 1.22 或更高版本${NC}"
    echo -e "${YELLOW}下载地址: https://golang.org/dl/${NC}"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
echo -e "${GREEN}✓ Go 版本: $GO_VERSION${NC}"
echo ""

# 步骤2: 检查源代码
echo -e "${YELLOW}[2/4] 检查源代码...${NC}"
MAIN_FILE="$SCRIPT_DIR/cmd/monolith/main.go"

if [ ! -f "$MAIN_FILE" ]; then
    echo -e "${RED}✗ 未找到主文件: $MAIN_FILE${NC}"
    echo -e "${YELLOW}提示：请确保源代码在当前目录下${NC}"
    echo -e "${YELLOW}目录结构应该是：${NC}"
    echo -e "${YELLOW}  microservice-deploy/${NC}"
    echo -e "${YELLOW}  ├── cmd/monolith/main.go${NC}"
    echo -e "${YELLOW}  ├── internal/...${NC}"
    echo -e "${YELLOW}  └── pkg/...${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 找到主文件: cmd/monolith/main.go${NC}"

# 检查 go.mod
if [ ! -f "$SCRIPT_DIR/go.mod" ]; then
    echo -e "${RED}✗ 未找到 go.mod 文件${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 找到 go.mod 文件${NC}"
echo ""

# 步骤3: 下载依赖
echo -e "${YELLOW}[3/4] 检查并下载依赖...${NC}"
cd "$SCRIPT_DIR"

# 检查网络连接（可选）
if go mod download 2>&1 | grep -q "no required module provides"; then
    echo -e "${YELLOW}⚠ 某些依赖可能不可用，继续编译...${NC}"
else
    echo -e "${GREEN}✓ 依赖检查完成${NC}"
fi
echo ""

# 步骤4: 编译
echo -e "${YELLOW}[4/4] 开始编译...${NC}"
OUTPUT_FILE="$SCRIPT_DIR/music_server"
AUTH_MAIN_FILE="$SCRIPT_DIR/cmd/auth/main.go"
AUTH_OUTPUT_FILE="$SCRIPT_DIR/auth_server"
CATALOG_MAIN_FILE="$SCRIPT_DIR/cmd/catalog/main.go"
CATALOG_OUTPUT_FILE="$SCRIPT_DIR/catalog_server"
PROFILE_MAIN_FILE="$SCRIPT_DIR/cmd/profile/main.go"
PROFILE_OUTPUT_FILE="$SCRIPT_DIR/profile_server"
MEDIA_MAIN_FILE="$SCRIPT_DIR/cmd/media/main.go"
MEDIA_OUTPUT_FILE="$SCRIPT_DIR/media_server"
VIDEO_MAIN_FILE="$SCRIPT_DIR/cmd/video/main.go"
VIDEO_OUTPUT_FILE="$SCRIPT_DIR/video_server"
EVENT_WORKER_MAIN_FILE="$SCRIPT_DIR/cmd/eventworker/main.go"
EVENT_WORKER_OUTPUT_FILE="$SCRIPT_DIR/event_worker"
MIGRATOR_MAIN_FILE="$SCRIPT_DIR/cmd/migrator/main.go"
MIGRATOR_OUTPUT_FILE="$SCRIPT_DIR/migrator"

# 编译选项说明：
# -o: 指定输出文件名
# -ldflags="-s -w": 去除调试信息，减小文件大小
#   -s: 去除符号表
#   -w: 去除 DWARF 调试信息
echo -e "${BLUE}编译命令: go build -ldflags=\"-s -w\" -o music_server cmd/monolith/main.go${NC}"

if go build -ldflags="-s -w" -o "$OUTPUT_FILE" "$MAIN_FILE"; then
    if [ -f "$AUTH_MAIN_FILE" ]; then
        echo -e "${BLUE}编译命令: go build -ldflags=\"-s -w\" -o auth_server cmd/auth/main.go${NC}"
        go build -ldflags="-s -w" -o "$AUTH_OUTPUT_FILE" "$AUTH_MAIN_FILE"
        chmod +x "$AUTH_OUTPUT_FILE"
    fi
    if [ -f "$CATALOG_MAIN_FILE" ]; then
        echo -e "${BLUE}编译命令: go build -ldflags=\"-s -w\" -o catalog_server cmd/catalog/main.go${NC}"
        go build -ldflags="-s -w" -o "$CATALOG_OUTPUT_FILE" "$CATALOG_MAIN_FILE"
        chmod +x "$CATALOG_OUTPUT_FILE"
    fi
    if [ -f "$PROFILE_MAIN_FILE" ]; then
        echo -e "${BLUE}编译命令: go build -ldflags=\"-s -w\" -o profile_server cmd/profile/main.go${NC}"
        go build -ldflags="-s -w" -o "$PROFILE_OUTPUT_FILE" "$PROFILE_MAIN_FILE"
        chmod +x "$PROFILE_OUTPUT_FILE"
    fi
    if [ -f "$MEDIA_MAIN_FILE" ]; then
        echo -e "${BLUE}编译命令: go build -ldflags=\"-s -w\" -o media_server cmd/media/main.go${NC}"
        go build -ldflags="-s -w" -o "$MEDIA_OUTPUT_FILE" "$MEDIA_MAIN_FILE"
        chmod +x "$MEDIA_OUTPUT_FILE"
    fi
    if [ -f "$VIDEO_MAIN_FILE" ]; then
        echo -e "${BLUE}编译命令: go build -ldflags=\"-s -w\" -o video_server cmd/video/main.go${NC}"
        go build -ldflags="-s -w" -o "$VIDEO_OUTPUT_FILE" "$VIDEO_MAIN_FILE"
        chmod +x "$VIDEO_OUTPUT_FILE"
    fi
    if [ -f "$EVENT_WORKER_MAIN_FILE" ]; then
        echo -e "${BLUE}编译命令: go build -ldflags=\"-s -w\" -o event_worker cmd/eventworker/main.go${NC}"
        go build -ldflags="-s -w" -o "$EVENT_WORKER_OUTPUT_FILE" "$EVENT_WORKER_MAIN_FILE"
        chmod +x "$EVENT_WORKER_OUTPUT_FILE"
    fi
    if [ -f "$MIGRATOR_MAIN_FILE" ]; then
        echo -e "${BLUE}编译命令: go build -ldflags=\"-s -w\" -o migrator cmd/migrator/main.go${NC}"
        go build -ldflags="-s -w" -o "$MIGRATOR_OUTPUT_FILE" "$MIGRATOR_MAIN_FILE"
        chmod +x "$MIGRATOR_OUTPUT_FILE"
    fi

    echo ""
    echo -e "${GREEN}================================================${NC}"
    echo -e "${GREEN}✓ 编译成功！${NC}"
    echo -e "${GREEN}================================================${NC}"
    echo ""
    
    # 设置可执行权限
    chmod +x "$OUTPUT_FILE"
    
    # 显示文件信息
    FILE_SIZE=$(du -h "$OUTPUT_FILE" | cut -f1)
    echo -e "${BLUE}可执行文件信息：${NC}"
    echo -e "  文件路径: ${GREEN}$OUTPUT_FILE${NC}"
    echo -e "  文件大小: ${GREEN}$FILE_SIZE${NC}"
    echo -e "  权限:     ${GREEN}$(ls -lh $OUTPUT_FILE | awk '{print $1}')${NC}"
    if [ -f "$AUTH_OUTPUT_FILE" ]; then
        AUTH_FILE_SIZE=$(du -h "$AUTH_OUTPUT_FILE" | cut -f1)
        echo -e "  文件路径: ${GREEN}$AUTH_OUTPUT_FILE${NC}"
        echo -e "  文件大小: ${GREEN}$AUTH_FILE_SIZE${NC}"
        echo -e "  权限:     ${GREEN}$(ls -lh $AUTH_OUTPUT_FILE | awk '{print $1}')${NC}"
    fi
    if [ -f "$CATALOG_OUTPUT_FILE" ]; then
        CATALOG_FILE_SIZE=$(du -h "$CATALOG_OUTPUT_FILE" | cut -f1)
        echo -e "  文件路径: ${GREEN}$CATALOG_OUTPUT_FILE${NC}"
        echo -e "  文件大小: ${GREEN}$CATALOG_FILE_SIZE${NC}"
        echo -e "  权限:     ${GREEN}$(ls -lh $CATALOG_OUTPUT_FILE | awk '{print $1}')${NC}"
    fi
    if [ -f "$PROFILE_OUTPUT_FILE" ]; then
        PROFILE_FILE_SIZE=$(du -h "$PROFILE_OUTPUT_FILE" | cut -f1)
        echo -e "  文件路径: ${GREEN}$PROFILE_OUTPUT_FILE${NC}"
        echo -e "  文件大小: ${GREEN}$PROFILE_FILE_SIZE${NC}"
        echo -e "  权限:     ${GREEN}$(ls -lh $PROFILE_OUTPUT_FILE | awk '{print $1}')${NC}"
    fi
    if [ -f "$MEDIA_OUTPUT_FILE" ]; then
        MEDIA_FILE_SIZE=$(du -h "$MEDIA_OUTPUT_FILE" | cut -f1)
        echo -e "  文件路径: ${GREEN}$MEDIA_OUTPUT_FILE${NC}"
        echo -e "  文件大小: ${GREEN}$MEDIA_FILE_SIZE${NC}"
        echo -e "  权限:     ${GREEN}$(ls -lh $MEDIA_OUTPUT_FILE | awk '{print $1}')${NC}"
    fi
    if [ -f "$VIDEO_OUTPUT_FILE" ]; then
        VIDEO_FILE_SIZE=$(du -h "$VIDEO_OUTPUT_FILE" | cut -f1)
        echo -e "  文件路径: ${GREEN}$VIDEO_OUTPUT_FILE${NC}"
        echo -e "  文件大小: ${GREEN}$VIDEO_FILE_SIZE${NC}"
        echo -e "  权限:     ${GREEN}$(ls -lh $VIDEO_OUTPUT_FILE | awk '{print $1}')${NC}"
    fi
    if [ -f "$EVENT_WORKER_OUTPUT_FILE" ]; then
        EVENT_WORKER_FILE_SIZE=$(du -h "$EVENT_WORKER_OUTPUT_FILE" | cut -f1)
        echo -e "  文件路径: ${GREEN}$EVENT_WORKER_OUTPUT_FILE${NC}"
        echo -e "  文件大小: ${GREEN}$EVENT_WORKER_FILE_SIZE${NC}"
        echo -e "  权限:     ${GREEN}$(ls -lh $EVENT_WORKER_OUTPUT_FILE | awk '{print $1}')${NC}"
    fi
    if [ -f "$MIGRATOR_OUTPUT_FILE" ]; then
        MIGRATOR_FILE_SIZE=$(du -h "$MIGRATOR_OUTPUT_FILE" | cut -f1)
        echo -e "  文件路径: ${GREEN}$MIGRATOR_OUTPUT_FILE${NC}"
        echo -e "  文件大小: ${GREEN}$MIGRATOR_FILE_SIZE${NC}"
        echo -e "  权限:     ${GREEN}$(ls -lh $MIGRATOR_OUTPUT_FILE | awk '{print $1}')${NC}"
    fi
    echo ""
    
    echo -e "${BLUE}后续操作：${NC}"
    echo -e "  1. 检查配置文件："
    echo -e "     ${YELLOW}vim config.yaml${NC}"
    echo ""
    echo -e "  2. 启动服务："
    echo -e "     ${YELLOW}./start.sh${NC}"
    echo ""
    echo -e "  3. 或直接运行："
    echo -e "     ${YELLOW}./music_server${NC}"
    echo ""
else
    echo ""
    echo -e "${RED}✗ 编译失败${NC}"
    echo ""
    echo -e "${YELLOW}常见问题：${NC}"
    echo -e "  1. 检查 Go 版本是否 >= 1.22"
    echo -e "  2. 检查网络连接（需要下载依赖）"
    echo -e "  3. 检查磁盘空间是否充足"
    echo -e "  4. 查看上面的错误信息"
    echo ""
    exit 1
fi
