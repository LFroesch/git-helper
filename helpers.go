package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/LFroesch/gitty/internal/git"
)

// Data loading commands

func (m model) loadGitChanges() tea.Cmd {
	return func() tea.Msg {
		changes := git.GetChanges(m.repoPath)
		return gitChangesMsg(changes)
	}
}

func (m model) loadGitStatus() tea.Cmd {
	return func() tea.Msg {
		status := git.GetStatus(m.repoPath)
		return gitStatusMsg(status)
	}
}

func (m model) loadBranches() tea.Cmd {
	return func() tea.Msg {
		branches := git.GetBranches(m.repoPath)
		remoteBranches := git.GetRemoteBranches(m.repoPath)
		return branchesMsg(append(branches, remoteBranches...))
	}
}

func (m model) loadRecentCommits() tea.Cmd {
	return func() tea.Msg {
		commits := git.GetCommitLog(m.repoPath, 3)
		return recentCommitsMsg(commits)
	}
}

func (m model) loadCommitHistory() tea.Cmd {
	return func() tea.Msg {
		commits := git.GetCommitLog(m.repoPath, 20)
		return commitsMsg(commits)
	}
}

func (m model) loadConflicts() tea.Cmd {
	return func() tea.Msg {
		files := git.GetConflictFiles(m.repoPath)
		var conflicts []git.ConflictFile
		for _, f := range files {
			conflicts = append(conflicts, git.ConflictFile{Path: f, IsResolved: false})
		}
		return conflictsMsg(conflicts)
	}
}

func (m model) loadFileDiff(filePath string) tea.Cmd {
	return func() tea.Msg {
		staged := git.IsFileStaged(m.repoPath, filePath)
		diff := git.GetFileDiff(m.repoPath, filePath, staged)
		return diffMsg(diff)
	}
}

func (m model) loadRebaseCommits() tea.Cmd {
	return func() tea.Msg {
		countStr := strings.TrimSpace(m.rebaseInput.Value())
		count, err := strconv.Atoi(countStr)
		if err != nil || count < 1 || count > 50 {
			return statusMsg{message: "Invalid count (1-50)"}
		}

		commits := git.GetCommitLog(m.repoPath, count)
		var rebaseCommits []git.RebaseCommit
		for _, c := range commits {
			rebaseCommits = append(rebaseCommits, git.RebaseCommit{
				Hash:    c.Hash,
				Message: c.Message,
				Action:  "pick",
			})
		}
		return rebaseCommitsMsg(rebaseCommits)
	}
}

// Staging operations

func (m model) toggleStaging(filePath string) tea.Cmd {
	return func() tea.Msg {
		isStaged := git.IsFileStaged(m.repoPath, filePath)

		var gitCmd []string
		var action string
		if isStaged {
			gitCmd = []string{"reset", "HEAD", filePath}
			action = "unstaged"
		} else {
			gitCmd = []string{"add", filePath}
			action = "staged"
		}

		output, err := git.Execute(m.repoPath, gitCmd...)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Failed to %s file: %v - %s", action, err, string(output))}
		}

		return tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			func() tea.Msg {
				return statusMsg{message: fmt.Sprintf("%s: %s", cases.Title(language.English).String(action), filePath)}
			},
		)()
	}
}

func (m model) gitAddAll() tea.Cmd {
	return func() tea.Msg {
		output, err := git.Execute(m.repoPath, "add", ".")
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Git add failed: %v - %s", err, string(output))}
		}

		return tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			func() tea.Msg {
				return statusMsg{message: "Added all files to staging"}
			},
		)()
	}
}

func (m model) gitReset() tea.Cmd {
	return func() tea.Msg {
		status := git.GetStatus(m.repoPath)
		if status.StagedFiles == 0 {
			return statusMsg{message: "No staged changes to reset"}
		}

		output, err := git.Execute(m.repoPath, "reset", "HEAD")
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Git reset failed: %v - %s", err, string(output))}
		}

		return tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			func() tea.Msg {
				return statusMsg{message: fmt.Sprintf("Reset %d staged file(s)", status.StagedFiles)}
			},
		)()
	}
}

