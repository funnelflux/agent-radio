package panel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/funnelflux/agent-radio/internal/config"
	"github.com/funnelflux/agent-radio/internal/store"
	"github.com/funnelflux/agent-radio/internal/tmuxradio"
)

const (
	tabWorkspaces = iota
	tabMessages
	tabLogs
	tabConfig
)

type sessionState struct {
	cfg       config.Session
	alive     bool
	active    bool
	unread    int
	latest    string
	snapshot  string
	history   []store.Message
	pathOK    bool
	lastError string
}

type workspaceState struct {
	cfg      config.Workspace
	sessions []sessionState
	running  int
	unread   int
}

type refreshMsg struct {
	workspaces []workspaceState
	recent     []store.Message
	snapshots  map[string]string
	router     bool
	tmuxOK     bool
	dbStatus   string
	err        error
}

type actionMsg struct {
	status string
	err    error
}

type tickMsg time.Time

type clickTarget struct {
	kind      string
	index     int
	workspace int
	x1        int
	x2        int
	y1        int
	y2        int
}

type model struct {
	ctx        context.Context
	cfg        config.Config
	configPath string

	width  int
	height int
	tab    int
	focus  int

	workspaceCursor int
	sessionCursor   int
	messageCursor   int

	workspaces []workspaceState
	recent     []store.Message
	snapshots  map[string]string
	router     bool
	tmuxOK     bool
	dbStatus   string
	status     string
	hoverKind  string
	frame      int

	confirm string
	targets []clickTarget
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62")).Padding(0, 1)
	tabStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	activeTab   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("36")).Padding(0, 1)
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	goodStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	badStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	selectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("24"))
)

func Run(ctx context.Context) error {
	cfg, path, err := config.LoadDefault()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	m := &model{
		ctx:        ctx,
		cfg:        cfg,
		configPath: path,
		tab:        tabWorkspaces,
		status:     "loading",
	}
	prog := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())
	_, err = prog.Run()
	return err
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.refreshCmd(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		m.frame++
		if m.frame%8 == 0 {
			return m, tea.Batch(m.refreshCmd(), tickCmd())
		}
		return m, tickCmd()
	case refreshMsg:
		if msg.err != nil {
			m.status = "refresh error: " + msg.err.Error()
			return m, nil
		}
		m.workspaces = msg.workspaces
		m.recent = msg.recent
		m.snapshots = msg.snapshots
		m.router = msg.router
		m.tmuxOK = msg.tmuxOK
		m.dbStatus = msg.dbStatus
		m.clamp()
		if m.status == "loading" {
			m.status = "ready"
		}
		return m, nil
	case actionMsg:
		if msg.err != nil {
			m.status = "error: " + msg.err.Error()
		} else if msg.status != "" {
			m.status = msg.status
		} else {
			m.status = ""
		}
		m.confirm = ""
		return m, m.refreshCmd()
	case tea.MouseMsg:
		if target, ok := m.targetAt(msg.X, msg.Y); ok {
			m.hoverKind = target.kind
		} else {
			m.hoverKind = ""
		}
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			return m, m.clickAt(msg.X, msg.Y)
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	default:
		return m, nil
	}
}

func (m *model) clickAt(x, y int) tea.Cmd {
	target, ok := m.targetAt(x, y)
	if !ok {
		return nil
	}
	switch target.kind {
	case "tab":
		m.tab = target.index
		m.focus = 0
	case "back":
		if m.focus > 0 {
			m.focus--
		}
	case "workspace":
		m.workspaceCursor = target.index
		m.focus = 0
		if m.isMobile() {
			m.focus = 1
		}
	case "session":
		if target.workspace >= 0 {
			m.workspaceCursor = target.workspace
		}
		m.sessionCursor = target.index
		m.focus = 1
	case "message":
		m.messageCursor = target.index
	case "open":
		if m.tab == tabWorkspaces {
			return m.openSelectedCmd()
		}
	case "start":
		return m.startSelectedCmd()
	case "start-workspace":
		return m.startWorkspaceCmd()
	case "kill":
		m.confirm = "kill-session"
		m.status = "kill selected session? y/n"
	case "kill-workspace":
		m.confirm = "kill-workspace"
		m.status = "kill all workspace sessions? y/n"
	}
	return nil
}

