package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/chihaya/citrus/internal/backend/uwuzu"
	"github.com/chihaya/citrus/internal/config"
	"github.com/chihaya/citrus/internal/core"
)

type screen int

const (
	screenLogin screen = iota
	screenWaitingAuth
	screenTimeline
	screenCompose
	screenNotifications
)

type tokenMsg struct {
	token  string
	userID string
	err    error
}
type timelineMsg struct {
	posts []core.Post
	err   error
}
type postedMsg struct {
	err error
}
type notificationMsg struct {
	items []core.Notification
	err   error
}

type model struct {
	screen screen

	serverInput  textinput.Model
	composeInput textinput.Model

	server  string
	session string
	authURL string

	client  *uwuzu.Client
	account config.Account
	cfg     config.Config

	posts  []core.Post
	cursor int

	notifications []core.Notification
	notifCursor   int

	replyTo string
	loading bool
	err     error

	width, height int
}

func NewModel() model {
	si := textinput.New()
	si.Placeholder = "dev.uwuzu.net"
	si.Focus()
	si.CharLimit = 256
	si.Width = 40

	ci := textinput.New()
	ci.Placeholder = "What's on your mind?"
	ci.CharLimit = 1024
	ci.Width = 60

	cfg, _ := config.Load()
	m := model{
		screen:       screenLogin,
		serverInput:  si,
		composeInput: ci,
		cfg:          cfg,
	}
	if len(cfg.Accounts) > 0 && cfg.Active < len(cfg.Accounts) {
		acc := cfg.Accounts[cfg.Active]
		m.account = acc
		m.client = uwuzu.New(acc.BaseURL, acc.Token)
		m.screen = screenTimeline
		m.loading = true
	}
	return m
}

func normalizeServer(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	return strings.TrimRight(s, "/")
}

func domainOf(server string) string {
	s := strings.TrimPrefix(server, "https://")
	s = strings.TrimPrefix(s, "http://")
	return strings.TrimRight(s, "/")
}

func (m model) Init() tea.Cmd {
	if m.client != nil {
		return loadTimelineCmd(m.client)
	}
	return textinput.Blink
}

func loadTimelineCmd(c *uwuzu.Client) tea.Cmd {
	return func() tea.Msg {
		posts, err := c.HomeTimeline(25, 0)
		return timelineMsg{posts: posts, err: err}
	}
}

func loadNotificationsCmd(c *uwuzu.Client) tea.Cmd {
	return func() tea.Msg {
		items, err := c.Notifications(25, 0)
		return notificationMsg{items: items, err: err}
	}
}

func fetchTokenCmd(server, session string) tea.Cmd {
	return func() tea.Msg {
		c := uwuzu.New(server, "")
		tr, err := c.GetToken(session)
		if err != nil {
			return tokenMsg{err: err}
		}
		return tokenMsg{token: tr.Token, userID: tr.UserID}
	}
}

