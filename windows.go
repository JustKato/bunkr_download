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
	windowConsole  = "console"
)

var windowBackground = application.RGBA{Red: 38, Green: 42, Blue: 34, Alpha: 255}

func requireApp() (*application.App, error) {
	app := application.Get()
	if app == nil {
		return nil, fmt.Errorf("application not ready")
	}
	return app, nil
}

func openSecondaryWindow(name string, opts application.WebviewWindowOptions, reloadJS string) error {
	app, err := requireApp()
	if err != nil {
		return err
	}

	opts.Name = name
	if opts.BackgroundColour.Alpha == 0 && opts.BackgroundColour.Red == 0 &&
		opts.BackgroundColour.Green == 0 && opts.BackgroundColour.Blue == 0 {
		opts.BackgroundColour = windowBackground
	}
	if opts.InitialPosition == 0 && opts.X == 0 && opts.Y == 0 {
		opts.InitialPosition = application.WindowCentered
	}

	appLog("info", "window", "opening %q", name)

	var openErr error
	application.InvokeSync(func() {
		if existing, ok := app.Window.GetByName(name); ok {
			if reloadJS != "" {
				existing.ExecJS(reloadJS)
			}
			existing.Show()
			existing.Focus()
			return
		}

		win := app.Window.NewWithOptions(opts)
		if win == nil {
			openErr = fmt.Errorf("failed to create window %q", name)
			return
		}
		win.Show()
		win.Focus()
	})

	if openErr != nil {
		appLog("error", "window", "open %q failed: %v", name, openErr)
	}
	return openErr
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
	return openSecondaryWindow(windowAbout, application.WebviewWindowOptions{
		Title:            "About Bunkr Downloader",
		Width:            440,
		Height:           220,
		MinWidth:         360,
		MinHeight:        180,
		BackgroundColour: windowBackground,
		URL:              "/about.html",
	}, "")
}

func (s *BunkrService) CloseAbout() {
	closeNamedWindow(windowAbout)
}

func (s *BunkrService) OpenFileInfo(index int) error {
	s.mu.Lock()
	s.fileInfoIndex = index
	s.mu.Unlock()

	return openSecondaryWindow(windowFileInfo, application.WebviewWindowOptions{
		Title:            "File Info",
		Width:            720,
		Height:           520,
		MinWidth:         560,
		MinHeight:        360,
		BackgroundColour: windowBackground,
		URL:              "/file-info.html",
	}, "if(window.reloadFileInfo){window.reloadFileInfo();}")
}

func (s *BunkrService) CloseFileInfo() {
	closeNamedWindow(windowFileInfo)
}

func (s *BunkrService) GetFileInfoIndex() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fileInfoIndex
}

func (s *BunkrService) OpenConsole() error {
	return openSecondaryWindow(windowConsole, application.WebviewWindowOptions{
		Title:            "Console",
		Width:            860,
		Height:           520,
		MinWidth:         560,
		MinHeight:        320,
		BackgroundColour: windowBackground,
		URL:              "/console.html",
	}, "if(window.reloadConsole){window.reloadConsole();}")
}

func (s *BunkrService) CloseConsole() {
	closeNamedWindow(windowConsole)
}