func (m *model) targetAt(x, y int) (clickTarget, bool) {
	for i := len(m.targets) - 1; i >= 0; i-- {
		target := m.targets[i]
		if x >= target.x1 && x <= target.x2 && y >= target.y1 && y <= target.y2 {
			return target, true
		}
	}
	return clickTarget{}, false
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.confirm != "" {
		switch key {
		case "y", "Y":
			if m.confirm == "kill-session" {
				return m, m.killSelectedCmd()
			}
			if m.confirm == "kill-workspace" {
				return m, m.killWorkspaceCmd()
			}
		case "n", "N", "esc":
			m.confirm = ""
			m.status = ""
		}
		return m, nil
	}
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab":
		m.tab = (m.tab + 1) % 4
		m.focus = 0
	case "shift+tab":
		m.tab = (m.tab + 3) % 4
		m.focus = 0
	case "left", "backspace":
		if m.isMobile() && m.focus > 0 {
			m.focus--
		} else if !m.isMobile() && m.focus > 0 {
			m.focus--
		}
	case "right":
		if m.tab == tabWorkspaces && m.focus < 1 {
			m.focus++
		}
	case "up", "p":
		m.move(-1)
	case "down", "j", "n":
		m.move(1)
	case "enter":
		if m.tab == tabWorkspaces {
			if m.isMobile() && m.focus == 0 {
				m.focus = 1
				return m, nil
			}
			return m, m.openSelectedCmd()
		}
	case "s":
		return m, m.startSelectedCmd()
	case "S":
		return m, m.startWorkspaceCmd()
	case "r":
		m.status = "refreshing"
		return m, m.refreshCmd()
	case "k":
		m.confirm = "kill-session"
		m.status = "kill selected session? y/n"
	case "K":
		m.confirm = "kill-workspace"
		m.status = "kill all workspace sessions? y/n"
	}
	return m, nil
}

func (m *model) move(delta int) {
	switch m.tab {
	case tabWorkspaces:
		if m.focus == 0 {
			m.workspaceCursor += delta
		} else {
			m.sessionCursor += delta
		}
	case tabLogs:
		m.moveLogSession(delta)
	case tabMessages:
		m.messageCursor += delta
	}
	m.clamp()
}

func (m *model) moveLogSession(delta int) {
	type pos struct {
		workspace int
		session   int
	}
	var positions []pos
	current := 0
	for wi, ws := range m.workspaces {
		for si := range ws.sessions {
			if wi == m.workspaceCursor && si == m.sessionCursor {
				current = len(positions)
			}
			positions = append(positions, pos{workspace: wi, session: si})
		}
	}
	if len(positions) == 0 {
		return
	}
	next := current + delta
	if next < 0 {
		next = 0
	}
	if next >= len(positions) {
		next = len(positions) - 1
	}
	m.workspaceCursor = positions[next].workspace
	m.sessionCursor = positions[next].session
	m.focus = 1
}

func (m *model) clamp() {
	if m.workspaceCursor < 0 {
		m.workspaceCursor = 0
	}
	if m.workspaceCursor >= len(m.workspaces) {
		m.workspaceCursor = len(m.workspaces) - 1
	}
	if m.workspaceCursor < 0 {
		m.workspaceCursor = 0
	}
	sessions := m.currentSessions()
	if m.sessionCursor < 0 {
		m.sessionCursor = 0
	}
	if m.sessionCursor >= len(sessions) {
		m.sessionCursor = len(sessions) - 1
	}
	if m.sessionCursor < 0 {
		m.sessionCursor = 0
	}
	if m.messageCursor < 0 {
		m.messageCursor = 0
	}
	if m.messageCursor >= len(m.recent) {
		m.messageCursor = len(m.recent) - 1
	}
	if m.messageCursor < 0 {
		m.messageCursor = 0
	}
}

func (m *model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		return m.refresh()
	}
}

