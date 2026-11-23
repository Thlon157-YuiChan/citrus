package tui

import (
	"bytes"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/charmbracelet/x/ansi/sixel"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"
	"golang.org/x/image/draw"

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
	imageInput       *tview.InputField
	composePostBtn   *tview.Button
	composeCancelBtn *tview.Button
	detailText       *tview.TextView
	detailImages     *tview.Flex
	detailFlex       *tview.Flex
	detailReplies    *tview.List
	detailReplyPosts []core.Post
	sixelData        string
	sixelReady       bool
	sixelIndex       int
	sixelImages      []string
	userText         *tview.TextView
	userScroll       int
	userProfile      core.User
	detailAuthorID   string
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

var termWidth int
var cellW, cellH int

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
	buildUserPage()
	buildNotificationsPage()

	if client != nil {
		pages.SwitchToPage("timeline")
		app.Sync()
		go loadTimeline()
	} else {
		pages.SwitchToPage("login")
	}

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			front, _ := pages.GetFrontPage()
			if front == "compose" || front == "detail" {
				composeArea.SetText("", true)
				imageInput.SetText("")
				replyTo = ""
				pages.SwitchToPage("timeline")
		app.Sync()
				return nil
			}
			if front == "notifications" {
				pages.SwitchToPage("timeline")
		app.Sync()
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
					app.SetFocus(imageInput)
				case imageInput:
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
					app.SetFocus(imageInput)
				case imageInput:
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
		front, _ := pages.GetFrontPage()
		if front != "login" && front != "compose" {
			switch event.Key() {
			case tcell.KeyLeft:
				pages.SwitchToPage("timeline")
		app.Sync()
				return nil
			case tcell.KeyRight:
				if front == "timeline" && len(posts) > 0 {
					idx := list.GetCurrentItem()
					if idx >= 0 && idx < len(posts) {
						showDetail(posts[idx])
					}
					return nil
				}
				if front == "notifications" && len(notifs) > 0 {
					if client != nil {
						go func() {
							_ = client.MarkNotificationsRead()
							loadNotifications()
						}()
					}
					return nil
				}
				if front == "detail" && detailReplies != nil {
					idx := detailReplies.GetCurrentItem()
					if idx >= 0 && idx < len(detailReplyPosts) {
						showDetail(detailReplyPosts[idx])
					}
					return nil
				}
			}
		}
		if event.Key() == tcell.KeyF5 {
			if client != nil {
				loadTimeline()
			}
			return nil
		}
		if event.Rune() == 'r' && event.Modifiers()&tcell.ModCtrl != 0 {
			if client != nil {
				loadTimeline()
			}
			return nil
		}
		if event.Rune() == 'u' {
			front, _ := pages.GetFrontPage()
			if front == "timeline" && len(posts) > 0 {
				idx := list.GetCurrentItem()
				if idx >= 0 && idx < len(posts) {
					loadUser(posts[idx].Author.ID)
				}
				return nil
			}
			if front == "detail" && detailAuthorID != "" {
				loadUser(detailAuthorID)
			}
		}
		front2, _ := pages.GetFrontPage()
		if front2 == "user" {
			if event.Key() == tcell.KeyUp {
				if userScroll > 0 {
					userScroll--
				}
				userText.ScrollTo(userScroll, 0)
				return nil
			}
			if event.Key() == tcell.KeyDown {
				userScroll++
				userText.ScrollTo(userScroll, 0)
				return nil
			}
		}
		front, _ = pages.GetFrontPage()
		if front == "detail" {
			if event.Rune() == 'n' {
				loadSixelImage(sixelIndex + 1)
				return nil
			}
			if event.Rune() == 'p' {
				loadSixelImage(sixelIndex - 1)
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

	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		w, h := screen.Size()
		if w > 0 {
			termWidth = w
		}
		if tty, ok := screen.Tty(); ok {
			if ws, err := tty.WindowSize(); err == nil {
				if cw, ch := ws.CellDimensions(); cw > 0 && ch > 0 {
					cellW, cellH = cw, ch
				}
			}
		}
		_ = h
		return false
	})
	header := tview.NewTextView().SetText("Citrus on Terminal").SetDynamicColors(true)
	app.SetAfterDrawFunc(func(screen tcell.Screen) {
		if !sixelReady {
			return
		}
		front, _ := pages.GetFrontPage()
		screen.Show()
		if front != "detail" {
			return
		}
		tty, ok := screen.Tty()
		if !ok {
			return
		}
		x, y, w, h := detailImages.GetRect()
		if w > 0 && h > 0 {
			space := strings.Repeat(" ", w)
			for row := 0; row < h; row++ {
				fmt.Fprintf(tty, "\x1b[%d;%dH%s", y+1+row, x+1, space)
			}
		}
		fmt.Fprintf(tty, "\x1b[%d;%dH%s", y+1, x+1, sixelData)
	})
	root := tview.NewFlex().SetDirection(tview.FlexRow)
	root.AddItem(header, 1, 0, false)
	root.AddItem(pages, 0, 1, true)
	app.SetRoot(root, true)
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
	flex.AddItem(status, 3, 0, false)
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
		app.Sync()
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
	flex.AddItem(btns, 2, 0, false)
	flex.AddItem(status, 3, 0, false)
	pages.AddPage("timeline", flex, true, false)
}

func refreshTimelineList() {
	list.Clear()
	w := termWidth
	if w <= 0 {
		w = 80
	}
	mainWidth := w - 4
	if mainWidth < 20 {
		mainWidth = 20
	}
	for _, p := range posts {
		text := strings.ReplaceAll(p.Text, "\n", " ")
		text = runewidth.Truncate(text, mainWidth-runewidth.StringWidth(p.Author.Name)-2, "...")
		text = mdToTview(text)
		secondary := p.CreatedAt.Format("01-02 15:04")
		if p.ReplyCount > 0 {
			if len(p.Repliers) > 0 {
				secondary = fmt.Sprintf("%s  replies:%d (%s)", secondary, p.ReplyCount, strings.Join(p.Repliers, ", "))
			} else {
				secondary = fmt.Sprintf("%s  replies:%d", secondary, p.ReplyCount)
			}
		}
		secondary = runewidth.Truncate(secondary, w-2, "")
		list.AddItem(p.Author.Name+": "+text, secondary, 0, nil)
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
			go enrichRepliers()
		})
	}()
}

