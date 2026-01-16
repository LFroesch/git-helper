package main

import (
	"os"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/LFroesch/gitty/internal/git"
)

// Constants
const uiOverhead = 9 // Header (1) + status (1) + borders (4) + padding (3)

// Additional types not in internal/git

type CommitSuggestion struct {
	Message string
	Type    string
}

// Message types for tea.Msg

type statusMsg struct{ message string }
type gitChangesMsg []git.Change
type commitSuggestionsMsg []CommitSuggestion
type gitStatusMsg git.Status
type branchesMsg []git.Branch
type commitsMsg []git.Commit
type recentCommitsMsg []git.Commit
type diffMsg string
type conflictsMsg []git.ConflictFile
type comparisonMsg git.BranchComparison
type rebaseCommitsMsg []git.RebaseCommit
type pushOutputMsg struct {
	output string
	commit string
}
type commitSuccessMsg struct {
	hash    string
	message string
	diff    string
	files   []string
}
type stashListMsg []git.Stash
type tagListMsg []git.Tag
type hookStatusMsg bool
type preCommitHookMsg bool
type stashDiffMsg string
type logCommitsMsg []git.Commit
type logDetailMsg git.CommitDetail
type logDiffMsg string
type blameMsg []git.BlameLine
type cloneResultMsg struct {
	output  string
	err     error
	newPath string
}
type repoSwitchMsg string

// Model

type model struct {
	// State management
	tab         string // "workspace", "commit", "branches", "tools"
	toolMode    string // when tab="tools": "menu", "undo", "rebase", "history", "remote", "stash", "tags", "hooks"
	toolSubmenu string // "local", "remote", "history", "advanced", "hooks"
	viewMode    string // workspace sub-states: "files", "diff", "conflicts"

	// Data
	changes          []git.Change
	suggestions      []CommitSuggestion
	gitState         git.Status
	branches         []git.Branch
	commits          []git.Commit
	conflicts        []git.ConflictFile
	branchComparison *git.BranchComparison
	rebaseCommits    []git.RebaseCommit

	// UI content
	diffContent   string
	pushOutput    string
	recentCommits []git.Commit
	commitSummary *commitSuccessMsg

	// List navigation (replaces tables)
	fileCursor     int
	fileOffset     int
	branchCursor   int
	branchOffset   int
	toolCursor     int
	historyCursor  int
	historyOffset  int
	conflictCursor int
	compareCursor  int
	rebaseCursor   int
	undoCursor     int
	undoOffset     int

	// Inputs
	commitInput textinput.Model
	branchInput textinput.Model
	rebaseInput textinput.Model

	// UI state
	width              int
	height             int
	statusMessage      string
	statusExpiry       time.Time
	showDiffPreview    bool
	selectedSuggestion int
	scrollOffset       int

	// Stash
	stashes     []git.Stash
	stashCursor int
	stashOffset int

	// Tags
	tags      []git.Tag
	tagCursor int
	tagOffset int
	tagInput  textinput.Model

	// Hooks
	commitMsgHookInstalled bool
	preCommitHookInstalled bool
	hookCursor             int

	// Clean
	cleanFiles  []string
	cleanCursor int

	// Log viewer
	logCommits     []git.Commit
	logCursor      int
	logOffset      int
	logSearch      string
	logSearchInput textinput.Model
	logDetail      *git.CommitDetail
	logDiff        string

	// Blame
	blameLines  []git.BlameLine
	blameCursor int
	blameOffset int
	blameFile   string

	// Clone/Init
	cloneInput textinput.Model
	initInput  textinput.Model

	// System
	repoPath         string
	lastCommit       string
	lastStatusUpdate time.Time
	confirmAction    string
}

// Styles

var (
	// Header bar style
	headerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	// Main panel border
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	// Panel header (inside bordered panel)
	panelHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("99")).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240")).
				MarginBottom(1)

	// List content padding
	listStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	// Title
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			Background(lipgloss.Color("236"))

	// Tab styles
	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("245")).
			Background(lipgloss.Color("236"))

	activeTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("212")).
			Background(lipgloss.Color("236")).
			Bold(true).
			Underline(true)

	// Item styles
	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("255")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Suggestion styles
	suggestionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			MarginLeft(2)

	selectedSuggestionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("46")).
				Bold(true).
				MarginLeft(2)

	// Status styles
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Background(lipgloss.Color("236"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	// Scroll indicators
	scrollIndicatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Italic(true)

	// Diff colors
	diffAddStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	diffRemoveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	diffHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("75")).
			Bold(true)

	diffHunkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141"))

	// Icon styles
	iconStagedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true)

	iconUnstagedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214"))

	iconUntrackedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	iconDeletedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))

	iconConflictStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)

	// Branch styles
	branchCurrentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	branchRemoteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("75"))

	branchAheadStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Background(lipgloss.Color("236"))

	branchBehindStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Background(lipgloss.Color("236"))

	// Pane border style
	paneBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	// Section header
	sectionHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")).
				Bold(true)

	// Keybind styles (scout-style)
	keyBindStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Background(lipgloss.Color("236")).
			Bold(true).
			Inline(true)

	keyDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("236")).
			Inline(true)
)

// Initialization

func initialModel() model {
	repoPath, err := os.Getwd()
	if err != nil {
		repoPath = "."
	}

	commitInput := textinput.New()
	commitInput.Placeholder = "Or type your custom commit message..."
	commitInput.CharLimit = 200

	branchInput := textinput.New()
	branchInput.Placeholder = "Branch name..."
	branchInput.CharLimit = 100

	rebaseInput := textinput.New()
	rebaseInput.Placeholder = "Number of commits to rebase..."
	rebaseInput.CharLimit = 3

	tagInput := textinput.New()
	tagInput.Placeholder = "Tag name (e.g. v1.0.0)..."
	tagInput.CharLimit = 50

	logSearchInput := textinput.New()
	logSearchInput.Placeholder = "Search commits..."
	logSearchInput.CharLimit = 100

	cloneInput := textinput.New()
	cloneInput.Placeholder = "Repository URL (https://... or git@...)..."
	cloneInput.CharLimit = 200

	initInput := textinput.New()
	initInput.Placeholder = "Directory path..."
	initInput.CharLimit = 200

	return model{
		tab:                    "workspace",
		toolMode:               "menu",
		toolSubmenu:            "",
		viewMode:               "files",
		repoPath:               repoPath,
		commitInput:            commitInput,
		branchInput:            branchInput,
		rebaseInput:            rebaseInput,
		tagInput:               tagInput,
		logSearchInput:         logSearchInput,
		cloneInput:             cloneInput,
		initInput:              initInput,
		showDiffPreview:        true,
		selectedSuggestion:     0,
		commitMsgHookInstalled: git.IsCommitMsgHookInstalled(repoPath),
		preCommitHookInstalled: git.IsPreCommitHookInstalled(repoPath),
	}
}