func (m *model) refresh() tea.Msg {
	ctx, cancel := context.WithTimeout(m.ctx, 2*time.Second)
	defer cancel()
	ss, tmuxErr := tmuxradio.Sessions(ctx)
	sessionMap := map[string]bool{}
	if tmuxErr == nil {
		for _, s := range ss {
			sessionMap[s.Name] = true
		}
	}
	st, _, dbErr := store.OpenDefault(ctx)
	dbStatus := "ok"
	if dbErr != nil {
		dbStatus = dbErr.Error()
	}
	var recent []store.Message
	if st != nil {
		defer st.Close()
		recent, _ = st.Recent(ctx, 12)
		version, err := st.SchemaVersion(ctx)
		if err == nil {
			dbStatus = fmt.Sprintf("v%d", version)
		}
	}
	out := make([]workspaceState, 0, len(m.cfg.Workspaces))
	snapshots := map[string]string{}
	for _, ws := range m.cfg.Workspaces {
		wsv := workspaceState{cfg: ws}
		for _, s := range ws.Sessions {
			state := sessionState{cfg: s, alive: sessionMap[s.Name], pathOK: pathExists(s.Path)}
			if st != nil {
				state.unread, _ = st.UnreadCount(ctx, s.AgentID)
				if latest, ok, err := st.LatestForAgent(ctx, s.AgentID); err == nil && ok {
					state.latest = fmt.Sprintf("#%d %s %s -> %s", latest.ID, latest.Kind, latest.From, latest.To)
				}
				state.history, _ = st.RecentForAgent(ctx, s.AgentID, 40)
			}
			if state.alive {
				wsv.running++
				snap, err := tmuxradio.Capture(ctx, s.Name, 10)
				if err == nil {
					state.snapshot = snap
					snapshots[s.Name] = snap
					if previous := m.snapshots[s.Name]; previous != "" && previous != snap {
						state.active = true
					}
				}
			}
			wsv.unread += state.unread
			wsv.sessions = append(wsv.sessions, state)
		}
		out = append(out, wsv)
	}
	return refreshMsg{
		workspaces: out,
		recent:     recent,
		snapshots:  snapshots,
		router:     sessionMap["agent-radio-router"],
		tmuxOK:     tmuxErr == nil,
		dbStatus:   dbStatus,
		err:        nil,
	}
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func (m *model) startSelectedCmd() tea.Cmd {
	s, ok := m.currentSession()
	if !ok {
		return nil
	}
	return m.startSessionsCmd([]config.Session{s.cfg})
}

func (m *model) startWorkspaceCmd() tea.Cmd {
	ws, ok := m.currentWorkspace()
	if !ok {
		return nil
	}
	var sessions []config.Session
	for _, s := range ws.sessions {
		if !s.alive {
			sessions = append(sessions, s.cfg)
		}
	}
	if len(sessions) == 0 {
		m.status = "all sessions already running"
		return nil
	}
	return m.startSessionsCmd(sessions)
}

func (m *model) startSessionsCmd(sessions []config.Session) tea.Cmd {
	m.status = progressText(0, len(sessions), "starting")
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 20*time.Second)
		defer cancel()
		for i, s := range sessions {
			_ = i
			var err error
			if strings.EqualFold(s.Type, "router") {
				err = tmuxradio.StartShell(ctx, s.Name, s.Path, s.Command, s.AgentID)
			} else {
				err = tmuxradio.StartInteractiveShell(ctx, s.Name, s.Path, s.Command, s.AgentID)
			}
			if err != nil {
				return actionMsg{err: err}
			}
		}
		return actionMsg{}
	}
}

func (m *model) openSelectedCmd() tea.Cmd {
	s, ok := m.currentSession()
	if !ok {
		return nil
	}
	if !s.alive {
		return func() tea.Msg { return actionMsg{err: fmt.Errorf("session %q is not running", s.cfg.Name)} }
	}
	var cmd *exec.Cmd
	if os.Getenv("TMUX") != "" {
		cmd = exec.Command("bash", "-lc", tmuxOpenScript("switch-client", s.cfg.Name))
	} else {
		cmd = exec.Command("bash", "-lc", tmuxOpenScript("attach-session", s.cfg.Name))
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{}
	})
}

func tmuxOpenScript(command, session string) string {
	q := shellQuote(session)
	if command == "switch-client" {
		return fmt.Sprintf("panel=$(tmux display-message -p '#S' 2>/dev/null || true); if [ -n \"$panel\" ]; then tmux bind-key -n C-g switch-client -t \"$panel\"; fi; if [ \"$panel\" != %s ]; then tmux detach-client -s %s 2>/dev/null || true; fi; tmux switch-client -t %s", q, q, q)
	}
	return fmt.Sprintf("tmux bind-key -n C-g detach-client; tmux detach-client -s %s 2>/dev/null || true; tmux attach-session -t %s", q, q)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func (m *model) killSelectedCmd() tea.Cmd {
	s, ok := m.currentSession()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
		defer cancel()
		if err := tmuxradio.Kill(ctx, s.cfg.Name); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{}
	}
}

func (m *model) killWorkspaceCmd() tea.Cmd {
	ws, ok := m.currentWorkspace()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
		defer cancel()
		for _, s := range ws.sessions {
			if err := tmuxradio.Kill(ctx, s.cfg.Name); err != nil {
				return actionMsg{err: err}
			}
		}
		return actionMsg{}
	}
}

func (m *model) currentWorkspace() (workspaceState, bool) {
	if len(m.workspaces) == 0 || m.workspaceCursor < 0 || m.workspaceCursor >= len(m.workspaces) {
		return workspaceState{}, false
	}
	return m.workspaces[m.workspaceCursor], true
}

func (m *model) currentSessions() []sessionState {
	ws, ok := m.currentWorkspace()
	if !ok {
		return nil
	}
	return ws.sessions
}

func (m *model) currentSession() (sessionState, bool) {
	sessions := m.currentSessions()
	if len(sessions) == 0 || m.sessionCursor < 0 || m.sessionCursor >= len(sessions) {
		return sessionState{}, false
	}
	return sessions[m.sessionCursor], true
}

