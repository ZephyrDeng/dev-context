---
created: 2025-09-06T05:07:21Z
last_updated: 2025-09-06T05:07:21Z
version: 1.0
author: Claude Code PM System
---

# Project Overview

## Feature Summary

### Core Workflow Features

**📋 PRD Management**
- `/pm:prd-new name` - Create structured product requirements documents
- `/pm:prd-parse name` - Convert PRDs to technical implementation plans
- `/pm:prd-list` - View all project PRDs and their status
- `/pm:prd-status name` - Detailed PRD analysis and progress

**🎯 Epic & Task Management**
- `/pm:epic-oneshot name` - Decompose PRDs into GitHub Issues automatically
- `/pm:epic-list` - View all active epics and their child tasks
- `/pm:epic-status name` - Detailed epic progress with task breakdown
- `/pm:epic-show name` - Display epic details and task relationships

**⚡ Execution & Coordination**
- `/pm:issue-start 1234` - Spawn dedicated agent to work on specific issue
- `/pm:next` - Get next priority task based on project status
- `/pm:status` - Overall project health and progress dashboard
- `/pm:sync` - Bidirectional synchronization with GitHub Issues
- `/pm:standup` - Generate team standup report from GitHub activity

### Context Management Features

**🧠 Project Context**
- `/context:create` - Initialize comprehensive project context
- `/context:prime` - Load project context into current session
- `/context:update` - Refresh context with latest project changes

**🔍 Search & Discovery**
- `/pm:search query` - Search across all issues, PRDs, and context
- `/pm:blocked` - Identify blocked tasks and dependencies
- `/pm:in-progress` - View all currently active work items

**🧪 Testing Integration**
- `/testing:run` - Execute tests via specialized test-runner agent
- Test result analysis with actionable failure summaries
- Integration with project status and issue tracking

### Advanced Features

**🔄 Parallel Execution System**
- Multiple Claude Code agents work simultaneously without conflicts
- Git worktree integration for isolated parallel development
- Automatic coordination and result aggregation
- Context firewall prevents main thread information overload

**🎭 Specialized Agent System**
- `code-analyzer` - Deep code analysis and vulnerability detection
- `file-analyzer` - Log file and verbose output summarization
- `test-runner` - Comprehensive test execution and result analysis
- `parallel-worker` - Multi-stream task coordination

**📊 GitHub Integration**
- Native GitHub Issues as project database
- Parent-child issue relationships for epic management
- Real-time bidirectional synchronization
- Team collaboration through issue comments and assignments

## Current System State

### ✅ Fully Operational Components

**Command System**: 40+ project management commands ready for use
- All PM workflow commands implemented and tested
- Context management system fully functional
- Testing integration operational

**Agent Infrastructure**: All specialized agents deployed and configured
- Context optimization prevents conversation overload
- Parallel execution system ready for multi-agent coordination
- Error handling and recovery mechanisms in place

**Documentation**: Complete system documentation and user guides
- CLAUDE.md provides guidance for future Claude instances
- README.md contains comprehensive workflow documentation
- Command reference with examples and use cases

### 🔧 Integration Points

**GitHub CLI Integration**:
- GitHub authentication configured and validated
- Issue creation, management, and synchronization
- Parent-child relationships via gh-sub-issue extension
- Project boards and milestone tracking

**Git Worktree System**:
- Parallel development without conflicts
- Automatic branch management and merging
- Isolated working directories for each agent
- Change coordination and integration

**Claude Code Integration**:
- Specialized agent prompts optimized for specific tasks
- Context preservation across long-running workflows
- Structured output formats for consistent results
- Error propagation and handling

## Key Capabilities

### Development Workflow
1. **Requirements Analysis**: PRD creation with business context
2. **Task Decomposition**: Automatic breakdown into actionable issues
3. **Parallel Execution**: Multiple agents work simultaneously
4. **Progress Tracking**: Real-time visibility through GitHub integration
5. **Context Continuity**: Session handoffs without information loss

### Team Collaboration
1. **Shared Project State**: GitHub Issues as single source of truth
2. **Real-time Updates**: Bidirectional sync keeps everyone current
3. **Task Coordination**: Clear ownership and dependency management
4. **Progress Visibility**: Stakeholders see real-time development status

### Quality Assurance
1. **Spec-driven Development**: Every change traceable to requirements
2. **Architectural Consistency**: Patterns maintained across features
3. **Automated Testing**: Integration with existing test suites
4. **Review Process**: GitHub PR workflow for code quality

## Usage Patterns

### New Project Initialization
```bash
/pm:init                    # Setup system dependencies
/context:create             # Initialize project context
/pm:prd-new project-name    # Create first PRD
/pm:prd-parse project-name  # Generate implementation plan
/pm:epic-oneshot project-name # Create GitHub issues
```

### Daily Development Workflow
```bash
/context:prime              # Load project context
/pm:status                  # Check overall project health
/pm:next                    # Get next priority task
/pm:issue-start 1234        # Start work on specific issue
/pm:sync                    # Sync with GitHub when done
```

### Team Coordination
```bash
/pm:standup                 # Generate team status report
/pm:blocked                 # Identify blocking dependencies
/pm:in-progress            # See all active work
/pm:search "feature-name"   # Find related work across project
```