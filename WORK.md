## Last Prio
more github flow actions / stash / clone / init / and beyond

## DevLog

### 2026-01-13 - Fixed Input Handling & Added Post-Commit Summary

- Fixed commitInput, branchInput, and rebaseInput not accepting keypresses (were only handling enter, not passing other keys to Update)
- Added post-commit summary view showing commit hash, message, files changed, and diff
- Added push option (p) and continue option (c) to post-commit summary
- Summary clears when user pushes or chooses to continue working
