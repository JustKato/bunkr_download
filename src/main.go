package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:embedded/frontend
var assets embed.FS

func main() {
	installLinuxDesktopIntegration()

	frontend, err := fs.Sub(assets, "embedded/frontend")
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:        "Bunkr Downloader",
		Description: "Bunkr album downloader",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(NewBunkrService()),
		},
		Assets: application.AssetOptions{
			Handler: newFrontendHandler(frontend),
		},
		Linux: application.LinuxOptions{
			ProgramName: "bunkrdownload",
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             windowMain,
		Title:            "Bunkr Downloader",
		Width:            1180,
		Height:           720,
		MinWidth:         900,
		MinHeight:        520,
		BackgroundColour: windowBackground,
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
