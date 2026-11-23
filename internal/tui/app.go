package tui

import (
	"bufio"
	"bytes"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/chihaya/citrus/internal/backend/uwuzu"
	"github.com/chihaya/citrus/internal/config"
	"github.com/chihaya/citrus/internal/core"
)

var (
	app              *tview.Application
	pages            *tview.Pages
	status           *tview.TextView
	list             *tview.List
	notifList        *tview.List
	composeArea      *tview.TextArea
	composePostBtn   *tview.Button
	composeCancelBtn *tview.Button
	detailText       *tview.TextView
	detailImages     *tview.Flex
	detailFlex       *tview.Flex
	serverInput      *tview.InputField
	waitingText      *tview.TextView

	client  *uwuzu.Client
	cfg     config.Config
	account config.Account
	server  string
	session string
	authURL string
	posts   []core.Post
	page    int
	loading bool
	replyTo string
	notifs  []core.Notification
)

func Run() error {
	cfg, _ = config.Load()
	if len(cfg.Accounts) > 0 && cfg.Active < len(cfg.Accounts) {
		acc := cfg.Accounts[cfg.Active]
		account = acc
		client = uwuzu.New(acc.BaseURL, acc.Token)
		server = acc.BaseURL
	}

	app = tview.NewApplication()
	pages = tview.NewPages()

	status = tview.NewTextView().SetDynamicColors(true)
	status.SetBorder(true).SetTitle("Status")

	buildLoginPage()
	buildWaitingPage()
	buildTimelinePage()
	buildComposePage()
	buildDetailPage()
	buildNotificationsPage()

	if client != nil {
		pages.SwitchToPage("timeline")
		go loadTimeline()
	} else {
		pages.SwitchToPage("login")
	}

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			front, _ := pages.GetFrontPage()
			if front == "compose" || front == "detail" {
				composeArea.SetText("", true)
				replyTo = ""
				pages.SwitchToPage("timeline")
				return nil
			}
			if front == "notifications" {
				pages.SwitchToPage("timeline")
				return nil
			}
		}
		if event.Key() == tcell.KeyEnter && event.Modifiers()&tcell.ModCtrl != 0 {
			front, _ := pages.GetFrontPage()
			if front == "compose" {
				submitCompose()
				return nil
			}
		}
		if event.Key() == tcell.KeyTab {
			front, _ := pages.GetFrontPage()
			if front == "compose" {
				cur := app.GetFocus()
				switch cur {
				case composeArea:
					app.SetFocus(composePostBtn)
				case composePostBtn:
					app.SetFocus(composeCancelBtn)
				default:
					app.SetFocus(composeArea)
				}
				return nil
			}
		}
		if event.Key() == tcell.KeyBacktab {
			front, _ := pages.GetFrontPage()
			if front == "compose" {
				cur := app.GetFocus()
				switch cur {
				case composeCancelBtn:
					app.SetFocus(composePostBtn)
				case composePostBtn:
					app.SetFocus(composeArea)
				default:
					app.SetFocus(composeCancelBtn)
				}
				return nil
			}
		}
		if event.Key() == tcell.KeyF2 {
			front, _ := pages.GetFrontPage()
			if front == "compose" {
				submitCompose()
				return nil
			}
		}
		if event.Rune() == 's' && event.Modifiers()&tcell.ModCtrl != 0 {
			front, _ := pages.GetFrontPage()
			if front == "compose" {
				submitCompose()
				return nil
			}
		}
		if _, ok := app.GetFocus().(*tview.InputField); ok {
			return event
		}
		if _, ok := app.GetFocus().(*tview.TextArea); ok {
			return event
		}
		switch event.Rune() {
		case 'q':
			app.Stop()
			return nil
		case 'r':
			if client != nil && len(posts) > 0 {
				idx := list.GetCurrentItem()
				if idx >= 0 && idx < len(posts) {
					replyTo = posts[idx].ID
					composeArea.SetText("", true)
					pages.SwitchToPage("compose")
				}
			}
			return nil
		case 'R':
			if client != nil {
				loadTimeline()
			}
			return nil
		case 'n':
			if client != nil {
				loadNotifications()
				pages.SwitchToPage("notifications")
			}
			return nil
		case 'c':
			if client != nil {
				replyTo = ""
				composeArea.SetText("", true)
				pages.SwitchToPage("compose")
			}
			return nil
		}
		if event.Key() == tcell.KeyEnter {
			// handled by list selected func
		}
		return event
	})

	list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(posts) && client != nil {
			showDetail(posts[index])
		}
	})

	app.SetRoot(pages, true)
	return app.Run()
}

