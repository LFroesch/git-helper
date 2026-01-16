package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LFroesch/gitty/internal/git"
)

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.loadGitChanges(),
		m.loadGitStatus(),
		m.loadRecentCommits(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case statusMsg:
		m.statusMessage = msg.message
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil

	case gitChangesMsg:
		m.changes = msg
		// Adjust cursor if needed
		if m.fileCursor >= len(m.changes) {
			m.fileCursor = max(0, len(m.changes)-1)
		}
		// Generate commit suggestions
		cmds = append(cmds, m.generateCommitSuggestions())
		// Load diff for selected file
		if len(m.changes) > 0 && m.fileCursor < len(m.changes) {
			cmds = append(cmds, m.loadFileDiff(m.changes[m.fileCursor].File))
		}
		return m, tea.Batch(cmds...)

	case gitStatusMsg:
		m.gitState = git.Status(msg)
		return m, nil

	case branchesMsg:
		m.branches = msg
		if m.branchCursor >= len(m.branches) {
			m.branchCursor = max(0, len(m.branches)-1)
		}
		return m, nil

	case commitsMsg:
		m.commits = msg
		return m, nil

	case recentCommitsMsg:
		m.recentCommits = msg
		return m, nil

	case diffMsg:
		m.diffContent = string(msg)
		return m, nil

	case conflictsMsg:
		m.conflicts = msg
		return m, nil

	case comparisonMsg:
		comparison := git.BranchComparison(msg)
		m.branchComparison = &comparison
		return m, nil

	case rebaseCommitsMsg:
		m.rebaseCommits = msg
		return m, nil

	case pushOutputMsg:
		m.pushOutput = msg.output
		m.lastCommit = msg.commit
		return m, nil

	case commitSuccessMsg:
		m.commitSummary = &msg
		m.scrollOffset = 0
		cmds = append(cmds, m.loadGitChanges(), m.loadGitStatus())
		return m, tea.Batch(cmds...)

	case commitSuggestionsMsg:
		m.suggestions = msg
		return m, nil

	case stashListMsg:
		m.stashes = msg
		if m.stashCursor >= len(m.stashes) {
			m.stashCursor = max(0, len(m.stashes)-1)
		}
		return m, nil

	case tagListMsg:
		m.tags = msg
		if m.tagCursor >= len(m.tags) {
			m.tagCursor = max(0, len(m.tags)-1)
		}
		return m, nil

	case hookStatusMsg:
		m.commitMsgHookInstalled = bool(msg)
		return m, nil

	case preCommitHookMsg:
		m.preCommitHookInstalled = bool(msg)
		return m, nil

	case stashDiffMsg:
		m.diffContent = string(msg)
		return m, nil

	case logCommitsMsg:
		m.logCommits = msg
		if m.logCursor >= len(m.logCommits) {
			m.logCursor = max(0, len(m.logCommits)-1)
		}
		return m, nil

	case logDetailMsg:
		detail := git.CommitDetail(msg)
		m.logDetail = &detail
		return m, nil

	case logDiffMsg:
		m.logDiff = string(msg)
		return m, nil

	case blameMsg:
		m.blameLines = msg
		m.blameCursor = 0
		m.blameOffset = 0
		return m, nil

	case cloneResultMsg:
		if msg.err != nil {
			return m, func() tea.Msg { return statusMsg{message: "Clone failed: " + msg.output} }
		}
		// Switch to the cloned repo
		return m, func() tea.Msg { return repoSwitchMsg(msg.newPath) }

	case cleanFilesMsg:
		m.cleanFiles = msg
		m.cleanCursor = 0
		return m, nil

	case repoSwitchMsg:
		newPath := string(msg)
		m.repoPath = newPath
		m.tab = "workspace"
		m.toolMode = "menu"
		// Reset all cursors and state
		m.fileCursor, m.fileOffset = 0, 0
		m.branchCursor, m.branchOffset = 0, 0
		m.commitSummary = nil
		m.diffContent = ""
		m.commitMsgHookInstalled = git.IsCommitMsgHookInstalled(newPath)
		m.preCommitHookInstalled = git.IsPreCommitHookInstalled(newPath)
		// Reload everything
		return m, tea.Batch(
			m.loadGitChanges(),
			m.loadGitStatus(),
			m.loadRecentCommits(),
			func() tea.Msg { return statusMsg{message: "Switched to " + newPath} },
		)
	}

	// Update text inputs
	if m.commitInput.Focused() {
		var cmd tea.Cmd
		m.commitInput, cmd = m.commitInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.branchInput.Focused() {
		var cmd tea.Cmd
		m.branchInput, cmd = m.branchInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.rebaseInput.Focused() {
		var cmd tea.Cmd
		m.rebaseInput, cmd = m.rebaseInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.logSearchInput.Focused() {
		var cmd tea.Cmd
		m.logSearchInput, cmd = m.logSearchInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.cloneInput.Focused() {
		var cmd tea.Cmd
		m.cloneInput, cmd = m.cloneInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.initInput.Focused() {
		var cmd tea.Cmd
		m.initInput, cmd = m.initInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "1":
		m.tab = "workspace"
		m.viewMode = "files"
		m.commitSummary = nil
		return m, tea.Batch(m.loadGitChanges(), m.loadGitStatus())
	case "2":
		m.tab = "commit"
		m.commitInput.Focus()
		return m, tea.Batch(m.loadGitStatus(), m.generateCommitSuggestions())
	case "3":
		m.tab = "branches"
		return m, m.loadBranches()
	case "4":
		m.tab = "tools"
		m.toolMode = "menu"
		return m, nil
	}

	// Tab-specific keys
	switch m.tab {
	case "workspace":
		return m.handleWorkspaceKey(key)
	case "commit":
		return m.handleCommitKey(key, msg)
	case "branches":
		return m.handleBranchesKey(key, msg)
	case "tools":
		return m.handleToolsKey(key, msg)
	}

	return m, nil
}

func (m model) handleWorkspaceKey(key string) (tea.Model, tea.Cmd) {
	if m.viewMode == "diff" {
		switch key {
		case "esc":
			m.viewMode = "files"
			return m, nil
		case "j", "down":
			m.scrollOffset++
			return m, nil
		case "k", "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
			return m, nil
		}
		return m, nil
	}

	if m.viewMode == "blame" {
		switch key {
		case "esc":
			m.viewMode = "files"
			m.blameLines = nil
			return m, nil
		case "j", "down":
			if m.blameCursor < len(m.blameLines)-1 {
				m.blameCursor++
				m.adjustBlameScroll()
			}
			return m, nil
		case "k", "up":
			if m.blameCursor > 0 {
				m.blameCursor--
				m.adjustBlameScroll()
			}
			return m, nil
		}
		return m, nil
	}

	if m.viewMode == "conflicts" {
		switch key {
		case "esc":
			m.viewMode = "files"
			m.conflicts = nil
			return m, nil
		case "j", "down":
			if m.conflictCursor < len(m.conflicts)-1 {
				m.conflictCursor++
			}
			return m, nil
		case "k", "up":
			if m.conflictCursor > 0 {
				m.conflictCursor--
			}
			return m, nil
		case "enter":
			// Open conflict file in diff view
			if m.conflictCursor < len(m.conflicts) {
				m.viewMode = "diff"
				return m, m.loadFileDiff(m.conflicts[m.conflictCursor].Path)
			}
			return m, nil
		}
		return m, nil
	}

	switch key {
	case "j", "down":
		if m.fileCursor < len(m.changes)-1 {
			m.fileCursor++
			m.scrollOffset = 0
			m.adjustFileScroll()
			if m.fileCursor < len(m.changes) {
				return m, m.loadFileDiff(m.changes[m.fileCursor].File)
			}
		}
		return m, nil

	case "k", "up":
		if m.fileCursor > 0 {
			m.fileCursor--
			m.scrollOffset = 0
			m.adjustFileScroll()
			if m.fileCursor < len(m.changes) {
				return m, m.loadFileDiff(m.changes[m.fileCursor].File)
			}
		}
		return m, nil

	case " ", "space":
		if m.fileCursor < len(m.changes) {
			return m, m.toggleStaging(m.changes[m.fileCursor].File)
		}
		return m, nil

	case "a":
		return m, m.gitAddAll()

	case "r":
		return m, m.gitReset()

	case "enter":
		m.viewMode = "diff"
		m.scrollOffset = 0
		return m, nil

	case "b":
		// Blame selected file
		if m.fileCursor < len(m.changes) {
			file := m.changes[m.fileCursor].File
			m.blameFile = file
			m.viewMode = "blame"
			return m, m.loadBlame(file)
		}
		return m, nil

	case "d":
		if m.fileCursor < len(m.changes) {
			if m.confirmAction == "" {
				m.confirmAction = "discard"
				m.statusMessage = "Press 'd' again to confirm discard"
				return m, nil
			} else if m.confirmAction == "discard" {
				m.confirmAction = ""
				return m, m.discardChanges(m.changes[m.fileCursor].File)
			}
		}
		return m, nil

	case "esc":
		m.confirmAction = ""
		m.statusMessage = ""
		return m, nil

	case "p":
		m.showDiffPreview = !m.showDiffPreview
		return m, nil

	case "w":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
		return m, nil

	case "s":
		m.scrollOffset++
		return m, nil

	case "c":
		// Enter conflicts view
		m.viewMode = "conflicts"
		return m, m.loadConflicts()

	case "R":
		// Reset last commit (mixed - keeps changes unstaged)
		if m.confirmAction == "" {
			m.confirmAction = "reset-commit"
			m.statusMessage = "Press 'R' again to reset last commit (changes kept)"
			return m, nil
		} else if m.confirmAction == "reset-commit" {
			m.confirmAction = ""
			return m, m.gitResetLastCommit()
		}
		return m, nil
	}

	return m, nil
}

func (m model) handleCommitKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If viewing commit summary
	if m.commitSummary != nil {
		switch key {
		case "p":
			return m, m.pushChanges()
		case "c":
			m.commitSummary = nil
			return m, tea.Batch(m.loadGitChanges(), m.loadGitStatus())
		case "j", "down":
			m.scrollOffset++
			return m, nil
		case "k", "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
			return m, nil
		}
		return m, nil
	}

	switch key {
	case "enter":
		message := strings.TrimSpace(m.commitInput.Value())
		if message != "" {
			return m, m.commitWithMessage(message)
		} else if m.selectedSuggestion > 0 && m.selectedSuggestion <= len(m.suggestions) {
			return m, m.commitWithMessage(m.suggestions[m.selectedSuggestion-1].Message)
		}
		return m, nil

	case "esc":
		m.commitInput.SetValue("")
		m.commitInput.Blur()
		m.selectedSuggestion = 0
		return m, nil

	case "up":
		if m.selectedSuggestion > 0 {
			m.selectedSuggestion--
		}
		return m, nil

	case "down":
		if m.selectedSuggestion < len(m.suggestions) {
			m.selectedSuggestion++
		}
		return m, nil

	case "tab":
		if !m.commitInput.Focused() {
			m.commitInput.Focus()
		}
		return m, nil
	}

	// Pass to text input
	var cmd tea.Cmd
	m.commitInput, cmd = m.commitInput.Update(msg)
	return m, cmd
}

func (m model) handleBranchesKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If comparing branches
	if m.branchComparison != nil {
		switch key {
		case "esc":
			m.branchComparison = nil
			return m, nil
		}
		return m, nil
	}

	// If creating new branch
	if m.branchInput.Focused() {
		switch key {
		case "enter":
			branchName := strings.TrimSpace(m.branchInput.Value())
			if branchName != "" {
				m.branchInput.SetValue("")
				m.branchInput.Blur()
				return m, m.createBranch(branchName)
			}
			return m, nil
		case "esc":
			m.branchInput.SetValue("")
			m.branchInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.branchInput, cmd = m.branchInput.Update(msg)
		return m, cmd
	}

	switch key {
	case "j", "down":
		if m.branchCursor < len(m.branches)-1 {
			m.branchCursor++
			m.adjustBranchScroll()
		}
		return m, nil

	case "k", "up":
		if m.branchCursor > 0 {
			m.branchCursor--
			m.adjustBranchScroll()
		}
		return m, nil

	case "enter":
		if m.branchCursor < len(m.branches) {
			return m, m.switchBranch(m.branches[m.branchCursor].Name)
		}
		return m, nil

	case "n":
		m.branchInput.Focus()
		return m, textinput.Blink

	case "d":
		if m.branchCursor < len(m.branches) {
			branch := m.branches[m.branchCursor]
			if !branch.IsCurrent {
				if m.confirmAction == "" {
					m.confirmAction = "delete-branch"
					m.statusMessage = fmt.Sprintf("Press 'd' to confirm delete '%s'", branch.Name)
					return m, nil
				} else if m.confirmAction == "delete-branch" {
					m.confirmAction = ""
					return m, m.deleteBranch(branch.Name)
				}
			}
		}
		return m, nil

	case "c":
		if m.branchCursor < len(m.branches) {
			return m, m.compareBranch(m.branches[m.branchCursor].Name)
		}
		return m, nil

	case "esc":
		m.confirmAction = ""
		m.statusMessage = ""
		return m, nil
	}

	return m, nil
}

func (m model) handleToolsKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle rebase input
	if m.toolMode == "rebase" && m.rebaseInput.Focused() {
		switch key {
		case "enter":
			m.rebaseInput.Blur()
			return m, m.loadRebaseCommits()
		case "esc":
			m.rebaseInput.Blur()
			m.toolMode = "menu"
			return m, nil
		}
		var cmd tea.Cmd
		m.rebaseInput, cmd = m.rebaseInput.Update(msg)
		return m, cmd
	}

	// Back to menu
	if key == "esc" {
		if m.toolMode != "menu" {
			m.toolMode = "menu"
			m.pushOutput = ""
			return m, nil
		}
		return m, nil
	}

	// Tool mode specific
	switch m.toolMode {
	case "menu":
		return m.handleToolsMenuKey(key)
	case "undo":
		return m.handleUndoKey(key)
	case "rebase":
		return m.handleRebaseKey(key)
	case "history":
		return m.handleHistoryKey(key)
	case "remote":
		return m.handleRemoteKey(key)
	case "stash":
		return m.handleStashKey(key, msg)
	case "tags":
		return m.handleTagsKey(key, msg)
	case "hooks":
		return m.handleHooksKey(key)
	case "log":
		return m.handleLogKey(key, msg)
	case "clone":
		return m.handleCloneKey(key, msg)
	case "init":
		return m.handleInitKey(key, msg)
	case "clean":
		return m.handleCleanKey(key)
	}

	return m, nil
}

func (m model) handleToolsMenuKey(key string) (tea.Model, tea.Cmd) {
	// Main tools menu (categories)
	maxCursor := 11 // 12 items: 0-11

	switch key {
	case "j", "down":
		if m.toolCursor < maxCursor {
			m.toolCursor++
		}
		return m, nil
	case "k", "up":
		if m.toolCursor > 0 {
			m.toolCursor--
		}
		return m, nil
	case "enter":
		return m.selectToolMenuItem()
	// Quick keys
	case "s":
		m.toolMode = "stash"
		return m, m.loadStashList()
	case "t":
		m.toolMode = "tags"
		return m, m.loadTags()
	case "h":
		m.toolMode = "history"
		return m, m.loadCommitHistory()
	case "u":
		m.toolMode = "undo"
		return m, m.loadCommitHistory()
	case "r":
		m.toolMode = "rebase"
		m.rebaseInput.Focus()
		return m, textinput.Blink
	case "p":
		if m.confirmAction == "" {
			m.confirmAction = "push"
			m.statusMessage = "Press p again to push to remote"
			return m, nil
		} else if m.confirmAction == "push" {
			m.confirmAction = ""
			return m, m.pushChanges()
		}
		return m, nil
	case "f":
		return m, m.fetchChanges()
	case "l":
		if m.confirmAction == "" {
			m.confirmAction = "pull"
			m.statusMessage = "Press l again to pull from remote"
			return m, nil
		} else if m.confirmAction == "pull" {
			m.confirmAction = ""
			return m, m.pullChanges()
		}
		return m, nil
	case "g":
		m.toolMode = "hooks"
		return m, nil
	case "o":
		m.toolMode = "log"
		return m, m.loadLogCommits("")
	case "c":
		m.toolMode = "clone"
		m.cloneInput.Focus()
		return m, textinput.Blink
	case "i":
		m.toolMode = "init"
		m.initInput.Focus()
		return m, textinput.Blink
	case "x":
		m.toolMode = "clean"
		return m, m.loadCleanFiles()
	}
	return m, nil
}

func (m model) selectToolMenuItem() (tea.Model, tea.Cmd) {
	switch m.toolCursor {
	case 0: // Log
		m.toolMode = "log"
		return m, m.loadLogCommits("")
	case 1: // Stash
		m.toolMode = "stash"
		return m, m.loadStashList()
	case 2: // Tags
		m.toolMode = "tags"
		return m, m.loadTags()
	case 3: // History
		m.toolMode = "history"
		return m, m.loadCommitHistory()
	case 4: // Undo
		m.toolMode = "undo"
		return m, m.loadCommitHistory()
	case 5: // Rebase
		m.toolMode = "rebase"
		m.rebaseInput.Focus()
		return m, textinput.Blink
	case 6: // Push
		if m.confirmAction == "" {
			m.confirmAction = "push"
			m.statusMessage = "Press enter again to push to remote"
			return m, nil
		} else if m.confirmAction == "push" {
			m.confirmAction = ""
			return m, m.pushChanges()
		}
		return m, nil
	case 7: // Fetch/Pull
		// Fetch is safe, no confirm needed
		return m, m.fetchChanges()
	case 8: // Hooks
		m.toolMode = "hooks"
		return m, nil
	case 9: // Clean
		m.toolMode = "clean"
		return m, m.loadCleanFiles()
	case 10: // Clone
		m.toolMode = "clone"
		m.cloneInput.Focus()
		return m, textinput.Blink
	case 11: // Init
		m.toolMode = "init"
		m.initInput.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m model) handleUndoKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if m.undoCursor < len(m.commits)-1 {
			m.undoCursor++
			m.adjustUndoScroll()
		}
		return m, nil
	case "k", "up":
		if m.undoCursor > 0 {
			m.undoCursor--
			m.adjustUndoScroll()
		}
		return m, nil
	case "enter":
		if m.undoCursor < len(m.commits) {
			if m.confirmAction == "" {
				m.confirmAction = "undo"
				m.statusMessage = fmt.Sprintf("Press enter again to reset to %s (soft reset, changes kept)", m.commits[m.undoCursor].Hash)
				return m, nil
			} else if m.confirmAction == "undo" {
				m.confirmAction = ""
				return m, m.undoToCommit(m.commits[m.undoCursor].Hash)
			}
		}
		return m, nil
	}
	m.confirmAction = ""
	return m, nil
}

func (m model) handleRebaseKey(key string) (tea.Model, tea.Cmd) {
	if len(m.rebaseCommits) == 0 {
		return m, nil
	}

	switch key {
	case "j", "down":
		if m.rebaseCursor < len(m.rebaseCommits)-1 {
			m.rebaseCursor++
		}
		return m, nil
	case "k", "up":
		if m.rebaseCursor > 0 {
			m.rebaseCursor--
		}
		return m, nil
	case "p":
		m.rebaseCommits[m.rebaseCursor].Action = "pick"
		return m, nil
	case "s":
		m.rebaseCommits[m.rebaseCursor].Action = "squash"
		return m, nil
	case "r":
		m.rebaseCommits[m.rebaseCursor].Action = "reword"
		return m, nil
	case "d":
		m.rebaseCommits[m.rebaseCursor].Action = "drop"
		return m, nil
	case "f":
		m.rebaseCommits[m.rebaseCursor].Action = "fixup"
		return m, nil
	case "enter":
		if m.confirmAction == "" {
			m.confirmAction = "rebase"
			m.statusMessage = "Press enter again to execute rebase (rewrites history!)"
			return m, nil
		} else if m.confirmAction == "rebase" {
			m.confirmAction = ""
			return m, m.executeRebase()
		}
		return m, nil
	}
	m.confirmAction = ""
	return m, nil
}

func (m model) handleHistoryKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if m.historyCursor < len(m.commits)-1 {
			m.historyCursor++
			m.adjustHistoryScroll()
		}
		return m, nil
	case "k", "up":
		if m.historyCursor > 0 {
			m.historyCursor--
			m.adjustHistoryScroll()
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleRemoteKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "p":
		if m.confirmAction == "" {
			m.confirmAction = "push"
			m.statusMessage = "Press p again to push to remote"
			return m, nil
		} else if m.confirmAction == "push" {
			m.confirmAction = ""
			return m, m.pushChanges()
		}
		return m, nil
	case "f":
		return m, m.fetchChanges()
	case "l":
		if m.confirmAction == "" {
			m.confirmAction = "pull"
			m.statusMessage = "Press l again to pull from remote"
			return m, nil
		} else if m.confirmAction == "pull" {
			m.confirmAction = ""
			return m, m.pullChanges()
		}
		return m, nil
	}
	m.confirmAction = ""
	return m, nil
}