func enrichRepliers() {
	if client == nil {
		return
	}
	for i := range posts {
		if posts[i].ReplyCount <= 0 {
			continue
		}
		idx := i
		go func() {
			replies, err := client.Replies(posts[idx].ID, 10, 0)
			if err != nil {
				return
			}
			seen := map[string]bool{}
			var names []string
			for j, r := range replies {
				if j == 0 {
					continue
				}
				if !seen[r.Author.Name] {
					seen[r.Author.Name] = true
					names = append(names, r.Author.Name)
					if len(names) >= 3 {
						break
					}
				}
			}
			app.QueueUpdateDraw(func() {
				if idx < len(posts) {
					posts[idx].Repliers = names
					refreshTimelineList()
				}
			})
		}()
	}
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
	var imagePaths []string
	for _, p := range strings.Split(imageInput.GetText(), ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			imagePaths = append(imagePaths, p)
		}
	}
	status.SetText("Posting...")
	go func() {
		_, err := client.CreatePost(text, replyTo, "", false, imagePaths)
		app.QueueUpdateDraw(func() {
			if err != nil {
				status.SetText("Post failed: " + err.Error())
				return
			}
			composeArea.SetText("", true)
			replyTo = ""
			pages.SwitchToPage("timeline")
		app.Sync()
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
		imageInput.SetText("")
		replyTo = ""
		pages.SwitchToPage("timeline")
		app.Sync()
	})
	imageInput = tview.NewInputField().SetLabel("Images: ").SetFieldWidth(60)
	imageInput.SetPlaceholder("Image paths (comma separated)")
	btns := tview.NewFlex().SetDirection(tview.FlexColumn)
	btns.AddItem(composePostBtn, 0, 1, false)
	btns.AddItem(composeCancelBtn, 0, 1, false)
	help := tview.NewTextView().SetText("Tab: move focus  Enter: newline  F2/Ctrl+S: post  Esc: cancel")
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(composeArea, 0, 1, true)
	flex.AddItem(imageInput, 3, 0, false)
	flex.AddItem(btns, 2, 0, false)
	flex.AddItem(help, 1, 0, false)
	flex.AddItem(status, 3, 0, false)
	pages.AddPage("compose", flex, true, false)
}

