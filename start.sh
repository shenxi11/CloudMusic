#!/bin/bash

##############################################
# 音乐平台微服务架构 - 启动脚本
# 版本: v3.0
# 架构: Microservice (DDD)
##############################################

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
APP_NAME="music_server"
PID_FILE="music_server.pid"
LOG_FILE="server.log"
PORT=8080

echo -e "${BLUE}╔════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   🎵 音乐平台微服务架构 - 启动脚本           ║${NC}"
echo -e "${BLUE}║   Version: v3.0                                 ║${NC}"
echo -e "${BLUE}║   Architecture: Domain-Driven Design           ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════╝${NC}"
echo ""

# 检查可执行文件是否存在
if [ ! -f "$APP_NAME" ]; then
    echo -e "${RED}✗ 可执行文件不存在: $APP_NAME${NC}"
    echo -e "${YELLOW}   请先编译: go build -o $APP_NAME ../cmd/monolith/main.go${NC}"
    exit 1
fi

# 检查是否已经运行
if [ -f "$PID_FILE" ]; then
    OLD_PID=$(cat "$PID_FILE")
    if ps -p "$OLD_PID" > /dev/null 2>&1; then
        echo -e "${YELLOW}⚠️  服务已在运行中 (PID: $OLD_PID)${NC}"
        echo -e "${YELLOW}   如需重启，请先运行 ./stop.sh${NC}"
        exit 1
    else
        echo -e "${YELLOW}⚠️  发现过期的PID文件，清理中...${NC}"
        rm -f "$PID_FILE"
    fi
fi

# 检查端口占用
if lsof -Pi :$PORT -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo -e "${RED}✗ 端口 $PORT 已被占用${NC}"
    echo -e "${YELLOW}占用进程：${NC}"
    lsof -Pi :$PORT -sTCP:LISTEN
    echo ""
    echo -e "${YELLOW}请先停止占用进程或修改配置文件中的端口号${NC}"
    exit 1
fi

# 检查依赖服务
echo -e "${BLUE}▶ 检查依赖服务...${NC}"

# 检查 MySQL
if ! nc -z localhost 3306 2>/dev/null; then
    echo -e "${YELLOW}⚠️  MySQL (3306) 可能未运行${NC}"
fi

# 检查 Redis
if ! nc -z localhost 6379 2>/dev/null; then
    echo -e "${YELLOW}⚠️  Redis (6379) 可能未运行${NC}"
fi

echo ""

# 启动服务
echo -e "${BLUE}▶ 启动微服务架构...${NC}"
nohup ./"$APP_NAME" > "$LOG_FILE" 2>&1 &
SERVER_PID=$!

# 保存PID
echo "$SERVER_PID" > "$PID_FILE"

# 等待启动
echo -e "${BLUE}▶ 等待服务启动...${NC}"
sleep 2

# 检查进程是否存在
if ! ps -p "$SERVER_PID" > /dev/null 2>&1; then
    echo -e "${RED}✗ 服务启动失败${NC}"
    echo -e "${YELLOW}查看日志: tail -f $LOG_FILE${NC}"
    rm -f "$PID_FILE"
    exit 1
fi

# 健康检查
echo -e "${BLUE}▶ 健康检查...${NC}"
RETRY=0
MAX_RETRY=10

while [ $RETRY -lt $MAX_RETRY ]; do
    if curl -s http://localhost:$PORT/health > /dev/null 2>&1; then
        echo -e "${GREEN}✓ 服务启动成功！${NC}"
        echo ""
        break
    fi
    RETRY=$((RETRY+1))
    sleep 1
done

if [ $RETRY -eq $MAX_RETRY ]; then
    echo -e "${RED}✗ 健康检查失败${NC}"
    echo -e "${YELLOW}服务可能未正常启动，请查看日志${NC}"
fi

# 获取本地IP
LOCAL_IP=$(hostname -I | awk '{print $1}')

echo -e "${GREEN}╔════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║           ✅ 服务启动成功                      ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}📊 服务信息:${NC}"
echo -e "   进程ID:    $SERVER_PID"
echo -e "   监听端口:  $PORT"
echo -e "   日志文件:  $LOG_FILE"
echo -e "   PID文件:   $PID_FILE"
echo -e "   工作目录:  $SCRIPT_DIR"
echo ""
echo -e "${BLUE}🌐 访问地址:${NC}"
echo -e "   本地:      http://localhost:$PORT"
echo -e "   局域网:    http://$LOCAL_IP:$PORT"
echo ""
echo -e "${BLUE}🔌 核心接口:${NC}"
echo -e "   健康检查:  http://localhost:$PORT/health"
echo -e "   音乐列表:  http://localhost:$PORT/files"
echo -e "   欢迎页:    http://localhost:$PORT/"
echo ""
echo -e "${BLUE}📝 管理命令:${NC}"
echo -e "   查看日志:  tail -f $LOG_FILE"
echo -e "   停止服务:  ./stop.sh"
echo -e "   重启服务:  ./stop.sh && ./start.sh"
echo ""
echo -e "${BLUE}🏗️  架构特性:${NC}"
echo -e "   ✓ 领域驱动设计 (DDD)"
echo -e "   ✓ 分层架构 (Handler/Service/Repository/Model)"
echo -e "   ✓ 依赖注入"
echo -e "   ✓ 100% 向后兼容"
echo ""
echo -e "${GREEN}════════════════════════════════════════════════${NC}"
echo -e "${GREEN}🎉 启动完成！祝您使用愉快！${NC}"
echo -e "${GREEN}════════════════════════════════════════════════${NC}"