func (m model) handleStashKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if m.stashCursor < len(m.stashes)-1 {
			m.stashCursor++
			m.adjustStashScroll()
			// Load stash diff preview
			if m.stashCursor < len(m.stashes) {
				return m, m.loadStashDiff(m.stashCursor)
			}
		}
		return m, nil
	case "k", "up":
		if m.stashCursor > 0 {
			m.stashCursor--
			m.adjustStashScroll()
			if m.stashCursor < len(m.stashes) {
				return m, m.loadStashDiff(m.stashCursor)
			}
		}
		return m, nil
	case "s":
		// Create new stash
		return m, m.stashPush("")
	case "p", "enter":
		// Pop stash (removes from stash list)
		if m.stashCursor < len(m.stashes) {
			if m.confirmAction == "" {
				m.confirmAction = "pop-stash"
				m.statusMessage = "Press p again to pop stash (removes from stash list)"
				return m, nil
			} else if m.confirmAction == "pop-stash" {
				m.confirmAction = ""
				return m, m.stashPop(m.stashCursor)
			}
		}
		return m, nil
	case "a":
		// Apply stash (without removing)
		if m.stashCursor < len(m.stashes) {
			return m, m.stashApply(m.stashCursor)
		}
		return m, nil
	case "d":
		// Drop stash
		if m.stashCursor < len(m.stashes) {
			if m.confirmAction == "" {
				m.confirmAction = "drop-stash"
				m.statusMessage = "Press 'd' to confirm drop stash"
				return m, nil
			} else if m.confirmAction == "drop-stash" {
				m.confirmAction = ""
				return m, m.stashDrop(m.stashCursor)
			}
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleTagsKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If creating new tag
	if m.tagInput.Focused() {
		switch key {
		case "enter":
			tagName := strings.TrimSpace(m.tagInput.Value())
			if tagName != "" {
				m.tagInput.SetValue("")
				m.tagInput.Blur()
				return m, m.createTag(tagName, "", false)
			}
			return m, nil
		case "esc":
			m.tagInput.SetValue("")
			m.tagInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.tagInput, cmd = m.tagInput.Update(msg)
		return m, cmd
	}

	switch key {
	case "j", "down":
		if m.tagCursor < len(m.tags)-1 {
			m.tagCursor++
			m.adjustTagScroll()
		}
		return m, nil
	case "k", "up":
		if m.tagCursor > 0 {
			m.tagCursor--
			m.adjustTagScroll()
		}
		return m, nil
	case "n":
		// Create new tag
		m.tagInput.Focus()
		return m, textinput.Blink
	case "d":
		// Delete tag
		if m.tagCursor < len(m.tags) {
			tag := m.tags[m.tagCursor]
			if m.confirmAction == "" {
				m.confirmAction = "delete-tag"
				m.statusMessage = fmt.Sprintf("Press 'd' to confirm delete tag '%s'", tag.Name)
				return m, nil
			} else if m.confirmAction == "delete-tag" {
				m.confirmAction = ""
				return m, m.deleteTag(tag.Name)
			}
		}
		return m, nil
	case "p":
		// Push tag to remote
		if m.tagCursor < len(m.tags) {
			return m, m.pushTag(m.tags[m.tagCursor].Name)
		}
		return m, nil
	case "P":
		// Push all tags
		return m, m.pushAllTags()
	}
	return m, nil
}

func (m model) handleHooksKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if m.hookCursor < 2 {
			m.hookCursor++
		}
		return m, nil
	case "k", "up":
		if m.hookCursor > 0 {
			m.hookCursor--
		}
		return m, nil
	case "1":
		// Install conventional commits hook
		return m, m.installConventionalCommitsHook()
	case "2":
		// Install no-large-files hook
		return m, m.installNoLargeFilesHook()
	case "3":
		// Install detect-secrets hook
		return m, m.installDetectSecretsHook()
	case "r":
		// Remove selected hook
		return m, m.removeSelectedHook()
	case "enter":
		// Install selected hook
		switch m.hookCursor {
		case 0:
			return m, m.installConventionalCommitsHook()
		case 1:
			return m, m.installNoLargeFilesHook()
		case 2:
			return m, m.installDetectSecretsHook()
		}
	}
	return m, nil
}

