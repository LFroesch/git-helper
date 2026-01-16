package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View is the main render function
func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// 3-section layout
	header := m.renderTopBar()
	content := m.renderMainPanel()
	footer := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

// Top header bar (full-width, bg 235)
func (m model) renderTopBar() string {
	title := titleStyle.Render("Gitty")
	repoName := lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Background(lipgloss.Color("236")).
		Render(fmt.Sprintf(" %s", filepath.Base(m.repoPath)))

	// Git status info
	statusInfo := m.renderGitStatusInfo()

	// Tabs
	tabs := m.renderTabs()

	spacer := lipgloss.NewStyle().Background(lipgloss.Color("236")).Render("  ")
	leftPart := lipgloss.JoinHorizontal(lipgloss.Top, title, repoName, spacer, statusInfo)

	// Fill both rows to full width with background
	rowStyle := lipgloss.NewStyle().Background(lipgloss.Color("236")).Width(m.width - 2)
	leftPartStyled := rowStyle.Render(leftPart)
	tabsStyled := rowStyle.Render(tabs)

	return headerStyle.Width(m.width).Render(
		lipgloss.JoinVertical(lipgloss.Left, leftPartStyled, tabsStyled),
	)
}

func (m model) renderGitStatusInfo() string {
	branchIcon := "🌿 "
	parts := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Background(lipgloss.Color("236")).Bold(true).Render(branchIcon + m.gitState.Branch),
	}

	if m.gitState.StagedFiles > 0 {
		parts = append(parts, iconStagedStyle.Render(fmt.Sprintf("✓ %d", m.gitState.StagedFiles)))
	}
	if m.gitState.UnstagedFiles > 0 {
		parts = append(parts, iconUnstagedStyle.Render(fmt.Sprintf("● %d", m.gitState.UnstagedFiles)))
	}
	if m.gitState.Ahead > 0 {
		parts = append(parts, branchAheadStyle.Render(fmt.Sprintf("↑ %d", m.gitState.Ahead)))
	}
	if m.gitState.Behind > 0 {
		parts = append(parts, branchBehindStyle.Render(fmt.Sprintf("↓ %d", m.gitState.Behind)))
	}

	styledSpace := lipgloss.NewStyle().Background(lipgloss.Color("236")).Render("  ")
	return strings.Join(parts, styledSpace)
}

func (m model) renderTabs() string {
	tab1 := m.renderTab("1", "Workspace", m.tab == "workspace")
	tab2 := m.renderTab("2", "Commit", m.tab == "commit")
	tab3 := m.renderTab("3", "Branches", m.tab == "branches")
	tab4 := m.renderTab("4", "Tools", m.tab == "tools")

	return lipgloss.JoinHorizontal(lipgloss.Top, tab1, tab2, tab3, tab4)
}

func (m model) renderTab(key, label string, active bool) string {
	style := tabStyle
	if active {
		style = activeTabStyle
	}
	return style.Render(fmt.Sprintf("[%s] %s", key, label))
}

// Main panel (bordered)
func (m model) renderMainPanel() string {
	panelWidth := m.width - 2
	contentHeight := m.height - uiOverhead

	if contentHeight < 3 {
		contentHeight = 3
	}

	var content string

	switch m.tab {
	case "workspace":
		_, content = m.renderWorkspaceContent(panelWidth-4, contentHeight)
	case "commit":
		_, content = m.renderCommitContent(panelWidth-4, contentHeight)
	case "branches":
		_, content = m.renderBranchesContent(panelWidth-4, contentHeight)
	case "tools":
		_, content = m.renderToolsContent(panelWidth-4, contentHeight)
	}

	panelContent := listStyle.Render(content)

	return borderStyle.Width(panelWidth).Height(contentHeight).Render(panelContent)
}

// Bottom status bar (full-width, bg 235)
func (m model) renderStatusBar() string {
	var helpText string

	// Build keybinds using scout-style: purple keys, white descriptions
	k := func(key string) string { return keyBindStyle.Render(key) }
	d := func(desc string) string { return keyDescStyle.Render(desc) }
	sep := keyDescStyle.Render(" | ")

	switch m.tab {
	case "workspace":
		if m.viewMode == "diff" || m.viewMode == "blame" || m.viewMode == "conflicts" {
			helpText = k("esc") + d(": back") + sep + k("j/k") + d(": scroll")
		} else {
			helpText = k("j/k") + d(": nav") + sep + k("space") + d(": stage") + sep +
				k("a") + d(": all") + sep + k("R") + d(": reset commit") + sep +
				k("enter") + d(": diff") + sep + k("b") + d(": blame") + sep + k("d") + d(": discard")
		}
	case "commit":
		if m.commitSummary != nil {
			helpText = k("p") + d(": push") + sep + k("c") + d(": continue") + sep + k("j/k") + d(": scroll")
		} else {
			helpText = k("↑/↓") + d(": select") + sep + k("enter") + d(": commit") + sep +
				k("tab") + d(": custom") + sep + k("esc") + d(": clear")
		}
	case "branches":
		helpText = k("j/k") + d(": nav") + sep + k("enter") + d(": checkout") + sep +
			k("n") + d(": new") + sep + k("d") + d(": delete") + sep + k("c") + d(": compare")
	case "tools":
		switch m.toolMode {
		case "stash":
			helpText = k("j/k") + d(": nav") + sep + k("s") + d(": stash") + sep +
				k("p") + d(": pop") + sep + k("a") + d(": apply") + sep + k("esc") + d(": back")
		case "tags":
			helpText = k("j/k") + d(": nav") + sep + k("n") + d(": new") + sep +
				k("d") + d(": delete") + sep + k("p") + d(": push") + sep + k("esc") + d(": back")
		case "hooks":
			helpText = k("i") + d(": install") + sep + k("r") + d(": remove") + sep +
				k("c") + d(": check") + sep + k("esc") + d(": back")
		default:
			helpText = k("j/k") + d(": nav") + sep + k("enter") + d(": select") + sep + k("esc") + d(": back")
		}
	}

	// Status message
	var statusText string
	if m.statusMessage != "" {
		statusText = m.statusMessage
	}

	// Layout: status on left, help on right
	leftSide := lipgloss.NewStyle().Inline(true).Background(lipgloss.Color("236")).Render(statusText)
	rightSide := helpText

	availableWidth := m.width - 4
	padding := availableWidth - lipgloss.Width(leftSide) - lipgloss.Width(rightSide)
	if padding < 1 {
		padding = 1
	}

	styledPadding := lipgloss.NewStyle().Background(lipgloss.Color("236")).Render(strings.Repeat(" ", padding))
	content := leftSide + styledPadding + rightSide

	return statusBarStyle.Width(m.width).Render(content)
}

