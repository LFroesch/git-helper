package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

type GitChange struct {
	File   string
	Status string
	Type   string
	Scope  string
}

type CommitSuggestion struct {
	Message string
	Type    string
}

type GitStatus struct {
	Branch        string
	Clean         bool
	StagedFiles   int
	UnstagedFiles int
	Ahead         int
	Behind        int
}

type Branch struct {
	Name      string
	IsCurrent bool
	Upstream  string
	Ahead     int
	Behind    int
}

type Commit struct {
	Hash    string
	Message string
	Author  string
	Date    string
}

type DiffInfo struct {
	LinesAdded   int
	LinesRemoved int
	Functions    []string
	Imports      []string
	HasTests     bool
	HasDocs      bool
	Variables    []string
	Keywords     []string
	Comments     []string
	Context      string
}

type ConflictFile struct {
	Path       string
	Conflicts  []Conflict
	IsResolved bool
}

type Conflict struct {
	LineStart    int
	OursContent  []string
	TheirsContent []string
}

type BranchComparison struct {
	SourceBranch   string
	TargetBranch   string
	AheadCommits   []Commit
	BehindCommits  []Commit
	DifferingFiles []string
}

type RebaseCommit struct {
	Hash    string
	Message string
	Action  string // pick, squash, reword, edit, drop
}

// ============================================================================
// MODEL
// ============================================================================

type model struct {
	// State management - cleaner hierarchy
	tab       string // "workspace", "commit", "branches", "tools"
	toolMode  string // when tab="tools": "menu", "undo", "rebase", "history", "remote"
	viewMode  string // workspace sub-states: "files", "diff", "conflicts"

	// Data
	changes           []GitChange
	suggestions       []CommitSuggestion
	gitState          GitStatus
	branches          []Branch
	commits           []Commit
	conflicts         []ConflictFile
	branchComparison  *BranchComparison
	rebaseCommits     []RebaseCommit

	// UI content
	diffContent       string
	pushOutput        string
	recentCommits     []Commit // Last 3 for commit tab

	// Tables
	filesTable        table.Model
	branchesTable     table.Model
	toolsTable        table.Model
	historyTable      table.Model
	conflictsTable    table.Model
	comparisonTable   table.Model
	rebaseTable       table.Model

	// Inputs
	commitInput       textinput.Model
	branchInput       textinput.Model
	rebaseInput       textinput.Model

	// UI state
	width             int
	height            int
	statusMsg         string
	statusExpiry      time.Time
	showDiffPreview   bool
	selectedSuggestion int // 0 = custom, 1-9 = suggestions

	// System
	repoPath          string
	lastCommit        string
	lastStatusUpdate  time.Time
	confirmAction     string
}

// ============================================================================
// MESSAGES
// ============================================================================

type statusMsg struct{ message string }
type gitChangesMsg []GitChange
type commitSuggestionsMsg []CommitSuggestion
type gitStatusMsg GitStatus
type branchesMsg []Branch
type commitsMsg []Commit
type recentCommitsMsg []Commit
type diffMsg string
type conflictsMsg []ConflictFile
type comparisonMsg BranchComparison
type rebaseCommitsMsg []RebaseCommit
type pushOutputMsg struct {
	output string
	commit string
}

// ============================================================================
// STYLES
// ============================================================================

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginLeft(2)

	tabStyle = lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(lipgloss.Color("240"))

	activeTabStyle = lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(lipgloss.Color("86")).
		Bold(true).
		Underline(true)

	suggestionStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		MarginLeft(2)

	selectedSuggestionStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("46")).
		Bold(true).
		MarginLeft(2)

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		MarginLeft(2)

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("46")).
		Bold(true)

	warningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)
)

// ============================================================================
// INITIALIZATION
// ============================================================================

