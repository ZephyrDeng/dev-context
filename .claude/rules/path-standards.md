# 路径标准化规范

## 概述
本规范定义了 Claude Code PM 系统中文件路径的使用标准，确保文档可移植性、隐私保护和一致性。

## 核心原则

### 1. 隐私保护原则
- **禁止**使用包含用户名的绝对路径
- **禁止**在公共文档中暴露本地目录结构  
- **禁止**在GitHub Issues评论中包含完整本地路径

### 2. 可移植性原则
- **优先**使用相对路径引用项目文件
- **确保**文档在不同开发环境中通用
- **避免**环境特定的路径格式

## 路径格式标准

### 项目内文件引用 ✅
```markdown
# 正确示例
- `internal/mcp/server.go` 
- `cmd/server/main.go`
- `.claude/commands/pm/sync.md`

# 错误示例 ❌
- `/Users/username/project/internal/mcp/server.go`
- `C:\Users\username\project\cmd\server\main.go`
```

### 跨项目/工作树引用 ✅
```markdown
# 正确示例
- `../project-name/internal/mcp/server.go`
- `../worktree-name/src/components/Button.tsx`

# 错误示例 ❌
- `/Users/username/parent-dir/project-name/internal/mcp/server.go`
- `/home/user/projects/worktree-name/src/components/Button.tsx`
```

### 代码注释中的文件引用 ✅
```go
// 正确示例
// See internal/processor/converter.go for data transformation
// Configuration loaded from configs/production.yml

// 错误示例 ❌  
// See /Users/username/parent-dir/project-name/internal/processor/converter.go
```

## 实施规则

### 文档生成规则
1. **Issue同步模板**：使用相对路径模板变量
2. **进度报告**：自动转换绝对路径为相对路径
3. **技术文档**：统一使用项目根目录相对路径

### 路径变量标准
```yaml
# 模板变量定义
project_root: "."              # 当前项目根目录
worktree_path: "../{name}"     # 工作树相对路径  
internal_path: "internal/"     # 内部模块目录
config_path: "configs/"        # 配置文件目录
```

### 自动清理规则
```bash
# 路径标准化函数
normalize_paths() {
  local content="$1"
  # 移除用户特定路径（通用模式）
  content=$(echo "$content" | sed "s|/Users/[^/]*/[^/]*/||g")
  content=$(echo "$content" | sed "s|/home/[^/]*/[^/]*/||g")  
  content=$(echo "$content" | sed "s|C:\\Users\\[^\\]*\\[^\\]*\\||g")
  echo "$content"
}
```

## PM命令集成

### issue-sync 命令更新
- 在同步前自动清理路径格式
- 使用相对路径模板生成评论
- 记录deliverable时使用标准化路径

### epic-sync 命令更新
- 任务文件路径标准化
- GitHub issue body路径清理
- 映射文件使用相对路径

## 验证检查

### 自动检查脚本
```bash
# 检查文档中的绝对路径
check_absolute_paths() {
  echo "检查绝对路径违规..."
  rg -n "/Users/|/home/|C:\\\\" .claude/ || echo "✅ 未发现绝对路径"
}

# 检查GitHub同步内容
check_sync_content() {
  echo "检查同步内容路径格式..."
  # 实施具体检查逻辑
}
```

### 手动审查清单
- [ ] GitHub Issues评论无绝对路径
- [ ] 本地文档统一使用相对路径
- [ ] 代码注释路径符合规范
- [ ] 配置文件路径标准化

## 错误处理

### 发现违规路径时
1. **立即处理**：清理已发布的公共内容
2. **批量修复**：更新本地文档格式
3. **预防措施**：更新生成模板

### 紧急情况处理
如果发现隐私信息已泄露：
1. 立即编辑GitHub Issues/评论
2. 清理Git历史记录（如需要）
3. 更新相关文档和模板
4. 建立监控机制防止复发

## 示例对比

### 文档更新前后对比
```markdown
# 更新前 ❌
- ✅ 实现了 `/Users/username/parent-dir/project-name/internal/mcp/server.go` 核心逻辑

# 更新后 ✅  
- ✅ 实现了 `../project-name/internal/mcp/server.go` 核心逻辑
```

### GitHub评论格式
```markdown
# 正确格式 ✅
## 📦 Deliverables
- `internal/formatter/batch.go` - 批量格式化器
- `internal/processor/sorter.go` - 排序算法  
- `cmd/server/main.go` - 服务器入口

# 错误格式 ❌
## 📦 Deliverables  
- `/Users/username/parent-dir/project-name/internal/formatter/batch.go`
```

这个规范将确保项目文档的专业性、可移植性和隐私安全。