// Workspace tab content
func (m model) renderWorkspaceContent(width, height int) (string, string) {
	if m.viewMode == "diff" {
		return "", m.renderDiff(width, height)
	}

	if m.viewMode == "blame" {
		return "", m.renderBlame(width, height)
	}

	if m.viewMode == "conflicts" {
		return "", m.renderConflictsList(width, height)
	}

	// Files view - split pane layout (scout style)
	if len(m.changes) == 0 {
		return "", m.renderEmptyWorkspace(width, height)
	}

	// Split pane: files on left, diff preview on right
	// Each pane is a self-contained bordered panel
	// Use full available height
	panelHeight := height
	leftWidth := width / 2
	rightWidth := width - leftWidth

	leftPane := m.renderFilePane(leftWidth, panelHeight)
	rightPane := m.renderDiffPane(rightWidth, panelHeight)

	// Force both panels to exact same height
	leftStyled := lipgloss.NewStyle().Height(panelHeight).Render(leftPane)
	rightStyled := lipgloss.NewStyle().Height(panelHeight).Render(rightPane)

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, rightStyled)

	return "", content
}

func (m model) renderEmptyWorkspace(width, height int) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(2, 4).
		Width(width - 4)

	content := lipgloss.JoinVertical(lipgloss.Center,
		sectionHeaderStyle.Render("✨ Working directory clean"),
		"",
		helpStyle.Render("No uncommitted changes"),
		"",
		normalStyle.Render("• Make changes to files to see them here"),
		normalStyle.Render("• Use "+lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Render("git add")+" to stage files"),
	)

	return box.Render(content)
}

// renderDiffPane renders the diff preview as a bordered panel (scout style)
func (m model) renderDiffPane(width, height int) string {
	// Calculate available content height (height minus header and borders ~4 lines)
	contentHeight := height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Width(width - 4)

	var headerText string
	var content string

	if m.diffContent == "" {
		headerText = "👁 Preview"
		content = helpStyle.Render("Select a file to preview changes")
	} else {
		lines := strings.Split(m.diffContent, "\n")
		maxLines := contentHeight
		if maxLines < 1 {
			maxLines = 1
		}

		// Check scroll indicators
		hasTop := m.scrollOffset > 0
		hasBottom := m.scrollOffset+maxLines < len(lines)

		if hasTop {
			maxLines--
		}
		if hasBottom {
			maxLines--
		}

		// Scroll info
		scrollInfo := ""
		if len(lines) > maxLines {
			scrollInfo = helpStyle.Render(fmt.Sprintf("[%d/%d]", m.scrollOffset+1, len(lines)))
		}
		headerText = ("👁 Preview ") + scrollInfo

		// Apply scroll
		startIdx := m.scrollOffset
		if startIdx > len(lines) {
			startIdx = 0
		}
		endIdx := startIdx + maxLines
		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		var items []string
		if hasTop {
			items = append(items, scrollIndicatorStyle.Render("▲"))
		}

		for i := startIdx; i < endIdx; i++ {
			// Truncate long lines
			line := lines[i]
			maxLineWidth := width - 6
			if len(line) > maxLineWidth {
				line = line[:maxLineWidth-3] + "..."
			}
			items = append(items, colorizeDiffLine(line))
		}

		if hasBottom {
			items = append(items, scrollIndicatorStyle.Render("▼"))
		}

		content = strings.Join(items, "\n")
	}

	header := headerStyle.Render(headerText)
	listContent := lipgloss.NewStyle().Padding(0, 1).Render(content)

	// Combine header and content with border - use height-2 for border box
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(width - 2).
		Height(height - 2)

	combined := header + "\n" + listContent
	return borderStyle.Render(combined)
}

