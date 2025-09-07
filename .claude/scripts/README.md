# 路径标准化工具使用指南

## 概述
本目录包含用于维护项目文档中路径标准化的工具脚本。

## 可用脚本

### 1. 检查脚本
```bash
./.claude/scripts/check-path-standards.sh
```
**功能**: 扫描项目文档，检测路径格式违规
**输出**: 彩色检查报告，显示通过/失败状态

### 2. 修复脚本  
```bash
./.claude/scripts/fix-path-standards.sh
```
**功能**: 自动修复文档中的绝对路径问题
**安全**: 自动创建备份文件，支持恢复

## 使用流程

### 日常维护
1. **定期检查**: `./check-path-standards.sh`
2. **发现问题时**: `./fix-path-standards.sh`  
3. **验证修复**: 再次运行检查脚本

### CI/CD 集成
可以将检查脚本加入到CI流程中：
```yaml
- name: 路径标准化检查
  run: ./.claude/scripts/check-path-standards.sh
```

### 清理备份文件
修复完成并验证正确后：
```bash
find .claude/ -name '*.backup' -delete
```

## 规范参考
详细的路径使用规范请参阅: `.claude/rules/path-standards.md`