func (m model) gitResetLastCommit() tea.Cmd {
	return func() tea.Msg {
		// Mixed reset: undo last commit, keep changes in working directory (unstaged)
		output, err := git.Execute(m.repoPath, "reset", "HEAD~1")
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Reset failed: %v - %s", err, string(output))}
		}

		return tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			m.loadRecentCommits(),
			func() tea.Msg {
				return statusMsg{message: "Reset last commit (changes kept in working directory)"}
			},
		)()
	}
}

func (m model) discardChanges(filePath string) tea.Cmd {
	return func() tea.Msg {
		output, err := git.Execute(m.repoPath, "checkout", "--", filePath)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Failed to discard changes: %v - %s", err, string(output))}
		}

		return tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			func() tea.Msg {
				return statusMsg{message: fmt.Sprintf("Discarded changes: %s", filePath)}
			},
		)()
	}
}

// Commit operations

func (m model) commitWithMessage(message string) tea.Cmd {
	return func() tea.Msg {
		files := git.GetStagedFiles(m.repoPath)
		if len(files) == 0 {
			return statusMsg{message: "No staged changes to commit"}
		}

		diff := git.GetStagedDiff(m.repoPath)

		_, err := git.Execute(m.repoPath, "commit", "-m", message)
		if err != nil {
			return statusMsg{message: "Commit failed - check commit message format"}
		}

		hash := git.GetCurrentCommitHash(m.repoPath)

		return commitSuccessMsg{
			hash:    hash,
			message: message,
			diff:    diff,
			files:   files,
		}
	}
}

func (m model) generateCommitSuggestions() tea.Cmd {
	return func() tea.Msg {
		changes := git.GetChanges(m.repoPath)
		if len(changes) == 0 {
			return commitSuggestionsMsg(nil)
		}

		var suggestions []CommitSuggestion
		typeCount := make(map[string]int)

		for _, change := range changes {
			changeType := categorizeChange(change)
			typeCount[changeType]++
		}

		// Generate suggestions based on change patterns
		for changeType, count := range typeCount {
			var msg string
			switch changeType {
			case "feat":
				msg = fmt.Sprintf("feat: add new feature (%d files)", count)
			case "fix":
				msg = fmt.Sprintf("fix: resolve issue (%d files)", count)
			case "docs":
				msg = fmt.Sprintf("docs: update documentation (%d files)", count)
			case "style":
				msg = fmt.Sprintf("style: improve formatting (%d files)", count)
			case "refactor":
				msg = fmt.Sprintf("refactor: improve code structure (%d files)", count)
			case "test":
				msg = fmt.Sprintf("test: add/update tests (%d files)", count)
			case "chore":
				msg = fmt.Sprintf("chore: update build/config (%d files)", count)
			default:
				msg = fmt.Sprintf("chore: update files (%d files)", count)
			}
			suggestions = append(suggestions, CommitSuggestion{Message: msg, Type: changeType})
		}

		return commitSuggestionsMsg(suggestions)
	}
}

func categorizeChange(change git.Change) string {
	file := strings.ToLower(change.File)

	if strings.Contains(file, "test") || strings.HasSuffix(file, "_test.go") {
		return "test"
	}
	if strings.HasSuffix(file, ".md") || strings.Contains(file, "doc") {
		return "docs"
	}
	if strings.Contains(file, "config") || strings.HasPrefix(file, ".") ||
		file == "makefile" || file == "dockerfile" {
		return "chore"
	}
	if change.Status == "A " {
		return "feat"
	}
	if strings.Contains(change.Status, "M") {
		return "refactor"
	}
	return "chore"
}

// Branch operations

func (m model) switchBranch(branchName string) tea.Cmd {
	return func() tea.Msg {
		var localBranchName string

		if strings.HasPrefix(branchName, "origin/") || strings.HasPrefix(branchName, "remotes/origin/") {
			localBranchName = strings.TrimPrefix(branchName, "remotes/origin/")
			localBranchName = strings.TrimPrefix(localBranchName, "origin/")

			output, err := git.Execute(m.repoPath, "checkout", "-b", localBranchName, branchName)
			if err != nil {
				if strings.Contains(string(output), "already exists") {
					_, err = git.Execute(m.repoPath, "checkout", localBranchName)
				}
				if err != nil {
					return statusMsg{message: fmt.Sprintf("Failed to switch branch: %s", string(output))}
				}
			}
		} else {
			localBranchName = branchName
			output, err := git.Execute(m.repoPath, "checkout", branchName)
			if err != nil {
				return statusMsg{message: fmt.Sprintf("Failed to switch branch: %s", string(output))}
			}
		}

		return tea.Batch(
			m.loadBranches(),
			m.loadGitStatus(),
			func() tea.Msg {
				return statusMsg{message: fmt.Sprintf("Switched to branch '%s'", localBranchName)}
			},
		)()
	}
}