// renderFilePane renders the file list as a bordered panel (scout style)
func (m model) renderFilePane(width, height int) string {
	// Calculate available content height (height minus header and borders ~4 lines)
	contentHeight := height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Width(width - 4)

	header := headerStyle.Render(fmt.Sprintf("📄 Files"))

	// Calculate scroll - use most of content height for items
	maxItems := contentHeight
	if maxItems < 1 {
		maxItems = 1
	}

	hasTopIndicator := m.fileOffset > 0
	hasBottomIndicator := m.fileOffset+maxItems < len(m.changes)

	if hasTopIndicator {
		maxItems--
	}
	if hasBottomIndicator {
		maxItems--
	}

	var items []string

	if hasTopIndicator {
		items = append(items, scrollIndicatorStyle.Render("▲ more above"))
	}

	endIdx := m.fileOffset + maxItems
	if endIdx > len(m.changes) {
		endIdx = len(m.changes)
	}

	for i := m.fileOffset; i < endIdx; i++ {
		change := m.changes[i]

		if i == m.fileCursor {
			iconChar, iconColor := getStatusIconParts(change.Status)
			selBg := lipgloss.Color("236")

			iconPart := lipgloss.NewStyle().Foreground(iconColor).Background(selBg).Bold(true).Render(iconChar)
			textPart := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(selBg).Bold(true).Render(" " + change.File)

			line := iconPart + textPart
			items = append(items, lipgloss.NewStyle().Width(width-6).Background(selBg).Render(line))
		} else {
			icon := getStatusIcon(change.Status)
			line := fmt.Sprintf("%s %s", icon, change.File)
			items = append(items, normalStyle.Render(line))
		}
	}

	if hasBottomIndicator {
		items = append(items, scrollIndicatorStyle.Render("▼ more below"))
	}

	listContent := lipgloss.NewStyle().Padding(0, 1).Render(strings.Join(items, "\n"))

	// Combine header and list with border - use height-2 for border box
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(width - 2).
		Height(height - 2)

	combined := header + "\n" + listContent
	return borderStyle.Render(combined)
}

func (m model) renderDiff(width, height int) string {
	if m.diffContent == "" {
		return helpStyle.Render("No diff to display")
	}

	lines := strings.Split(m.diffContent, "\n")

	// Apply scroll
	maxLines := height - 2
	if maxLines < 1 {
		maxLines = 1
	}

	hasTop := m.scrollOffset > 0
	hasBottom := m.scrollOffset+maxLines < len(lines)

	var result []string

	if hasTop {
		result = append(result, scrollIndicatorStyle.Render("scroll up for more..."))
		maxLines--
	}

	endIdx := m.scrollOffset + maxLines
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	for i := m.scrollOffset; i < endIdx; i++ {
		result = append(result, colorizeDiffLine(lines[i]))
	}

	if hasBottom {
		result = append(result, scrollIndicatorStyle.Render("scroll down for more..."))
	}

	return strings.Join(result, "\n")
}

func (m model) renderConflictsList(width, height int) string {
	if len(m.conflicts) == 0 {
		return helpStyle.Render("No conflicts found")
	}

	var lines []string
	for i, conflict := range m.conflicts {
		icon := "!"
		if conflict.IsResolved {
			icon = "ok"
		}
		line := fmt.Sprintf("%s %s", icon, conflict.Path)

		if i == m.conflictCursor {
			lines = append(lines, selectedStyle.Width(width-4).Render(line))
		} else {
			lines = append(lines, normalStyle.Render(line))
		}
	}

	return strings.Join(lines, "\n")
}

// Commit tab content
func (m model) renderCommitContent(width, height int) (string, string) {
	if m.commitSummary != nil {
		return "", m.renderCommitSummary(width, height)
	}

	if m.gitState.StagedFiles == 0 {
		return "", helpStyle.Render("No files staged. Go to Workspace and stage files first.")
	}

	var sections []string

	// Recent commits
	if len(m.recentCommits) > 0 {
		sections = append(sections, helpStyle.Render("Recent:"))
		for _, commit := range m.recentCommits {
			sections = append(sections, fmt.Sprintf("  %s %s",
				lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(commit.Hash),
				commit.Message))
		}
		sections = append(sections, "")
	}

	// Suggestions
	if len(m.suggestions) > 0 {
		sections = append(sections, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Render("Suggestions (↑/↓ to select, enter to commit):"))
		for i, suggestion := range m.suggestions {
			style := suggestionStyle
			indicator := "  "
			if m.selectedSuggestion == i+1 {
				style = selectedSuggestionStyle
				indicator = "> "
			}
			sections = append(sections, style.Render(fmt.Sprintf("%s%s", indicator, suggestion.Message)))
		}
		sections = append(sections, "")
	}

	// Custom input
	sections = append(sections, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Render("Custom message:"))
	sections = append(sections, m.commitInput.View())

	return "", strings.Join(sections, "\n")
}

func (m model) renderCommitSummary(width, height int) string {
	summary := m.commitSummary

	var lines []string

	lines = append(lines, successStyle.Render(fmt.Sprintf("Commit %s", summary.hash)))
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Message: ")+summary.message)
	lines = append(lines, "")

	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Files (%d):", len(summary.files))))
	for _, file := range summary.files {
		lines = append(lines, "  "+file)
	}
	lines = append(lines, "")

	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Diff:"))
	diffLines := strings.Split(summary.diff, "\n")
	for _, line := range diffLines {
		lines = append(lines, colorizeDiffLine(line))
	}
	lines = append(lines, "")

	lines = append(lines, warningStyle.Render("Actions: [p] Push  [c] Continue  [1] Workspace"))

	// Apply scroll
	maxLines := height - 2
	hasTop := m.scrollOffset > 0
	hasBottom := m.scrollOffset+maxLines < len(lines)

	var result []string

	if hasTop {
		result = append(result, scrollIndicatorStyle.Render("scroll up..."))
		maxLines--
	}

	endIdx := m.scrollOffset + maxLines
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	for i := m.scrollOffset; i < endIdx; i++ {
		result = append(result, lines[i])
	}

	if hasBottom {
		result = append(result, scrollIndicatorStyle.Render("scroll down..."))
	}

	return strings.Join(result, "\n")
}