func buildLoginPage() {
	form := tview.NewForm()
	serverInput = tview.NewInputField().SetLabel("Server domain: ").SetFieldWidth(40)
	serverInput.SetText("dev.uwuzu.net")
	form.AddFormItem(serverInput)
	form.AddButton("Authorize", func() {
		server = normalizeServer(serverInput.GetText())
		if server == "" {
			status.SetText("Server required")
			return
		}
		session = uwuzu.NewSession()
		scopes := "read:me,read:ueuse,read:users,write:ueuse,write:follow,write:favorite,read:notifications,write:notifications"
		authURL = uwuzu.AuthURL(domainOf(server), session, "Citrus", scopes, "", "", "")
		status.SetText("Opening browser. Approve, then fetch token.")
		_ = exec.Command("xdg-open", authURL).Start()
		pages.SwitchToPage("waiting")
		waitingText.SetText(authURL)
	})
	form.SetBorder(true).SetTitle("Citrus Login")
	pages.AddPage("login", centered(form, 60, 12), true, false)
}

func buildWaitingPage() {
	waitingText = tview.NewTextView().SetDynamicColors(true)
	waitingText.SetBorder(true).SetTitle("Authorize in browser")
	fetchBtn := tview.NewButton("Fetch token").SetSelectedFunc(func() {
		go fetchToken()
	})
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(waitingText, 0, 1, false)
	flex.AddItem(fetchBtn, 3, 0, false)
	flex.AddItem(status, 5, 0, false)
	pages.AddPage("waiting", flex, true, false)
}

func fetchToken() {
	if server == "" || session == "" {
		app.QueueUpdateDraw(func() { status.SetText("Authorize first") })
		return
	}
	c := uwuzu.New(server, "")
	tr, err := c.GetToken(session)
	if err != nil {
		app.QueueUpdateDraw(func() { status.SetText("Token fetch failed: " + err.Error()) })
		return
	}
	client = uwuzu.New(server, tr.Token)
	account = config.Account{Backend: "uwuzu", BaseURL: server, UserID: tr.UserID, Token: tr.Token}
	cfg.Accounts = append(cfg.Accounts, account)
	cfg.Active = len(cfg.Accounts) - 1
	_ = config.Save(cfg)
	app.QueueUpdateDraw(func() {
		status.SetText("Logged in as " + tr.UserID)
		pages.SwitchToPage("timeline")
		loadTimeline()
	})
}

func buildTimelinePage() {
	list = tview.NewList().ShowSecondaryText(true)
	list.SetBorder(true).SetTitle("Timeline")
	list.SetWrapAround(false)
	list.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if len(posts) == 0 || loading {
			return
		}
		if index == len(posts)-1 {
			loadMore()
		}
	})
	btns := tview.NewFlex().SetDirection(tview.FlexColumn)
	for _, b := range []struct {
		label string
		fn    func()
	}{
		{"Compose", func() { replyTo = ""; composeArea.SetText("", true); pages.SwitchToPage("compose") }},
		{"Favorite", func() { favoriteSelected() }},
		{"Delete", func() { deleteSelected() }},
		{"Notifications", func() { loadNotifications(); pages.SwitchToPage("notifications") }},
		{"Refresh", func() { loadTimeline() }},
		{"Quit", func() { app.Stop() }},
	} {
		btn := tview.NewButton(b.label).SetSelectedFunc(b.fn)
		btns.AddItem(btn, 0, 1, false)
	}
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(list, 0, 1, true)
	flex.AddItem(btns, 3, 0, false)
	flex.AddItem(status, 5, 0, false)
	pages.AddPage("timeline", flex, true, false)
}

func refreshTimelineList() {
	list.Clear()
	for _, p := range posts {
		list.AddItem(p.Author.Name+": "+strings.ReplaceAll(p.Text, "\n", " "), p.CreatedAt.Format("01-02 15:04"), 0, nil)
	}
}

func loadTimeline() {
	if client == nil {
		return
	}
	loading = true
	status.SetText("Loading timeline...")
	go func() {
		ps, err := client.HomeTimeline(25, 0)
		app.QueueUpdateDraw(func() {
			loading = false
			if err != nil {
				status.SetText("Timeline error: " + err.Error())
				return
			}
			posts = ps
			page = 2
			refreshTimelineList()
			status.SetText(fmt.Sprintf("Loaded %d posts", len(posts)))
		})
	}()
}

