## Current Tasks

Nothing major - app is feature-complete for a git helper TUI.

## DevLog

### 2026-01-15 - Reset Commit & Tool Menu Background Fix

- **Reset Last Commit**: Added "R" key in workspace to do `git reset HEAD~1` (mixed reset - keeps changes unstaged)
- **Tool Menu Selection**: Fixed background styling on selected rows so text elements have consistent highlight

### 2026-01-15 - Confirmation Checks

Added double-press confirmation for irreversible/remote operations:
- Undo/reset commits
- Rebase execute
- Stash pop (removes from stash)
- Push to remote
- Pull from remote

Existing confirmations: discard changes, delete branch, drop stash, delete tag, revert, clean

### 2026-01-15 - Ahead/Behind & Hooks Polish

**Ahead/Behind**: Fixed tracking by parsing `git status -sb` output instead of `@{upstream}` (fails silently when no upstream)

**Hooks Menu**: Expanded to 3 pre-built hooks with selectable install
- Conventional Commits (commit-msg) - enforces format
- No Large Files (pre-commit) - blocks files >5MB
- Detect Secrets (pre-commit) - blocks passwords/API keys

UI: `1/2/3` or `enter` to install, `r` to remove, `j/k` to navigate

### 2026-01-15 - Major Fixes & Feature Completion

**Bug Fixes:**
- Fixed deprecated `strings.Title()` → `cases.Title(language.English)`
- Fixed commit number keys (1-9) conflicting with tab keys (1-4) - now uses arrow selection only

**Features Completed:**
- **Rebase**: Now executes via GIT_SEQUENCE_EDITOR script (pick/squash/reword/drop/fixup)
- **Clone/Init**: Switches to new repo after operation, reloads all data
- **Cherry-pick**: In log view, press 'c' to cherry-pick selected commit
- **Revert**: In log view, press 'R' (capital) to revert selected commit
- **Clean**: New tool mode ('x') - shows untracked files, 'd' to delete all
- **Conflicts**: Press 'c' in workspace to view merge conflicts, enter to see diff

**Code Cleanup:**
- Removed `renderDiffPreview()`, `renderFileList()`, `validateCommitMessage()`, `DiffInfo` struct

### 2026-01-15 - Core Git Flow Features

Added 4 major features to complete the git workflow:

**Log Viewer** (Tools > Log, key 'o')
- Browse 50 commits with search/filter (`/` key)
- View commit details: author, date, message, files, full diff
- Scrollable list and detail view

**Blame View** (Workspace, key 'b' on any file)
- Line-by-line authorship view
- Shows hash, author, date, line number, content
- Scrollable with j/k navigation

**Clone** (Tools > Clone, key 'c')
- Enter repo URL (HTTPS or SSH)
- Clones to current directory with repo name

**Init** (Tools > Init, key 'i')
- Initialize new git repo at specified path

Files changed: git.go (Clone/Init/GetCommitLog2/GetCommitDetail/GetCommitDiff/GetBlame), model.go, update.go, view.go, helpers.go

### 2026-01-14 - Major Tools Expansion & Scout-Style Keybinds

**Keybindings**: Refactored status bar to use scout-style formatting (purple keys + white descriptions + pipe separators)

**Git Hooks**: Implemented full hook management
- `internal/git/hooks.go` with install/remove/check functions
- Conventional commit validator hook script
- Tools > Hooks submenu with i/r/c keybinds

**Tools Tab Expansion**: Restructured with new features
- **Stash**: list, push, pop, apply, drop (s/p/a/d keys)
- **Tags**: list, create, delete, push, push all (n/d/p/P keys)
- **Hooks**: install, remove, check status (i/r/c keys)
- Reordered menu: Stash, Tags, History, Undo, Rebase, Push, Fetch/Pull, Hooks

**New Git Operations** (`internal/git/git.go`):
- Stash: GetStashList, StashPush, StashPop, StashApply, StashDrop, StashShow
- Tags: GetTags, CreateTag, DeleteTag, PushTag, PushAllTags
- Cherry-pick: CherryPick, CherryPickAbort, CherryPickContinue
- Revert: RevertCommit, RevertAbort
- Clean: CleanDryRun, CleanForce


### 2026-01-14 - Fixed Workspace Title Position & Gap Styling

- Workspace tab now wraps content in outer border like other tabs (title no longer floats above)
- Added background color (236) to all spacing: header bar rows, git status info, status bar padding

### 2026-01-13 - Fixed Foreground/Background Styling & Emojis

- Added `.Background("236")` to header/status bar styles (title, tabs, help, icons, branch ahead/behind)
- Fixed emojis: `⎇`→🌿 (branch), `⌂`→🏠 (local), `☁`→☁️ (remote)

### 2026-01-13 - Renamed to Gitty

- Renamed project from git-helper to gitty (shorter CLI name)
- Updated go.mod module path and README references

### 2026-01-13 - Fixed Input Handling & Added Post-Commit Summary

- Fixed commitInput, branchInput, and rebaseInput not accepting keypresses (were only handling enter, not passing other keys to Update)
- Added post-commit summary view showing commit hash, message, files changed, and diff
- Added push option (p) and continue option (c) to post-commit summary
- Summary clears when user pushes or chooses to continue working

### 2026-01-13 - Implemented Scrollable Content with Fixed Height Boxes

- Added scrollOffset field to model for tracking scroll position
- Completely rewrote renderCommitSummary to use scout-style scrolling pattern:
  - Content locked into fixed-height boxes based on terminal size
  - Only visible content rendered (offset-based slicing)
  - Scroll indicators (▲/▼) show when more content above/below
  - j/k/up/down keys scroll through content
- Applied height constraints following scout's pattern (availableHeight = m.height - uiOverhead)
- Content now properly fits within terminal bounds and scrolls smoothly