// Branches tab content
func (m model) renderBranchesContent(width, height int) (string, string) {
	if m.branchComparison != nil {
		return "", m.renderBranchComparison(width, height)
	}

	if m.branchInput.Focused() {
		return "", m.branchInput.View()
	}

	if len(m.branches) == 0 {
		return "", helpStyle.Render("Loading branches...")
	}

	return "", m.renderBranchList(width, height)
}

func (m model) renderBranchList(width, height int) string {
	// Count local vs remote
	localCount := 0
	remoteCount := 0
	for _, b := range m.branches {
		if b.IsRemote {
			remoteCount++
		} else {
			localCount++
		}
	}

	header := sectionHeaderStyle.Render("Branches") + " " +
		branchCurrentStyle.Render(fmt.Sprintf("🏠%d", localCount)) + " " +
		branchRemoteStyle.Render(fmt.Sprintf("☁️%d", remoteCount))

	maxItems := height - 4
	if maxItems < 1 {
		maxItems = 1
	}

	hasTop := m.branchOffset > 0
	hasBottom := m.branchOffset+maxItems < len(m.branches)

	if hasTop {
		maxItems--
	}
	if hasBottom {
		maxItems--
	}

	var lines []string
	lines = append(lines, header)
	lines = append(lines, helpStyle.Render(strings.Repeat("─", width-6)))

	if hasTop {
		lines = append(lines, scrollIndicatorStyle.Render("  ▲ more above"))
	}

	endIdx := m.branchOffset + maxItems
	if endIdx > len(m.branches) {
		endIdx = len(m.branches)
	}

	for i := m.branchOffset; i < endIdx; i++ {
		branch := m.branches[i]

		// Icon based on branch type
		var icon string
		var nameStyle lipgloss.Style
		if branch.IsCurrent {
			icon = branchCurrentStyle.Render("🏠")
			nameStyle = branchCurrentStyle
		} else if branch.IsRemote {
			icon = branchRemoteStyle.Render("☁️")
			nameStyle = branchRemoteStyle
		} else {
			icon = helpStyle.Render("🌿")
			nameStyle = normalStyle
		}

		// Tracking info with colored ahead/behind
		tracking := ""
		if branch.Upstream != "" {
			tracking = helpStyle.Render(" → " + branch.Upstream)
			if branch.Ahead > 0 {
				tracking += " " + branchAheadStyle.Render(fmt.Sprintf("↑%d", branch.Ahead))
			}
			if branch.Behind > 0 {
				tracking += " " + branchBehindStyle.Render(fmt.Sprintf("↓%d", branch.Behind))
			}
		}

		line := fmt.Sprintf(" %s %s%s", icon, nameStyle.Render(branch.Name), tracking)

		if i == m.branchCursor {
			lines = append(lines, selectedStyle.Width(width-4).Render(line))
		} else {
			lines = append(lines, line)
		}
	}

	if hasBottom {
		lines = append(lines, scrollIndicatorStyle.Render("  ▼ more below"))
	}

	return strings.Join(lines, "\n")
}

func (m model) renderBranchComparison(width, height int) string {
	if m.branchComparison == nil {
		return ""
	}

	var lines []string

	lines = append(lines, fmt.Sprintf("%s vs %s",
		m.branchComparison.SourceBranch,
		m.branchComparison.TargetBranch))
	lines = append(lines, "")

	lines = append(lines, fmt.Sprintf("Ahead: %d commits", len(m.branchComparison.AheadCommits)))
	for _, commit := range m.branchComparison.AheadCommits {
		lines = append(lines, fmt.Sprintf("  %s %s", commit.Hash, commit.Message))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Behind: %d commits", len(m.branchComparison.BehindCommits)))
	for _, commit := range m.branchComparison.BehindCommits {
		lines = append(lines, fmt.Sprintf("  %s %s", commit.Hash, commit.Message))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Files changed: %d", len(m.branchComparison.DifferingFiles)))

	return strings.Join(lines, "\n")
}

// Tools tab content
func (m model) renderToolsContent(width, height int) (string, string) {
	switch m.toolMode {
	case "log":
		return "", m.renderLogContent(width, height)
	case "undo":
		return "", m.renderUndoList(width, height)
	case "rebase":
		return "", m.renderRebaseContent(width, height)
	case "history":
		return "", m.renderHistoryList(width, height)
	case "remote":
		return "", m.renderRemoteContent(width, height)
	case "stash":
		return "", m.renderStashList(width, height)
	case "tags":
		return "", m.renderTagsList(width, height)
	case "hooks":
		return "", m.renderHooksContent(width, height)
	case "clone":
		return "", m.renderCloneContent(width, height)
	case "init":
		return "", m.renderInitContent(width, height)
	case "clean":
		return "", m.renderCleanContent(width, height)
	default:
		return "", m.renderToolsMenu(width, height)
	}
}

func (m model) renderToolsMenu(width, height int) string {
	tools := []struct {
		key  string
		icon string
		name string
		desc string
	}{
		{"o", "📜", "Log", "Browse commit history"},
		{"s", "📦", "Stash", "Save/restore work in progress"},
		{"t", "🏷️", "Tags", "Manage version tags"},
		{"h", "📜", "History", "View reflog"},
		{"u", "⏪", "Undo", "Undo recent commits"},
		{"r", "📝", "Rebase", "Interactive rebase"},
		{"p", "⬆️", "Push", "Push to remote"},
		{"f", "⬇️", "Fetch/Pull", "Sync with remote"},
		{"g", "🔒", "Hooks", "Git hooks management"},
		{"x", "🧹", "Clean", "Remove untracked files"},
		{"c", "📥", "Clone", "Clone a repository"},
		{"i", "🆕", "Init", "Initialize new repo"},
	}

	var lines []string
	lines = append(lines, sectionHeaderStyle.Render("Git Tools"))
	lines = append(lines, helpStyle.Render(strings.Repeat("─", width-6)))

	for i, tool := range tools {
		selBg := lipgloss.Color("236")

		if i == m.toolCursor {
			sp := lipgloss.NewStyle().Background(selBg).Render(" ")
			sp2 := lipgloss.NewStyle().Background(selBg).Render("  ")
			keyStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("75")).
				Background(selBg).
				Bold(true)
			iconStyle := lipgloss.NewStyle().Background(selBg)
			nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(selBg).Bold(true)
			descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Background(selBg)

			line := sp + iconStyle.Render(tool.icon) + sp + keyStyle.Render("["+tool.key+"]") + sp + nameStyle.Render(tool.name) + sp2 + descStyle.Render(tool.desc)

			lines = append(lines, lipgloss.NewStyle().Width(width-4).Background(selBg).Render(line))
		} else {
			keyStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("75")).
				Bold(true)

			line := fmt.Sprintf(" %s %s %s  %s",
				tool.icon,
				keyStyle.Render("["+tool.key+"]"),
				tool.name,
				helpStyle.Render(tool.desc))

			lines = append(lines, line)
		}
	}

	// Show hook status indicator
	lines = append(lines, "")
	hookStatus := "❌ Hook not installed"
	if m.commitMsgHookInstalled {
		hookStatus = "✅ Commit-msg hook active"
	}
	lines = append(lines, helpStyle.Render(hookStatus))

	return strings.Join(lines, "\n")
}

