---
created: 2025-09-06T05:07:21Z
last_updated: 2025-09-06T05:07:21Z
version: 1.0
author: Claude Code PM System
---

# Product Context

## Target Users

### Primary: Development Teams Using Claude Code
**Profile**: Software development teams leveraging Claude Code for accelerated development
**Pain Points**:
- Context loss between Claude sessions
- Parallel work conflicts and blocking
- Requirements drift and specification gaps
- Invisible progress until delivery
- Difficulty tracking work across team members

**Goals**:
- Maintain context continuity across development sessions
- Enable parallel development without conflicts
- Enforce spec-driven development practices
- Provide real-time progress visibility
- Scale development velocity with AI assistance

### Secondary: Solo Developers and Consultants
**Profile**: Individual developers managing complex projects
**Pain Points**:
- Overwhelming context when returning to projects
- Difficulty maintaining development momentum
- Lack of structured approach to AI-assisted development

**Goals**:
- Quick project re-entry after breaks
- Structured development workflow
- Consistent code quality and architecture

## Core Functionality

### 1. Spec-Driven Development Workflow
**PRD Creation**: Product Requirements Document as starting point
- Structured template for capturing requirements
- Business context and user needs documentation
- Technical constraints and architectural decisions

**Epic Decomposition**: Break PRDs into manageable GitHub Issues
- Automatic task breakdown from requirements
- Parent-child issue relationships
- Proper scoping and dependency management

**Traceability**: Full requirement-to-code tracking
- Links from code commits back to original requirements
- Change impact analysis across the stack
- Audit trail for compliance and reviews

### 2. Parallel Execution System
**Git Worktree Integration**: Multiple agents work simultaneously
- Isolated working directories for each agent
- Conflict-free parallel development
- Automatic merge and integration

**Agent Coordination**: Specialized Claude instances
- Context-optimized sub-agents for specific tasks
- Prevent main conversation context explosion
- Maintain coherence across complex operations

**GitHub Issues as Database**: Centralized coordination
- Single source of truth for project state
- Team-wide visibility and collaboration
- Real-time progress tracking

### 3. Context Management System
**Project Context Preservation**: Maintain development state
- Automatic context capture and storage
- Quick context loading for new sessions
- Structured knowledge base for complex projects

**Session Continuity**: Seamless handoffs
- Previous work summary and current state
- Next steps and priority identification
- Team member coordination and status

## Use Cases

### Development Team Scenarios

**Sprint Planning**:
1. Product manager creates PRD with requirements
2. System decomposes into GitHub Issues automatically
3. Team members claim issues and work in parallel
4. Progress visible in real-time through GitHub integration

**Feature Development**:
1. Developer starts issue with `/pm:issue-start 1234`
2. Dedicated agent works on feature implementation
3. Main developer continues on other tasks
4. Agent completes work and reports back
5. Developer reviews and integrates changes

**Bug Fixing**:
1. Bug reported as GitHub Issue with reproduction steps
2. Agent analyzes codebase to identify root cause
3. Implements fix while maintaining architectural patterns
4. Runs tests and validates solution
5. Links fix back to original issue for traceability

### Individual Developer Scenarios

**Project Re-entry**:
1. Developer returns to project after weeks away
2. Runs `/context:prime` to load project context
3. Uses `/pm:status` to see current state
4. Identifies next priority with `/pm:next`
5. Continues development with full context

**Complex Refactoring**:
1. Create epic for refactoring initiative
2. Break into smaller tasks with clear scope
3. Multiple agents work on different aspects
4. Coordinated integration without conflicts
5. Full test coverage and validation

## Success Metrics

### Development Velocity
- **Context Recovery Time**: Minutes instead of hours to understand current state
- **Parallel Efficiency**: Multiple workstreams without blocking
- **Bug Reduction**: Spec-driven development reduces specification gaps

### Team Collaboration
- **Visibility**: Real-time progress tracking through GitHub Issues
- **Coordination**: Clear task ownership and dependencies
- **Knowledge Sharing**: Structured context available to all team members

### Code Quality
- **Traceability**: Every change linked to business requirement
- **Consistency**: Architectural patterns maintained across features
- **Documentation**: Self-documenting through issue history and comments