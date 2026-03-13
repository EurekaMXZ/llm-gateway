# Development Constraints

## Purpose
This document defines repository-level engineering constraints for contributors.

## Mandatory Constraints
1. Library-first
- Prefer mature external libraries when they can meet the requirement.
- For major features, record Build-vs-Buy rationale in ADR or PR description.

2. Scaffold-first submodule initialization
- Backend submodules must be initialized by `go mod init`.
- Frontend submodules must be initialized by `pnpm create vue` (or equivalent official scaffolds).
- Do not handcraft critical initialization files to fake submodule creation.

3. Documentation-only governance
- Constraints are documented to guide engineering consistency.
- Compliance is reviewed by maintainers.
- Merge decisions are made by maintainers; these constraints are not hard CI merge gates by default.

## Reviewer Checklist
- Build-vs-Buy rationale is documented.
- Submodule scaffold command is documented.
- Session status files are updated (`current.md`, history snapshot, `project_state.json`).