func (m model) renderUndoList(width, height int) string {
	commits := m.commits
	if len(commits) == 0 {
		return helpStyle.Render("No commits to undo")
	}

	maxItems := height - 2
	if maxItems < 1 {
		maxItems = 1
	}

	hasTop := m.undoOffset > 0
	hasBottom := m.undoOffset+maxItems < len(commits)

	if hasTop {
		maxItems--
	}
	if hasBottom {
		maxItems--
	}

	var lines []string

	if hasTop {
		lines = append(lines, scrollIndicatorStyle.Render("more above..."))
	}

	endIdx := m.undoOffset + maxItems
	if endIdx > len(commits) {
		endIdx = len(commits)
	}

	for i := m.undoOffset; i < endIdx; i++ {
		commit := commits[i]
		line := fmt.Sprintf("%s %s (%s)", commit.Hash, commit.Message, commit.Date)

		if i == m.undoCursor {
			lines = append(lines, selectedStyle.Width(width-4).Render(line))
		} else {
			lines = append(lines, normalStyle.Render(line))
		}
	}

	if hasBottom {
		lines = append(lines, scrollIndicatorStyle.Render("more below..."))
	}

	return strings.Join(lines, "\n")
}

func (m model) renderRebaseContent(width, height int) string {
	if m.rebaseInput.Focused() {
		return "Enter number of commits: " + m.rebaseInput.View()
	}

	if len(m.rebaseCommits) == 0 {
		return helpStyle.Render("Enter number of commits (1-50)")
	}

	var lines []string
	for i, commit := range m.rebaseCommits {
		action := commit.Action
		if action == "" {
			action = "pick"
		}
		line := fmt.Sprintf("[%s] %s %s", action, commit.Hash, commit.Message)

		if i == m.rebaseCursor {
			lines = append(lines, selectedStyle.Width(width-4).Render(line))
		} else {
			lines = append(lines, normalStyle.Render(line))
		}
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("p=pick s=squash r=reword d=drop enter=execute"))

	return strings.Join(lines, "\n")
}

func (m model) renderHistoryList(width, height int) string {
	if len(m.commits) == 0 {
		return helpStyle.Render("Loading history...")
	}

	maxItems := height - 2
	if maxItems < 1 {
		maxItems = 1
	}

	hasTop := m.historyOffset > 0
	hasBottom := m.historyOffset+maxItems < len(m.commits)

	if hasTop {
		maxItems--
	}
	if hasBottom {
		maxItems--
	}

	var lines []string

	if hasTop {
		lines = append(lines, scrollIndicatorStyle.Render("more above..."))
	}

	endIdx := m.historyOffset + maxItems
	if endIdx > len(m.commits) {
		endIdx = len(m.commits)
	}

	for i := m.historyOffset; i < endIdx; i++ {
		commit := m.commits[i]
		line := fmt.Sprintf("%s %s (%s - %s)",
			lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(commit.Hash),
			commit.Message,
			commit.Author,
			commit.Date)

		if i == m.historyCursor {
			lines = append(lines, selectedStyle.Width(width-4).Render(line))
		} else {
			lines = append(lines, normalStyle.Render(line))
		}
	}

	if hasBottom {
		lines = append(lines, scrollIndicatorStyle.Render("more below..."))
	}

	return strings.Join(lines, "\n")
}

func (m model) renderRemoteContent(width, height int) string {
	if m.pushOutput != "" {
		return m.pushOutput
	}

	var lines []string
	lines = append(lines, "[p] Push to origin")
	lines = append(lines, "[f] Fetch from origin")
	lines = append(lines, "[l] Pull from origin")

	return strings.Join(lines, "\n")
}

