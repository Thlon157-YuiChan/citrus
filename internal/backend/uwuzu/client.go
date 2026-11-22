package uwuzu

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chihaya/citrus/internal/core"
	"github.com/google/uuid"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Name() string { return "uwuzu" }

// AuthURL builds the user authorization URL.
func AuthURL(domain, session, clientName, scope, icon, about, callback string) string {
	u := "https://" + domain + "/api/auth?session=" + url.QueryEscape(session) +
		"&client=" + url.QueryEscape(clientName) +
		"&scope=" + url.QueryEscape(scope)
	if icon != "" {
		u += "&icon=" + url.QueryEscape(icon)
	}
	if about != "" {
		u += "&about=" + url.QueryEscape(about)
	}
	if callback != "" {
		u += "&callback=" + url.QueryEscape(callback)
	}
	return u
}

func NewSession() string { return uuid.NewString() }

type tokenResp struct {
	Username string `json:"username"`
	UserID   string `json:"userid"`
	Token    string `json:"token"`
}

func (c *Client) GetToken(session string) (tokenResp, error) {
	var tr tokenResp
	u := c.BaseURL + "/api/token/get?session=" + url.QueryEscape(session)
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return tr, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return tr, err
	}
	if tr.Token == "" {
		return tr, fmt.Errorf("token not available")
	}
	return tr, nil
}

// --- API models ---

type uwuzuUser struct {
	Username string `json:"username"`
	UserID   string `json:"userid"`
	UserIcon string `json:"user_icon"`
}

type uwuzuPost struct {
	Uniqid      string   `json:"uniqid"`
	Relpyid     string   `json:"relpyid"`
	Reuseid     string   `json:"reuseid"`
	Text        string   `json:"text"`
	Account     struct {
		Username string `json:"username"`
		UserID   string `json:"userid"`
		UserIcon string `json:"user_icon"`
		IsBot    bool   `json:"is_bot"`
	} `json:"account"`
	Favorite    []string `json:"favorite"`
	FavoriteCnt string   `json:"favorite_cnt"`
	Datetime    string   `json:"datetime"`
	NSFW        bool     `json:"nsfw"`
}

func parseTime(s string) time.Time {
	t, _ := time.Parse("2006-01-02 15:04:05", s)
	return t
}

func toPost(p uwuzuPost) core.Post {
	fc, _ := strconv.Atoi(p.FavoriteCnt)
	return core.Post{
		ID:            p.Uniqid,
		Text:          p.Text,
		Author:        core.User{ID: p.Account.UserID, Name: p.Account.Username, IconURL: p.Account.UserIcon},
		CreatedAt:     parseTime(p.Datetime),
		ReplyTo:       p.Relpyid,
		ReuseID:       p.Reuseid,
		Favorites:     p.Favorite,
		FavoriteCount: fc,
		NSFW:          p.NSFW,
	}
}

func (c *Client) doJSON(method, path string, body interface{}, out interface{}) error {
	var req *http.Request
	var err error
	u := c.BaseURL + path
	if body != nil {
		b, _ := json.Marshal(body)
		req, err = http.NewRequest(method, u, strings.NewReader(string(b)))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, u, nil)
		if err != nil {
			return err
		}
	}
	if c.Token != "" {
		q := req.URL.Query()
		q.Set("token", c.Token)
		req.URL.RawQuery = q.Encode()
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *Client) Me() (core.User, error) {
	var u struct {
		Username string `json:"username"`
		UserID   string `json:"userid"`
		UserIcon string `json:"user_icon"`
	}
	if err := c.doJSON("GET", "/api/me/", map[string]string{"token": c.Token}, &u); err != nil {
		// fallback query style
	}
	_ = u
	// simpler: direct GET with query
	resp, err := c.HTTP.Get(c.BaseURL + "/api/me/?token=" + url.QueryEscape(c.Token))
	if err != nil {
		return core.User{}, err
	}
	defer resp.Body.Close()
	var m struct {
		Username string `json:"username"`
		UserID   string `json:"userid"`
		UserIcon string `json:"user_icon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return core.User{}, err
	}
	return core.User{ID: m.UserID, Name: m.Username, IconURL: m.UserIcon}, nil
}

func (c *Client) HomeTimeline(limit, page int) ([]core.Post, error) {
	u := c.BaseURL + "/api/ueuse/?token=" + url.QueryEscape(c.Token)
	if limit > 0 {
		u += "&limit=" + strconv.Itoa(limit)
	}
	if page > 0 {
		u += "&page=" + strconv.Itoa(page)
	}
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var posts []uwuzuPost
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil, err
	}
	out := make([]core.Post, 0, len(posts))
	for _, p := range posts {
		out = append(out, toPost(p))
	}
	return out, nil
}

func (c *Client) Mentions(limit, page int) ([]core.Post, error) {
	u := c.BaseURL + "/api/ueuse/mentions?token=" + url.QueryEscape(c.Token)
	if limit > 0 {
		u += "&limit=" + strconv.Itoa(limit)
	}
	if page > 0 {
		u += "&page=" + strconv.Itoa(page)
	}
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var posts []uwuzuPost
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil, err
	}
	out := make([]core.Post, 0, len(posts))
	for _, p := range posts {
		out = append(out, toPost(p))
	}
	return out, nil
}

func (c *Client) Replies(postID string, limit, page int) ([]core.Post, error) {
	u := c.BaseURL + "/api/ueuse/replies?token=" + url.QueryEscape(c.Token) + "&uniqid=" + url.QueryEscape(postID)
	if limit > 0 {
		u += "&limit=" + strconv.Itoa(limit)
	}
	if page > 0 {
		u += "&page=" + strconv.Itoa(page)
	}
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var posts []uwuzuPost
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil, err
	}
	out := make([]core.Post, 0, len(posts))
	for _, p := range posts {
		out = append(out, toPost(p))
	}
	return out, nil
}

func (c *Client) User(userID string) (core.User, error) {
	u := c.BaseURL + "/api/users/?token=" + url.QueryEscape(c.Token) + "&userid=" + url.QueryEscape(userID)
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return core.User{}, err
	}
	defer resp.Body.Close()
	var m struct {
		Username string `json:"username"`
		UserID   string `json:"userid"`
		UserIcon string `json:"user_icon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return core.User{}, err
	}
	return core.User{ID: m.UserID, Name: m.Username, IconURL: m.UserIcon}, nil
}

