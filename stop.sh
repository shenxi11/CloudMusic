#!/bin/bash

##############################################
# 音乐平台微服务架构 - 停止脚本
# 版本: v3.0
##############################################

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 配置
APP_NAME="music_server"
PID_FILE="music_server.pid"
LOG_FILE="server.log"
PORT=8080

echo -e "${BLUE}╔════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   🎵 音乐平台微服务架构 - 停止脚本           ║${NC}"
echo -e "${BLUE}║   Version: v3.0                                 ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════╝${NC}"
echo ""

# 检查PID文件
if [ ! -f "$PID_FILE" ]; then
    echo -e "${YELLOW}⚠️  PID文件不存在${NC}"
    echo -e "${BLUE}▶ 尝试通过进程名查找...${NC}"
    
    PID=$(pgrep -f "$APP_NAME" | head -1)
    if [ -z "$PID" ]; then
        echo -e "${GREEN}✓ 没有运行中的服务${NC}"
        exit 0
    fi
else
    PID=$(cat "$PID_FILE")
    echo -e "${BLUE}▶ 读取PID: $PID${NC}"
fi

# 检查进程是否存在
if ! ps -p "$PID" > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  进程不存在 (PID: $PID)${NC}"
    echo -e "${BLUE}▶ 清理PID文件...${NC}"
    rm -f "$PID_FILE"
    echo -e "${GREEN}✓ 清理完成${NC}"
    exit 0
fi

# 显示进程信息
echo -e "${BLUE}▶ 进程信息:${NC}"
ps -p "$PID" -o pid,ppid,user,%cpu,%mem,etime,cmd --no-headers | while read line; do
    echo -e "${YELLOW}$line${NC}"
done
echo ""

# 优雅停止
echo -e "${BLUE}▶ 发送SIGTERM信号 (优雅停止)...${NC}"
kill -TERM "$PID"

# 等待进程停止
echo -e "${BLUE}▶ 等待进程停止...${NC}"
WAIT=0
MAX_WAIT=10

while [ $WAIT -lt $MAX_WAIT ]; do
    if ! ps -p "$PID" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ 进程已停止${NC}"
        break
    fi
    sleep 1
    WAIT=$((WAIT+1))
    echo -ne "${YELLOW}   等待中... ${WAIT}/${MAX_WAIT}\r${NC}"
done
echo ""

# 如果还在运行，强制停止
if ps -p "$PID" > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  进程仍在运行，发送SIGKILL信号...${NC}"
    kill -9 "$PID"
    sleep 1
    
    if ps -p "$PID" > /dev/null 2>&1; then
        echo -e "${RED}✗ 无法停止进程${NC}"
        exit 1
    else
        echo -e "${GREEN}✓ 进程已强制停止${NC}"
    fi
fi

# 清理PID文件
if [ -f "$PID_FILE" ]; then
    rm -f "$PID_FILE"
    echo -e "${BLUE}▶ 已清理PID文件${NC}"
fi

# 检查端口释放
echo ""
echo -e "${BLUE}▶ 检查端口释放...${NC}"
if lsof -Pi :$PORT -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  端口 $PORT 仍被占用${NC}"
    lsof -Pi :$PORT -sTCP:LISTEN
else
    echo -e "${GREEN}✓ 端口 $PORT 已释放${NC}"
fi

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║           ✅ 服务已停止                        ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════╝${NC}"
echo ""

# 日志信息
if [ -f "$LOG_FILE" ]; then
    LOG_SIZE=$(du -h "$LOG_FILE" | cut -f1)
    echo -e "${BLUE}📝 日志信息:${NC}"
    echo -e "   文件: $LOG_FILE"
    echo -e "   大小: $LOG_SIZE"
    echo ""
fi

echo -e "${BLUE}💡 提示:${NC}"
echo -e "   查看日志: tail -100 $LOG_FILE"
echo -e "   清理日志: rm $LOG_FILE"
echo ""
echo -e "${GREEN}════════════════════════════════════════════════${NC}"
echo -e "${GREEN}👋 服务已安全停止！${NC}"
echo -e "${GREEN}════════════════════════════════════════════════${NC}"