func (m model) renderStashList(width, height int) string {
	k := func(key string) string { return keyBindStyle.Render(key) }
	d := func(desc string) string { return keyDescStyle.Render(desc) }
	sep := keyDescStyle.Render(" | ")

	header := sectionHeaderStyle.Render("Stash List")
	help := k("s") + d(": stash") + sep + k("p/enter") + d(": pop") + sep +
		k("a") + d(": apply") + sep + k("d") + d(": drop")

	if len(m.stashes) == 0 {
		return header + "\n" + helpStyle.Render(strings.Repeat("─", width-6)) + "\n\n" +
			helpStyle.Render("No stashes. Press 's' to stash current changes.") + "\n\n" + help
	}

	maxItems := height - 4
	if maxItems < 1 {
		maxItems = 1
	}

	hasTop := m.stashOffset > 0
	hasBottom := m.stashOffset+maxItems < len(m.stashes)

	if hasTop {
		maxItems--
	}
	if hasBottom {
		maxItems--
	}

	var lines []string
	lines = append(lines, header)
	lines = append(lines, helpStyle.Render(strings.Repeat("─", width-6)))

	if hasTop {
		lines = append(lines, scrollIndicatorStyle.Render("  ▲ more above"))
	}

	endIdx := m.stashOffset + maxItems
	if endIdx > len(m.stashes) {
		endIdx = len(m.stashes)
	}

	for i := m.stashOffset; i < endIdx; i++ {
		stash := m.stashes[i]
		line := fmt.Sprintf(" 📦 stash@{%d}: %s  %s",
			stash.Index,
			stash.Message,
			helpStyle.Render(stash.Date))

		if i == m.stashCursor {
			lines = append(lines, selectedStyle.Width(width-4).Render(line))
		} else {
			lines = append(lines, line)
		}
	}

	if hasBottom {
		lines = append(lines, scrollIndicatorStyle.Render("  ▼ more below"))
	}

	lines = append(lines, "")
	lines = append(lines, help)

	return strings.Join(lines, "\n")
}

func (m model) renderTagsList(width, height int) string {
	k := func(key string) string { return keyBindStyle.Render(key) }
	d := func(desc string) string { return keyDescStyle.Render(desc) }
	sep := keyDescStyle.Render(" | ")

	header := sectionHeaderStyle.Render("Tags")
	help := k("n") + d(": new tag") + sep + k("d") + d(": delete") + sep +
		k("p") + d(": push tag") + sep + k("P") + d(": push all")

	if m.tagInput.Focused() {
		return header + "\n" + helpStyle.Render(strings.Repeat("─", width-6)) + "\n\n" +
			"Create new tag:\n" + m.tagInput.View()
	}

	if len(m.tags) == 0 {
		return header + "\n" + helpStyle.Render(strings.Repeat("─", width-6)) + "\n\n" +
			helpStyle.Render("No tags. Press 'n' to create a new tag.") + "\n\n" + help
	}

	maxItems := height - 4
	if maxItems < 1 {
		maxItems = 1
	}

	hasTop := m.tagOffset > 0
	hasBottom := m.tagOffset+maxItems < len(m.tags)

	if hasTop {
		maxItems--
	}
	if hasBottom {
		maxItems--
	}

	var lines []string
	lines = append(lines, header)
	lines = append(lines, helpStyle.Render(strings.Repeat("─", width-6)))

	if hasTop {
		lines = append(lines, scrollIndicatorStyle.Render("  ▲ more above"))
	}

	endIdx := m.tagOffset + maxItems
	if endIdx > len(m.tags) {
		endIdx = len(m.tags)
	}

	for i := m.tagOffset; i < endIdx; i++ {
		tag := m.tags[i]
		icon := "🏷️"
		if tag.IsAnnotated {
			icon = "📝"
		}

		commitInfo := ""
		if tag.Commit != "" {
			commitInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(" " + tag.Commit[:7])
		}

		line := fmt.Sprintf(" %s %s%s  %s",
			icon,
			tag.Name,
			commitInfo,
			helpStyle.Render(tag.Date))

		if i == m.tagCursor {
			lines = append(lines, selectedStyle.Width(width-4).Render(line))
		} else {
			lines = append(lines, line)
		}
	}

	if hasBottom {
		lines = append(lines, scrollIndicatorStyle.Render("  ▼ more below"))
	}

	lines = append(lines, "")
	lines = append(lines, help)

	return strings.Join(lines, "\n")
}

func (m model) renderHooksContent(width, height int) string {
	k := func(key string) string { return keyBindStyle.Render(key) }
	d := func(desc string) string { return keyDescStyle.Render(desc) }
	sep := keyDescStyle.Render(" | ")

	header := sectionHeaderStyle.Render("Git Hooks")

	var lines []string
	lines = append(lines, header)
	lines = append(lines, helpStyle.Render(strings.Repeat("─", width-6)))
	lines = append(lines, "")

	// Available hooks
	hooks := []struct {
		name      string
		desc      string
		installed bool
		key       string
	}{
		{"Conventional Commits", "Enforce commit message format", m.commitMsgHookInstalled, "1"},
		{"No Large Files", "Block files >5MB", m.preCommitHookInstalled, "2"},
		{"Detect Secrets", "Block passwords/API keys", m.preCommitHookInstalled, "3"},
	}

	for i, hook := range hooks {
		status := warningStyle.Render("○")
		if hook.installed {
			status = successStyle.Render("●")
		}

		line := fmt.Sprintf(" %s [%s] %s  %s", status, hook.key, hook.name, helpStyle.Render(hook.desc))
		if i == m.hookCursor {
			lines = append(lines, selectedStyle.Width(width-4).Render(line))
		} else {
			lines = append(lines, line)
		}
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render(strings.Repeat("─", width-6)))
	lines = append(lines, "")

	// Help text
	help := k("1/2/3") + d(": install") + sep + k("r") + d(": remove selected") + sep + k("j/k") + d(": nav")
	lines = append(lines, help)

	return strings.Join(lines, "\n")
}

