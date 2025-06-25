package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

var (
	home, _     = os.UserHomeDir()
	downloadDir = home + "/Downloads"
	url         = ""
)

func main() {
	fmt.Print("Input FF URL: ")
	fmt.Scan(&url)

	//
	downloadID := strings.Split(url, "#")[1]
	downloadDir = downloadDir + "/" + downloadID
	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		os.MkdirAll(downloadDir, 0777)
	}

	pref := fmt.Sprintf(`{
		"download": {
		  "default_directory": "%s"
		}
	}`, downloadDir)

	fmt.Println("Spawning chromium...")
	l := launcher.New().Preferences(pref).Headless(true)
	u := l.MustLaunch()
	page := rod.New().ControlURL(u).MustConnect().MustPage(url)

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
			fmt.Println(fmt.Sprintf("%s: %s", filename, humanize.Bytes(uint64(downloadSize))))

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
			if fileInfo, err := file.Info(); err == nil {
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
			if err := os.Remove(downloadDir + "/" + file.Name()); err != nil {
				panic(err)
			}
		}
	}
}
