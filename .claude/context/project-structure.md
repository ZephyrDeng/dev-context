---
created: 2025-09-06T05:07:21Z
last_updated: 2025-09-06T05:07:21Z
version: 1.0
author: Claude Code PM System
---

# Project Structure

## Root Directory

```
.
├── .claude/                 # Claude Code PM system core
├── .git/                    # Git version control
├── AGENTS.md               # Agent system documentation
├── CLAUDE.md               # Project guidance for Claude instances
├── COMMANDS.md             # Command reference documentation
├── LICENSE                 # MIT license
├── README.md               # Project overview and workflow guide
└── screenshot.webp         # System workflow visualization
```

## Claude System Architecture

```
.claude/
├── agents/                 # Specialized agent definitions
│   ├── code-analyzer.md    # Code analysis & vulnerability detection
│   ├── file-analyzer.md    # Log & file content summarization
│   ├── parallel-worker.md  # Multi-stream coordination
│   └── test-runner.md      # Test execution & analysis
├── commands/               # Command system (40+ commands)
│   ├── pm/                 # Project management commands
│   ├── context/            # Context management
│   └── testing/            # Test-related commands
├── context/                # Project context storage
│   └── *.md               # Context files (this directory)
├── rules/                  # Standardized patterns & rules
└── scripts/                # Executable shell scripts
    └── pm/                 # Project management scripts
```

## File Organization Patterns

**Documentation Structure**:
- Root-level `.md` files: Public-facing documentation
- `.claude/` directory: Internal system files and configurations
- Context files: Structured project knowledge base

**Command System**:
- Commands defined as markdown files with specialized prompts
- Shell scripts handle GitHub CLI integration and system operations
- Agent definitions specify context-optimized sub-processes

**Naming Conventions**:
- Kebab-case for files: `code-analyzer.md`, `epic-status.sh`
- Descriptive names that indicate function: `parallel-worker`, `test-runner`
- Consistent prefixes for related functionality: `pm:`, `context:`, `testing:`

## Module Organization

**Core Components**:
1. **Agent System**: Context-preserving specialized processors
2. **Command System**: Markdown-driven workflow automation
3. **Script Layer**: GitHub CLI and system integration
4. **Context Management**: Project knowledge persistence
5. **Documentation**: User and system guidance

**Data Flow**:
- Commands → Agents → Scripts → GitHub API → Issues/Code
- Context flows bidirectionally between all components
- Full traceability from requirements to implementation