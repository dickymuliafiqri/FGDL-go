package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

var (
	home, _     = os.UserHomeDir()
	downloadDir = filepath.Join(home, "Downloads")
	url         = ""

	osPlatform = runtime.GOOS
)

func main() {
	fmt.Print("Input FF URL: ")
	fmt.Scan(&url)

	// Specify download dir for each os
	switch osPlatform {
	case "darwin", "linux":
		downloadID := strings.Split(url, "#")[1]
		downloadDir = filepath.Join(downloadDir, downloadID)
		if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
			os.MkdirAll(downloadDir, 0777)
		}
	}

	fmt.Printf("Download dir: %s\n", downloadDir)
	fmt.Println("Spawning chromium...")

	profileDir := filepath.Join(os.TempDir(), "rod-profile")
	l := launcher.New().
		Set("download.default_directory", downloadDir).
		Set("savefile.default_directory", downloadDir).
		Set("safebrowsing-disable-download-protection", "true").
		Set("download.prompt_for_download", "false").
		Set("disable-popup-blocking", "true").
		Set("user-data-dir", profileDir).
		Headless(false)
	u := l.MustLaunch()
	page := rod.New().ControlURL(u).MustConnect().MustPage(url)

	defer l.Cleanup()
	defer page.Browser().Close()

	// Wait for page
	page.MustWaitStable()

	// Get download links
	fmt.Println("Accessing download links...")
	links := strings.SplitSeq(page.MustElement("#plaintext > ul:nth-child(2)").MustText(), "\n")

	// Delete obsoleted downloads
	deleteDownloads()

	go func(originPageID string) {
		for {
			closePopupPages(originPageID, page)
			time.Sleep(30 * time.Millisecond)
		}
	}(string(page.FrameID))

	for link := range links {
		fmt.Println(link)
		page.Navigate(link)
		page.MustWaitStable()

		filename := page.MustElement(".text-xl").MustText()
		if isFileExists(filename) {
			fmt.Printf("%s: Exists", filename)
			continue
		}

		// Try to download
		for {
			if isDownloadActive() > 0 {
				break
			}

			page.MustElement(".link-button").MustClick()
			time.Sleep(3 * time.Second)
		}

		// Wait for download
		var lastSize string
		var breakCount int = 30
		for {
			downloadSize := isDownloadActive()
			currentSize := humanize.Bytes(uint64(downloadSize))
			fmt.Printf("%s: %s\n", filename, humanize.Bytes(uint64(downloadSize)))

			if downloadSize < 1 {
				break
			} else if breakCount < 1 {
				deleteDownloads()
			} else if lastSize == currentSize {
				breakCount -= 1
			}

			lastSize = currentSize
			time.Sleep(time.Second)
		}
	}

	fmt.Println("Exiting...")
	time.Sleep(10 * time.Second)
}

func closePopupPages(originPageID string, page *rod.Page) {
	pages, err := page.Browser().Pages()
	if err != nil {
		panic(err)
	}

	for _, selPage := range pages {
		if originPageID != string(selPage.FrameID) {
			selPage.MustClose()
		}
	}
}

func isFileExists(filename string) bool {
	files, err := os.ReadDir(downloadDir)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if file.Name() == filename {
			return true
		}
	}

	return false
}

func isDownloadActive() int64 {
	files, err := os.ReadDir(downloadDir)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), "download") {
			if fileInfo, err := os.Stat(filepath.Join(downloadDir, file.Name())); err == nil {
				return fileInfo.Size()
			}
			return 1
		}
	}

	return 0
}

func deleteDownloads() {
	files, err := os.ReadDir(downloadDir)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), "download") {
			if err := os.Remove(filepath.Join(downloadDir, file.Name())); err != nil {
				panic(err)
			}
		}
	}
}
