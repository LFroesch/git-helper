package git

import (
	"os"
	"path/filepath"
)

// HookType represents a type of git hook
type HookType string

const (
	HookConventionalCommits HookType = "conventional-commits"
	HookNoLargeFiles        HookType = "no-large-files"
	HookDetectSecrets       HookType = "detect-secrets"
)

// HookInfo describes an available hook
type HookInfo struct {
	Type        HookType
	Name        string
	Description string
	HookName    string // git hook name (commit-msg, pre-commit, etc.)
}

// AvailableHooks returns all hooks that can be installed
func AvailableHooks() []HookInfo {
	return []HookInfo{
		{HookConventionalCommits, "Conventional Commits", "Enforce conventional commit format", "commit-msg"},
		{HookNoLargeFiles, "No Large Files", "Block files >5MB from commits", "pre-commit"},
		{HookDetectSecrets, "Detect Secrets", "Block commits with passwords/keys", "pre-commit"},
	}
}

// Hook script for conventional commit validation
const commitMsgHookScript = `#!/bin/sh
# Conventional Commit Message Validator
# Installed by gitty

commit_msg_file=$1
commit_msg=$(cat "$commit_msg_file")

# Pattern: type(scope): description or type: description
pattern="^(feat|fix|docs|style|refactor|test|chore|perf|ci|build|revert)(\([a-zA-Z0-9_-]+\))?: .{1,}"

if ! echo "$commit_msg" | grep -qE "$pattern"; then
    echo "ERROR: Commit message does not follow conventional commit format."
    echo ""
    echo "Expected format: type(scope): description"
    echo "  or: type: description"
    echo ""
    echo "Valid types: feat, fix, docs, style, refactor, test, chore, perf, ci, build, revert"
    echo ""
    echo "Examples:"
    echo "  feat(auth): add login functionality"
    echo "  fix: resolve memory leak"
    echo "  docs: update README"
    echo ""
    echo "Your message: $commit_msg"
    exit 1
fi

exit 0
`

// Hook script to prevent large files
const noLargeFilesHookScript = `#!/bin/sh
# No Large Files Hook
# Installed by gitty
# Prevents files larger than 5MB from being committed

max_size=5242880  # 5MB in bytes

# Get list of staged files
staged_files=$(git diff --cached --name-only --diff-filter=ACM)

for file in $staged_files; do
    if [ -f "$file" ]; then
        file_size=$(wc -c < "$file" | tr -d ' ')
        if [ "$file_size" -gt "$max_size" ]; then
            size_mb=$(echo "scale=2; $file_size / 1048576" | bc)
            echo "ERROR: File '$file' is ${size_mb}MB which exceeds the 5MB limit."
            echo ""
            echo "Consider:"
            echo "  - Adding to .gitignore"
            echo "  - Using Git LFS for large files"
            echo "  - Compressing the file"
            exit 1
        fi
    fi
done

exit 0
`

// Hook script to detect secrets
const detectSecretsHookScript = `#!/bin/sh
# Detect Secrets Hook
# Installed by gitty
# Prevents commits containing passwords, API keys, or other secrets

# Get staged file contents
staged_diff=$(git diff --cached)

# Patterns to detect (case-insensitive where possible)
patterns="
password\s*[:=]\s*['\"][^'\"]+['\"]
api[_-]?key\s*[:=]\s*['\"][^'\"]+['\"]
secret[_-]?key\s*[:=]\s*['\"][^'\"]+['\"]
private[_-]?key\s*[:=]\s*['\"][^'\"]+['\"]
access[_-]?token\s*[:=]\s*['\"][^'\"]+['\"]
auth[_-]?token\s*[:=]\s*['\"][^'\"]+['\"]
bearer\s+[a-zA-Z0-9_-]+
-----BEGIN\s+(RSA|DSA|EC|OPENSSH)\s+PRIVATE\s+KEY-----
AKIA[0-9A-Z]{16}
"

found_secrets=0

echo "$patterns" | while read -r pattern; do
    if [ -n "$pattern" ]; then
        if echo "$staged_diff" | grep -qiE "$pattern"; then
            if [ "$found_secrets" -eq 0 ]; then
                echo "ERROR: Potential secrets detected in staged changes!"
                echo ""
            fi
            echo "  Pattern matched: $pattern"
            found_secrets=1
        fi
    fi
done

if echo "$staged_diff" | grep -qiE "password\s*[:=]\s*['\"][^'\"]+['\"]|api[_-]?key\s*[:=]|secret[_-]?key\s*[:=]|private[_-]?key|-----BEGIN.*(RSA|DSA|EC|OPENSSH).*PRIVATE.*KEY-----|AKIA[0-9A-Z]{16}"; then
    echo "ERROR: Potential secrets detected in staged changes!"
    echo ""
    echo "If this is a false positive, you can:"
    echo "  - Use environment variables instead of hardcoding"
    echo "  - Add the file to .gitignore"
    echo "  - Remove the hook with: gitty > Tools > Hooks > Remove"
    exit 1
fi

exit 0
`