func loadMore() {
	if client == nil || loading {
		return
	}
	loading = true
	status.SetText("Loading more...")
	go func() {
		ps, err := client.HomeTimeline(25, page)
		app.QueueUpdateDraw(func() {
			if err != nil {
				loading = false
				status.SetText("Load more error: " + err.Error())
				return
			}
			oldCount := len(posts)
			seen := make(map[string]bool)
			for _, p := range posts {
				seen[p.ID] = true
			}
			for _, p := range ps {
				if !seen[p.ID] {
					posts = append(posts, p)
					seen[p.ID] = true
				}
			}
			if len(ps) > 0 {
				page++
			}
			refreshTimelineList()
			if oldCount < len(posts) {
				list.SetCurrentItem(oldCount)
			}
			loading = false
			status.SetText(fmt.Sprintf("Loaded %d posts", len(posts)))
		})
	}()
}

func favoriteSelected() {
	if client == nil || len(posts) == 0 {
		return
	}
	idx := list.GetCurrentItem()
	if idx < 0 || idx >= len(posts) {
		return
	}
	id := posts[idx].ID
	status.SetText("Favoriting...")
	go func() {
		err := client.Favorite(id)
		app.QueueUpdateDraw(func() {
			if err != nil {
				status.SetText("Favorite error: " + err.Error())
				return
			}
			status.SetText("Favorited")
		})
	}()
}

func deleteSelected() {
	if client == nil || len(posts) == 0 {
		return
	}
	idx := list.GetCurrentItem()
	if idx < 0 || idx >= len(posts) {
		return
	}
	id := posts[idx].ID
	status.SetText("Deleting...")
	go func() {
		err := client.DeletePost(id)
		app.QueueUpdateDraw(func() {
			if err != nil {
				status.SetText("Delete error: " + err.Error())
				return
			}
			loadTimeline()
		})
	}()
}

func submitCompose() {
	if client == nil {
		return
	}
	text := strings.TrimSpace(composeArea.GetText())
	if text == "" {
		return
	}
	status.SetText("Posting...")
	go func() {
		_, err := client.CreatePost(text, replyTo, "", false)
		app.QueueUpdateDraw(func() {
			if err != nil {
				status.SetText("Post failed: " + err.Error())
				return
			}
			composeArea.SetText("", true)
			replyTo = ""
			pages.SwitchToPage("timeline")
			loadTimeline()
		})
	}()
}

func buildComposePage() {
	composeArea = tview.NewTextArea()
	composeArea.SetBorder(true).SetTitle("Compose")
	composePostBtn = tview.NewButton("Post (F2/Ctrl+S)").SetSelectedFunc(submitCompose)
	composeCancelBtn = tview.NewButton("Cancel").SetSelectedFunc(func() {
		composeArea.SetText("", true)
		replyTo = ""
		pages.SwitchToPage("timeline")
	})
	btns := tview.NewFlex().SetDirection(tview.FlexColumn)
	btns.AddItem(composePostBtn, 0, 1, false)
	btns.AddItem(composeCancelBtn, 0, 1, false)
	help := tview.NewTextView().SetText("Tab: move focus  Enter: newline  F2/Ctrl+S: post  Esc: cancel")
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(composeArea, 0, 1, true)
	flex.AddItem(btns, 3, 0, false)
	flex.AddItem(help, 1, 0, false)
	flex.AddItem(status, 5, 0, false)
	pages.AddPage("compose", flex, true, false)
}

func buildDetailPage() {
	detailText = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	detailText.SetBorder(true).SetTitle("Post detail")
	detailImages = tview.NewFlex().SetDirection(tview.FlexColumn)
	backBtn := tview.NewButton("Back").SetSelectedFunc(func() {
		pages.SwitchToPage("timeline")
	})
	detailFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	detailFlex.AddItem(detailImages, 20, 0, false)
	detailFlex.AddItem(detailText, 0, 1, true)
	flex := detailFlex
	flex.AddItem(backBtn, 3, 0, false)
	flex.AddItem(status, 5, 0, false)
	pages.AddPage("detail", flex, true, false)
}

