package cli

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/amxv/adm/internal/db"
	"github.com/amxv/adm/internal/server"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Start the local operator web UI",
	Long:  "Starts a local HTTP server serving the ADM operator dashboard. Read-only; all write actions remain CLI-driven.",
	RunE:  runUI,
}

var (
	uiHost string
	uiPort int
	uiOpen bool
)

func init() {
	uiCmd.Flags().StringVar(&uiHost, "host", "127.0.0.1", "Bind host/IP")
	uiCmd.Flags().IntVar(&uiPort, "port", 7777, "Bind port")
	uiCmd.Flags().BoolVar(&uiOpen, "open", false, "Auto-open browser after server start")
}

func runUI(cmd *cobra.Command, args []string) error {
	d, err := db.Open()
	if err != nil {
		return err
	}
	defer d.Close()

	srv := server.New(d, uiHost, uiPort)
	srv.SetupStaticHandler()

	server.SetVersion(rootCmd.Version)

	url := fmt.Sprintf("http://%s", srv.Addr())
	fmt.Printf("ADM UI running at %s\n", url)
	fmt.Println("Press Ctrl+C to stop.")

	if uiOpen {
		openBrowser(url)
	}

	return http.ListenAndServe(srv.Addr(), srv.Handler())
}

func openBrowser(url string) {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	case "windows":
		cmd = "start"
	default:
		return
	}
	exec.Command(cmd, url).Start()
}