func buildDetailPage() {
	detailText = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	detailText.SetBorder(true).SetTitle("Post detail")
	detailImages = tview.NewFlex().SetDirection(tview.FlexColumn)
	detailReplies = tview.NewList().ShowSecondaryText(true)
	detailReplies.SetBorder(true).SetTitle("Replies")
	detailReplies.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(detailReplyPosts) {
			showDetail(detailReplyPosts[index])
		}
	})
	backBtn := tview.NewButton("Back").SetSelectedFunc(func() {
		pages.SwitchToPage("timeline")
		app.Sync()
	})
	detailFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	detailFlex.AddItem(detailImages, 20, 0, false)
	detailFlex.AddItem(detailText, 0, 1, true)
	detailFlex.AddItem(detailReplies, 4, 0, true)
	flex := detailFlex
	help := tview.NewTextView().SetText("Enter: open reply detail  Esc: back")
	flex.AddItem(help, 1, 0, false)
	flex.AddItem(backBtn, 2, 0, false)
	flex.AddItem(status, 3, 0, false)
	pages.AddPage("detail", flex, true, false)
}

func loadSixelImage(idx int) {
	if len(sixelImages) == 0 {
		return
	}
	if idx < 0 {
		idx = len(sixelImages) - 1
	}
	if idx >= len(sixelImages) {
		idx = 0
	}
	sixelIndex = idx
	url := sixelImages[idx]
	go func() {
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
		im, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			app.QueueUpdateDraw(func() { status.SetText("Image decode failed: " + err.Error()) })
			return
		}
		maxW, maxH := cellW*(termWidth-4), cellH*20
		if maxW <= 0 { maxW = 400 }
		if maxH <= 0 { maxH = 320 }
		im = scaleImage(im, maxW, maxH)
		var buf bytes.Buffer
		buf.WriteString("\x1bPq")
		enc := sixel.Encoder{}
		if err := enc.Encode(&buf, im); err != nil {
			app.QueueUpdateDraw(func() { status.SetText("Sixel encode failed: " + err.Error()) })
			return
		}
		buf.WriteString("\x1b\\")
		app.QueueUpdateDraw(func() {
			sixelData = buf.String()
			sixelReady = true
			status.SetText(fmt.Sprintf("Image %d/%d (Sixel)", sixelIndex+1, len(sixelImages)))
		})
	}()
}

func scaleImage(img image.Image, maxW, maxH int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxW && h <= maxH {
		return img
	}
	scale := 1.0
	if w > maxW {
		scale = float64(maxW) / float64(w)
	}
	if float64(h)*scale > float64(maxH) {
		scale = float64(maxH) / float64(h)
	}
	nw := int(float64(w) * scale)
	nh := int(float64(h) * scale)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
	return dst
}

func showDetail(p core.Post) {
	detailAuthorID = p.Author.ID
	sixelReady = false
	sixelData = ""
	sixelImages = p.Images
	sixelIndex = 0

	// Encode first image to Sixel if any.
	if len(p.Images) > 0 {
		go func(url string) {
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
			im, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				app.QueueUpdateDraw(func() { status.SetText("Image decode failed: " + err.Error()) })
				return
			}
			maxW, maxH := cellW*(termWidth-4), cellH*20
			if maxW <= 0 { maxW = 400 }
			if maxH <= 0 { maxH = 320 }
			im = scaleImage(im, maxW, maxH)
			var buf bytes.Buffer
			buf.WriteString("\x1bPq")
			enc := sixel.Encoder{}
			if err := enc.Encode(&buf, im); err != nil {
				app.QueueUpdateDraw(func() { status.SetText("Sixel encode failed: " + err.Error()) })
				return
			}
			buf.WriteString("\x1b\\")
			app.QueueUpdateDraw(func() {
				sixelData = buf.String()
				sixelReady = true
				status.SetText(fmt.Sprintf("Image %d/%d (Sixel)", sixelIndex+1, len(sixelImages)))
			})
		}(p.Images[0])
	}

	text := mdToTview(p.Text)
	if p.Abi != "" {
		text += "\n\n追記: " + mdToTview(p.Abi)
	}
	detailText.SetText(fmt.Sprintf("%s (%s)  %s\nFavs: %d  Replies: %d\n\n%s", p.Author.Name, p.Author.ID, p.CreatedAt.Format("2006-01-02 15:04"), p.FavoriteCount, p.ReplyCount, text))

	if len(p.Images) == 0 {
		detailFlex.ResizeItem(detailImages, 0, 0)
	} else {
		detailFlex.ResizeItem(detailImages, 20, 0)
	}
	detailImages.Clear()
	if !sixelReady && len(p.Images) > 0 {
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

	detailReplies.Clear()
	detailReplyPosts = []core.Post{}
	detailReplies.AddItem("Loading replies...", "", 0, nil)
	go func(id string) {
		replies, err := client.Replies(id, 25, 0)
		app.QueueUpdateDraw(func() {
			if err != nil {
				detailReplies.Clear()
				detailReplies.AddItem("Replies error: "+err.Error(), "", 0, nil)
				return
			}
			detailReplyPosts = []core.Post{}
			detailReplies.Clear()
			count := 0
			for i, r := range replies {
				if i == 0 {
					continue
				}
				count++
				detailReplyPosts = append(detailReplyPosts, r)
				txt := runewidth.Truncate(strings.ReplaceAll(r.Text, "\n", " "), termWidth-10, "...")
				txt = mdToTview(txt)
				detailReplies.AddItem(r.Author.Name+": "+txt, r.CreatedAt.Format("01-02 15:04"), 0, nil)
			}
			if count == 0 {
				detailReplies.AddItem("No replies.", "", 0, nil)
			}
			if count > 0 {
				app.SetFocus(detailReplies)
			}
		})
	}(p.ID)

	// key handling for image switching
	// (registered globally; store current images already in sixelImages)

	pages.SwitchToPage("detail")
}

