package main

import (
	"embed"
	"io/fs"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"herdworkbench/internal/cli"
	"herdworkbench/internal/mcp"
	"herdworkbench/internal/storage"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Handle CLI and MCP flags before initializing the graphical shell
	if len(os.Args) > 1 {
		if os.Args[1] == "--cli" {
			handler := cli.NewCLIHandler()
			os.Exit(handler.Run(os.Args[2:]))
		}
		if os.Args[1] == "--mcp" {
			server := mcp.NewServer()
			err := server.ServeStdio()
			server.Close()
			if err != nil {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	profile, profileErr := storage.WebviewProfile("")
	if profileErr != nil {
		println("Cannot select browser profile:", profileErr.Error())
		return
	}
	subFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		subFS = assets
	}
	// Create an instance of the app structure
	app := NewApp(subFS)

	// Create application with options
	err = wails.Run(&options.App{
		Windows:   &windows.Options{WebviewUserDataPath: profile},
		Title:     "LaraDeck — Mission Control for Laravel",
		Width:     1380,
		Height:    880,
		MinWidth:  1000,
		MinHeight: 660,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 255},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
