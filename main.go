package main

import (
	"embed"
	"log"
	"os"
	"slices"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/icons"

	"timetrack/internal/store"
	"timetrack/internal/tracker"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Registered events get strongly typed TS bindings generated for them.
	application.RegisterEvent[string](DayChangedEvent)
}

func main() {
	dbPath, err := store.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	svc := NewTrackerService(tracker.New(db))

	app := application.New(application.Options{
		Name:        "timetrack",
		Description: "Gapless personal time tracking",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Windows: application.WindowsOptions{
			// The app lives in the tray; hiding the last window must not
			// quit it.
			DisableQuitOnLastWindowClosed: true,
		},
	})
	// The service emits change events through the app; it exists only now.
	svc.app = app

	// --hidden starts straight to the tray (for a shell:startup shortcut).
	startHidden := slices.Contains(os.Args[1:], "--hidden")

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "timetrack",
		Width:            1100,
		Height:           760,
		MinWidth:         800,
		MinHeight:        600,
		Hidden:           startHidden,
		BackgroundColour: application.NewRGB(9, 9, 11),
		URL:              "/",
	})

	// Closing the main window hides it to the tray instead of quitting.
	// Hooks run before the built-in close listener; Cancel stops it.
	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		mainWindow.Hide()
		e.Cancel()
	})

	// Small always-on-top popup attached to the tray icon. It loads the same
	// SPA; the "#/quick" route renders the compact view.
	quickWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "timetrack quick",
		Width:            380,
		Height:           500,
		Hidden:           true,
		Frameless:        true,
		AlwaysOnTop:      true,
		DisableResize:    true,
		HideOnFocusLost:  true,
		HideOnEscape:     true,
		BackgroundColour: application.NewRGB(9, 9, 11),
		URL:              "/#/quick",
	})

	tray := app.SystemTray.New()
	tray.SetIcon(icons.SystrayLight)
	tray.SetDarkModeIcon(icons.SystrayDark)
	tray.SetTooltip("timetrack")
	// Left click toggles the attached quick window (smart default);
	// right click opens the menu.
	tray.AttachWindow(quickWindow).WindowOffset(8)

	menu := app.NewMenu()
	menu.Add("Open timetrack").OnClick(func(_ *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