func (c *Client) CreatePost(text, replyTo, reuseTo string, nsfw bool) (core.Post, error) {
	body := map[string]interface{}{
		"token": c.Token,
		"text":  text,
	}
	if replyTo != "" {
		body["replyid"] = replyTo
	}
	if reuseTo != "" {
		body["reuseid"] = reuseTo
	}
	body["nsfw"] = nsfw
	b, _ := json.Marshal(body)
	resp, err := c.HTTP.Post(c.BaseURL+"/api/ueuse/create", "application/json", strings.NewReader(string(b)))
	if err != nil {
		return core.Post{}, err
	}
	defer resp.Body.Close()
	var r struct {
		Success bool   `json:"success"`
		Uniqid  string `json:"uniqid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return core.Post{}, err
	}
	if !r.Success {
		return core.Post{}, fmt.Errorf("create failed")
	}
	// fetch created post
	posts, err := c.Replies(r.Uniqid, 1, 0)
	if err == nil && len(posts) > 0 {
		return posts[0], nil
	}
	return core.Post{ID: r.Uniqid, Text: text}, nil
}

func (c *Client) DeletePost(postID string) error {
	u := c.BaseURL + "/api/ueuse/delete?token=" + url.QueryEscape(c.Token) + "&uniqid=" + url.QueryEscape(postID)
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) Favorite(postID string) error {
	u := c.BaseURL + "/api/ueuse/favorite?token=" + url.QueryEscape(c.Token) + "&uniqid=" + url.QueryEscape(postID)
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) Unfavorite(postID string) error {
	u := c.BaseURL + "/api/ueuse/unfavorite?token=" + url.QueryEscape(c.Token) + "&uniqid=" + url.QueryEscape(postID)
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) Follow(userID string) error {
	u := c.BaseURL + "/api/users/follow?token=" + url.QueryEscape(c.Token) + "&userid=" + url.QueryEscape(userID)
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) Unfollow(userID string) error {
	u := c.BaseURL + "/api/users/unfollow?token=" + url.QueryEscape(c.Token) + "&userid=" + url.QueryEscape(userID)
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

type uwuzuNotif struct {
	From struct {
		Username string `json:"username"`
		UserID   string `json:"userid"`
		UserIcon string `json:"user_icon"`
	} `json:"from"`
	Category string `json:"category"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Datetime string `json:"datetime"`
	ValueID  string `json:"valueid"`
	Checked  bool   `json:"is_checked"`
}

func (c *Client) Notifications(limit, page int) ([]core.Notification, error) {
	u := c.BaseURL + "/api/me/notification/?token=" + url.QueryEscape(c.Token)
	if limit > 0 {
		u += "&limit=" + strconv.Itoa(limit)
	}
	if page > 0 {
		u += "&page=" + strconv.Itoa(page)
	}
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	var out []core.Notification
	for k, v := range raw {
		if k == "success" {
			continue
		}
		var n uwuzuNotif
		if err := json.Unmarshal(v, &n); err != nil {
			continue
		}
		out = append(out, core.Notification{
			ID:        k,
			Category:  n.Category,
			Title:     n.Title,
			Text:      n.Text,
			From:      core.User{ID: n.From.UserID, Name: n.From.Username, IconURL: n.From.UserIcon},
			CreatedAt: parseTime(n.Datetime),
			ValueID:   n.ValueID,
			Checked:   n.Checked,
		})
	}
	return out, nil
}

func (c *Client) MarkNotificationsRead() error {
	u := c.BaseURL + "/api/me/notification/read?token=" + url.QueryEscape(c.Token)
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
