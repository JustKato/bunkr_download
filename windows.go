package main

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	windowMain     = "main"
	windowPreview  = "preview"
	windowAbout    = "about"
	windowFileInfo = "file-info"
)

var windowBackground = application.RGBA{Red: 38, Green: 42, Blue: 34, Alpha: 255}

func requireApp() (*application.App, error) {
	app := application.Get()
	if app == nil {
		return nil, fmt.Errorf("application not ready")
	}
	return app, nil
}

func focusNamedWindow(name, js string) bool {
	app, err := requireApp()
	if err != nil {
		return false
	}
	window, ok := app.Window.GetByName(name)
	if !ok {
		return false
	}
	if js != "" {
		window.ExecJS(js)
	}
	window.Focus()
	return true
}

func closeNamedWindow(name string) {
	app := application.Get()
	if app == nil {
		return
	}
	if window, ok := app.Window.GetByName(name); ok {
		window.Close()
	}
}

func (s *BunkrService) OpenAbout() error {
	if focusNamedWindow(windowAbout, "") {
		return nil
	}

	app, err := requireApp()
	if err != nil {
		return err
	}

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             windowAbout,
		Title:            "About Bunkr Downloader",
		Width:            440,
		Height:           220,
		MinWidth:         360,
		MinHeight:        180,
		BackgroundColour: windowBackground,
		URL:              "/about.html",
	})
	return nil
}

func (s *BunkrService) CloseAbout() {
	closeNamedWindow(windowAbout)
}

func (s *BunkrService) OpenFileInfo(index int) error {
	s.mu.Lock()
	s.fileInfoIndex = index
	s.mu.Unlock()

	if focusNamedWindow(windowFileInfo, "if(window.reloadFileInfo){window.reloadFileInfo();}") {
		return nil
	}

	app, err := requireApp()
	if err != nil {
		return err
	}

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             windowFileInfo,
		Title:            "File Info",
		Width:            720,
		Height:           520,
		MinWidth:         560,
		MinHeight:        360,
		BackgroundColour: windowBackground,
		URL:              "/file-info.html",
	})
	return nil
}

func (s *BunkrService) CloseFileInfo() {
	closeNamedWindow(windowFileInfo)
}

func (s *BunkrService) GetFileInfoIndex() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fileInfoIndex
}
