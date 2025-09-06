# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **Claude Code PM System** - a GitHub Issues-based project management workflow designed specifically for Claude Code instances. It implements a complete PRD → Epic → Task → GitHub Issues → Code traceability system with parallel agent execution capabilities.

## Core Architecture

**Shell-based GitHub CLI Integration**: All operations use `gh` CLI for GitHub API interactions, treating GitHub Issues as the single source of truth for project coordination.

**Specialized Agent System**: Uses context-preserving sub-agents to prevent main thread context explosion:
- `code-analyzer` - Code analysis, vulnerability detection, logic flow tracing
- `file-analyzer` - Log file and verbose output summarization  
- `test-runner` - Test execution with comprehensive result analysis
- `parallel-worker` - Multi-stream coordination in git worktrees

**Command-as-Markdown Pattern**: All functionality exposed through `.claude/commands/` markdown files that define prompts for specialized agents.

## Essential Development Commands

### Initial Setup
```bash
/pm:init              # Install dependencies (gh CLI + extensions), configure GitHub auth
```

### PRD Workflow (Product Requirements → Code)
```bash
/pm:prd-new name      # Create new product requirements document
/pm:prd-parse name    # Convert PRD to technical implementation plan
/pm:epic-oneshot name # Decompose into tasks and push to GitHub Issues
```

### Execution & Monitoring
```bash
/pm:issue-start 1234  # Spawn dedicated agent to work on specific GitHub issue
/pm:next              # Get next priority task from GitHub
/pm:status            # Overall project status across all issues
/pm:sync              # Bidirectional sync with GitHub Issues
```

### Context Management
```bash
/context:create       # Create initial project context file
/context:prime        # Load project context into current conversation
/testing:run          # Execute tests via test-runner agent
```

## Key Configuration Requirements

**Dependencies**: 
- GitHub CLI (`gh`) must be installed and authenticated
- `gh-sub-issue` extension for parent-child issue relationships
- Git repository with configured remote origin

**Directory Structure**:
- `.claude/agents/` - Specialized agent definitions
- `.claude/commands/` - Command system (40+ PM commands)
- `.claude/rules/` - Standardized patterns and error handling
- `.claude/scripts/` - Executable shell scripts
- `.claude/context/` - Project context storage

## Core Design Principles

**Trust System**: Don't over-verify things that rarely fail. Check critical preconditions then proceed.

**Fail Fast Philosophy**: Critical configuration (missing GitHub auth) fails immediately; optional features (missing extensions) log and continue.

**Context Firewall**: Agents read multiple files and return single summaries to prevent main thread context explosion.

**GitHub as Database**: Issues serve as coordination layer for team collaboration, with full traceability from requirements to code.

**Parallel Execution**: Multiple agents can work simultaneously in same git worktree without interference.

## Absolute Quality Rules

From existing .claude/CLAUDE.md:
- NO PARTIAL IMPLEMENTATION
- NO CODE DUPLICATION - reuse existing functions
- IMPLEMENT TEST FOR EVERY FUNCTION  
- NO OVER-ENGINEERING - simple functions over enterprise patterns
- NO MIXED CONCERNS - proper separation of validation, API, database, UI
- NO RESOURCE LEAKS - close connections, clear timeouts, remove listeners

## Agent Usage Requirements

**MUST use specialized agents for**:
- File analysis: Always use `file-analyzer` for reading logs or verbose files
- Code analysis: Always use `code-analyzer` for searching code or tracing logic
- Test execution: Always use `test-runner` for running tests and analyzing results

This prevents context explosion and maintains conversation coherence while handling complex multi-file operations.