func initialModel() model {
	// Get repository path
	repoPath, err := os.Getwd()
	if err != nil {
		repoPath = "."
	}

	// Initialize commit input (always visible in commit tab)
	commitInput := textinput.New()
	commitInput.Placeholder = "Or type your custom commit message..."
	commitInput.CharLimit = 200

	// Initialize branch input
	branchInput := textinput.New()
	branchInput.Placeholder = "Branch name..."
	branchInput.CharLimit = 100

	// Initialize rebase input
	rebaseInput := textinput.New()
	rebaseInput.Placeholder = "Number of commits to rebase..."
	rebaseInput.CharLimit = 3

	m := model{
		tab:                "workspace",
		toolMode:           "menu",
		viewMode:           "files",
		repoPath:           repoPath,
		commitInput:        commitInput,
		branchInput:        branchInput,
		rebaseInput:        rebaseInput,
		showDiffPreview:    true,
		selectedSuggestion: 0,
	}

	// Initialize tables (will be populated later)
	m.filesTable = createFilesTable()
	m.branchesTable = createBranchesTable()
	m.toolsTable = createToolsTable()
	m.historyTable = createHistoryTable()
	m.conflictsTable = createConflictsTable()
	m.comparisonTable = createComparisonTable()
	m.rebaseTable = createRebaseTable()

	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.loadGitChanges(),
		m.loadGitStatus(),
		m.loadRecentCommits(),
	)
}

// ============================================================================
// TABLE CREATION
// ============================================================================

