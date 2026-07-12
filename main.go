package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:        "Bunkr Downloader",
		Description: "Bunkr album downloader",
		Services: []application.Service{
			application.NewService(NewBunkrService()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(frontend),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "Bunkr Downloader",
		Width:            1180,
		Height:           720,
		MinWidth:         900,
		MinHeight:        520,
		BackgroundColour: application.RGBA{Red: 38, Green: 42, Blue: 34, Alpha: 255},
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
