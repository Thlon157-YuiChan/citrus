package gui

import (
	"github.com/mappu/miqt/qt6"
)

func Run() error {
	qt6.NewQApplication([]string{})
	window := qt6.NewQMainWindow2()
	window.SetWindowTitle("Citrus")
	window.Resize(800, 600)
	window.Show()
	qt6.QApplication_Exec()
	return nil
}
