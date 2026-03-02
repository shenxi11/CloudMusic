#!/bin/bash

# ====================================================
# 微服务部署包打包脚本
# 功能：将部署文件夹打包成 tar.gz 文件，方便传输到新服务器
# ====================================================

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 脚本所在目录
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
PROJECT_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
DEPLOY_DIR_NAME="microservice-deploy"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
PACKAGE_NAME="music_server_deploy_${TIMESTAMP}.tar.gz"

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}    微服务音乐平台 - 部署包打包工具${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

# 步骤1: 检查必要文件
echo -e "${YELLOW}[1/6] 检查必要文件...${NC}"

REQUIRED_FILES=(
    "start.sh"
    "stop.sh"
    "build.sh"
    "backup_database.sh"
    "restore_database.sh"
    "README.md"
    "DEPLOY.md"
    "config.yaml"
)

MISSING_FILES=()
for file in "${REQUIRED_FILES[@]}"; do
    if [ ! -f "$SCRIPT_DIR/$file" ]; then
        MISSING_FILES+=("$file")
    fi
done

if [ ${#MISSING_FILES[@]} -gt 0 ]; then
    echo -e "${RED}✗ 缺少以下必要文件：${NC}"
    for file in "${MISSING_FILES[@]}"; do
        echo -e "${RED}  - $file${NC}"
    done
    echo -e "${YELLOW}提示：请确保所有必要文件都在 $DEPLOY_DIR_NAME 目录中${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 所有必要文件检查通过${NC}"
echo ""

# 步骤2: 数据库备份
echo -e "${YELLOW}[2/6] 数据库备份...${NC}"

read -p "是否备份数据库？(y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    if [ -f "$SCRIPT_DIR/backup_database.sh" ]; then
        bash "$SCRIPT_DIR/backup_database.sh"
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✓ 数据库备份完成${NC}"
            # 将最新的备份文件包含到部署包中
            LATEST_BACKUP=$(ls -t "$SCRIPT_DIR/database_backup"/*.sql.gz 2>/dev/null | head -1)
            if [ -n "$LATEST_BACKUP" ]; then
                echo -e "${BLUE}最新备份: $(basename $LATEST_BACKUP)${NC}"
            fi
        else
            echo -e "${RED}✗ 数据库备份失败${NC}"
            read -p "是否继续打包？(y/n) " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                exit 1
            fi
        fi
    else
        echo -e "${YELLOW}⚠ 未找到备份脚本，跳过数据库备份${NC}"
    fi
else
    echo -e "${YELLOW}⚠ 跳过数据库备份${NC}"
fi
echo ""

# 步骤3: 检查是否已编译
echo -e "${YELLOW}[3/6] 检查可执行文件...${NC}"

if [ ! -f "$SCRIPT_DIR/music_server" ]; then
    echo -e "${YELLOW}⚠ 未找到编译后的可执行文件${NC}"
    read -p "是否现在编译？(y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        cd "$SCRIPT_DIR"
        bash build.sh
        if [ $? -ne 0 ]; then
            echo -e "${RED}✗ 编译失败，无法继续打包${NC}"
            exit 1
        fi
    else
        echo -e "${YELLOW}⚠ 跳过编译，将打包不含可执行文件的部署包${NC}"
    fi
else
    FILE_SIZE=$(du -h "$SCRIPT_DIR/music_server" | cut -f1)
    echo -e "${GREEN}✓ 找到可执行文件 (大小: $FILE_SIZE)${NC}"
fi
echo ""

# 步骤4: 检查配置文件
echo -e "${YELLOW}[4/6] 检查配置文件...${NC}"

if [ ! -f "$SCRIPT_DIR/config.yaml.example" ]; then
    echo -e "${YELLOW}⚠ 未找到 config.yaml.example${NC}"
    echo -e "${YELLOW}  将复制现有的 config.yaml 作为示例${NC}"
    if [ -f "$SCRIPT_DIR/config.yaml" ]; then
        cp "$SCRIPT_DIR/config.yaml" "$SCRIPT_DIR/config.yaml.example"
        echo -e "${GREEN}✓ 已创建 config.yaml.example${NC}"
    fi
fi

if [ -f "$SCRIPT_DIR/config.yaml" ]; then
    echo -e "${GREEN}✓ 找到 config.yaml (将一起打包)${NC}"
else
    echo -e "${YELLOW}⚠ 未找到 config.yaml，新服务器需要手动创建${NC}"
fi
echo ""

# 步骤5: 创建临时打包目录
echo -e "${YELLOW}[5/6] 准备打包文件...${NC}"

cd "$PROJECT_ROOT"

# 创建排除文件列表
EXCLUDE_FILE=$(mktemp)
cat > "$EXCLUDE_FILE" << EOF
$DEPLOY_DIR_NAME/*.pid
$DEPLOY_DIR_NAME/logs/*.log
$DEPLOY_DIR_NAME/uploads/*
$DEPLOY_DIR_NAME/*.tar.gz
$DEPLOY_DIR_NAME/database_backup/before_restore/*
EOF

echo -e "${BLUE}打包内容：${NC}"
echo -e "  - 所有脚本文件 (*.sh)"
echo -e "  - 配置文件 (config.yaml*)"
echo -e "  - 文档文件 (*.md)"
echo -e "  - 可执行文件 (music_server)"
echo -e "  - 数据库备份 (database_backup/*.sql.gz)"
echo -e "  - 目录结构 (uploads/, logs/)"
echo ""
echo -e "${BLUE}排除内容：${NC}"
echo -e "  - 日志文件 (*.log)"
echo -e "  - PID 文件 (*.pid)"
echo -e "  - 上传文件内容 (uploads/*)"
echo -e "  - 旧的打包文件 (*.tar.gz)"
echo -e "  - 恢复前备份 (database_backup/before_restore/*)"
echo ""

# 步骤6: 打包
echo -e "${YELLOW}[6/6] 开始打包...${NC}"

tar -czf "$PACKAGE_NAME" \
    --exclude-from="$EXCLUDE_FILE" \
    "$DEPLOY_DIR_NAME"

# 清理临时文件
rm -f "$EXCLUDE_FILE"

if [ $? -eq 0 ]; then
    PACKAGE_SIZE=$(du -h "$PACKAGE_NAME" | cut -f1)
    echo ""
    echo -e "${GREEN}================================================${NC}"
    echo -e "${GREEN}✓ 打包成功！${NC}"
    echo -e "${GREEN}================================================${NC}"
    echo ""
    echo -e "${BLUE}打包文件信息：${NC}"
    echo -e "  文件名: ${GREEN}$PACKAGE_NAME${NC}"
    echo -e "  大小:   ${GREEN}$PACKAGE_SIZE${NC}"
    echo -e "  路径:   ${GREEN}$PROJECT_ROOT/$PACKAGE_NAME${NC}"
    echo ""
    echo -e "${BLUE}使用方法：${NC}"
    echo -e "  1. 传输到新服务器："
    echo -e "     ${YELLOW}scp $PACKAGE_NAME user@new-server:/path/to/deploy/${NC}"
    echo ""
    echo -e "  2. 在新服务器上解压："
    echo -e "     ${YELLOW}tar -xzf $PACKAGE_NAME${NC}"
    echo ""
    echo -e "  3. 恢复数据库："
    echo -e "     ${YELLOW}cd $DEPLOY_DIR_NAME${NC}"
    echo -e "     ${YELLOW}# 首先修改 config.yaml 中的数据库配置${NC}"
    echo -e "     ${YELLOW}vim config.yaml${NC}"
    echo -e "     ${YELLOW}# 然后恢复数据库${NC}"
    echo -e "     ${YELLOW}./restore_database.sh database_backup/music_users_*.sql.gz${NC}"
    echo ""
    echo -e "  4. 修改配置文件："
    echo -e "     ${YELLOW}vim config.yaml  # 修改服务器地址等${NC}"
    echo ""
    echo -e "  5. 启动服务："
    echo -e "     ${YELLOW}./start.sh${NC}"
    echo ""
    echo -e "${GREEN}详细部署说明请查看解压后的 DEPLOY.md 文件${NC}"
    echo ""
else
    echo -e "${RED}✗ 打包失败${NC}"
    exit 1
fi