func createFilesTable() table.Model {
	columns := []table.Column{
		{Title: "Status", Width: 10},
		{Title: "File", Width: 50},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.SetStyles(s)
	return t
}

func createBranchesTable() table.Model {
	columns := []table.Column{
		{Title: "Branch", Width: 30},
		{Title: "Status", Width: 20},
		{Title: "Upstream", Width: 30},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.SetStyles(s)
	return t
}

func createToolsTable() table.Model {
	columns := []table.Column{
		{Title: "#", Width: 5},
		{Title: "Tool", Width: 25},
		{Title: "Description", Width: 50},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(6),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.SetStyles(s)
	return t
}

func createHistoryTable() table.Model {
	columns := []table.Column{
		{Title: "Hash", Width: 10},
		{Title: "Message", Width: 50},
		{Title: "Author", Width: 20},
		{Title: "Date", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.SetStyles(s)
	return t
}

func createConflictsTable() table.Model {
	columns := []table.Column{
		{Title: "File", Width: 50},
		{Title: "Conflicts", Width: 15},
		{Title: "Status", Width: 15},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	}

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.SetStyles(s)
	return t
}

func createComparisonTable() table.Model {
	columns := []table.Column{
		{Title: "Type", Width: 15},
		{Title: "Hash", Width: 10},
		{Title: "Message", Width: 55},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.SetStyles(s)
	return t
}

func createRebaseTable() table.Model {
	columns := []table.Column{
		{Title: "Action", Width: 10},
		{Title: "Hash", Width: 10},
		{Title: "Message", Width: 60},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.SetStyles(s)
	return t
}

// ============================================================================
// UPDATE
// ============================================================================

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustTableSizes()
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case gitChangesMsg:
		m.changes = []GitChange(msg)
		m.updateFilesTable()
		// Auto-detect conflicts
		if m.hasConflicts() {
			return m, m.loadConflicts()
		}
		return m, m.generateCommitSuggestions()

	case commitSuggestionsMsg:
		m.suggestions = []CommitSuggestion(msg)
		return m, nil

	case gitStatusMsg:
		m.gitState = GitStatus(msg)
		m.lastStatusUpdate = time.Now()
		return m, nil

	case branchesMsg:
		m.branches = []Branch(msg)
		m.updateBranchesTable()
		return m, nil

	case commitsMsg:
		m.commits = []Commit(msg)
		m.updateHistoryTable()
		return m, nil

	case recentCommitsMsg:
		m.recentCommits = []Commit(msg)
		return m, nil

	case diffMsg:
		m.diffContent = string(msg)
		return m, nil

	case conflictsMsg:
		m.conflicts = []ConflictFile(msg)
		m.updateConflictsTable()
		// Auto-switch to conflicts view
		if len(m.conflicts) > 0 {
			m.viewMode = "conflicts"
		}
		return m, nil

	case comparisonMsg:
		comp := BranchComparison(msg)
		m.branchComparison = &comp
		m.updateComparisonTable()
		return m, nil

	case rebaseCommitsMsg:
		m.rebaseCommits = []RebaseCommit(msg)
		m.updateRebaseTable()
		return m, nil

	case statusMsg:
		m.statusMsg = msg.message
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil

	case pushOutputMsg:
		m.pushOutput = msg.output
		m.lastCommit = msg.commit
		m.statusMsg = "✅ Pushed successfully"
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil
	}

	// Update appropriate component based on state
	switch m.tab {
	case "workspace":
		if m.viewMode == "files" {
			m.filesTable, cmd = m.filesTable.Update(msg)
			cmds = append(cmds, cmd)
		} else if m.viewMode == "conflicts" {
			m.conflictsTable, cmd = m.conflictsTable.Update(msg)
			cmds = append(cmds, cmd)
		}
	case "commit":
		if m.commitInput.Focused() {
			m.commitInput, cmd = m.commitInput.Update(msg)
			cmds = append(cmds, cmd)
		}
	case "branches":
		if m.branchInput.Focused() {
			m.branchInput, cmd = m.branchInput.Update(msg)
			cmds = append(cmds, cmd)
		} else if m.branchComparison != nil {
			m.comparisonTable, cmd = m.comparisonTable.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			m.branchesTable, cmd = m.branchesTable.Update(msg)
			cmds = append(cmds, cmd)
		}
	case "tools":
		switch m.toolMode {
		case "menu":
			m.toolsTable, cmd = m.toolsTable.Update(msg)
			cmds = append(cmds, cmd)
		case "history":
			m.historyTable, cmd = m.historyTable.Update(msg)
			cmds = append(cmds, cmd)
		case "rebase":
			if m.rebaseInput.Focused() {
				m.rebaseInput, cmd = m.rebaseInput.Update(msg)
				cmds = append(cmds, cmd)
			} else {
				m.rebaseTable, cmd = m.rebaseTable.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// ============================================================================
// KEY HANDLING
// ============================================================================

func (m model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global escape handling
	if msg.String() == "esc" {
		return m.handleEscape()
	}

	// Global quit
	if msg.String() == "ctrl+c" || (msg.String() == "q" && !m.anyInputFocused()) {
		return m, tea.Quit
	}

	// Tab switching (1-4) - only when no input is focused
	if !m.anyInputFocused() {
		switch msg.String() {
		case "1":
			m.tab = "workspace"
			m.viewMode = "files"
			return m, nil
		case "2":
			if m.gitState.StagedFiles > 0 {
				m.tab = "commit"
				m.selectedSuggestion = 0
			} else {
				m.statusMsg = "❌ No files staged. Stage files first in workspace."
				m.statusExpiry = time.Now().Add(3 * time.Second)
			}
			return m, nil
		case "3":
			m.tab = "branches"
			m.branchComparison = nil // Clear comparison when entering tab
			return m, m.loadBranches()
		case "4":
			m.tab = "tools"
			m.toolMode = "menu"
			m.updateToolsTable()
			return m, nil
		}
	}

	// Handle based on current tab
	switch m.tab {
	case "workspace":
		return m.handleWorkspaceKeys(msg)
	case "commit":
		return m.handleCommitKeys(msg)
	case "branches":
		return m.handleBranchesKeys(msg)
	case "tools":
		return m.handleToolsKeys(msg)
	}

	return m, nil
}

func (m model) handleEscape() (tea.Model, tea.Cmd) {
	// Blur any focused inputs
	if m.commitInput.Focused() {
		m.commitInput.Blur()
		m.selectedSuggestion = 0
		return m, nil
	}
	if m.branchInput.Focused() {
		m.branchInput.Blur()
		m.branchInput.SetValue("")
		return m, nil
	}
	if m.rebaseInput.Focused() {
		m.rebaseInput.Blur()
		m.rebaseInput.SetValue("")
		return m, nil
	}

	// Exit sub-modes
	if m.viewMode == "diff" {
		m.viewMode = "files"
		return m, nil
	}
	if m.branchComparison != nil {
		m.branchComparison = nil
		return m, nil
	}
	if m.tab == "tools" && m.toolMode != "menu" {
		m.toolMode = "menu"
		m.updateToolsTable()
		return m, nil
	}

	// Clear confirmation
	if m.confirmAction != "" {
		m.confirmAction = ""
		m.statusMsg = "❌ Action cancelled"
		m.statusExpiry = time.Now().Add(2 * time.Second)
		return m, nil
	}

	return m, nil
}

func (m model) anyInputFocused() bool {
	return m.commitInput.Focused() || m.branchInput.Focused() || m.rebaseInput.Focused()
}

// ============================================================================
// WORKSPACE TAB KEY HANDLING
// ============================================================================

func (m model) handleWorkspaceKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.viewMode == "conflicts" {
		return m.handleConflictKeys(msg)
	}

	switch msg.String() {
	case " ": // Space - toggle staging
		if len(m.changes) > 0 {
			selectedIndex := m.filesTable.Cursor()
			if selectedIndex < len(m.changes) {
				return m, m.toggleStaging(m.changes[selectedIndex].File)
			}
		}
		return m, nil

	case "a": // Stage all
		return m, tea.Batch(m.gitAddAll(), m.refreshAfterStaging())

	case "r": // Refresh
		return m, tea.Batch(m.loadGitChanges(), m.loadGitStatus())

	case "v": // Toggle diff preview
		m.showDiffPreview = !m.showDiffPreview
		if m.showDiffPreview && len(m.changes) > 0 {
			selectedIndex := m.filesTable.Cursor()
			if selectedIndex < len(m.changes) {
				return m, m.viewDiff(m.changes[selectedIndex].File)
			}
		}
		return m, nil

	case "d": // View full diff
		if len(m.changes) > 0 {
			selectedIndex := m.filesTable.Cursor()
			if selectedIndex < len(m.changes) {
				m.viewMode = "diff"
				return m, m.viewDiff(m.changes[selectedIndex].File)
			}
		}
		return m, nil

	case "R": // Reset (unstage all)
		return m, tea.Batch(m.gitReset(), m.refreshAfterStaging())
	}

	return m, nil
}

func (m model) handleConflictKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.conflicts) == 0 {
		return m, nil
	}

	selectedIndex := m.conflictsTable.Cursor()
	if selectedIndex >= len(m.conflicts) {
		return m, nil
	}

	switch msg.String() {
	case "o": // Accept ours
		return m, m.resolveConflict(selectedIndex, "ours")
	case "t": // Accept theirs
		return m, m.resolveConflict(selectedIndex, "theirs")
	case "b": // Accept both
		return m, m.resolveConflict(selectedIndex, "both")
	case "enter": // View conflict details
		// TODO: Show detailed conflict view
		return m, nil
	case "c": // Continue merge (if all resolved)
		if m.allConflictsResolved() {
			return m, m.continueMerge()
		} else {
			m.statusMsg = "❌ Resolve all conflicts before continuing"
			m.statusExpiry = time.Now().Add(3 * time.Second)
			return m, nil
		}
	}

	return m, nil
}

// ============================================================================
// COMMIT TAB KEY HANDLING
// ============================================================================

func (m model) handleCommitKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If input is focused, handle text input
	if m.commitInput.Focused() {
		if msg.String() == "enter" {
			customMsg := m.commitInput.Value()
			if customMsg != "" {
				m.commitInput.SetValue("")
				m.commitInput.Blur()
				m.selectedSuggestion = 0
				return m, tea.Batch(
					m.commitWithMessage(customMsg),
					m.refreshAfterCommit(),
				)
			}
		}
		return m, nil
	}

	// Handle number keys for suggestions (1-9)
	if msg.String() >= "1" && msg.String() <= "9" {
		num, _ := strconv.Atoi(msg.String())
		if num <= len(m.suggestions) {
			suggestion := m.suggestions[num-1]
			return m, tea.Batch(
				m.commitWithMessage(suggestion.Message),
				m.refreshAfterCommit(),
			)
		}
		return m, nil
	}

	// Enter or 'c' to focus custom input
	if msg.String() == "enter" || msg.String() == "c" {
		m.commitInput.Focus()
		m.selectedSuggestion = -1
		return m, nil
	}

	// Arrow keys to select suggestion
	if msg.String() == "up" || msg.String() == "k" {
		if m.selectedSuggestion > 0 {
			m.selectedSuggestion--
		}
		return m, nil
	}
	if msg.String() == "down" || msg.String() == "j" {
		if m.selectedSuggestion < len(m.suggestions) {
			m.selectedSuggestion++
		}
		return m, nil
	}

	// Spacebar or enter on selected suggestion
	if msg.String() == " " || (msg.String() == "enter" && m.selectedSuggestion > 0) {
		if m.selectedSuggestion > 0 && m.selectedSuggestion <= len(m.suggestions) {
			suggestion := m.suggestions[m.selectedSuggestion-1]
			return m, tea.Batch(
				m.commitWithMessage(suggestion.Message),
				m.refreshAfterCommit(),
			)
		}
		return m, nil
	}

	return m, nil
}

// ============================================================================
// BRANCHES TAB KEY HANDLING
// ============================================================================

func (m model) handleBranchesKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle branch input
	if m.branchInput.Focused() {
		if msg.String() == "enter" {
			branchName := m.branchInput.Value()
			if branchName != "" {
				m.branchInput.SetValue("")
				m.branchInput.Blur()
				return m, m.createBranch(branchName)
			}
		}
		return m, nil
	}

	// If in comparison mode, handle that
	if m.branchComparison != nil {
		// Navigation handled by table update
		return m, nil
	}

	switch msg.String() {
	case "enter": // Switch to selected branch
		if len(m.branches) > 0 {
			selectedIndex := m.branchesTable.Cursor()
			if selectedIndex < len(m.branches) {
				branch := m.branches[selectedIndex]
				if !branch.IsCurrent {
					return m, m.switchBranch(branch.Name)
				}
			}
		}
		return m, nil

	case "n": // New branch
		m.branchInput.Focus()
		return m, nil

	case "d": // Delete branch
		if len(m.branches) > 0 {
			selectedIndex := m.branchesTable.Cursor()
			if selectedIndex < len(m.branches) {
				branch := m.branches[selectedIndex]
				if branch.IsCurrent {
					m.statusMsg = "❌ Cannot delete current branch"
					m.statusExpiry = time.Now().Add(3 * time.Second)
					return m, nil
				}
				m.confirmAction = "delete-branch:" + branch.Name
				m.statusMsg = fmt.Sprintf("⚠️ Press 'y' to confirm delete '%s', or ESC to cancel", branch.Name)
				m.statusExpiry = time.Now().Add(10 * time.Second)
			}
		}
		return m, nil

	case "y": // Confirm delete
		if strings.HasPrefix(m.confirmAction, "delete-branch:") {
			branchName := strings.TrimPrefix(m.confirmAction, "delete-branch:")
			m.confirmAction = ""
			return m, m.deleteBranch(branchName)
		}
		return m, nil

	case "c": // Compare with another branch
		// Default compare with main/master
		targetBranch := "main"
		// Check if main exists, otherwise try master
		for _, b := range m.branches {
			if b.Name == "main" {
				targetBranch = "main"
				break
			} else if b.Name == "master" {
				targetBranch = "master"
			}
		}
		return m, m.loadBranchComparison(targetBranch)

	case "r": // Refresh branches
		return m, m.loadBranches()
	}

	return m, nil
}

// ============================================================================
// TOOLS TAB KEY HANDLING
// ============================================================================

func (m model) handleToolsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.toolMode {
	case "menu":
		return m.handleToolsMenuKeys(msg)
	case "undo":
		return m.handleUndoKeys(msg)
	case "rebase":
		return m.handleRebaseKeys(msg)
	case "history":
		return m.handleHistoryKeys(msg)
	case "remote":
		return m.handleRemoteKeys(msg)
	}
	return m, nil
}

func (m model) handleToolsMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1", "enter":
		if m.toolsTable.Cursor() == 0 || msg.String() == "1" {
			m.toolMode = "undo"
			return m, nil
		}
		// Handle other menu items based on cursor
		switch m.toolsTable.Cursor() {
		case 1:
			m.toolMode = "rebase"
			m.rebaseInput.Focus()
			return m, nil
		case 2:
			m.toolMode = "history"
			return m, m.loadHistory()
		case 3:
			m.toolMode = "remote"
			return m, nil
		}
		return m, nil

	case "2":
		m.toolMode = "rebase"
		m.rebaseInput.Focus()
		return m, nil

	case "3":
		m.toolMode = "history"
		return m, m.loadHistory()

	case "4":
		m.toolMode = "remote"
		return m, nil
	}

	return m, nil
}

func (m model) handleUndoKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1": // Soft reset (keep changes)
		m.confirmAction = "soft-reset"
		m.statusMsg = "⚠️ Press 'y' to undo last commit (keep changes), or ESC to cancel"
		m.statusExpiry = time.Now().Add(10 * time.Second)
		return m, nil

	case "2": // Mixed reset (unstage)
		m.confirmAction = "mixed-reset"
		m.statusMsg = "⚠️ Press 'y' to undo last commit (unstage changes), or ESC to cancel"
		m.statusExpiry = time.Now().Add(10 * time.Second)
		return m, nil

	case "3": // Hard reset (DANGEROUS)
		m.confirmAction = "hard-reset"
		m.statusMsg = "⚠️⚠️⚠️ DANGEROUS: Press 'y' to undo and DISCARD changes, or ESC to cancel"
		m.statusExpiry = time.Now().Add(15 * time.Second)
		return m, nil

	case "4": // View reflog
		return m, m.loadReflog()

	case "y": // Confirm action
		switch m.confirmAction {
		case "soft-reset":
			m.confirmAction = ""
			return m, m.softReset(1)
		case "mixed-reset":
			m.confirmAction = ""
			return m, m.mixedReset(1)
		case "hard-reset":
			m.confirmAction = ""
			return m, m.hardReset(1)
		}
		return m, nil
	}

	return m, nil
}