// Helper functions

func colorizeDiffLine(line string) string {
	if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
		return diffAddStyle.Render(line)
	}
	if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
		return diffRemoveStyle.Render(line)
	}
	if strings.HasPrefix(line, "@@") {
		return diffHunkStyle.Render(line)
	}
	if strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") ||
		strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
		return diffHeaderStyle.Render(line)
	}
	return line
}

func getStatusIcon(status string) string {
	switch status {
	case "M ":
		return iconStagedStyle.Render("✓") // Modified (staged)
	case "MM":
		return iconStagedStyle.Render("✓") + iconUnstagedStyle.Render("●") // Both
	case " M":
		return iconUnstagedStyle.Render("●") // Modified (unstaged)
	case "A ":
		return iconStagedStyle.Render("+") // Added (staged)
	case "D ":
		return iconDeletedStyle.Render("−") // Deleted (staged)
	case " D":
		return iconDeletedStyle.Render("×") // Deleted (unstaged)
	case "R ":
		return iconStagedStyle.Render("→") // Renamed
	case "??":
		return iconUntrackedStyle.Render("?") // Untracked
	case "UU":
		return iconConflictStyle.Render("⚠") // Conflict
	default:
		return " "
	}
}

func getStatusIconParts(status string) (string, lipgloss.Color) {
	switch status {
	case "M ":
		return "✓", lipgloss.Color("82")
	case "MM":
		return "✓●", lipgloss.Color("82")
	case " M":
		return "●", lipgloss.Color("214")
	case "A ":
		return "+", lipgloss.Color("82")
	case "D ":
		return "−", lipgloss.Color("196")
	case " D":
		return "×", lipgloss.Color("196")
	case "R ":
		return "→", lipgloss.Color("82")
	case "??":
		return "?", lipgloss.Color("245")
	case "UU":
		return "⚠", lipgloss.Color("196")
	default:
		return " ", lipgloss.Color("252")
	}
}

// Log viewer

func (m model) renderLogContent(width, height int) string {
	// If viewing commit detail
	if m.logDetail != nil {
		return m.renderLogDetail(width, height)
	}

	k := func(key string) string { return keyBindStyle.Render(key) }
	d := func(desc string) string { return keyDescStyle.Render(desc) }
	sep := keyDescStyle.Render(" | ")

	searchInfo := ""
	if m.logSearch != "" {
		searchInfo = helpStyle.Render(fmt.Sprintf(" (filter: %s)", m.logSearch))
	}

	header := sectionHeaderStyle.Render("Commit Log") + searchInfo
	help := k("/") + d(": search") + sep + k("enter") + d(": detail") + sep +
		k("c") + d(": cherry-pick") + sep + k("R") + d(": revert") + sep + k("esc") + d(": back")

	if m.logSearchInput.Focused() {
		return header + "\n" + helpStyle.Render(strings.Repeat("─", width-6)) + "\n\n" +
			"Search: " + m.logSearchInput.View()
	}

	if len(m.logCommits) == 0 {
		return header + "\n" + helpStyle.Render(strings.Repeat("─", width-6)) + "\n\n" +
			helpStyle.Render("No commits found.") + "\n\n" + help
	}

	maxItems := height - 4
	if maxItems < 1 {
		maxItems = 1
	}

	hasTop := m.logOffset > 0
	hasBottom := m.logOffset+maxItems < len(m.logCommits)

	if hasTop {
		maxItems--
	}
	if hasBottom {
		maxItems--
	}

	var lines []string
	lines = append(lines, header)
	lines = append(lines, helpStyle.Render(strings.Repeat("─", width-6)))

	if hasTop {
		lines = append(lines, scrollIndicatorStyle.Render("  ▲ more above"))
	}

	endIdx := m.logOffset + maxItems
	if endIdx > len(m.logCommits) {
		endIdx = len(m.logCommits)
	}

	for i := m.logOffset; i < endIdx; i++ {
		commit := m.logCommits[i]
		hashStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
		dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

		line := fmt.Sprintf(" %s %s  %s",
			hashStyle.Render(commit.Hash),
			commit.Message,
			dateStyle.Render(commit.Date))

		if i == m.logCursor {
			lines = append(lines, selectedStyle.Width(width-4).Render(line))
		} else {
			lines = append(lines, line)
		}
	}

	if hasBottom {
		lines = append(lines, scrollIndicatorStyle.Render("  ▼ more below"))
	}

	lines = append(lines, "")
	lines = append(lines, help)

	return strings.Join(lines, "\n")
}

func (m model) renderLogDetail(width, height int) string {
	detail := m.logDetail
	if detail == nil {
		return ""
	}

	var lines []string

	// Header info
	hashStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	lines = append(lines, hashStyle.Render("Commit: "+detail.Hash))
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Author: ")+detail.Author+" <"+detail.Email+">")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Date:   ")+detail.Date)
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Message: ")+detail.Message)
	if detail.Body != "" {
		lines = append(lines, detail.Body)
	}
	lines = append(lines, "")

	// Files
	if len(detail.Files) > 0 {
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Files (%d):", len(detail.Files))))
		for _, f := range detail.Files {
			lines = append(lines, "  "+f)
		}
		lines = append(lines, "")
	}

	// Diff
	if m.logDiff != "" {
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Diff:"))
		diffLines := strings.Split(m.logDiff, "\n")
		for _, dl := range diffLines {
			lines = append(lines, colorizeDiffLine(dl))
		}
	}

	// Apply scroll
	maxLines := height - 2
	hasTop := m.scrollOffset > 0
	hasBottom := m.scrollOffset+maxLines < len(lines)

	var result []string

	if hasTop {
		result = append(result, scrollIndicatorStyle.Render("scroll up..."))
		maxLines--
	}

	endIdx := m.scrollOffset + maxLines
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	for i := m.scrollOffset; i < endIdx; i++ {
		result = append(result, lines[i])
	}

	if hasBottom {
		result = append(result, scrollIndicatorStyle.Render("scroll down..."))
	}

	return strings.Join(result, "\n")
}

