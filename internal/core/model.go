package core

import "time"

type User struct {
	ID      string
	Name    string
	IconURL string
}

type Post struct {
	ID            string
	Text          string
	Author        User
	CreatedAt     time.Time
	ReplyTo       string
	ReuseID       string
	Favorites     []string
	FavoriteCount int
	NSFW          bool
}

type Notification struct {
	ID        string
	Category  string
	Title     string
	Text      string
	From      User
	CreatedAt time.Time
	ValueID   string
	Checked   bool
}

type TimelineBackend interface {
	HomeTimeline(limit, page int) ([]Post, error)
	Mentions(limit, page int) ([]Post, error)
	Replies(postID string, limit, page int) ([]Post, error)
}

type SocialBackend interface {
	CreatePost(text, replyTo, reuseTo string, nsfw bool) (Post, error)
	DeletePost(postID string) error
	Favorite(postID string) error
	Unfavorite(postID string) error
	Follow(userID string) error
	Unfollow(userID string) error
}

type UserBackend interface {
	Me() (User, error)
	User(userID string) (User, error)
}

type NotificationBackend interface {
	Notifications(limit, page int) ([]Notification, error)
	MarkNotificationsRead() error
}