func (m model) handleRebaseKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If input is focused, handle count input
	if m.rebaseInput.Focused() {
		if msg.String() == "enter" {
			countStr := m.rebaseInput.Value()
			if countStr != "" {
				count, err := strconv.Atoi(countStr)
				if err == nil && count > 0 && count <= 50 {
					m.rebaseInput.SetValue("")
					m.rebaseInput.Blur()
					return m, m.loadRebaseCommits(count)
				} else {
					m.statusMsg = "❌ Invalid count (must be 1-50)"
					m.statusExpiry = time.Now().Add(3 * time.Second)
				}
			}
		}
		return m, nil
	}

	// If commits are loaded, handle rebase actions
	if len(m.rebaseCommits) > 0 {
		selectedIndex := m.rebaseTable.Cursor()
		if selectedIndex >= len(m.rebaseCommits) {
			return m, nil
		}

		switch msg.String() {
		case "p":
			m.rebaseCommits[selectedIndex].Action = "pick"
			m.updateRebaseTable()
			return m, nil
		case "s":
			m.rebaseCommits[selectedIndex].Action = "squash"
			m.updateRebaseTable()
			return m, nil
		case "r":
			m.rebaseCommits[selectedIndex].Action = "reword"
			m.updateRebaseTable()
			return m, nil
		case "d":
			m.rebaseCommits[selectedIndex].Action = "drop"
			m.updateRebaseTable()
			return m, nil
		case "f":
			m.rebaseCommits[selectedIndex].Action = "fixup"
			m.updateRebaseTable()
			return m, nil
		case "enter":
			// Execute rebase
			m.confirmAction = "execute-rebase"
			m.statusMsg = "⚠️ Press 'y' to execute rebase, or ESC to cancel"
			m.statusExpiry = time.Now().Add(10 * time.Second)
			return m, nil
		case "y":
			if m.confirmAction == "execute-rebase" {
				m.confirmAction = ""
				return m, m.executeRebase()
			}
			return m, nil
		}
	}

	return m, nil
}