func progressText(done, total int, label string) string {
	if total <= 0 {
		return label
	}
	width := 16
	filled := width * done / total
	return fmt.Sprintf("%s %s %d/%d", label, strings.Repeat("█", filled)+strings.Repeat("░", width-filled), done, total)
}

func (m *model) View() string {
	m.targets = nil
	if m.width == 0 {
		return "loading\n"
	}
	lines := []string{m.header(), m.navBar()}
	switch m.tab {
	case tabWorkspaces:
		lines = append(lines, m.workspacesView())
	case tabMessages:
		lines = append(lines, m.messagesView())
	case tabLogs:
		lines = append(lines, m.logsView())
	case tabConfig:
		lines = append(lines, m.configView())
	}
	lines = append(lines, m.footer())
	return m.fitToScreen(strings.Join(lines, "\n"))
}

func (m *model) header() string {
	titleLines := logoLines()
	if m.isMobile() && m.focus > 0 {
		back := activeTab.Render("‹ Back")
		x := lipgloss.Width(titleLines[0]) + 1
		m.addTarget("back", 0, x, x+lipgloss.Width(back), 0, 0)
		titleLines[0] += " " + back
	}
	right := m.statusLine()
	if lipgloss.Width(titleLines[0])+lipgloss.Width(right)+2 <= m.width {
		gap := m.width - lipgloss.Width(titleLines[0]) - lipgloss.Width(right)
		if gap < 1 {
			gap = 1
		}
		titleLines[0] += strings.Repeat(" ", gap) + right
	} else if lipgloss.Width(right) <= m.width {
		titleLines = append(titleLines, strings.Repeat(" ", max(0, m.width-lipgloss.Width(right)))+right)
	}
	return strings.Join(titleLines, "\n")
}

func (m *model) navBar() string {
	names := []string{"Workspaces", "Messages", "Logs", "Config"}
	parts := make([]string, 0, len(names))
	x := 0
	y := m.navY()
	for i, name := range names {
		var rendered string
		if i == m.tab {
			rendered = activeTab.Render(name)
		} else {
			rendered = tabStyle.Render(name)
		}
		m.addTarget("tab", i, x, x+lipgloss.Width(rendered), y, y)
		parts = append(parts, rendered)
		x += lipgloss.Width(rendered) + 1
	}
	if m.confirm != "" {
		return crop(strings.Join(parts, " ")+"  "+warnStyle.Render(m.status)+"  "+mutedStyle.Render("y confirm  n cancel"), m.width)
	}
	return crop(strings.Join(parts, " "), m.width)
}

func (m *model) workspacesView() string {
	contentH := m.contentHeight()
	if m.isMobile() {
		if m.focus == 0 {
			return m.mobileWorkspaces()
		}
		return m.mobileSessions()
	}
	leftW := max(28, m.width/3)
	rightW := m.width - leftW - 3
	detailH := min(7, max(4, contentH*20/100))
	listH := max(3, contentH-detailH-1)
	y := m.contentY()
	left := m.workspaceList(leftW, listH, 0, y)
	right := m.sessionList(rightW, listH, leftW+1, y)
	detail := m.detailBox(m.width-2, detailH)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right) + "\n" + detail
}

func (m *model) mobileWorkspaces() string {
	return m.workspaceList(m.width-2, m.contentHeight(), 0, m.contentY())
}

func (m *model) mobileSessions() string {
	contentH := m.contentHeight()
	detailH := min(7, max(4, contentH*20/100))
	listH := max(3, contentH-detailH-1)
	return m.sessionList(m.width-2, listH, 0, m.contentY()) + "\n" + m.detailBox(m.width-2, detailH)
}

func (m *model) workspaceList(width, height, x, y int) string {
	var lines []string
	for i, ws := range m.workspaces {
		m.addTarget("workspace", i, x+1, x+width-2, y+1+i, y+1+i)
		nameW := max(8, width-14)
		name := padPlain(cropPlain(ws.cfg.Name, nameW), nameW)
		line := fmt.Sprintf("%s %2d/%-2d %2d",
			colorize(name, ws.cfg.Color),
			ws.running,
			len(ws.sessions),
			ws.unread,
		)
		if i == m.workspaceCursor {
			line = selectedRow(fmt.Sprintf("%s %2d/%-2d %2d", name, ws.running, len(ws.sessions), ws.unread), width-4)
		}
		lines = append(lines, line)
	}
	return renderBox("Workspaces", width, height, m.tab == tabWorkspaces && m.focus == 0, lines)
}

