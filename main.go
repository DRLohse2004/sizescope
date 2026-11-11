//go:build !dev

package main

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:            "SizeScope",
		Width:            1360,
		Height:           860,
		MinWidth:         900,
		MinHeight:        560,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 23, G: 27, B: 33, A: 255},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
		Windows: &windows.Options{
			Theme: windows.SystemDefault,
			// keep WebView2 data out of %APPDATA% so the app stays portable
			WebviewUserDataPath:               filepath.Join(os.TempDir(), "SizeScope-WebView2"),
			DisableFramelessWindowDecorations: false,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
