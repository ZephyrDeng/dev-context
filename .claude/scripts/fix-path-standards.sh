#!/bin/bash

# 路径标准化修复脚本
# 自动将绝对路径转换为相对路径

set -e

echo "🔧 路径标准化修复开始..."

# 颜色输出函数
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${BLUE}ℹ️ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# 路径标准化函数
normalize_paths() {
    local file="$1"
    local backup_file="${file}.backup"
    
    print_info "处理文件: $file"
    
    # 创建备份
    cp "$file" "$backup_file"
    
    # 应用路径转换规则
    sed -i.tmp \
        -e 's|/Users/[^/]*/[^/]*/|../|g' \
        -e 's|/home/[^/]*/[^/]*/|../|g' \
        -e 's|C:\\Users\\[^\\]*\\[^\\]*\\|..\\|g' \
        -e 's|\./\([^./]\)|\1|g' \
        "$file"
    
    # 清理临时文件
    rm -f "${file}.tmp"
    
    # 检查是否有变化
    if ! diff -q "$file" "$backup_file" >/dev/null 2>&1; then
        print_success "文件已修复: $(basename "$file")"
        return 0
    else
        print_info "文件无需修改: $(basename "$file")"
        rm "$backup_file"  # 删除不必要的备份
        return 1
    fi
}

# 修复统计
files_processed=0
files_modified=0

# 处理.claude目录下的所有markdown文件
echo -e "\n🔍 扫描需要修复的文件..."

while IFS= read -r -d '' file; do
    # 跳过备份文件
    [[ "$file" == *.backup ]] && continue
    
    # 检查文件是否包含需要修复的路径
    if grep -q "/Users/\|/home/\|C:\\\\" "$file" 2>/dev/null; then
        files_processed=$((files_processed + 1))
        
        if normalize_paths "$file"; then
            files_modified=$((files_modified + 1))
        fi
    fi
done < <(find .claude/ -name "*.md" -type f -print0 2>/dev/null)

# 特殊处理：检查并修复git相关文件中的路径引用
if [ -f ".git/config" ]; then
    print_info "检查git配置文件..."
    # 这里通常不需要修改，只是检查
fi

# 输出统计结果
echo -e "\n📊 修复结果统计:"
echo "处理文件数: $files_processed"
echo "修改文件数: $files_modified"

if [ $files_modified -gt 0 ]; then
    print_success "成功修复 $files_modified 个文件"
    
    echo -e "\n💾 备份信息:"
    echo "原始文件已备份为 .backup 后缀"
    echo "如需恢复，可使用: mv file.backup file"
    
    print_warning "建议运行检查脚本验证修复结果:"
    echo "./.claude/scripts/check-path-standards.sh"
    
elif [ $files_processed -eq 0 ]; then
    print_success "未发现需要修复的文件 🎉"
else
    print_info "所有文件均无需修改"
fi

# 提供清理备份文件的选项
if [ $files_modified -gt 0 ]; then
    echo -e "\n🧹 清理备份文件 (可选):"
    echo "如确认修复正确，可运行:"
    echo "find .claude/ -name '*.backup' -delete"
fi

echo -e "\n✨ 路径标准化修复完成！"