// IsHookInstalled checks if a git hook is installed
func IsHookInstalled(repoPath, hookName string) bool {
	hookPath := filepath.Join(repoPath, ".git", "hooks", hookName)
	info, err := os.Stat(hookPath)
	if err != nil {
		return false
	}
	// Check if it's executable
	return info.Mode()&0111 != 0
}

// InstallHook installs a git hook with the given content
func InstallHook(repoPath, hookName, content string) error {
	hooksDir := filepath.Join(repoPath, ".git", "hooks")

	// Ensure hooks directory exists
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return err
	}

	hookPath := filepath.Join(hooksDir, hookName)

	// Write hook file
	if err := os.WriteFile(hookPath, []byte(content), 0755); err != nil {
		return err
	}

	return nil
}

// RemoveHook removes a git hook
func RemoveHook(repoPath, hookName string) error {
	hookPath := filepath.Join(repoPath, ".git", "hooks", hookName)
	return os.Remove(hookPath)
}

// InstallCommitMsgHook installs the conventional commit validator hook
func InstallCommitMsgHook(repoPath string) error {
	return InstallHook(repoPath, "commit-msg", commitMsgHookScript)
}

// RemoveCommitMsgHook removes the commit-msg hook
func RemoveCommitMsgHook(repoPath string) error {
	return RemoveHook(repoPath, "commit-msg")
}

// IsCommitMsgHookInstalled checks if the commit-msg hook is installed
func IsCommitMsgHookInstalled(repoPath string) bool {
	return IsHookInstalled(repoPath, "commit-msg")
}

// InstallNoLargeFilesHook installs the no-large-files pre-commit hook
func InstallNoLargeFilesHook(repoPath string) error {
	return InstallHook(repoPath, "pre-commit", noLargeFilesHookScript)
}

// InstallDetectSecretsHook installs the detect-secrets pre-commit hook
func InstallDetectSecretsHook(repoPath string) error {
	return InstallHook(repoPath, "pre-commit", detectSecretsHookScript)
}

// RemovePreCommitHook removes the pre-commit hook
func RemovePreCommitHook(repoPath string) error {
	return RemoveHook(repoPath, "pre-commit")
}

// IsPreCommitHookInstalled checks if any pre-commit hook is installed
func IsPreCommitHookInstalled(repoPath string) bool {
	return IsHookInstalled(repoPath, "pre-commit")
}

// InstallHookByType installs a hook by its type
func InstallHookByType(repoPath string, hookType HookType) error {
	switch hookType {
	case HookConventionalCommits:
		return InstallCommitMsgHook(repoPath)
	case HookNoLargeFiles:
		return InstallNoLargeFilesHook(repoPath)
	case HookDetectSecrets:
		return InstallDetectSecretsHook(repoPath)
	default:
		return nil
	}
}

// GetInstalledHooks returns which hooks are currently installed
func GetInstalledHooks(repoPath string) []HookType {
	var installed []HookType
	if IsCommitMsgHookInstalled(repoPath) {
		installed = append(installed, HookConventionalCommits)
	}
	if IsPreCommitHookInstalled(repoPath) {
		// We can't tell which pre-commit hook is installed, so mark both as potentially installed
		installed = append(installed, HookNoLargeFiles)
	}
	return installed
}