func (m model) handleHistoryKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		return m, m.loadHistory()
	case "c": // Copy hash
		if len(m.commits) > 0 {
			selectedIndex := m.historyTable.Cursor()
			if selectedIndex < len(m.commits) {
				// Would need clipboard support - for now just show message
				m.statusMsg = fmt.Sprintf("📋 Hash: %s", m.commits[selectedIndex].Hash)
				m.statusExpiry = time.Now().Add(5 * time.Second)
			}
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleRemoteKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "p": // Push
		return m, m.gitPush()
	case "l": // Pull
		return m, m.gitPull()
	case "f": // Fetch
		return m, m.gitFetch()
	}
	return m, nil
}

// ============================================================================
// GIT OPERATIONS - Loading Data
// ============================================================================

func (m model) loadGitChanges() tea.Cmd {
	return func() tea.Msg {
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = m.repoPath
		statusOutput, err := statusCmd.Output()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to get status: %v", err)}
		}

		changes := parseGitStatus(string(statusOutput))
		return gitChangesMsg(changes)
	}
}

func (m model) loadGitStatus() tea.Cmd {
	return func() tea.Msg {
		status := GitStatus{}
		status.Branch = getBranchName(m.repoPath)

		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = m.repoPath
		statusOutput, err := statusCmd.Output()
		if err == nil {
			statusText := string(statusOutput)
			status.StagedFiles, status.UnstagedFiles, status.Clean = parseGitStatusOutput(statusText)
		}

		status.Ahead, status.Behind = getAheadBehindCount(m.repoPath)
		return gitStatusMsg(status)
	}
}

