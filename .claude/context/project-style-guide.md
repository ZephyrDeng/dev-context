---
created: 2025-09-06T05:07:21Z
last_updated: 2025-09-06T05:07:21Z
version: 1.0
author: Claude Code PM System
---

# Project Style Guide

## Code Organization Standards

### Directory Structure Conventions
```
.claude/                    # System root directory
├── agents/                 # Agent definitions (kebab-case)
│   └── {purpose}-{type}.md
├── commands/               # Command definitions
│   ├── {category}/         # Command categories
│   └── {category}:{name}.md
├── context/                # Project context storage
│   └── {context-type}.md
├── rules/                  # System rules and patterns
│   └── {rule-category}.md
└── scripts/                # Executable scripts
    └── {category}/{command}.sh
```

### File Naming Patterns
**Consistency Rules**:
- **kebab-case** for all file names: `code-analyzer.md`, `epic-status.sh`
- **Descriptive names** that indicate purpose: `parallel-worker`, `test-runner`
- **Category prefixes** for commands: `pm:`, `context:`, `testing:`
- **Type suffixes** for clarity: `.md` for documentation, `.sh` for scripts

**Examples**:
```
✓ Good: file-analyzer.md, epic-oneshot.sh, context-create.md
✗ Bad: fileAnalyzer.md, EpicOneshot.sh, contextCreate.md
```

## Documentation Standards

### Markdown Structure
**Required Frontmatter** for all context files:
```yaml
---
created: YYYY-MM-DDTHH:MM:SSZ     # Real datetime from system
last_updated: YYYY-MM-DDTHH:MM:SSZ # Updated on each modification
version: X.Y                       # Semantic versioning
author: Claude Code PM System      # System attribution
---
```

**Document Structure**:
```markdown
# Title (H1 - exactly one per document)

## Section (H2 - main sections)

### Subsection (H3 - detailed topics)

**Bold** for emphasis and labels
*Italic* for terminology and concepts
`code` for commands, filenames, and technical terms
```

### Content Guidelines
**Writing Style**:
- **Concise and actionable**: Focus on what needs to be done
- **Specific examples**: Include concrete commands and code snippets
- **Structured information**: Use consistent formatting for similar content types
- **Progressive detail**: Start with overview, then dive into specifics

**Technical Documentation**:
- **Commands**: Always include example usage with expected output
- **Workflows**: Step-by-step instructions with validation points
- **Architecture**: Explain both structure and rationale
- **Patterns**: Include both implementation and usage examples

## Command System Conventions

### Command Definition Format
```markdown
# Command Name

Brief description of command purpose and context.

## Required Rules

**IMPORTANT:** Before executing this command, read and follow:
- Rule references for validation

## Preflight Checklist

Validation steps (do not bother user with progress)

## Instructions

Detailed execution steps

$ARGUMENTS (parameter injection point)
```

### Command Naming
**Pattern**: `{category}:{action}-{object}`

**Examples**:
- `pm:prd-new` - Project Management: Create new PRD
- `pm:issue-start` - Project Management: Start working on issue
- `context:create` - Context Management: Create initial context
- `testing:run` - Testing: Execute test suite

**Categories**:
- `pm:` - Project management and GitHub integration
- `context:` - Project context and knowledge management
- `testing:` - Test execution and analysis

## Shell Script Standards

### Script Structure
```bash
#!/bin/bash

# Script: {purpose}
# Usage: {usage_pattern}
# Dependencies: {required_tools}

set -euo pipefail  # Strict error handling

# Configuration
DEFAULT_VALUE="example"
REQUIRED_TOOL="gh"

# Validation
command -v "$REQUIRED_TOOL" >/dev/null || {
    echo "❌ $REQUIRED_TOOL required but not installed"
    exit 1
}

# Main function
main() {
    # Implementation
}

# Execute
main "$@"
```

### Error Handling Patterns
```bash
# Standardized error messages
echo "❌ {Component}: {Issue}. {Action}"          # Critical errors
echo "⚠️ {Component}: {Issue}. {Recommendation}"   # Warnings
echo "ℹ️ {Status}: {Information}"               # Information
echo "✅ {Action}: {Success_message}"             # Success
```

### Validation Patterns
```bash
# File existence and content
[[ -f "$file" && -s "$file" ]] || error "File missing or empty: $file"

# Command success with output
output=$(command_here) || error "Command failed"
[[ -n "$output" ]] || warn "No output from command"

# Directory and permissions
[[ -d "$dir" && -w "$dir" ]] || error "Directory not writable: $dir"
```

## Agent Definition Standards

### Agent Specification Format
```markdown
# Agent Name

**Purpose**: One-line description of agent responsibility

**Specialization**: Detailed explanation of unique capabilities

## Context Optimization

Explanation of how this agent prevents context explosion

## Usage Patterns

When and how to use this agent effectively

## Integration Points

How this agent coordinates with other system components
```

### Agent Naming
**Pattern**: `{function}-{type}`

**Examples**:
- `code-analyzer` - Code analysis functionality
- `file-analyzer` - File content processing
- `test-runner` - Test execution management
- `parallel-worker` - Multi-stream coordination

## Quality Standards

### Code Quality Rules
From existing .claude/CLAUDE.md (enforced absolutely):
- **NO PARTIAL IMPLEMENTATION** - Complete all functionality
- **NO CODE DUPLICATION** - Reuse existing functions and patterns
- **IMPLEMENT TEST FOR EVERY FUNCTION** - Comprehensive test coverage
- **NO OVER-ENGINEERING** - Simple solutions over complex abstractions
- **NO MIXED CONCERNS** - Clear separation of responsibilities
- **NO RESOURCE LEAKS** - Proper cleanup of connections and handles

### Documentation Quality
- **Accurate examples**: All code snippets must be tested and working
- **Current information**: Regular updates to reflect system changes
- **Complete coverage**: Document all features and edge cases
- **User-focused**: Write for the person who will use the system

### System Integration
- **Trust-based validation**: Fail fast on critical items, trust system for routine
- **Graceful degradation**: Continue with warnings for optional features
- **Error propagation**: Clear error messages with actionable guidance
- **Context preservation**: Always maintain conversation coherence

## Formatting Conventions

### Command Examples
```bash
# Always show full command with context
/pm:prd-new my-feature     # Creates PRD for new feature

# Include expected output when helpful
$ gh issue list
#123  Feature: User authentication  open  2024-01-15
#124  Bug: Login form validation      open  2024-01-16
```

### File References
- **Absolute paths**: `/Users/project/.claude/context/progress.md`
- **Relative paths**: `.claude/commands/pm/prd-new.md`
- **With line numbers**: `src/auth.js:42` (for code references)

### Status Indicators
- ✅ **Completed**: Tasks or features that are done
- 🔧 **In Progress**: Currently active work
- ⚠️ **Warning**: Issues that need attention
- ❌ **Error**: Critical problems requiring immediate action
- ℹ️ **Information**: Helpful context or status updates