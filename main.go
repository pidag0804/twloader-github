//go:build windows

// twloader-tool/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"twloader-tool/api"
	"twloader-tool/config"
	"twloader-tool/game"
	"twloader-tool/optimizer"
	"twloader-tool/ui"
	"twloader-tool/utils"

	"github.com/faiface/mainthread"
)

const serverAddr = "127.0.0.1:8787"

var logger = log.New(os.Stdout, "TWLOADERWEB | ", log.LstdFlags)

func runApp() {
	// Redirects log output to a file
	logFile, err := os.OpenFile("twloader-tool.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(logFile)
	}
	log.Println("======================================")
	log.Println("====== Application starting, logging initiated ======")
	log.Println("======================================")

	// 1. Initialization
	if err := config.Load(); err != nil {
		logger.Printf("Warning: Error reading configuration file: %v", err)
	}
	if err := optimizer.FetchItemsFromServer(); err != nil {
		logger.Fatalf("Initialization failed, could not get optimization item list: %v", err)
	}
	if err := api.FetchStaticAssets(); err != nil {
		logger.Fatalf("Initialization failed, could not get front-end interface files: %v", err)
	}

	// 2. Start background tasks
	go game.CheckVersion()
	// 【MODIFIED】: Handles the error returned from SetupGamePathLink
	if err := game.SetupGamePathLink(); err != nil {
		// Logs the error as a warning, as this is not a fatal error that should stop the program
		logger.Printf("Warning: An error occurred while setting up the game path link: %v", err)
	}

	// 3. Start GUI Manager
	guiReadyChan := make(chan bool)
	go ui.GUIManager(guiReadyChan)
	<-guiReadyChan
	logger.Println("GUI Manager is ready.")

	// 4. Set up HTTP server and routes
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	server := &http.Server{
		Addr:    serverAddr,
		Handler: mux,
	}

	// 5. Start the server and open the browser
	go func() {
		logger.Printf("Server is listening on http://%s\n", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Could not start server: %v", err)
		}
	}()
	time.Sleep(500 * time.Millisecond)
	utils.OpenBrowser(fmt.Sprintf("http://%s", serverAddr))

	// 6. Wait for shutdown signal
	<-api.ShutdownChan
	logger.Println("Front-end closure detected, shutting down HTTP server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Printf("An error occurred during server shutdown: %v", err)
	}

	ui.CloseGUIManager()
	logger.Println("runApp function has finished.")
}

func main() {
	mainthread.Run(runApp)
	logger.Println("Program has completely shut down.")
}
