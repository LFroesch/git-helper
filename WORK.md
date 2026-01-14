## Last Prio
more github flow actions / stash / clone / init / and beyond
fix ahead/behind tracking?
refactor into internal

## DevLog

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
