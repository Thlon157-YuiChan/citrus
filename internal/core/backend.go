package core

// Backend groups the capability interfaces. Implementations may support
// only a subset; callers should type-assert to the needed interface.
type Backend interface {
	Name() string
}