func openBrowserCmd(u string) tea.Cmd {
	return func() tea.Msg {
		_ = exec.Command("xdg-open", u).Start()
		return nil
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tokenMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.screen = screenLogin
			return m, nil
		}
		m.account = config.Account{
			Backend: "uwuzu",
			BaseURL: m.server,
			UserID:  msg.userID,
			Token:   msg.token,
		}
		m.cfg.Accounts = append(m.cfg.Accounts, m.account)
		m.cfg.Active = len(m.cfg.Accounts) - 1
		_ = config.Save(m.cfg)
		m.client = uwuzu.New(m.server, msg.token)
		m.screen = screenTimeline
		m.loading = true
		return m, loadTimelineCmd(m.client)
	case timelineMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.posts = msg.posts
		m.cursor = 0
		m.err = nil
		return m, nil
	case postedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.screen = screenTimeline
		m.replyTo = ""
		m.composeInput.SetValue("")
		m.loading = true
		return m, loadTimelineCmd(m.client)
	case notificationMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.notifications = msg.items
		m.notifCursor = 0
		m.err = nil
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	var cmd tea.Cmd
	switch m.screen {
	case screenLogin:
		m.serverInput, cmd = m.serverInput.Update(msg)
	case screenCompose:
		m.composeInput, cmd = m.composeInput.Update(msg)
	}
	return m, cmd
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenLogin:
		if msg.String() == "enter" {
			m.server = normalizeServer(m.serverInput.Value())
			if m.server == "" {
				return m, nil
			}
			m.session = uwuzu.NewSession()
			scopes := "read:me,read:ueuse,read:users,write:ueuse,write:follow,write:favorite,read:notifications,write:notifications"
			m.authURL = uwuzu.AuthURL(domainOf(m.server), m.session, "Citrus", scopes, "", "", "")
			m.screen = screenWaitingAuth
			m.err = nil
			return m, openBrowserCmd(m.authURL)
		}
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	case screenWaitingAuth:
		if msg.String() == "enter" {
			m.loading = true
			m.err = nil
			return m, fetchTokenCmd(m.server, m.session)
		}
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	case screenTimeline:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.posts)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "r":
			m.loading = true
			m.err = nil
			return m, loadTimelineCmd(m.client)
		case "n":
			m.screen = screenNotifications
			m.loading = true
			m.err = nil
			return m, loadNotificationsCmd(m.client)
		case "f":
			if len(m.posts) > 0 {
				id := m.posts[m.cursor].ID
				m.loading = true
				client := m.client
				return m, func() tea.Msg {
					_ = client.Favorite(id)
					posts, err := client.HomeTimeline(25, 0)
					return timelineMsg{posts: posts, err: err}
				}
			}
		case "d":
			if len(m.posts) > 0 {
				id := m.posts[m.cursor].ID
				m.loading = true
				client := m.client
				return m, func() tea.Msg {
					_ = client.DeletePost(id)
					posts, err := client.HomeTimeline(25, 0)
					return timelineMsg{posts: posts, err: err}
				}
			}
		case "p":
			m.screen = screenCompose
			m.replyTo = ""
			m.composeInput.Focus()
			return m, textinput.Blink
		case "enter":
			if len(m.posts) > 0 {
				m.screen = screenCompose
				m.replyTo = m.posts[m.cursor].ID
				m.composeInput.Focus()
				return m, textinput.Blink
			}
		}
	case screenNotifications:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.screen = screenTimeline
		case "j", "down":
			if m.notifCursor < len(m.notifications)-1 {
				m.notifCursor++
			}
		case "k", "up":
			if m.notifCursor > 0 {
				m.notifCursor--
			}
		case "r":
			m.loading = true
			m.err = nil
			return m, loadNotificationsCmd(m.client)
		case "enter":
			m.loading = true
			client := m.client
			return m, func() tea.Msg {
				_ = client.MarkNotificationsRead()
				items, err := client.Notifications(25, 0)
				return notificationMsg{items: items, err: err}
			}
		}
	case screenCompose:
		switch msg.String() {
		case "esc":
			m.screen = screenTimeline
			m.composeInput.Blur()
			m.replyTo = ""
			m.composeInput.SetValue("")
			return m, nil
		case "enter":
			text := strings.TrimSpace(m.composeInput.Value())
			if text == "" {
				return m, nil
			}
			m.loading = true
			m.err = nil
			client := m.client
			replyTo := m.replyTo
			return m, func() tea.Msg {
				_, err := client.CreatePost(text, replyTo, "", false)
				return postedMsg{err: err}
			}
		}
	}
	var cmd tea.Cmd
	switch m.screen {
	case screenLogin:
		m.serverInput, cmd = m.serverInput.Update(msg)
	case screenCompose:
		m.composeInput, cmd = m.composeInput.Update(msg)
	}
	return m, cmd
}

func (m model) View() string {
	switch m.screen {
	case screenLogin:
		return "Citrus login\n\nServer domain:\n" + m.serverInput.View() + "\n\nEnter to continue, q to quit\n" + errView(m.err)
	case screenWaitingAuth:
		return "Authorize in browser:\n" + m.authURL + "\n\nAfter approving, press Enter to fetch token.\n" + errView(m.err)
	case screenTimeline:
		if m.loading {
			return "Loading timeline...\n"
		}
		if len(m.posts) == 0 {
			return "No posts.\n(p compose, r refresh, q quit)\n" + errView(m.err)
		}
		var b strings.Builder
		b.WriteString("Timeline (j/k move, Enter reply, p compose, f favorite, d delete, n notifications, r refresh, q quit)\n\n")
		for i, p := range m.posts {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}
			fmt.Fprintf(&b, "%s%s: %s  [%s]\n", cursor, p.Author.Name, strings.ReplaceAll(p.Text, "\n", " "), p.CreatedAt.Format("01-02 15:04"))
		}
		if m.err != nil {
			b.WriteString("\n" + m.err.Error())
		}
		return b.String()
	case screenNotifications:
		if m.loading {
			return "Loading notifications...\n"
		}
		if len(m.notifications) == 0 {
			return "No notifications.\n(Esc back, r refresh)\n" + errView(m.err)
		}
		var b strings.Builder
		b.WriteString("Notifications (j/k move, Enter mark read, Esc back)\n\n")
		for i, n := range m.notifications {
			cursor := "  "
			if i == m.notifCursor {
				cursor = "> "
			}
			fmt.Fprintf(&b, "%s%s: %s - %s\n", cursor, n.From.Name, n.Title, n.Text)
		}
		if m.err != nil {
			b.WriteString("\n" + m.err.Error())
		}
		return b.String()
	case screenCompose:
		title := "Compose"
		if m.replyTo != "" {
			title = "Reply to " + m.replyTo
		}
		return title + "\n\n" + m.composeInput.View() + "\n\nEnter to post, Esc to cancel\n" + errView(m.err)
	}
	return ""
}

func errView(err error) string {
	if err == nil {
		return ""
	}
	return "\nError: " + err.Error() + "\n"
}

func Run() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