func (m model) createBranch(branchName string) tea.Cmd {
	return func() tea.Msg {
		output, err := git.Execute(m.repoPath, "checkout", "-b", branchName)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Failed to create branch: %s", string(output))}
		}

		return tea.Batch(
			m.loadBranches(),
			m.loadGitStatus(),
			func() tea.Msg {
				return statusMsg{message: fmt.Sprintf("Created and switched to branch '%s'", branchName)}
			},
		)()
	}
}

func (m model) deleteBranch(branchName string) tea.Cmd {
	return func() tea.Msg {
		output, err := git.Execute(m.repoPath, "branch", "-d", branchName)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Failed to delete branch: %s", string(output))}
		}

		return tea.Batch(
			m.loadBranches(),
			func() tea.Msg {
				return statusMsg{message: fmt.Sprintf("Deleted branch '%s'", branchName)}
			},
		)()
	}
}

func (m model) compareBranch(targetBranch string) tea.Cmd {
	return func() tea.Msg {
		currentBranch := git.GetBranchName(m.repoPath)
		comparison := git.GetBranchComparison(m.repoPath, currentBranch, targetBranch)
		return comparisonMsg(comparison)
	}
}

// Remote operations

func (m model) pushChanges() tea.Cmd {
	return func() tea.Msg {
		output, err := git.Execute(m.repoPath, "push")
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Push failed: %s", string(output))}
		}

		hash := git.GetCurrentCommitHash(m.repoPath)
		return pushOutputMsg{output: string(output), commit: hash}
	}
}

func (m model) pullChanges() tea.Cmd {
	return func() tea.Msg {
		output, err := git.Execute(m.repoPath, "pull")
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Pull failed: %s", string(output))}
		}

		return tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			m.loadBranches(),
			func() tea.Msg {
				return statusMsg{message: "Pull successful"}
			},
		)()
	}
}

func (m model) fetchChanges() tea.Cmd {
	return func() tea.Msg {
		output, err := git.Execute(m.repoPath, "fetch")
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Fetch failed: %s", string(output))}
		}

		return tea.Batch(
			m.loadGitStatus(),
			func() tea.Msg {
				return statusMsg{message: "Fetch successful"}
			},
		)()
	}
}

// Undo operations

func (m model) undoToCommit(hash string) tea.Cmd {
	return func() tea.Msg {
		output, err := git.Execute(m.repoPath, "reset", "--soft", hash)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Undo failed: %s", string(output))}
		}

		return tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			m.loadCommitHistory(),
			func() tea.Msg {
				return statusMsg{message: fmt.Sprintf("Reset to commit %s", hash)}
			},
		)()
	}
}

// Rebase operations

func (m model) executeRebase() tea.Cmd {
	return func() tea.Msg {
		if len(m.rebaseCommits) == 0 {
			return statusMsg{message: "No commits to rebase"}
		}

		err := git.ExecuteRebase(m.repoPath, m.rebaseCommits)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Rebase failed: %v", err)}
		}

		return tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			m.loadCommitHistory(),
			func() tea.Msg {
				return statusMsg{message: "Rebase completed successfully"}
			},
		)()
	}
}

// Stash operations

func (m model) loadStashList() tea.Cmd {
	return func() tea.Msg {
		stashes := git.GetStashList(m.repoPath)
		return stashListMsg(stashes)
	}
}

func (m model) loadStashDiff(index int) tea.Cmd {
	return func() tea.Msg {
		diff := git.StashShow(m.repoPath, index)
		return stashDiffMsg(diff)
	}
}