func (m model) loadBranches() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("git", "branch", "-vv")
		cmd.Dir = m.repoPath
		output, err := cmd.Output()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to load branches: %v", err)}
		}

		var branches []Branch
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			branch := Branch{}
			if strings.HasPrefix(line, "*") {
				branch.IsCurrent = true
				line = strings.TrimPrefix(line, "* ")
			} else {
				line = strings.TrimPrefix(line, "  ")
			}

			parts := strings.Fields(line)
			if len(parts) > 0 {
				branch.Name = parts[0]
			}
			if len(parts) > 2 {
				branch.Upstream = parts[2]
			}

			// Parse ahead/behind from upstream info
			if strings.Contains(line, "ahead") {
				re := regexp.MustCompile(`ahead (\d+)`)
				if matches := re.FindStringSubmatch(line); len(matches) > 1 {
					branch.Ahead, _ = strconv.Atoi(matches[1])
				}
			}
			if strings.Contains(line, "behind") {
				re := regexp.MustCompile(`behind (\d+)`)
				if matches := re.FindStringSubmatch(line); len(matches) > 1 {
					branch.Behind, _ = strconv.Atoi(matches[1])
				}
			}

			branches = append(branches, branch)
		}

		return branchesMsg(branches)
	}
}

func (m model) loadHistory() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("git", "log", "-20", "--pretty=format:%h|%s|%an|%ar")
		cmd.Dir = m.repoPath
		output, err := cmd.Output()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to load history: %v", err)}
		}

		var commits []Commit
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			parts := strings.Split(line, "|")
			if len(parts) >= 4 {
				commits = append(commits, Commit{
					Hash:    parts[0],
					Message: parts[1],
					Author:  parts[2],
					Date:    parts[3],
				})
			}
		}

		return commitsMsg(commits)
	}
}

