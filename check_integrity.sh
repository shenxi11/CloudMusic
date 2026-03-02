#!/bin/bash

# ====================================================
# 完整性检查脚本
# 功能：验证 microservice-deploy 目录是否包含所有必要文件
# ====================================================

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "$SCRIPT_DIR"

echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}    微服务部署包 - 完整性检查${NC}"
echo -e "${BLUE}================================================${NC}"
echo ""

MISSING_COUNT=0
TOTAL_COUNT=0

# 检查函数
check_file() {
    local file=$1
    local desc=$2
    TOTAL_COUNT=$((TOTAL_COUNT + 1))
    
    if [ -f "$file" ]; then
        echo -e "${GREEN}✓${NC} $desc"
        return 0
    else
        echo -e "${RED}✗${NC} $desc (缺失: $file)"
        MISSING_COUNT=$((MISSING_COUNT + 1))
        return 1
    fi
}

check_dir() {
    local dir=$1
    local desc=$2
    TOTAL_COUNT=$((TOTAL_COUNT + 1))
    
    if [ -d "$dir" ]; then
        echo -e "${GREEN}✓${NC} $desc"
        return 0
    else
        echo -e "${RED}✗${NC} $desc (缺失: $dir)"
        MISSING_COUNT=$((MISSING_COUNT + 1))
        return 1
    fi
}

echo -e "${YELLOW}[1/7] 检查核心可执行文件...${NC}"
check_file "music_server" "可执行文件"
if [ -f "music_server" ]; then
    SIZE=$(du -h music_server | cut -f1)
    echo -e "  ${BLUE}→ 文件大小: $SIZE${NC}"
fi
echo ""

echo -e "${YELLOW}[2/7] 检查配置文件...${NC}"
check_file "config.yaml" "主配置文件"
check_file "config.yaml.example" "配置模板"
check_file "go.mod" "Go 模块定义"
check_file "go.sum" "Go 依赖锁定"
echo ""

echo -e "${YELLOW}[3/7] 检查管理脚本...${NC}"
check_file "start.sh" "启动脚本"
check_file "stop.sh" "停止脚本"
check_file "build.sh" "编译脚本"
check_file "backup_database.sh" "数据库备份脚本"
check_file "restore_database.sh" "数据库恢复脚本"
check_file "package.sh" "打包脚本"
echo ""

echo -e "${YELLOW}[4/7] 检查文档文件...${NC}"
check_file "README.md" "使用手册"
check_file "DEPLOY.md" "部署指南"
check_file "QUICKSTART.md" "快速参考"
check_file "MIGRATION_FULL.md" "完整迁移指南"
check_file "FILE_MANIFEST.md" "文件清单"
echo ""

echo -e "${YELLOW}[5/7] 检查源代码目录...${NC}"
check_dir "cmd/monolith" "主程序入口"
check_file "cmd/monolith/main.go" "main.go"
check_dir "internal" "内部包目录"
check_dir "internal/music" "音乐模块"
check_dir "internal/user" "用户模块"
check_dir "internal/usermusic" "用户音乐模块"
check_dir "internal/video" "视频模块"
check_dir "internal/artist" "艺术家模块"
check_dir "internal/media" "媒体模块"
check_dir "internal/common" "公共模块"
check_dir "pkg" "公共包目录"
echo ""

echo -e "${YELLOW}[6/7] 检查运行目录...${NC}"
check_dir "uploads" "上传文件目录"
check_dir "logs" "日志目录"
check_dir "migrations" "数据库迁移脚本"
echo ""

echo -e "${YELLOW}[7/7] 统计源代码文件...${NC}"
GO_FILES=$(find . -name "*.go" | wc -l)
echo -e "${GREEN}✓${NC} Go 源文件数量: ${BLUE}$GO_FILES${NC}"

MODULES=$(find internal -maxdepth 1 -type d | wc -l)
echo -e "${GREEN}✓${NC} 内部模块数量: ${BLUE}$((MODULES - 1))${NC}"
echo ""

# 显示结果
echo -e "${BLUE}================================================${NC}"
if [ $MISSING_COUNT -eq 0 ]; then
    echo -e "${GREEN}✓ 完整性检查通过！所有文件齐全 ($TOTAL_COUNT/$TOTAL_COUNT)${NC}"
    echo -e "${BLUE}================================================${NC}"
    echo ""
    echo -e "${GREEN}✅ 部署包已准备就绪！${NC}"
    echo ""
    echo -e "${BLUE}后续操作：${NC}"
    echo -e "  1. 修改配置文件："
    echo -e "     ${YELLOW}vim config.yaml${NC}"
    echo ""
    echo -e "  2. 备份数据库（可选）："
    echo -e "     ${YELLOW}./backup_database.sh${NC}"
    echo ""
    echo -e "  3. 测试启动服务："
    echo -e "     ${YELLOW}./start.sh${NC}"
    echo ""
    echo -e "  4. 打包部署（迁移到其他服务器）："
    echo -e "     ${YELLOW}./package.sh${NC}"
    echo ""
    exit 0
else
    echo -e "${RED}✗ 完整性检查失败！缺少 $MISSING_COUNT 个文件或目录${NC}"
    echo -e "${BLUE}================================================${NC}"
    echo ""
    echo -e "${YELLOW}建议操作：${NC}"
    echo -e "  1. 重新运行复制命令"
    echo -e "  2. 检查上级目录是否有源文件"
    echo -e "  3. 如果缺少可执行文件，运行 ./build.sh 编译"
    echo ""
    exit 1
fi