func (m *model) sessionList(width, height, x, y int) string {
	var lines []string
	nameW, typeW, statusW, countW, activityW := sessionColumns(width)
	for i, s := range m.currentSessions() {
		m.addTarget("session", i, x+1, x+width-2, y+1+i, y+1+i)
		status := "stopped"
		if s.alive {
			status = "alive"
		}
		name := padPlain(cropPlain(s.cfg.Name, nameW), nameW)
		kind := agentType(s.cfg)
		count := fmt.Sprintf("%d", s.unread)
		line := fmt.Sprintf("%s %s %s %s %s",
			colorize(name, s.cfg.Color),
			padPlain(kind, typeW),
			padPlain(status, statusW),
			padPlain(count, countW),
			activityIndicator(s, m.frame, activityW),
		)
		if i == m.sessionCursor {
			line = selectedRow(fmt.Sprintf("%s %s %s %s %s", name, padPlain(kind, typeW), padPlain(status, statusW), padPlain(count, countW), activityPlain(s, m.frame, activityW)), width-4)
		}
		lines = append(lines, line)
	}
	return renderBox("Sessions", width, height, m.tab == tabWorkspaces && m.focus == 1, lines)
}

func (m *model) detailBox(width, height int) string {
	var lines []string
	s, ok := m.currentSession()
	if !ok {
		return renderBox("Details", width, height, false, []string{mutedStyle.Render("No session selected.")})
	}
	lines = append(lines,
		"Name: "+s.cfg.Name,
		"Path: "+cropPlain(s.cfg.Path, width-12),
		"Command: "+cropPlain(s.cfg.Command, width-15),
		"Agent ID: "+s.cfg.AgentID,
	)
	if len(s.cfg.Tags) > 0 {
		lines = append(lines, "Tags: "+strings.Join(s.cfg.Tags, ", "))
	}
	if !s.pathOK {
		lines = append(lines, badStyle.Render("Path missing"))
	}
	if s.latest != "" {
		lines = append(lines, "Latest: "+cropPlain(s.latest, width-12))
	}
	return renderBox("Details", width, height, false, lines)
}

func (m *model) messagesView() string {
	var lines []string
	for i, msg := range m.recent {
		m.addTarget("message", i, 1, m.width-3, m.contentY()+1+i, m.contentY()+1+i)
		prefix := " "
		if i == m.messageCursor {
			prefix = "> "
		}
		line := messageLine(prefix, msg, m.width-6)
		if i == m.messageCursor {
			line = selectStyle.Width(m.width - 6).Render(line)
		}
		lines = append(lines, line)
	}
	return renderBox("Recent Messages", m.width-2, m.contentHeight(), true, lines)
}

