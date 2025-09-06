---
created: 2025-09-06T05:07:21Z
last_updated: 2025-09-06T05:07:21Z
version: 1.0
author: Claude Code PM System
---

# Technology Context

## Primary Technologies

**Core Platform**: Claude Code PM System
- **Language**: Shell scripting (Bash)
- **Platform**: Cross-platform (macOS, Linux)
- **Version Control**: Git
- **Documentation**: Markdown

## Dependencies

**Required Tools**:
- **GitHub CLI** (`gh`): GitHub API integration and issue management
  - Must be authenticated with GitHub account
  - Used for all GitHub operations (issues, PRs, repositories)
- **Git**: Version control and worktree management
  - Required for parallel execution system
  - Handles branch and worktree coordination

**GitHub CLI Extensions**:
- **`gh-sub-issue`**: Parent-child issue relationship management
  - Enables epic → task decomposition
  - Provides hierarchical issue organization

## System Architecture

**Execution Environment**:
- **Shell-based**: All core operations use Bash scripts
- **Markdown-driven**: Commands defined as structured prompts
- **Git worktree integration**: Parallel execution without conflicts
- **GitHub Issues as database**: Single source of truth for project state

**Development Tools**:
- **No build system**: Pure shell script execution
- **No package manager**: System dependencies only
- **No test framework**: Validation through system integration
- **No CI/CD pipeline**: GitHub Actions integration possible but not required

## Integration Points

**GitHub API Integration**:
- **Issues API**: Task creation, status tracking, comments
- **Projects API**: Board management and workflow visualization
- **Repository API**: Code integration and branch management
- **Search API**: Cross-repository task discovery

**Claude Code Integration**:
- **Agent System**: Specialized sub-processes for context optimization
- **Command System**: Structured prompts for reproducible workflows
- **Context Management**: Project knowledge persistence and loading

## File Formats

**Configuration**:
- **Markdown**: All documentation and command definitions
- **YAML frontmatter**: Metadata and versioning
- **Shell scripts**: Executable system integration

**Data Storage**:
- **GitHub Issues**: Task and epic tracking
- **Local context files**: Project knowledge base
- **Git history**: Change tracking and audit trail

## Version Requirements

**Minimum Versions**:
- **Git**: 2.23+ (worktree support)
- **GitHub CLI**: 2.0+ (modern API features)
- **Bash**: 4.0+ (associative arrays)

**Platform Support**:
- **macOS**: Primary development platform
- **Linux**: Full compatibility
- **Windows**: Limited (requires WSL or Git Bash)

## Performance Characteristics

**Scaling**:
- **Issues**: No practical limit (GitHub handles scaling)
- **Parallel agents**: Limited by system resources
- **Context files**: Recommended max 50MB total
- **Worktrees**: Limited by disk space

**Network Dependencies**:
- **GitHub API**: Required for issue management
- **Git remote**: Optional for code synchronization
- **Claude Code API**: Required for agent execution