package gui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/mappu/miqt/qt6"

	"github.com/chihaya/citrus/internal/backend/uwuzu"
	"github.com/chihaya/citrus/internal/config"
)

func Run() error {
	qt6.NewQApplication([]string{})

	window := qt6.NewQMainWindow2()
	window.SetWindowTitle("Citrus")
	window.Resize(900, 700)

	central := qt6.NewQWidget2()
	layout := qt6.NewQVBoxLayout2()

	serverEdit := qt6.NewQLineEdit3("dev.uwuzu.net")
	layout.AddWidget(serverEdit.QWidget)

	statusLabel := qt6.NewQLabel3("Not logged in")
	layout.AddWidget(statusLabel.QWidget)

	authBtn := qt6.NewQPushButton3("Authorize in browser")
	layout.AddWidget(authBtn.QWidget)

	fetchBtn := qt6.NewQPushButton3("Fetch token")
	layout.AddWidget(fetchBtn.QWidget)

	timeline := qt6.NewQListWidget2()
	layout.AddWidget(timeline.QWidget)

	composeEdit := qt6.NewQLineEdit2()
	composeEdit.SetPlaceholderText("What's on your mind?")
	layout.AddWidget(composeEdit.QWidget)

	postBtn := qt6.NewQPushButton3("Post")
	layout.AddWidget(postBtn.QWidget)

	central.SetLayout(layout.QLayout)
	window.SetCentralWidget(central)

	var client *uwuzu.Client
	var session string
	var server string

	authBtn.OnClicked(func() {
		server = normalizeServerGUI(serverEdit.Text())
		if server == "" {
			statusLabel.SetText("Server required")
			return
		}
		session = uwuzu.NewSession()
		scopes := "read:me,read:ueuse,read:users,write:ueuse,write:follow,write:favorite,read:notifications,write:notifications"
		url := uwuzu.AuthURL(domainOfGUI(server), session, "Citrus", scopes, "", "", "")
		statusLabel.SetText("Opening browser. Approve, then click Fetch token.")
		_ = exec.Command("xdg-open", url).Start()
	})

	fetchBtn.OnClicked(func() {
		if server == "" || session == "" {
			statusLabel.SetText("Authorize first")
			return
		}
		c := uwuzu.New(server, "")
		tr, err := c.GetToken(session)
		if err != nil {
			statusLabel.SetText("Token fetch failed: " + err.Error())
			return
		}
		client = uwuzu.New(server, tr.Token)
		acc := config.Account{Backend: "uwuzu", BaseURL: server, UserID: tr.UserID, Token: tr.Token}
		cfg, _ := config.Load()
		cfg.Accounts = append(cfg.Accounts, acc)
		cfg.Active = len(cfg.Accounts) - 1
		_ = config.Save(cfg)
		statusLabel.SetText("Logged in as " + tr.UserID)
		loadTimelineGUI(client, timeline)
	})

	postBtn.OnClicked(func() {
		if client == nil {
			statusLabel.SetText("Not logged in")
			return
		}
		text := strings.TrimSpace(composeEdit.Text())
		if text == "" {
			return
		}
		_, err := client.CreatePost(text, "", "", false)
		if err != nil {
			statusLabel.SetText("Post failed: " + err.Error())
			return
		}
		composeEdit.Clear()
		loadTimelineGUI(client, timeline)
	})

	// load existing account
	cfg, _ := config.Load()
	if len(cfg.Accounts) > 0 && cfg.Active < len(cfg.Accounts) {
		acc := cfg.Accounts[cfg.Active]
		client = uwuzu.New(acc.BaseURL, acc.Token)
		server = acc.BaseURL
		serverEdit.SetText(acc.BaseURL)
		statusLabel.SetText("Logged in as " + acc.UserID)
		loadTimelineGUI(client, timeline)
	}

	window.Show()
	qt6.QApplication_Exec()
	return nil
}

func loadTimelineGUI(client *uwuzu.Client, list *qt6.QListWidget) {
	list.Clear()
	posts, err := client.HomeTimeline(25, 0)
	if err != nil {
		list.AddItem("Error: " + err.Error())
		return
	}
	for _, p := range posts {
		list.AddItem(fmt.Sprintf("%s: %s", p.Author.Name, strings.ReplaceAll(p.Text, "\n", " ")))
	}
}

func normalizeServerGUI(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	return strings.TrimRight(s, "/")
}

func domainOfGUI(server string) string {
	s := strings.TrimPrefix(server, "https://")
	s = strings.TrimPrefix(s, "http://")
	return strings.TrimRight(s, "/")
}