func (m *model) logsView() string {
	if m.isMobile() {
		if m.focus == 0 {
			return m.mobileWorkspaces()
		}
		return m.sessionHistoryView(m.width-2, m.contentHeight(), 0, m.contentY())
	}
	leftW := max(42, m.width*45/100)
	rightW := m.width - leftW - 3
	y := m.contentY()
	left := m.workspaceTree(leftW, m.contentHeight(), 0, y)
	right := m.sessionHistoryView(rightW, m.contentHeight(), leftW+1, y)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (m *model) configView() string {
	var lines []string
	lines = append(lines, "Source: "+m.configPath)
	lines = append(lines, fmt.Sprintf("Workspaces: %d", len(m.cfg.Workspaces)))
	for _, ws := range m.cfg.Workspaces {
		lines = append(lines, fmt.Sprintf("%s (%d sessions)", ws.Name, len(ws.Sessions)))
		for _, s := range ws.Sessions {
			mark := "ok"
			if !pathExists(s.Path) {
				mark = "missing"
			}
			lines = append(lines, fmt.Sprintf("  %-8s %-24s %s", mark, cropPlain(s.Name, 24), cropPlain(s.Path, m.width-42)))
		}
	}
	return renderBox("Config", m.width-2, m.contentHeight(), true, lines)
}

func (m *model) footer() string {
	help := keyHint("enter", "open") + "  " + keyHint("s", "start") + "  " + keyHint("S", "start all") + "  " + keyHint("k", "kill") + "  " + keyHint("K", "kill workspace") + "  " + keyHint("ctrl+g", "back") + "  " + keyHint("tab", "tabs") + "  " + keyHint("q", "quit")
	if m.isMobile() {
		help = keyHint("enter", "open") + "  " + keyHint("s", "start") + "  " + keyHint("K", "kill ws") + "  " + keyHint("ctrl+g", "back") + "  " + keyHint("tab", "tabs") + "  " + keyHint("q", "quit")
	}
	if m.confirm != "" {
		help = warnStyle.Render(m.status) + "  y confirm  n cancel"
	} else if m.status != "" {
		help = mutedStyle.Render(m.status) + "  " + help
	}
	return crop(help, m.width)
}

func (m *model) workspaceTree(width, height, x, y int) string {
	var lines []string
	for i, ws := range m.workspaces {
		m.addTarget("workspace", i, x+1, x+width-2, y+1+len(lines), y+1+len(lines))
		nameW := max(8, width-12)
		line := fmt.Sprintf("%s %2d/%-2d", colorize(padPlain(cropPlain(ws.cfg.Name, nameW), nameW), ws.cfg.Color), ws.running, len(ws.sessions))
		lines = append(lines, line)
		for si, s := range ws.sessions {
			m.addSessionTarget(i, si, x+1, x+width-2, y+1+len(lines), y+1+len(lines))
			sessionNameW := max(14, width-24)
			name := padPlain(cropPlain(s.cfg.Name, sessionNameW), sessionNameW)
			kind := agentType(s.cfg)
			status := "stop"
			if s.alive {
				status = "live"
			}
			sline := fmt.Sprintf("  %s %s %s", colorize(name, s.cfg.Color), padPlain(kind, 8), padPlain(status, 4))
			if i == m.workspaceCursor && si == m.sessionCursor {
				sline = selectedRow(fmt.Sprintf("  %s %s %s", name, padPlain(kind, 8), padPlain(status, 4)), width-4)
			}
			lines = append(lines, sline)
		}
	}
	return renderBox("Workspaces", width, height, m.tab == tabLogs && m.focus == 0, lines)
}

func (m *model) sessionHistoryView(width, height, x, y int) string {
	_ = x
	_ = y
	s, ok := m.currentSession()
	if !ok {
		return renderBox("Message History", width, height, false, []string{mutedStyle.Render("No session selected.")})
	}
	lines := []string{fmt.Sprintf("%s  %s", s.cfg.Name, tagsInline(s.cfg.Tags))}
	if len(s.history) == 0 {
		lines = append(lines, mutedStyle.Render("No radio messages for this session yet."))
		return renderBox("Message History", width, height, m.tab == tabLogs && m.focus == 1, lines)
	}
	for _, msg := range s.history {
		lines = append(lines, messageLine(" ", msg, width-6))
	}
	return renderBox("Message History", width, height, m.tab == tabLogs && m.focus == 1, lines)
}

func (m *model) isMobile() bool {
	return m.width < 92
}

func (m *model) contentHeight() int {
	return max(2, m.height-m.headerHeight()-2)
}

func (m *model) headerHeight() int {
	if m.width > 0 && lipgloss.Width(logoLines()[0])+lipgloss.Width(m.statusLine())+2 > m.width {
		return len(logoLines()) + 1
	}
	return len(logoLines())
}

func (m *model) navY() int {
	return m.headerHeight()
}

func (m *model) contentY() int {
	return m.headerHeight() + 1
}

func (m *model) statusLine() string {
	return strings.Join([]string{badge("router", m.router), badge("tmux", m.tmuxOK), goodStyle.Render("db " + m.dbStatus)}, "  ")
}

func (m *model) fitToScreen(s string) string {
	if m.width <= 0 || m.height <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for i, line := range lines {
		lines[i] = crop(line, m.width)
	}
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m *model) addTarget(kind string, index, x1, x2, y1, y2 int) {
	m.targets = append(m.targets, clickTarget{
		kind:      kind,
		index:     index,
		workspace: -1,
		x1:        x1,
		x2:        x2,
		y1:        y1,
		y2:        y2,
	})
}

func (m *model) addSessionTarget(workspace, index, x1, x2, y1, y2 int) {
	m.targets = append(m.targets, clickTarget{
		kind:      "session",
		index:     index,
		workspace: workspace,
		x1:        x1,
		x2:        x2,
		y1:        y1,
		y2:        y2,
	})
}

func badge(label string, ok bool) string {
	if ok {
		return goodStyle.Render(label + " ok")
	}
	return warnStyle.Render(label + " off")
}

func logoLines() []string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_RADIO_LOGO_STYLE"))) {
	case "ansi-shadow", "shadow", "ansi":
		return ansiShadowLogoLines()
	case "rebel":
		return rebelLogoLines()
	case "dos-rebel", "dos":
		return dosRebelLogoLines()
	default:
		return ansiShadowLogoLines()
	}
}

func ansiShadowLogoLines() []string {
	agent := []string{
		" █████╗  ██████╗ ███████╗███╗   ██╗████████╗",
		"██╔══██╗██╔════╝ ██╔════╝████╗  ██║╚══██╔══╝",
		"███████║██║  ███╗█████╗  ██╔██╗ ██║   ██║   ",
		"██╔══██║██║   ██║██╔══╝  ██║╚██╗██║   ██║   ",
		"██║  ██║╚██████╔╝███████╗██║ ╚████║   ██║   ",
		"╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═══╝   ╚═╝   ",
	}
	radio := [][]string{
		{"██████╗", "  ", "█████╗", " ", "██████╗", " ", "██╗", " ", "██████╗ "},
		{"██╔══██╗", "", "██╔══██╗", "", "██╔══██╗", "", "██║", "", "██╔═══██╗"},
		{"██████╔╝", "", "███████║", "", "██║  ██║", "", "██║", "", "██║   ██║"},
		{"██╔══██╗", "", "██╔══██║", "", "██║  ██║", "", "██║", "", "██║   ██║"},
		{"██║  ██║", "", "██║  ██║", "", "██████╔╝", "", "██║", "", "╚██████╔╝"},
		{"╚═╝  ╚═╝", "", "╚═╝  ╚═╝", "", "╚═════╝ ", "", "╚═╝", " ", "╚═════╝ "},
	}
	return combineAnsiShadowLogo(agent, radio)
}

