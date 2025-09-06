---
created: 2025-09-06T05:07:21Z
last_updated: 2025-09-06T05:07:21Z
version: 1.0
author: Claude Code PM System
---

# System Patterns

## Core Architectural Patterns

### 1. Context Firewall Pattern
**Problem**: Claude Code conversations lose context as they grow
**Solution**: Specialized agents read multiple files and return single summaries

```
Traditional: Main thread reads 10 files → Context explosion → Lost coherence
Agent Pattern: Agent reads 10 files → Main thread gets 1 summary → Context preserved
```

**Implementation**:
- `file-analyzer` agent for log files and verbose outputs
- `code-analyzer` agent for code analysis and bug tracing
- `test-runner` agent for test execution and result analysis
- `parallel-worker` agent for multi-stream coordination

### 2. Command-as-Markdown Pattern
**Problem**: Complex workflows difficult to reproduce and version
**Solution**: Commands defined as structured markdown with specialized prompts

**Structure**:
```markdown
# Command Name
Description and context

## Instructions
Detailed execution steps

$ARGUMENTS (dynamic parameter injection)
```

**Benefits**:
- Version-controlled workflow definitions
- Self-documenting command system
- Consistent execution patterns

### 3. Trust-Based Validation Pattern
**Problem**: Over-validation slows development velocity
**Solution**: Fail fast on critical items, trust system for routine operations

**Rules**:
- **Critical**: GitHub auth, write permissions, git repository
- **Optional**: Extensions, remote repositories, specific configurations
- **Trusted**: GitHub CLI commands, git operations, file system access

### 4. GitHub-as-Database Pattern
**Problem**: Project state scattered across tools and conversations
**Solution**: GitHub Issues serve as single source of truth

**Implementation**:
- **Epics**: Parent issues for major features
- **Tasks**: Child issues for specific work items
- **Status**: Issue states track progress
- **Context**: Comments preserve decisions and context
- **Traceability**: Links connect requirements to code

## Design Principles

### Fail Fast Philosophy
```bash
# Check critical preconditions immediately
command -v gh >/dev/null || { echo "GitHub CLI required"; exit 1; }

# Continue with optional features
gh extension list | grep -q sub-issue || echo "Warning: sub-issue extension recommended"
```

### Graceful Degradation
- Core functionality works without optional dependencies
- Warning messages for missing enhancements
- Fallback modes for network issues

### Minimal Flying Checks
- Validate only what frequently breaks
- Skip verification of stable system components
- Focus preflight checks on user-configurable items

## Data Flow Patterns

### 1. PRD → Epic → Task → Code Flow
```
PRD (Requirements) → Epic (GitHub Issue) → Tasks (Child Issues) → Code (Commits)
```

### 2. Bidirectional Sync Pattern
```
Local Context ⇌ GitHub Issues ⇌ Remote Repository
```

### 3. Parallel Execution Pattern
```
Main Thread → Multiple Agents → Git Worktrees → Consolidated Results
```

## Error Handling Patterns

### Progressive Error Response
1. **Detection**: Identify issue immediately
2. **Classification**: Critical vs. optional failure
3. **Response**: Fail fast or continue with degradation
4. **Recovery**: Provide specific remediation steps

### Standardized Error Messages
```bash
# Format: ❌ {Component}: {Issue}. {Action}
echo "❌ GitHub CLI: Not authenticated. Run 'gh auth login'"
echo "⚠️ Extension: sub-issue not found. Install with 'gh extension install owner/gh-sub-issue'"
echo "ℹ️ Status: No active issues found. Create with '/pm:prd-new name'"
```

## Integration Patterns

### Agent Coordination
- **Single responsibility**: Each agent handles one concern
- **Context isolation**: Agents don't share state
- **Result aggregation**: Main thread combines agent outputs
- **Error propagation**: Agent failures reported to main thread

### GitHub API Interaction
- **CLI-first**: Use `gh` command instead of direct API calls
- **Batch operations**: Group related API calls
- **Rate limiting**: Respect GitHub API limits
- **Offline resilience**: Cache critical data locally

## Quality Assurance Patterns

### File Validation
```bash
# Verify file creation and content
[[ -f "$file" && -s "$file" ]] || error "File creation failed: $file"
```

### Command Validation
```bash
# Verify command success and expected output
command_output=$(gh issue list) || error "GitHub CLI command failed"
[[ -n "$command_output" ]] || warn "No issues found"
```

### Context Validation
```bash
# Verify context files are properly formatted
grep -q "^---$" "$context_file" || error "Missing YAML frontmatter: $context_file"
```