func buildUserPage() {
	userText = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetScrollable(true)
	userText.SetBorder(true).SetTitle("User")
	followBtn := tview.NewButton("Follow").SetSelectedFunc(func() {
		if client == nil || userProfile.ID == "" {
			return
		}
		go func() {
			err := client.Follow(userProfile.ID)
			app.QueueUpdateDraw(func() {
				if err != nil {
					status.SetText("Follow failed: " + err.Error())
				} else {
					status.SetText("Followed " + userProfile.ID)
				}
			})
		}()
	})
	unfollowBtn := tview.NewButton("Unfollow").SetSelectedFunc(func() {
		if client == nil || userProfile.ID == "" {
			return
		}
		go func() {
			err := client.Unfollow(userProfile.ID)
			app.QueueUpdateDraw(func() {
				if err != nil {
					status.SetText("Unfollow failed: " + err.Error())
				} else {
					status.SetText("Unfollowed " + userProfile.ID)
				}
			})
		}()
	})
	backBtn := tview.NewButton("Back").SetSelectedFunc(func() {
		pages.SwitchToPage("timeline")
		app.Sync()
	})
	btns := tview.NewFlex().SetDirection(tview.FlexColumn)
	btns.AddItem(followBtn, 0, 1, false)
	btns.AddItem(unfollowBtn, 0, 1, false)
	btns.AddItem(backBtn, 0, 1, false)
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(userText, 0, 1, true)
	flex.AddItem(btns, 2, 0, false)
	flex.AddItem(status, 3, 0, false)
	pages.AddPage("user", flex, true, false)
}

func loadUser(userID string) {
	if client == nil || userID == "" {
		return
	}
	go func() {
		u, err := client.User(userID)
		app.QueueUpdateDraw(func() {
			if err != nil {
				status.SetText("User load failed: " + err.Error())
				return
			}
			userProfile = u
			userScroll = 0
			app.SetFocus(userText)
			sep := strings.Repeat("─", 40)
			userText.SetText(fmt.Sprintf("Name: %s\nID: %s\nOnline: %s  Role: %s\n%s\nProfile:\n%s\n%s\nFollowee: %d  Follower: %d  Posts: %d", u.Name, u.ID, u.OnlineStatus, u.Role, sep, u.Profile, sep, u.FolloweeCount, u.FollowerCount, u.UeuseCount))
			pages.SwitchToPage("user")
		})
	}()
}

func buildNotificationsPage() {
	notifList = tview.NewList().ShowSecondaryText(true)
	notifList.SetBorder(true).SetTitle("Notifications")
	notifList.SetWrapAround(false)
	notifList.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(notifs) && client != nil {
			loadUser(notifs[index].From.ID)
		}
	})
	backBtn := tview.NewButton("Back").SetSelectedFunc(func() {
		pages.SwitchToPage("timeline")
		app.Sync()
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
	flex.AddItem(btns, 2, 0, false)
	flex.AddItem(status, 3, 0, false)
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
				notifList.AddItem(n.From.Name+": "+n.Title, n.CreatedAt.Format("01-02 15:04")+"  "+n.Text, 0, nil)
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