func rebelLogoLines() []string {
	agent := []string{
		"   █████████     █████████  ██████████ ██████   █████ ███████████",
		"  ███▒▒▒▒▒███   ███▒▒▒▒▒███▒▒███▒▒▒▒▒█▒▒██████ ▒▒███ ▒█▒▒▒███▒▒▒█",
		" ▒███    ▒███  ███     ▒▒▒  ▒███  █ ▒  ▒███▒███ ▒███ ▒   ▒███  ▒ ",
		" ▒███████████ ▒███          ▒██████    ▒███▒▒███▒███     ▒███    ",
		" ▒███▒▒▒▒▒███ ▒███    █████ ▒███▒▒█    ▒███ ▒▒██████     ▒███    ",
		" ▒███    ▒███ ▒▒███  ▒▒███  ▒███ ▒   █ ▒███  ▒▒█████     ▒███    ",
		" █████   █████ ▒▒█████████  ██████████ █████  ▒▒█████    █████   ",
		"▒▒▒▒▒   ▒▒▒▒▒   ▒▒▒▒▒▒▒▒▒  ▒▒▒▒▒▒▒▒▒▒ ▒▒▒▒▒    ▒▒▒▒▒    ▒▒▒▒▒    ",
	}
	radio := []string{
		"   ███████████     █████████   ██████████   █████    ███████   ",
		"  ▒▒███▒▒▒▒▒███   ███▒▒▒▒▒███ ▒▒███▒▒▒▒███ ▒▒███   ███▒▒▒▒▒███ ",
		"   ▒███    ▒███  ▒███    ▒███  ▒███   ▒▒███ ▒███  ███     ▒▒███",
		"   ▒██████████   ▒███████████  ▒███    ▒███ ▒███ ▒███      ▒███",
		"   ▒███▒▒▒▒▒███  ▒███▒▒▒▒▒███  ▒███    ▒███ ▒███ ▒███      ▒███",
		"   ▒███    ▒███  ▒███    ▒███  ▒███    ███  ▒███ ▒▒███     ███ ",
		"   █████   █████ █████   █████ ██████████   █████ ▒▒▒███████▒  ",
		"  ▒▒▒▒▒   ▒▒▒▒▒ ▒▒▒▒▒   ▒▒▒▒▒ ▒▒▒▒▒▒▒▒▒▒   ▒▒▒▒▒    ▒▒▒▒▒▒▒    ",
	}
	return combineLogo(agent, radio)
}

func dosRebelLogoLines() []string {
	return strings.Split(strings.ReplaceAll(strings.Join(rebelLogoLines(), "\n"), "▒", "░"), "\n")
}

func combineLogo(agent, radio []string) []string {
	out := make([]string, 0, len(agent))
	for i := range agent {
		left := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Render(agent[i])
		out = append(out, left+"  "+gradientRadio(radio[i]))
	}
	return out
}

func combineAnsiShadowLogo(agent []string, radio [][]string) []string {
	colors := logoGradientColors()
	out := make([]string, 0, len(agent))
	for i := range agent {
		left := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Render(agent[i])
		var right strings.Builder
		for letter := 0; letter < 5; letter++ {
			idx := letter * 2
			right.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colors[letter])).Render(radio[i][idx]))
			if idx+1 < len(radio[i]) && radio[i][idx+1] != "" {
				sep := radio[i][idx+1]
				right.WriteString(sep)
			}
		}
		out = append(out, left+"  "+right.String())
	}
	return out
}

func gradientRadio(s string) string {
	colors := logoGradientColors()
	runes := []rune(s)
	var b strings.Builder
	visible := 0
	for _, r := range runes {
		color := colors[min(len(colors)-1, visible*len(colors)/max(1, len(runes)))]
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color)).Render(string(r)))
		visible++
	}
	return b.String()
}

func logoGradientColors() []string {
	// Smooth pink -> coral -> orange in the xterm 256-color cube.
	return []string{"201", "207", "213", "209", "208"}
}

func button(label string, hover bool) string {
	bg := lipgloss.Color("238")
	fg := lipgloss.Color("15")
	if hover {
		bg = lipgloss.Color("36")
		fg = lipgloss.Color("15")
	}
	return lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Padding(0, 1).
		Render(label)
}

