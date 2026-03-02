#!/bin/bash

# ====================================================
# 数据库恢复脚本
# 功能：在新服务器上恢复 MySQL 数据库
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

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}    数据库恢复工具${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

# 检查参数
if [ $# -eq 0 ]; then
    echo -e "${YELLOW}用法: $0 <备份文件路径>${NC}"
    echo ""
    echo -e "${BLUE}示例：${NC}"
    echo -e "  $0 database_backup/music_users_20260225_120000.sql.gz"
    echo -e "  $0 database_backup/music_users_20260225_120000.sql"
    echo ""
    
    # 列出可用的备份文件
    if [ -d "$SCRIPT_DIR/database_backup" ]; then
        BACKUP_FILES=$(ls -1 "$SCRIPT_DIR/database_backup"/*.sql* 2>/dev/null)
        if [ -n "$BACKUP_FILES" ]; then
            echo -e "${BLUE}可用的备份文件：${NC}"
            ls -lh "$SCRIPT_DIR/database_backup"/*.sql* 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}'
            echo ""
        fi
    fi
    exit 1
fi

BACKUP_FILE="$1"

# 支持相对路径和绝对路径
if [ ! -f "$BACKUP_FILE" ]; then
    # 尝试在脚本目录下查找
    if [ -f "$SCRIPT_DIR/$BACKUP_FILE" ]; then
        BACKUP_FILE="$SCRIPT_DIR/$BACKUP_FILE"
    else
        echo -e "${RED}✗ 备份文件不存在: $BACKUP_FILE${NC}"
        exit 1
    fi
fi

echo -e "${YELLOW}[1/6] 检查备份文件...${NC}"
BACKUP_SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
echo -e "${GREEN}✓ 找到备份文件: $(basename $BACKUP_FILE) ($BACKUP_SIZE)${NC}"
echo ""

# 读取配置文件
echo -e "${YELLOW}[2/6] 读取配置文件...${NC}"
if [ ! -f "$SCRIPT_DIR/config.yaml" ]; then
    echo -e "${RED}✗ 未找到配置文件: config.yaml${NC}"
    echo -e "${YELLOW}提示: 请确保 config.yaml 文件存在并已配置${NC}"
    exit 1
fi

# 解析配置文件
DB_HOST=$(grep -A 5 "^database:" "$SCRIPT_DIR/config.yaml" | grep "host:" | awk '{print $2}' | tr -d '"')
DB_PORT=$(grep -A 5 "^database:" "$SCRIPT_DIR/config.yaml" | grep "port:" | awk '{print $2}')
DB_USER=$(grep -A 5 "^database:" "$SCRIPT_DIR/config.yaml" | grep "user:" | awk '{print $2}' | tr -d '"')
DB_PASSWORD=$(grep -A 5 "^database:" "$SCRIPT_DIR/config.yaml" | grep "password:" | awk '{print $2}' | tr -d '"')
DB_NAME=$(grep -A 5 "^database:" "$SCRIPT_DIR/config.yaml" | grep "dbname:" | awk '{print $2}' | tr -d '"')

if [ -z "$DB_HOST" ] || [ -z "$DB_USER" ] || [ -z "$DB_NAME" ]; then
    echo -e "${RED}✗ 无法从 config.yaml 中读取数据库配置${NC}"
    exit 1
fi

echo -e "  主机: ${GREEN}$DB_HOST:${DB_PORT:-3306}${NC}"
echo -e "  用户: ${GREEN}$DB_USER${NC}"
echo -e "  数据库: ${GREEN}$DB_NAME${NC}"
echo ""

# 检查 mysql 命令
echo -e "${YELLOW}[3/6] 检查 MySQL 客户端...${NC}"
if ! command -v mysql &> /dev/null; then
    echo -e "${RED}✗ 未找到 mysql 命令${NC}"
    echo -e "${YELLOW}请安装 MySQL 客户端工具：${NC}"
    echo -e "  Ubuntu/Debian: ${BLUE}sudo apt-get install mysql-client${NC}"
    echo -e "  CentOS/RHEL:   ${BLUE}sudo yum install mysql${NC}"
    exit 1
fi
echo -e "${GREEN}✓ MySQL 客户端已安装${NC}"
echo ""

# 测试数据库连接
echo -e "${YELLOW}[4/6] 测试数据库连接...${NC}"
MYSQL_CMD="mysql -h $DB_HOST"
if [ -n "$DB_PORT" ]; then
    MYSQL_CMD="$MYSQL_CMD -P $DB_PORT"
fi
MYSQL_CMD="$MYSQL_CMD -u $DB_USER"
if [ -n "$DB_PASSWORD" ]; then
    MYSQL_CMD="$MYSQL_CMD -p$DB_PASSWORD"
fi

if ! $MYSQL_CMD -e "SELECT 1;" &>/dev/null; then
    echo -e "${RED}✗ 数据库连接失败${NC}"
    echo -e "${YELLOW}请检查：${NC}"
    echo -e "  1. MySQL 服务是否运行"
    echo -e "  2. config.yaml 中的数据库配置是否正确"
    echo -e "  3. 数据库用户是否有权限"
    exit 1
fi
echo -e "${GREEN}✓ 数据库连接成功${NC}"
echo ""

# 确认操作
echo -e "${YELLOW}[5/6] 确认恢复操作${NC}"
echo -e "${RED}⚠ 警告: 此操作将覆盖数据库 '$DB_NAME' 中的所有数据！${NC}"
echo ""
read -p "是否继续？(yes/no) " -r
echo
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo -e "${YELLOW}已取消恢复操作${NC}"
    exit 0
fi

# 备份当前数据库（如果存在）
echo -e "${YELLOW}[6/6] 恢复数据库...${NC}"
echo -e "${BLUE}正在备份当前数据库...${NC}"

CURRENT_BACKUP_DIR="$SCRIPT_DIR/database_backup/before_restore"
mkdir -p "$CURRENT_BACKUP_DIR"
CURRENT_BACKUP_FILE="$CURRENT_BACKUP_DIR/backup_$(date +%Y%m%d_%H%M%S).sql.gz"

if $MYSQL_CMD -e "USE $DB_NAME;" &>/dev/null; then
    mysqldump -h $DB_HOST \
        $([ -n "$DB_PORT" ] && echo "-P $DB_PORT") \
        -u $DB_USER \
        $([ -n "$DB_PASSWORD" ] && echo "-p$DB_PASSWORD") \
        --single-transaction \
        "$DB_NAME" | gzip > "$CURRENT_BACKUP_FILE" 2>/dev/null || true
    
    if [ -f "$CURRENT_BACKUP_FILE" ]; then
        CURRENT_SIZE=$(du -h "$CURRENT_BACKUP_FILE" | cut -f1)
        echo -e "${GREEN}✓ 当前数据已备份: $CURRENT_BACKUP_FILE ($CURRENT_SIZE)${NC}"
    fi
else
    echo -e "${YELLOW}⚠ 数据库不存在，将创建新数据库${NC}"
fi
echo ""

# 创建数据库（如果不存在）
echo -e "${BLUE}确保数据库存在...${NC}"
$MYSQL_CMD -e "CREATE DATABASE IF NOT EXISTS \`$DB_NAME\` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" 2>/dev/null || true

# 恢复数据库
echo -e "${BLUE}开始导入数据...${NC}"

# 判断是否为压缩文件
if [[ "$BACKUP_FILE" == *.gz ]]; then
    # 解压并导入
    gunzip -c "$BACKUP_FILE" | $MYSQL_CMD "$DB_NAME"
else
    # 直接导入
    $MYSQL_CMD "$DB_NAME" < "$BACKUP_FILE"
fi

if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}================================================${NC}"
    echo -e "${GREEN}✓ 数据库恢复成功！${NC}"
    echo -e "${GREEN}================================================${NC}"
    echo ""
    
    # 验证数据
    echo -e "${BLUE}数据库统计信息：${NC}"
    
    # 获取表信息
    TABLES=$($MYSQL_CMD "$DB_NAME" -e "SHOW TABLES;" -s)
    TABLE_COUNT=$(echo "$TABLES" | wc -l)
    
    echo -e "  数据库: ${GREEN}$DB_NAME${NC}"
    echo -e "  表数量: ${GREEN}$TABLE_COUNT${NC}"
    echo ""
    echo -e "${BLUE}表名及记录数：${NC}"
    
    for table in $TABLES; do
        COUNT=$($MYSQL_CMD "$DB_NAME" -e "SELECT COUNT(*) FROM \`$table\`;" -s)
        printf "  %-30s %s 条记录\n" "$table" "$COUNT"
    done
    echo ""
    
    echo -e "${GREEN}数据库已成功恢复并可以使用${NC}"
    echo ""
else
    echo -e "${RED}✗ 数据库恢复失败${NC}"
    echo ""
    if [ -f "$CURRENT_BACKUP_FILE" ]; then
        echo -e "${YELLOW}可以从备份恢复：${NC}"
        echo -e "  $0 $CURRENT_BACKUP_FILE"
    fi
    exit 1
fi
