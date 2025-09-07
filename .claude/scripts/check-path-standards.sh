#!/bin/bash

# 路径标准化检查脚本
# 用于验证项目文档中的路径格式是否符合规范

set -e

echo "🔍 路径标准化检查开始..."

# 颜色输出函数
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# 检查计数器
total_checks=0
passed_checks=0
failed_checks=0

# 检查函数
check_absolute_paths() {
    echo -e "\n📋 检查1: 扫描绝对路径违规..."
    total_checks=$((total_checks + 1))
    
    # 检查.claude目录中的绝对路径
    if rg -q "/Users/|/home/|C:\\\\" .claude/ 2>/dev/null; then
        print_error "发现绝对路径违规:"
        rg -n "/Users/|/home/|C:\\\\" .claude/ | head -10
        failed_checks=$((failed_checks + 1))
        return 1
    else
        print_success "未发现绝对路径违规"
        passed_checks=$((passed_checks + 1))
        return 0
    fi
}

check_user_specific_paths() {
    echo -e "\n📋 检查2: 扫描用户特定路径..."
    total_checks=$((total_checks + 1))
    
    # 检查包含用户名的路径
    if rg -q "/[Uu]sers/[^/]*/|/home/[^/]*/" .claude/ 2>/dev/null; then
        print_error "发现用户特定路径:"
        rg -n "/[Uu]sers/[^/]*/|/home/[^/]*/" .claude/ | head -10
        failed_checks=$((failed_checks + 1))
        return 1
    else
        print_success "未发现用户特定路径"
        passed_checks=$((passed_checks + 1))
        return 0
    fi
}

check_path_format_consistency() {
    echo -e "\n📋 检查3: 检查路径格式一致性..."
    total_checks=$((total_checks + 1))
    
    # 检查是否使用了标准的相对路径格式
    inconsistent_found=false
    
    # 检查是否有混合使用./和不带./的路径
    if rg -q "\.\/" .claude/ 2>/dev/null && rg -q "internal/|cmd/|configs/" .claude/ 2>/dev/null; then
        print_warning "发现路径格式不一致（混合使用 ./ 和直接路径）"
        inconsistent_found=true
    fi
    
    if [ "$inconsistent_found" = false ]; then
        print_success "路径格式一致"
        passed_checks=$((passed_checks + 1))
    else
        print_warning "建议统一路径格式"
        passed_checks=$((passed_checks + 1))  # 这是警告，不算失败
    fi
}

check_github_sync_content() {
    echo -e "\n📋 检查4: 验证同步内容路径格式..."
    total_checks=$((total_checks + 1))
    
    # 检查更新文件中的路径格式
    update_files=$(find .claude/epics/*/updates/ -name "*.md" 2>/dev/null | head -10)
    
    if [ -z "$update_files" ]; then
        print_warning "未找到更新文件，跳过此检查"
        passed_checks=$((passed_checks + 1))
        return 0
    fi
    
    violations_found=false
    for file in $update_files; do
        if rg -q "/Users/|/home/|C:\\\\" "$file" 2>/dev/null; then
            print_error "文件 $file 包含绝对路径"
            violations_found=true
        fi
    done
    
    if [ "$violations_found" = false ]; then
        print_success "更新文件路径格式正确"
        passed_checks=$((passed_checks + 1))
    else
        failed_checks=$((failed_checks + 1))
        return 1
    fi
}

check_rule_compliance() {
    echo -e "\n📋 检查5: 验证规范文件存在性..."
    total_checks=$((total_checks + 1))
    
    if [ -f ".claude/rules/path-standards.md" ]; then
        print_success "路径标准化规范文件存在"
        passed_checks=$((passed_checks + 1))
    else
        print_error "缺少路径标准化规范文件"
        failed_checks=$((failed_checks + 1))
        return 1
    fi
}

# 运行所有检查
echo "开始路径标准化检查..."

check_absolute_paths
check_user_specific_paths  
check_path_format_consistency
check_github_sync_content
check_rule_compliance

# 输出总结
echo -e "\n📊 检查结果总结:"
echo "总检查项: $total_checks"
echo "通过: $passed_checks"
echo "失败: $failed_checks"

if [ $failed_checks -eq 0 ]; then
    print_success "所有检查通过！路径标准化合规 🎉"
    exit 0
else
    print_error "发现 $failed_checks 个问题需要修复"
    echo -e "\n💡 修复建议:"
    echo "1. 运行路径清理脚本修复绝对路径"
    echo "2. 检查并更新相关文档格式"  
    echo "3. 确保遵循 .claude/rules/path-standards.md 规范"
    exit 1
fi