func (m model) loadRecentCommits() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("git", "log", "-3", "--pretty=format:%h|%s|%an|%ar")
		cmd.Dir = m.repoPath
		output, err := cmd.Output()
		if err != nil {
			return recentCommitsMsg([]Commit{})
		}

		var commits []Commit
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.Split(line, "|")
			if len(parts) >= 4 {
				commits = append(commits, Commit{
					Hash:    parts[0],
					Message: parts[1],
					Author:  parts[2],
					Date:    parts[3],
				})
			}
		}

		return recentCommitsMsg(commits)
	}
}

func (m model) loadConflicts() tea.Cmd {
	return func() tea.Msg {
		// Get list of conflicted files
		cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
		cmd.Dir = m.repoPath
		output, err := cmd.Output()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to detect conflicts: %v", err)}
		}

		var conflictFiles []ConflictFile
		files := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, file := range files {
			if file == "" {
				continue
			}

			// Read file and parse conflicts
			content, err := os.ReadFile(filepath.Join(m.repoPath, file))
			if err != nil {
				continue
			}

			conflicts := parseConflictMarkers(string(content))
			if len(conflicts) > 0 {
				conflictFiles = append(conflictFiles, ConflictFile{
					Path:       file,
					Conflicts:  conflicts,
					IsResolved: false,
				})
			}
		}

		return conflictsMsg(conflictFiles)
	}
}

