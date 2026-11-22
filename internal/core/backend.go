package core

// Backend abstracts a social service (uwuzu, Mastodon, Misskey, ...).
type Backend interface {
	Name() string
	// TODO: timeline, post, user, follow, notifications, ...
}

// Account represents a logged-in account on a backend.
type Account struct {
	Backend string
	BaseURL string
	Token   string
	UserID  string
}