func (m model) handleLogKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If viewing commit detail
	if m.logDetail != nil {
		switch key {
		case "esc":
			m.logDetail = nil
			m.logDiff = ""
			return m, nil
		case "j", "down":
			m.scrollOffset++
			return m, nil
		case "k", "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
			return m, nil
		}
		return m, nil
	}

	// If searching
	if m.logSearchInput.Focused() {
		switch key {
		case "enter":
			search := strings.TrimSpace(m.logSearchInput.Value())
			m.logSearchInput.Blur()
			m.logSearch = search
			return m, m.loadLogCommits(search)
		case "esc":
			m.logSearchInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.logSearchInput, cmd = m.logSearchInput.Update(msg)
		return m, cmd
	}

	switch key {
	case "j", "down":
		if m.logCursor < len(m.logCommits)-1 {
			m.logCursor++
			m.adjustLogScroll()
		}
		return m, nil
	case "k", "up":
		if m.logCursor > 0 {
			m.logCursor--
			m.adjustLogScroll()
		}
		return m, nil
	case "enter":
		if m.logCursor < len(m.logCommits) {
			return m, m.loadLogDetail(m.logCommits[m.logCursor].Hash)
		}
		return m, nil
	case "/":
		m.logSearchInput.Focus()
		return m, textinput.Blink
	case "c":
		// Cherry-pick selected commit
		if m.logCursor < len(m.logCommits) {
			return m, m.cherryPickCommit(m.logCommits[m.logCursor].Hash)
		}
		return m, nil
	case "R":
		// Revert selected commit (capital R to avoid conflict)
		if m.logCursor < len(m.logCommits) {
			if m.confirmAction == "" {
				m.confirmAction = "revert"
				m.statusMessage = fmt.Sprintf("Press R again to confirm revert %s", m.logCommits[m.logCursor].Hash)
				return m, nil
			} else if m.confirmAction == "revert" {
				m.confirmAction = ""
				return m, m.revertCommit(m.logCommits[m.logCursor].Hash)
			}
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleCleanKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if m.cleanCursor < len(m.cleanFiles)-1 {
			m.cleanCursor++
		}
		return m, nil
	case "k", "up":
		if m.cleanCursor > 0 {
			m.cleanCursor--
		}
		return m, nil
	case "d", "enter":
		// Execute clean
		if len(m.cleanFiles) > 0 {
			if m.confirmAction == "" {
				m.confirmAction = "clean"
				m.statusMessage = "Press d again to confirm deleting untracked files"
				return m, nil
			} else if m.confirmAction == "clean" {
				m.confirmAction = ""
				return m, m.executeClean()
			}
		}
		return m, nil
	case "r":
		// Refresh the list
		return m, m.loadCleanFiles()
	}
	return m, nil
}

func (m model) handleCloneKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.cloneInput.Focused() {
		switch key {
		case "enter":
			url := strings.TrimSpace(m.cloneInput.Value())
			if url != "" {
				m.cloneInput.SetValue("")
				m.cloneInput.Blur()
				m.toolMode = "menu"
				return m, m.cloneRepo(url)
			}
			return m, nil
		case "esc":
			m.cloneInput.SetValue("")
			m.cloneInput.Blur()
			m.toolMode = "menu"
			return m, nil
		}
		var cmd tea.Cmd
		m.cloneInput, cmd = m.cloneInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) handleInitKey(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.initInput.Focused() {
		switch key {
		case "enter":
			path := strings.TrimSpace(m.initInput.Value())
			if path != "" {
				m.initInput.SetValue("")
				m.initInput.Blur()
				m.toolMode = "menu"
				return m, m.initRepo(path)
			}
			return m, nil
		case "esc":
			m.initInput.SetValue("")
			m.initInput.Blur()
			m.toolMode = "menu"
			return m, nil
		}
		var cmd tea.Cmd
		m.initInput, cmd = m.initInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

// Scroll adjustment helpers

func (m *model) adjustFileScroll() {
	visibleItems := m.height - uiOverhead - 7
	if visibleItems < 1 {
		visibleItems = 1
	}

	if m.fileCursor < m.fileOffset {
		m.fileOffset = m.fileCursor
	}
	if m.fileCursor >= m.fileOffset+visibleItems {
		m.fileOffset = m.fileCursor - visibleItems + 1
	}
}

func (m *model) adjustBranchScroll() {
	visibleItems := m.height - uiOverhead - 4
	if visibleItems < 1 {
		visibleItems = 1
	}

	if m.branchCursor < m.branchOffset {
		m.branchOffset = m.branchCursor
	}
	if m.branchCursor >= m.branchOffset+visibleItems {
		m.branchOffset = m.branchCursor - visibleItems + 1
	}
}

func (m *model) adjustUndoScroll() {
	visibleItems := m.height - uiOverhead - 4
	if visibleItems < 1 {
		visibleItems = 1
	}

	if m.undoCursor < m.undoOffset {
		m.undoOffset = m.undoCursor
	}
	if m.undoCursor >= m.undoOffset+visibleItems {
		m.undoOffset = m.undoCursor - visibleItems + 1
	}
}

func (m *model) adjustHistoryScroll() {
	visibleItems := m.height - uiOverhead - 4
	if visibleItems < 1 {
		visibleItems = 1
	}

	if m.historyCursor < m.historyOffset {
		m.historyOffset = m.historyCursor
	}
	if m.historyCursor >= m.historyOffset+visibleItems {
		m.historyOffset = m.historyCursor - visibleItems + 1
	}
}

func (m *model) adjustStashScroll() {
	visibleItems := m.height - uiOverhead - 4
	if visibleItems < 1 {
		visibleItems = 1
	}

	if m.stashCursor < m.stashOffset {
		m.stashOffset = m.stashCursor
	}
	if m.stashCursor >= m.stashOffset+visibleItems {
		m.stashOffset = m.stashCursor - visibleItems + 1
	}
}

func (m *model) adjustTagScroll() {
	visibleItems := m.height - uiOverhead - 4
	if visibleItems < 1 {
		visibleItems = 1
	}

	if m.tagCursor < m.tagOffset {
		m.tagOffset = m.tagCursor
	}
	if m.tagCursor >= m.tagOffset+visibleItems {
		m.tagOffset = m.tagCursor - visibleItems + 1
	}
}

func (m *model) adjustLogScroll() {
	visibleItems := m.height - uiOverhead - 4
	if visibleItems < 1 {
		visibleItems = 1
	}

	if m.logCursor < m.logOffset {
		m.logOffset = m.logCursor
	}
	if m.logCursor >= m.logOffset+visibleItems {
		m.logOffset = m.logCursor - visibleItems + 1
	}
}

func (m *model) adjustBlameScroll() {
	visibleItems := m.height - uiOverhead - 4
	if visibleItems < 1 {
		visibleItems = 1
	}

	if m.blameCursor < m.blameOffset {
		m.blameOffset = m.blameCursor
	}
	if m.blameCursor >= m.blameOffset+visibleItems {
		m.blameOffset = m.blameCursor - visibleItems + 1
	}
}