func (m model) loadBranchComparison(targetBranch string) tea.Cmd {
	return func() tea.Msg {
		comparison := BranchComparison{
			SourceBranch: getBranchName(m.repoPath),
			TargetBranch: targetBranch,
		}

		// Get commits ahead (on current branch but not on target)
		aheadCmd := exec.Command("git", "log", "--pretty=format:%h|%s|%an|%ar", targetBranch+"..HEAD")
		aheadCmd.Dir = m.repoPath
		aheadOutput, err := aheadCmd.Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(aheadOutput)), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				parts := strings.Split(line, "|")
				if len(parts) >= 4 {
					comparison.AheadCommits = append(comparison.AheadCommits, Commit{
						Hash:    parts[0],
						Message: parts[1],
						Author:  parts[2],
						Date:    parts[3],
					})
				}
			}
		}

		// Get commits behind (on target branch but not on current)
		behindCmd := exec.Command("git", "log", "--pretty=format:%h|%s|%an|%ar", "HEAD.."+targetBranch)
		behindCmd.Dir = m.repoPath
		behindOutput, err := behindCmd.Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(behindOutput)), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				parts := strings.Split(line, "|")
				if len(parts) >= 4 {
					comparison.BehindCommits = append(comparison.BehindCommits, Commit{
						Hash:    parts[0],
						Message: parts[1],
						Author:  parts[2],
						Date:    parts[3],
					})
				}
			}
		}

		// Get differing files
		diffCmd := exec.Command("git", "diff", "--name-only", targetBranch+"...HEAD")
		diffCmd.Dir = m.repoPath
		diffOutput, err := diffCmd.Output()
		if err == nil {
			files := strings.Split(strings.TrimSpace(string(diffOutput)), "\n")
			for _, file := range files {
				if file != "" {
					comparison.DifferingFiles = append(comparison.DifferingFiles, file)
				}
			}
		}

		return comparisonMsg(comparison)
	}
}

func (m model) loadRebaseCommits(count int) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("git", "log", fmt.Sprintf("-%d", count), "--pretty=format:%h|%s")
		cmd.Dir = m.repoPath
		output, err := cmd.Output()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to load commits: %v", err)}
		}

		var commits []RebaseCommit
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		// Reverse order (oldest first for rebase)
		for i := len(lines) - 1; i >= 0; i-- {
			line := lines[i]
			if line == "" {
				continue
			}
			parts := strings.Split(line, "|")
			if len(parts) >= 2 {
				commits = append(commits, RebaseCommit{
					Hash:    parts[0],
					Message: parts[1],
					Action:  "pick", // Default action
				})
			}
		}

		return rebaseCommitsMsg(commits)
	}
}

func (m model) loadReflog() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("git", "reflog", "-20", "--pretty=format:%h|%s|%ar")
		cmd.Dir = m.repoPath
		output, err := cmd.Output()
		if err != nil {
			return statusMsg{message: fmt.Sprintf("❌ Failed to load reflog: %v", err)}
		}

		var commits []Commit
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			parts := strings.Split(line, "|")
			if len(parts) >= 3 {
				commits = append(commits, Commit{
					Hash:    parts[0],
					Message: parts[1],
					Date:    parts[2],
				})
			}
		}

		return commitsMsg(commits)
	}
}

// (Continuing in next part due to length...)