func (m model) stashPush(message string) tea.Cmd {
	return func() tea.Msg {
		err := git.StashPush(m.repoPath, message)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Stash failed: %v", err)}
		}

		return tea.Batch(
			m.loadStashList(),
			m.loadGitChanges(),
			m.loadGitStatus(),
			func() tea.Msg {
				return statusMsg{message: "Changes stashed"}
			},
		)()
	}
}

func (m model) stashPop(index int) tea.Cmd {
	return func() tea.Msg {
		err := git.StashPop(m.repoPath, index)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Stash pop failed: %v", err)}
		}

		return tea.Batch(
			m.loadStashList(),
			m.loadGitChanges(),
			m.loadGitStatus(),
			func() tea.Msg {
				return statusMsg{message: "Stash popped"}
			},
		)()
	}
}

func (m model) stashApply(index int) tea.Cmd {
	return func() tea.Msg {
		err := git.StashApply(m.repoPath, index)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Stash apply failed: %v", err)}
		}

		return tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			func() tea.Msg {
				return statusMsg{message: "Stash applied (kept in stash list)"}
			},
		)()
	}
}

func (m model) stashDrop(index int) tea.Cmd {
	return func() tea.Msg {
		err := git.StashDrop(m.repoPath, index)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Stash drop failed: %v", err)}
		}

		return tea.Batch(
			m.loadStashList(),
			func() tea.Msg {
				return statusMsg{message: "Stash dropped"}
			},
		)()
	}
}

// Tag operations

func (m model) loadTags() tea.Cmd {
	return func() tea.Msg {
		tags := git.GetTags(m.repoPath)
		return tagListMsg(tags)
	}
}

func (m model) createTag(name, message string, annotated bool) tea.Cmd {
	return func() tea.Msg {
		err := git.CreateTag(m.repoPath, name, message, annotated)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Create tag failed: %v", err)}
		}

		return tea.Batch(
			m.loadTags(),
			func() tea.Msg {
				return statusMsg{message: fmt.Sprintf("Created tag '%s'", name)}
			},
		)()
	}
}

func (m model) deleteTag(name string) tea.Cmd {
	return func() tea.Msg {
		err := git.DeleteTag(m.repoPath, name)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Delete tag failed: %v", err)}
		}

		return tea.Batch(
			m.loadTags(),
			func() tea.Msg {
				return statusMsg{message: fmt.Sprintf("Deleted tag '%s'", name)}
			},
		)()
	}
}

func (m model) pushTag(name string) tea.Cmd {
	return func() tea.Msg {
		err := git.PushTag(m.repoPath, name)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Push tag failed: %v", err)}
		}

		return statusMsg{message: fmt.Sprintf("Pushed tag '%s' to remote", name)}
	}
}

func (m model) pushAllTags() tea.Cmd {
	return func() tea.Msg {
		err := git.PushAllTags(m.repoPath)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Push tags failed: %v", err)}
		}

		return statusMsg{message: "Pushed all tags to remote"}
	}
}

// Hook operations

func (m model) installConventionalCommitsHook() tea.Cmd {
	return func() tea.Msg {
		err := git.InstallCommitMsgHook(m.repoPath)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Install failed: %v", err)}
		}

		return tea.Batch(
			func() tea.Msg { return hookStatusMsg(true) },
			func() tea.Msg { return statusMsg{message: "Installed conventional commits hook"} },
		)()
	}
}

func (m model) installNoLargeFilesHook() tea.Cmd {
	return func() tea.Msg {
		err := git.InstallNoLargeFilesHook(m.repoPath)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Install failed: %v", err)}
		}

		return tea.Batch(
			func() tea.Msg { return preCommitHookMsg(true) },
			func() tea.Msg { return statusMsg{message: "Installed no-large-files hook (blocks files >5MB)"} },
		)()
	}
}

func (m model) installDetectSecretsHook() tea.Cmd {
	return func() tea.Msg {
		err := git.InstallDetectSecretsHook(m.repoPath)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Install failed: %v", err)}
		}

		return tea.Batch(
			func() tea.Msg { return preCommitHookMsg(true) },
			func() tea.Msg { return statusMsg{message: "Installed detect-secrets hook"} },
		)()
	}
}