// Blame view

func (m model) renderBlame(width, height int) string {
	if len(m.blameLines) == 0 {
		return helpStyle.Render("Loading blame...")
	}

	k := func(key string) string { return keyBindStyle.Render(key) }
	d := func(desc string) string { return keyDescStyle.Render(desc) }

	header := sectionHeaderStyle.Render("Blame: " + m.blameFile)
	help := k("j/k") + d(": nav") + " | " + k("esc") + d(": back")

	maxItems := height - 4
	if maxItems < 1 {
		maxItems = 1
	}

	hasTop := m.blameOffset > 0
	hasBottom := m.blameOffset+maxItems < len(m.blameLines)

	if hasTop {
		maxItems--
	}
	if hasBottom {
		maxItems--
	}

	var lines []string
	lines = append(lines, header)
	lines = append(lines, helpStyle.Render(strings.Repeat("─", width-6)))

	if hasTop {
		lines = append(lines, scrollIndicatorStyle.Render("  ▲ more above"))
	}

	endIdx := m.blameOffset + maxItems
	if endIdx > len(m.blameLines) {
		endIdx = len(m.blameLines)
	}

	hashStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	authorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	lineNumStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	for i := m.blameOffset; i < endIdx; i++ {
		bl := m.blameLines[i]
		// Truncate author name
		author := bl.Author
		if len(author) > 10 {
			author = author[:10]
		}

		line := fmt.Sprintf("%s %s %s %s %s",
			hashStyle.Render(bl.Hash),
			authorStyle.Render(fmt.Sprintf("%-10s", author)),
			dateStyle.Render(bl.Date),
			lineNumStyle.Render(fmt.Sprintf("%4d", bl.LineNum)),
			bl.Content)

		// Truncate if too long
		if len(line) > width-4 {
			line = line[:width-7] + "..."
		}

		if i == m.blameCursor {
			lines = append(lines, selectedStyle.Width(width-4).Render(line))
		} else {
			lines = append(lines, line)
		}
	}

	if hasBottom {
		lines = append(lines, scrollIndicatorStyle.Render("  ▼ more below"))
	}

	lines = append(lines, "")
	lines = append(lines, help)

	return strings.Join(lines, "\n")
}

// Clean view

func (m model) renderCleanContent(width, height int) string {
	k := func(key string) string { return keyBindStyle.Render(key) }
	d := func(desc string) string { return keyDescStyle.Render(desc) }
	sep := keyDescStyle.Render(" | ")

	header := sectionHeaderStyle.Render("Clean Untracked Files")
	help := k("d") + d(": delete all") + sep + k("r") + d(": refresh") + sep + k("esc") + d(": back")

	if len(m.cleanFiles) == 0 {
		return header + "\n" + helpStyle.Render(strings.Repeat("─", width-6)) + "\n\n" +
			successStyle.Render("✨ No untracked files to clean") + "\n\n" + help
	}

	var lines []string
	lines = append(lines, header)
	lines = append(lines, helpStyle.Render(strings.Repeat("─", width-6)))
	lines = append(lines, "")
	lines = append(lines, warningStyle.Render(fmt.Sprintf("⚠️  %d untracked file(s) will be deleted:", len(m.cleanFiles))))
	lines = append(lines, "")

	for i, file := range m.cleanFiles {
		line := "  " + file
		if i == m.cleanCursor {
			lines = append(lines, selectedStyle.Width(width-4).Render(line))
		} else {
			lines = append(lines, normalStyle.Render(line))
		}
	}

	lines = append(lines, "")
	lines = append(lines, help)

	return strings.Join(lines, "\n")
}

// Clone/Init views

func (m model) renderCloneContent(width, height int) string {
	var lines []string
	lines = append(lines, sectionHeaderStyle.Render("Clone Repository"))
	lines = append(lines, helpStyle.Render(strings.Repeat("─", width-6)))
	lines = append(lines, "")
	lines = append(lines, normalStyle.Render("Enter repository URL:"))
	lines = append(lines, "")
	lines = append(lines, m.cloneInput.View())
	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("Examples:"))
	lines = append(lines, helpStyle.Render("  https://github.com/user/repo.git"))
	lines = append(lines, helpStyle.Render("  git@github.com:user/repo.git"))
	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("Press enter to clone, esc to cancel"))

	return strings.Join(lines, "\n")
}

func (m model) renderInitContent(width, height int) string {
	var lines []string
	lines = append(lines, sectionHeaderStyle.Render("Initialize Repository"))
	lines = append(lines, helpStyle.Render(strings.Repeat("─", width-6)))
	lines = append(lines, "")
	lines = append(lines, normalStyle.Render("Enter directory path:"))
	lines = append(lines, "")
	lines = append(lines, m.initInput.View())
	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("Leave empty for current directory, or enter a path"))
	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("Press enter to init, esc to cancel"))

	return strings.Join(lines, "\n")
}