func keyHint(key, label string) string {
	box := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("201")).
		Padding(0, 1).
		Render(key)
	return box + " " + mutedStyle.Render(label)
}

func iconChip(v, color string) string {
	code := colorCode(color)
	if code == "" {
		code = "245"
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color(code)).
		Padding(0, 1).
		Render(icon(v))
}

func sessionColumns(width int) (nameW, typeW, statusW, countW, activityW int) {
	typeW = 9
	statusW = 8
	countW = 4
	activityW = 10
	nameW = max(10, width-4-typeW-statusW-countW-activityW-9)
	return
}

func activityIndicator(s sessionState, frame int, width int) string {
	if !s.alive {
		return mutedStyle.Render(padPlain("idle", width))
	}
	if s.active {
		return goodStyle.Render(padPlain(activityPulse(frame), width))
	}
	return mutedStyle.Render(padPlain("idle", width))
}

func activityPlain(s sessionState, frame int, width int) string {
	if !s.alive || !s.active {
		return padPlain("idle", width)
	}
	return padPlain(activityPulse(frame), width)
}

func activityPulse(frame int) string {
	frames := []string{
		"⠋ active",
		"⠙ active",
		"⠹ active",
		"⠸ active",
		"⠼ active",
		"⠴ active",
		"⠦ active",
		"⠧ active",
		"⠇ active",
		"⠏ active",
	}
	return frames[frame%len(frames)]
}

func agentType(s config.Session) string {
	haystack := strings.ToLower(s.Command + " " + s.Name + " " + s.AgentID)
	switch {
	case strings.Contains(haystack, "claude"):
		return "Claude"
	case strings.Contains(haystack, "codex"):
		return "Codex"
	case strings.Contains(haystack, "opencode"):
		return "OpenCode"
	case strings.Contains(haystack, "cursor"):
		return "Cursor"
	case strings.Contains(haystack, "agent-radio"):
		return "Radio"
	default:
		return "Shell"
	}
}

func selectedRow(line string, width int) string {
	return selectStyle.Render(padPlain(cropPlain(line, width), width))
}

func icon(v string) string {
	switch v {
	case "broadcast":
		return "●"
	case "server-process":
		return "▣"
	case "star-full":
		return "★"
	case "repo-forked":
		return "⑂"
	case "browser":
		return "◈"
	case "server":
		return "■"
	case "key":
		return "◆"
	case "comment-discussion":
		return "◇"
	case "":
		return "-"
	default:
		return cropPlain(v, 2)
	}
}

func tags(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return strings.Join(v, ", ")
}

func tagsInline(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return mutedStyle.Render("(" + strings.Join(v, ", ") + ")")
}

func colorize(s, color string) string {
	code := colorCode(color)
	if code == "" {
		return s
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(code)).Render(s)
}

func colorCode(color string) string {
	switch strings.ToLower(strings.TrimSpace(color)) {
	case "red":
		return "203"
	case "green":
		return "42"
	case "yellow":
		return "214"
	case "blue":
		return "39"
	case "magenta":
		return "201"
	case "cyan":
		return "45"
	case "white":
		return "15"
	case "gray", "grey":
		return "245"
	default:
		return ""
	}
}

func crop(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r)) > width-1 {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func cropPlain(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(r[:width-1]) + "…"
}

func padPlain(s string, width int) string {
	w := lipgloss.Width(s)
	if w > width {
		return cropPlain(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

func renderBox(title string, width, height int, focused bool, lines []string) string {
	width = max(8, width)
	height = max(3, height)
	contentW := width - 4
	border := "─"
	borderColor := lipgloss.Color("238")
	if focused {
		borderColor = lipgloss.Color("36")
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	titleText := " " + cropPlain(title, max(1, width-6)) + " "
	topFill := max(0, width-2-lipgloss.Width(titleText))
	top := "╭" + titleText + strings.Repeat(border, topFill) + "╮"
	out := []string{borderStyle.Render(top)}
	bodyH := height - 2
	for i := 0; i < bodyH; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		line = padPlain(line, contentW)
		out = append(out, borderStyle.Render("│")+" "+line+" "+borderStyle.Render("│"))
	}
	out = append(out, borderStyle.Render("╰"+strings.Repeat(border, width-2)+"╯"))
	return strings.Join(out, "\n")
}

func messageLine(prefix string, msg store.Message, width int) string {
	routeW := max(12, width-34)
	route := cropPlain(msg.From+" -> "+msg.To, routeW)
	bodyW := max(8, width-routeW-26)
	return fmt.Sprintf("%s#%-3d %-7s %-*s %-8s %s",
		prefix,
		msg.ID,
		msg.Kind,
		routeW,
		route,
		msg.Status,
		cropPlain(msg.Body, bodyW),
	)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func ConfigExists() bool {
	path, err := config.DefaultPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Clean(path))
	return err == nil
}
