package tui

import (
	"regexp"
	"strings"
)

var (
	reBold      = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reBoldU     = regexp.MustCompile(`__(.+?)__`)
	reStrike    = regexp.MustCompile(`~~(.+?)~~`)
	reItalic    = regexp.MustCompile(`\*(.+?)\*`)
	reItalicU   = regexp.MustCompile(`_(.+?)_`)
)

func mdToTview(s string) string {
	// Escape existing tview color tags.
	s = strings.ReplaceAll(s, "[", "[[")
	s = strings.ReplaceAll(s, "]", "]]")
	// Bold
	s = reBold.ReplaceAllString(s, "[::b]$1[::-]")
	s = reBoldU.ReplaceAllString(s, "[::b]$1[::-]")
	// Strikethrough
	s = reStrike.ReplaceAllString(s, "[::s]$1[::-]")
	// Italic
	s = reItalic.ReplaceAllString(s, "[::i]$1[::-]")
	s = reItalicU.ReplaceAllString(s, "[::i]$1[::-]")
	return s
}