func showDetail(p core.Post) {
	sixelShown := false
	// Prioritize Sixel rendering when supported.
	if len(p.Images) > 0 {
		if _, err := exec.LookPath("img2sixel"); err == nil {
			func() {
				// Download to temp file.
				resp, err := http.Get(p.Images[0])
				if err != nil {
					app.QueueUpdateDraw(func() { status.SetText("Image download failed: " + err.Error()) })
					return
				}
				defer resp.Body.Close()
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					app.QueueUpdateDraw(func() { status.SetText("Image read failed: " + err.Error()) })
					return
				}
				im, _, err := image.Decode(bytes.NewReader(data))
				if err != nil {
					app.QueueUpdateDraw(func() { status.SetText("Image decode failed: " + err.Error()) })
					return
				}
				tmp, err := os.CreateTemp("", "citrus-img-*.png")
				if err != nil {
					app.QueueUpdateDraw(func() { status.SetText("Temp file failed: " + err.Error()) })
					return
				}
				if err := png.Encode(tmp, im); err != nil {
					tmp.Close()
					os.Remove(tmp.Name())
					app.QueueUpdateDraw(func() { status.SetText("PNG encode failed: " + err.Error()) })
					return
				}
				tmp.Close()
				path := tmp.Name()
				app.Suspend(func() {
					cmd := exec.Command("img2sixel", "-w", "400", path)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					_ = cmd.Run()
					fmt.Println()
					fmt.Println("Press Enter to return...")
					bufio.NewReader(os.Stdin).ReadString('\n')
				})
				os.Remove(path)
				sixelShown = true
			}()
		}
	}
	detailText.SetText(fmt.Sprintf("%s  %s\nFavs: %d\n\n%s", p.Author.Name, p.CreatedAt.Format("2006-01-02 15:04"), p.FavoriteCount, p.Text))
	if sixelShown {
		detailFlex.ResizeItem(detailImages, 0, 0)
	} else {
		detailFlex.ResizeItem(detailImages, 20, 0)
	}
	detailImages.Clear()
	if !sixelShown {
		for _, u := range p.Images {
			imgView := tview.NewImage()
			imgView.SetSize(20, 40)
			imgView.SetColors(tview.TrueColor)
			detailImages.AddItem(imgView, 40, 0, false)
			go func(url string, iv *tview.Image) {
				resp, err := http.Get(url)
				if err != nil {
					app.QueueUpdateDraw(func() { status.SetText("Image load failed: " + err.Error()) })
					return
				}
				defer resp.Body.Close()
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					app.QueueUpdateDraw(func() { status.SetText("Image read failed: " + err.Error()) })
					return
				}
				im, format, err := image.Decode(bytes.NewReader(data))
				if err != nil {
					app.QueueUpdateDraw(func() { status.SetText("Image decode failed: " + err.Error()) })
					return
				}
				app.QueueUpdateDraw(func() {
					iv.SetImage(im)
					status.SetText("Image loaded (" + format + ")")
				})
			}(u, imgView)
		}
	}
	pages.SwitchToPage("detail")
}

func buildNotificationsPage() {
	notifList = tview.NewList().ShowSecondaryText(true)
	notifList.SetBorder(true).SetTitle("Notifications")
	notifList.SetWrapAround(false)
	backBtn := tview.NewButton("Back").SetSelectedFunc(func() {
		pages.SwitchToPage("timeline")
	})
	markBtn := tview.NewButton("Mark read").SetSelectedFunc(func() {
		if client == nil {
			return
		}
		go func() {
			_ = client.MarkNotificationsRead()
			loadNotifications()
		}()
	})
	btns := tview.NewFlex().SetDirection(tview.FlexColumn)
	btns.AddItem(backBtn, 0, 1, false)
	btns.AddItem(markBtn, 0, 1, false)
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(notifList, 0, 1, true)
	flex.AddItem(btns, 3, 0, false)
	flex.AddItem(status, 5, 0, false)
	pages.AddPage("notifications", flex, true, false)
}

func loadNotifications() {
	if client == nil {
		return
	}
	go func() {
		items, err := client.Notifications(25, 0)
		app.QueueUpdateDraw(func() {
			if err != nil {
				status.SetText("Notifications error: " + err.Error())
				return
			}
			notifs = items
			notifList.Clear()
			for _, n := range notifs {
				notifList.AddItem(n.From.Name+": "+n.Title, n.Text, 0, nil)
			}
			status.SetText(fmt.Sprintf("Loaded %d notifications", len(notifs)))
		})
	}()
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

func centered(p tview.Primitive, width, height int) tview.Primitive {
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
	return flex
}
