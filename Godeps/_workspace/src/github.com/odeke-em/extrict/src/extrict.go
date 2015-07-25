package extrict

import (
	"fmt"
	"regexp"
)

const (
	HttpPattern = "(https?://[\\w\\d\\./]+)(\\w+|\\.c[ao]m?|gov)"
)

func ExtensionToUrlApplication(ext string) string {
	return fmt.Sprintf("(https?://[^\"()=]+)\\.(%s)", ext)
}

func GetAndMatch(uri string, pattern string) chan string {
	return crawl(uri, regexer(pattern), 1)
}

func GetAndMatchHttpLinks(uri string) chan string {
	return GetAndMatch(uri, HttpPattern)
}

func CrawlAndMatchByExtension(uri, extRegStr string, depth int32) chan string {
	return crawl(uri, regexer(ExtensionToUrlApplication(extRegStr)), depth)
}

func crawl(uri string, pattern *regexp.Regexp, depth int32) (saveChan chan string) {
	saveChan = make(chan string)

	if depth == 0 {
		close(saveChan)
		return saveChan
	}

	go func() {
		defer close(saveChan)

		if depth >= 1 {
			depth -= 1
		}

		visited := make(map[string]bool)

		kPair := <-getAndMatch(uri, pattern)
		uris, matchesChan := kPair.uriChan, kPair.matchesChan

		done := make(chan chan string)
		doneCount := uint(0)

		unloadStringChan(matchesChan, saveChan)

		for subUri := range uris {
			doneCount += 1
			if _, ok := visited[subUri]; ok {
				return
			}

			visited[subUri] = true

			go func(u string) {
				// fmt.Println("subUri", u, pattern)
				subCrawl := crawl(u, pattern, depth)
				done <- subCrawl
			}(subUri)
		}

		for i := uint(0); i < doneCount; i += 1 {
			subCrawl := <-done
			unloadStringChan(subCrawl, saveChan)
		}
	}()

	return

}
