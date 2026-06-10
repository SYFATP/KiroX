package main

import (
	"embed"
	"flag"
	"log"
	"os"
	"strings"

	"reg_go/internal/storage"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	webMode := flag.Bool("web", false, "run as headless web server")
	addr := flag.String("addr", "127.0.0.1:8171", "web server listen address")
	webPassword := flag.String("web-password", os.Getenv("KIROX_WEB_PASSWORD"), "web login password")
	flag.Parse()

	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })

	if *webMode {
		password := ""
		if explicit["web-password"] {
			password = strings.TrimSpace(*webPassword)
		} else if env := strings.TrimSpace(os.Getenv("KIROX_WEB_PASSWORD")); env != "" {
			password = env
		} else {
			password = storage.GetWebPassword()
		}
		runWebServer(*addr, password)
		return
	}

	runDesktopApp()
}

// runDesktopApp 运行桌面应用
func runDesktopApp() {
	app := NewApp()

	appOpts := &options.App{
		Title:  "Kiro 注册机",
		Width:  900,
		Height: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 245, G: 240, B: 235, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		StartHidden:      false,
		Frameless:        true,
		Bind: []interface{}{
			app,
		},
	}

	// 应用平台特定选项
	for _, opt := range getPlatformOptions() {
		opt(appOpts)
	}

	err := wails.Run(appOpts)

	if err != nil {
		log.Fatal(err)
	}
}
