package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	progressbar "github.com/schollz/progressbar/v3"
)

var (
	home, _           = os.UserHomeDir()
	downloadDir       = ""
	url               = ""
	downloadLinkRegex = regexp.MustCompile(`window\.open\("(.+)"\)`)
	pathRegex         = regexp.MustCompile(`['"\n\\]`)
)

func main() {
	fmt.Println(`
 ███████████ ███  █████              ███          ████                                                                        
░░███░░░░░░█░░░  ░░███              ░░░          ░░███                                                                        
 ░███   █ ░ ████ ███████    ███████ ████ ████████ ░███                                                                        
 ░███████  ░░███░░░███░    ███░░███░░███░░███░░███░███                                                                        
 ░███░░░█   ░███  ░███    ░███ ░███ ░███ ░███ ░░░ ░███                                                                        
 ░███  ░    ░███  ░███ ███░███ ░███ ░███ ░███     ░███                                                                        
 █████      █████ ░░█████ ░░███████ ██████████    █████                                                                       
░░░░░      ░░░░░   ░░░░░   ░░░░░███░░░░░░░░░░    ░░░░░                                                                        
                           ███ ░███                                                                                           
                          ░░██████                                                                                            
                           ░░░░░░                                                                                             
 ███████████ ███████████    ██████████                                    ████                        █████                   
░░███░░░░░░█░░███░░░░░░█   ░░███░░░░███                                  ░░███                       ░░███                    
 ░███   █ ░  ░███   █ ░     ░███   ░░███  ██████  █████ ███ █████████████ ░███   ██████  ██████    ███████   ██████  ████████ 
 ░███████    ░███████       ░███    ░███ ███░░███░░███ ░███░░███░░███░░███░███  ███░░███░░░░░███  ███░░███  ███░░███░░███░░███
 ░███░░░█    ░███░░░█       ░███    ░███░███ ░███ ░███ ░███ ░███ ░███ ░███░███ ░███ ░███ ███████ ░███ ░███ ░███████  ░███ ░░░ 
 ░███  ░     ░███  ░        ░███    ███ ░███ ░███ ░░███████████  ░███ ░███░███ ░███ ░██████░░███ ░███ ░███ ░███░░░   ░███     
 █████       █████          ██████████  ░░██████   ░░████░████   ████ █████████░░██████░░████████░░████████░░██████  █████    
░░░░░       ░░░░░          ░░░░░░░░░░    ░░░░░░     ░░░░ ░░░░   ░░░░ ░░░░░░░░░  ░░░░░░  ░░░░░░░░  ░░░░░░░░  ░░░░░░  ░░░░░     
                                                                                                                              
                                                                                                                              
by. dickymuliafiqri - awokwokwokwokwokwo                                                                                      `)

	scanner := bufio.NewReader(os.Stdin)

	fmt.Print("Input FF URL: ")
	url, _ = scanner.ReadString('\n')

	fmt.Printf("Input Download Dir (Blank for Default): ")
	downloadDir, _ = scanner.ReadString('\n')

	if len(downloadDir) < 2 {
		downloadID := strings.Split(url, "#")[1]
		downloadDir = filepath.Join(filepath.Join(home, "Downloads"), downloadID)
	} else {
		downloadDir = pathRegex.ReplaceAllString(downloadDir, "")
		if string(downloadDir[len(downloadDir)-1]) == " " {
			downloadDir = downloadDir[:len(downloadDir)-1]
		}
	}
	downloadDir = filepath.Clean(downloadDir)

	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		os.MkdirAll(downloadDir, 0777)
	}

	fmt.Printf("Download dir: %s\n", downloadDir)
	fmt.Println("Spawning chromium...")

	l := launcher.New().
		Set("no-sandbox").
		Headless(true)
	u := l.MustLaunch()
	page := rod.New().ControlURL(u).MustConnect().MustPage(url)

	// Wait for page
	page.MustWaitStable()

	// Get download links
	fmt.Println("Accessing download links...")
	links := strings.Split(page.MustElement("#plaintext > ul:nth-child(2)").MustText(), "\n")

	// Close browser
	page.Browser().Close()
	l.Cleanup()

	// Delete obsoleted downloads
	deleteDownloads()

	// Download list
	var downloadIndexes = make([]int, len(links))
	for i := range links {
		downloadIndexes[i] = i
	}

	for {
		for index, link := range links {
			if slices.Contains(downloadIndexes, index) {
				fmt.Printf("[+] %d. %s\n", index, strings.Split(link, "#")[1])
			} else {
				fmt.Printf("[-] %d. %s\n", index, strings.Split(link, "#")[1])
			}
		}

		var selectIndex = 0

		fmt.Printf("Select number to switch, type '30035' to start: ")
		fmt.Scan(&selectIndex)

		if selectIndex == 30035 {
			break
		} else {
			if selectIndex > len(links)-1 {
				fmt.Println("Index out of range")
			} else {
				if slices.Contains(downloadIndexes, selectIndex) {
					newS := []int{}
					for _, i := range downloadIndexes {
						if i != selectIndex {
							newS = append(newS, i)
						}
					}
					downloadIndexes = newS
				} else {
					downloadIndexes = append(downloadIndexes, selectIndex)
				}
			}
		}
		fmt.Println("")
	}

	fmt.Printf("\n\nStarting download...\n")

	for index, link := range links {
		if !slices.Contains(downloadIndexes, index) {
			continue
		}

		func() {
			fmt.Printf("[+] URL: %s\n", link)

			resp, err := http.Get(link)
			if err != nil {
				fmt.Printf("[-] Error: %s\n", err)
				return
			}
			defer resp.Body.Close()

			doc, err := goquery.NewDocumentFromReader(resp.Body)
			if err != nil {
				fmt.Printf("[-] Error: %s\n", err)
				return
			}

			htmlString, _ := doc.Html()

			filename := doc.Find(".text-xl").Text()
			tempFilename := filename + ".download"
			fmt.Printf("[+] Filename: %s\n", filename)
			if isFileExists(filename) {
				fmt.Printf("[-] %s: Exists\n", filename)
				return
			}

			defer os.Rename(filepath.Join(downloadDir, tempFilename), filepath.Join(downloadDir, filename))

			downloadLink := downloadLinkRegex.FindStringSubmatch(htmlString)[1]
			resp, err = http.Get(downloadLink)
			if err != nil {
				fmt.Printf("[-] Error: %s\n", err)
				return
			}
			defer resp.Body.Close()

			f, _ := os.OpenFile(filepath.Join(downloadDir, tempFilename), os.O_CREATE|os.O_WRONLY, 0644)
			defer f.Close()

			bar := progressbar.DefaultBytes(
				resp.ContentLength,
				"Downloading",
			)

			io.Copy(io.MultiWriter(f, bar), resp.Body)
		}()
	}

	fmt.Println("Exiting...")
	time.Sleep(10 * time.Second)
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

func deleteDownloads() {
	files, err := os.ReadDir(downloadDir)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".download") {
			if err := os.Remove(filepath.Join(downloadDir, file.Name())); err != nil {
				panic(err)
			}
		}
	}
}
