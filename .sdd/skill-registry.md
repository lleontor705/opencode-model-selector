# Skill Registry — opencode-model-selector

Generated: 2026-07-22T11:31:00Z

## Skills

| Name | Source | Path | Trigger / Description |
|------|--------|------|-----------------------|
| bootstrap | user | `C:/Users/usrluisleon/.claude/skills/bootstrap/SKILL.md` | Initialize SDD context: detect tech stack, conventions, persistence backend, build skill registry |
| go-testing | user | `C:/Users/usrluisleon/.claude/skills/go-testing/SKILL.md` | **Go tests, go test coverage, Bubbletea teatest, golden files. Table-driven patterns, TUI testing.** |
| implement | user | `C:/Users/usrluisleon/.claude/skills/implement/SKILL.md` | Execute SDD implementation tasks, writing production code that satisfies specs and design |
| validate | user | `C:/Users/usrluisleon/.claude/skills/validate/SKILL.md` | Verify implementation satisfies specs, design, and tasks with real execution evidence |
| draft-proposal | user | `C:/Users/usrluisleon/.claude/skills/draft-proposal/SKILL.md` | Create a change proposal with intent, scope, approach, risks, and rollback plan |
| write-specs | user | `C:/Users/usrluisleon/.claude/skills/write-specs/SKILL.md` | Transform SDD proposal into domain-grouped delta specs with Given/When/Then scenarios |
| architect | user | `C:/Users/usrluisleon/.claude/skills/architect/SKILL.md` | Design technical architecture for an SDD change: decisions, data flows, file changes, testing strategy |
| decompose | user | `C:/Users/usrluisleon/.claude/skills/decompose/SKILL.md` | Break SDD design into phased, dependency-ordered tasks with parallel groups |
| team-lead | user | `C:/Users/usrluisleon/.claude/skills/team-lead/SKILL.md` | Coordinate apply phase execution — launches @implement sub-agents in parallel |
| finalize | user | `C:/Users/usrluisleon/.claude/skills/finalize/SKILL.md` | Merge delta specs into main specs, archive completed change, generate retrospective |
| investigate | user | `C:/Users/usrluisleon/.claude/skills/investigate/SKILL.md` | Explore codebase, diagnose bugs, or assess migrations with structured analysis |
| ideate | user | `C:/Users/usrluisleon/.claude/skills/ideate/SKILL.md` | Collaborative ideation — explore user intent, requirements, constraints before implementation |
| debug | user | `C:/Users/usrluisleon/.claude/skills/debug/SKILL.md` | Systematic root-cause debugging for bugs, test failures, unexpected behavior |
| debate | user | `C:/Users/usrluisleon/.claude/skills/debate/SKILL.md` | Adversarial debate moderator — parallel agents defending competing positions |
| judgment-day | user | `C:/Users/usrluisleon/.claude/skills/judgment-day/SKILL.md` | Blind dual review, fix confirmed issues, then re-judge |
| work-unit-commits | user | `C:/Users/usrluisleon/.claude/skills/work-unit-commits/SKILL.md` | **Plan commits as reviewable work units — implementation, commit splitting, chained PRs** |
| open-pr | user | `C:/Users/usrluisleon/.claude/skills/open-pr/SKILL.md` | Create pull requests following issue-first enforcement |
| file-issue | user | `C:/Users/usrluisleon/.claude/skills/file-issue/SKILL.md` | Create GitHub issues using required templates (bug report or feature request) |
| branch-pr | user | `C:/Users/usrluisleon/.claude/skills/branch-pr/SKILL.md` | Create Gentle AI pull requests with issue-first checks |
| chained-pr | user | `C:/Users/usrluisleon/.claude/skills/chained-pr/SKILL.md` | **Split oversized changes into chained PRs that protect review focus** |
| issue-creation | user | `C:/Users/usrluisleon/.claude/skills/issue-creation/SKILL.md` | Create Gentle AI issues with issue-first checks |
| monitor | user | `C:/Users/usrluisleon/.claude/skills/monitor/SKILL.md` | Generate HTML dashboard visualizing SDD pipeline state, tasks, agent activity |
| parallel-dispatch | user | `C:/Users/usrluisleon/.claude/skills/parallel-dispatch/SKILL.md` | Dispatch independent tasks to parallel sub-agents with isolated context |
| execute-plan | user | `C:/Users/usrluisleon/.claude/skills/execute-plan/SKILL.md` | Execute a written implementation plan with review checkpoints and progress tracking |
| skill-registry | user | `C:/Users/usrluisleon/.claude/skills/skill-registry/SKILL.md` | Index available skills by trigger and path |
| skill-creator | user | `C:/Users/usrluisleon/.claude/skills/skill-creator/SKILL.md` | Create LLM-first skills with valid frontmatter |
| skill-improver | user | `C:/Users/usrluisleon/.claude/skills/skill-improver/SKILL.md` | Audit and upgrade existing LLM-first skills |
| cognitive-doc-design | user | `C:/Users/usrluisleon/.claude/skills/cognitive-doc-design/SKILL.md` | **Design docs that reduce cognitive load — guides, READMEs, RFCs, onboarding** |
| comment-writer | user | `C:/Users/usrluisleon/.claude/skills/comment-writer/SKILL.md` | **Write warm, direct collaboration comments — PR feedback, reviews** |
| sdd-init | user | `C:/Users/usrluisleon/.claude/skills/sdd-init/SKILL.md` | Initialize SDD context, testing capabilities, registry, persistence (delegate-only) |
| sdd-explore | user | `C:/Users/usrluisleon/.claude/skills/sdd-explore/SKILL.md` | Explore SDD ideas before committing to a change (delegate-only) |
| sdd-propose | user | `C:/Users/usrluisleon/.claude/skills/sdd-propose/SKILL.md` | Create SDD change proposal with intent, scope, approach (delegate-only) |
| sdd-spec | user | `C:/Users/usrluisleon/.claude/skills/sdd-spec/SKILL.md` | Write SDD delta specs with requirements and scenarios (delegate-only) |
| sdd-design | user | `C:/Users/usrluisleon/.claude/skills/sdd-design/SKILL.md` | Create SDD technical design and architecture approach (delegate-only) |
| sdd-tasks | user | `C:/Users/usrluisleon/.claude/skills/sdd-tasks/SKILL.md` | Break SDD change into implementation tasks (delegate-only) |
| sdd-apply | user | `C:/Users/usrluisleon/.claude/skills/sdd-apply/SKILL.md` | Implement SDD tasks from specs and design (delegate-only) |
| sdd-verify | user | `C:/Users/usrluisleon/.claude/skills/sdd-verify/SKILL.md` | Execute tests and prove implementation matches specs (delegate-only) |
| sdd-archive | user | `C:/Users/usrluisleon/.claude/skills/sdd-archive/SKILL.md` | Archive a completed SDD change by syncing delta specs (delegate-only) |
| sdd-onboard | user | `C:/Users/usrluisleon/.claude/skills/sdd-onboard/SKILL.md` | Walk users through the SDD workflow on the real codebase |

## Convention Files

| File | Path |
|------|------|
| CLAUDE.md | `C:/Users/usrluisleon/.claude/CLAUDE.md` |
| Cortex Convention | `C:/Users/usrluisleon/.cortex-ia/skills/_shared/cortex-convention.md` |

## Project-Relevant Skills (Priority)

Skills marked with **bold** triggers above are highest priority for this Go CLI + Bubbletea project:

1. **go-testing** — Go table-driven tests, Bubbletea `Model.Update()` testing, `teatest` for interactive flows, golden files
2. **work-unit-commits** — commit planning for reviewable work units
3. **chained-pr** — if implementation exceeds 400 lines per PR
4. **cognitive-doc-design** — README and CLI help docs
5. **comment-writer** — review feedback style

## Excluded (Not Relevant to Go Project)

- dotnet-best-practices, dotnet-design-pattern-review, dotnet-upgrade — .NET/C# only
- frontend-design — web UI design, not CLI
- find-skills, brainstorming — general purpose, not project-specific
