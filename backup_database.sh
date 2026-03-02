#!/bin/bash

# ====================================================
# 数据库备份脚本
# 功能：备份 MySQL 数据库，包含结构和数据
# ====================================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 脚本所在目录
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="$SCRIPT_DIR/database_backup"
BACKUP_FILE="$BACKUP_DIR/music_users_${TIMESTAMP}.sql"

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}    数据库备份工具${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

# 读取配置文件
if [ ! -f "$SCRIPT_DIR/config.yaml" ]; then
    echo -e "${RED}✗ 未找到配置文件: config.yaml${NC}"
    echo -e "${YELLOW}提示: 请确保 config.yaml 文件存在${NC}"
    exit 1
fi

# 解析配置文件（简单的 grep 方式）
DB_HOST=$(grep -A 5 "^database:" "$SCRIPT_DIR/config.yaml" | grep "host:" | awk '{print $2}' | tr -d '"')
DB_PORT=$(grep -A 5 "^database:" "$SCRIPT_DIR/config.yaml" | grep "port:" | awk '{print $2}')
DB_USER=$(grep -A 5 "^database:" "$SCRIPT_DIR/config.yaml" | grep "user:" | awk '{print $2}' | tr -d '"')
DB_PASSWORD=$(grep -A 5 "^database:" "$SCRIPT_DIR/config.yaml" | grep "password:" | awk '{print $2}' | tr -d '"')
DB_NAME=$(grep -A 5 "^database:" "$SCRIPT_DIR/config.yaml" | grep "dbname:" | awk '{print $2}' | tr -d '"')

# 参数校验
if [ -z "$DB_HOST" ] || [ -z "$DB_USER" ] || [ -z "$DB_NAME" ]; then
    echo -e "${RED}✗ 无法从 config.yaml 中读取数据库配置${NC}"
    exit 1
fi

echo -e "${YELLOW}[1/4] 数据库配置信息${NC}"
echo -e "  主机: ${GREEN}$DB_HOST:${DB_PORT:-3306}${NC}"
echo -e "  用户: ${GREEN}$DB_USER${NC}"
echo -e "  数据库: ${GREEN}$DB_NAME${NC}"
echo ""

# 创建备份目录
echo -e "${YELLOW}[2/4] 创建备份目录...${NC}"
mkdir -p "$BACKUP_DIR"
echo -e "${GREEN}✓ 备份目录: $BACKUP_DIR${NC}"
echo ""

# 检查 mysqldump 命令
echo -e "${YELLOW}[3/4] 检查备份工具...${NC}"
if ! command -v mysqldump &> /dev/null; then
    echo -e "${RED}✗ 未找到 mysqldump 命令${NC}"
    echo -e "${YELLOW}请安装 MySQL 客户端工具：${NC}"
    echo -e "  Ubuntu/Debian: ${BLUE}sudo apt-get install mysql-client${NC}"
    echo -e "  CentOS/RHEL:   ${BLUE}sudo yum install mysql${NC}"
    exit 1
fi
echo -e "${GREEN}✓ mysqldump 已安装${NC}"
echo ""

# 执行备份
echo -e "${YELLOW}[4/4] 开始备份数据库...${NC}"
echo -e "${BLUE}备份文件: $BACKUP_FILE${NC}"
echo ""

# 构建 mysqldump 命令
MYSQLDUMP_CMD="mysqldump -h $DB_HOST"
if [ -n "$DB_PORT" ]; then
    MYSQLDUMP_CMD="$MYSQLDUMP_CMD -P $DB_PORT"
fi
MYSQLDUMP_CMD="$MYSQLDUMP_CMD -u $DB_USER"
if [ -n "$DB_PASSWORD" ]; then
    MYSQLDUMP_CMD="$MYSQLDUMP_CMD -p$DB_PASSWORD"
fi

# 备份选项说明：
# --single-transaction: 保证数据一致性（InnoDB）
# --routines: 备份存储过程和函数
# --triggers: 备份触发器
# --events: 备份事件
# --hex-blob: 二进制数据使用十六进制格式
# --default-character-set=utf8mb4: 使用 UTF-8 字符集
$MYSQLDUMP_CMD \
    --single-transaction \
    --routines \
    --triggers \
    --events \
    --hex-blob \
    --default-character-set=utf8mb4 \
    "$DB_NAME" > "$BACKUP_FILE"

if [ $? -eq 0 ]; then
    BACKUP_SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
    
    # 压缩备份文件
    echo -e "${YELLOW}压缩备份文件...${NC}"
    gzip "$BACKUP_FILE"
    COMPRESSED_FILE="${BACKUP_FILE}.gz"
    COMPRESSED_SIZE=$(du -h "$COMPRESSED_FILE" | cut -f1)
    
    echo ""
    echo -e "${GREEN}================================================${NC}"
    echo -e "${GREEN}✓ 数据库备份成功！${NC}"
    echo -e "${GREEN}================================================${NC}"
    echo ""
    echo -e "${BLUE}备份信息：${NC}"
    echo -e "  原始大小: ${GREEN}$BACKUP_SIZE${NC}"
    echo -e "  压缩后:   ${GREEN}$COMPRESSED_SIZE${NC}"
    echo -e "  备份文件: ${GREEN}$COMPRESSED_FILE${NC}"
    echo ""
    echo -e "${BLUE}使用方法：${NC}"
    echo -e "  1. 将备份文件复制到新服务器："
    echo -e "     ${YELLOW}scp $(basename $COMPRESSED_FILE) user@new-server:/path/to/deploy/database_backup/${NC}"
    echo ""
    echo -e "  2. 在新服务器上恢复数据库："
    echo -e "     ${YELLOW}cd microservice-deploy${NC}"
    echo -e "     ${YELLOW}./restore_database.sh database_backup/$(basename $COMPRESSED_FILE)${NC}"
    echo ""
    
    # 列出所有备份文件
    echo -e "${BLUE}历史备份文件：${NC}"
    ls -lh "$BACKUP_DIR"/*.sql.gz 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}'
    echo ""
    
    # 清理旧备份提示
    BACKUP_COUNT=$(ls -1 "$BACKUP_DIR"/*.sql.gz 2>/dev/null | wc -l)
    if [ "$BACKUP_COUNT" -gt 5 ]; then
        echo -e "${YELLOW}⚠ 发现 $BACKUP_COUNT 个备份文件，建议清理旧备份${NC}"
        echo -e "${YELLOW}删除最旧的备份：${NC}"
        echo -e "  ${BLUE}ls -t $BACKUP_DIR/*.sql.gz | tail -n +6 | xargs rm -f${NC}"
        echo ""
    fi
else
    echo -e "${RED}✗ 数据库备份失败${NC}"
    exit 1
fi