func (m model) removeSelectedHook() tea.Cmd {
	return func() tea.Msg {
		var err error
		var hookName string

		switch m.hookCursor {
		case 0:
			err = git.RemoveCommitMsgHook(m.repoPath)
			hookName = "conventional commits"
		case 1, 2:
			err = git.RemovePreCommitHook(m.repoPath)
			hookName = "pre-commit"
		}

		if err != nil {
			return statusMsg{message: fmt.Sprintf("Remove failed: %v", err)}
		}

		return tea.Batch(
			func() tea.Msg {
				if m.hookCursor == 0 {
					return hookStatusMsg(false)
				}
				return preCommitHookMsg(false)
			},
			func() tea.Msg { return statusMsg{message: fmt.Sprintf("Removed %s hook", hookName)} },
		)()
	}
}

// Log viewer operations

func (m model) loadLogCommits(search string) tea.Cmd {
	return func() tea.Msg {
		commits := git.GetCommitLog2(m.repoPath, 50, search)
		return logCommitsMsg(commits)
	}
}

func (m model) loadLogDetail(hash string) tea.Cmd {
	return func() tea.Msg {
		detail := git.GetCommitDetail(m.repoPath, hash)
		diff := git.GetCommitDiff(m.repoPath, hash)
		return tea.Batch(
			func() tea.Msg { return logDetailMsg(detail) },
			func() tea.Msg { return logDiffMsg(diff) },
		)()
	}
}

// Blame operations

func (m model) loadBlame(filePath string) tea.Cmd {
	return func() tea.Msg {
		lines := git.GetBlame(m.repoPath, filePath)
		return blameMsg(lines)
	}
}

// Cherry-pick and Revert operations

func (m model) cherryPickCommit(hash string) tea.Cmd {
	return func() tea.Msg {
		err := git.CherryPick(m.repoPath, hash)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Cherry-pick failed: %v", err)}
		}

		return tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			m.loadRecentCommits(),
			func() tea.Msg {
				return statusMsg{message: fmt.Sprintf("Cherry-picked %s", hash)}
			},
		)()
	}
}

func (m model) revertCommit(hash string) tea.Cmd {
	return func() tea.Msg {
		err := git.RevertCommit(m.repoPath, hash)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Revert failed: %v", err)}
		}

		return tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			m.loadRecentCommits(),
			func() tea.Msg {
				return statusMsg{message: fmt.Sprintf("Reverted %s", hash)}
			},
		)()
	}
}

// Clean operations

type cleanFilesMsg []string

func (m model) loadCleanFiles() tea.Cmd {
	return func() tea.Msg {
		files, err := git.CleanDryRun(m.repoPath)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Clean check failed: %v", err)}
		}
		return cleanFilesMsg(files)
	}
}

func (m model) executeClean() tea.Cmd {
	return func() tea.Msg {
		err := git.CleanForce(m.repoPath)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Clean failed: %v", err)}
		}

		return tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			func() tea.Msg {
				return statusMsg{message: "Cleaned untracked files"}
			},
		)()
	}
}

// Clone/Init operations

func (m model) cloneRepo(url string) tea.Cmd {
	return func() tea.Msg {
		// Clone to current directory with repo name
		parts := strings.Split(url, "/")
		repoName := strings.TrimSuffix(parts[len(parts)-1], ".git")
		output, err := git.Clone(url, repoName)

		// Get absolute path to the cloned repo
		cwd, _ := os.Getwd()
		newPath := filepath.Join(cwd, repoName)

		return cloneResultMsg{output: output, err: err, newPath: newPath}
	}
}

func (m model) initRepo(path string) tea.Cmd {
	return func() tea.Msg {
		// Use current directory if path is empty
		targetPath := path
		if targetPath == "" {
			targetPath = "."
		}

		// Make absolute
		absPath, err := filepath.Abs(targetPath)
		if err != nil {
			return statusMsg{message: fmt.Sprintf("Invalid path: %v", err)}
		}

		// Create directory if it doesn't exist
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return statusMsg{message: fmt.Sprintf("Failed to create directory: %v", err)}
		}

		// Initialize git repo
		if err := git.Init(absPath); err != nil {
			return statusMsg{message: fmt.Sprintf("Init failed: %v", err)}
		}

		// Switch to the new repo
		return repoSwitchMsg(absPath)
	}
}
