package extrict

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"

	expb "github.com/odeke-em/exponential-backoff"
)

type consumer func(string) string
type producer func(uri string, pattern *regexp.Regexp) chan string
type regexper func(string) *regexp.Regexp

var (
	uriRegCompile = regexer(HttpPattern)
)

type uriMatchChanPair struct {
	uriChan     chan string
	matchesChan chan string
}

func regexpGenerator() regexper {
	cache := map[string]*regexp.Regexp{}

	return func(pat string) *regexp.Regexp {
		memoized, ok := cache[pat]
		if ok && memoized != nil {
			return memoized
		}

		// TODO: handle regexp.Compile err
		reg, _ := regexp.Compile(pat)
		cache[pat] = memoized
		return reg
	}
}

var regexer = regexpGenerator()

func responseStringer(result interface{}, err error) (stringified []string) {
	if err != nil {
		return
	}
	resp := result.(*http.Response)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("err", err)
		return
	}

	splits := bytes.Split(body, []byte{'\n'})
	for _, row := range splits {
		stringified = append(stringified, string(row))
	}

	return stringified
}

func extractAndMatch(lines []string, pattern *regexp.Regexp) chan string {
	matchesChan := make(chan string)
	go func() {
		defer close(matchesChan)

		for _, line := range lines {
			if len(line) < 1 {
				continue
			}

			matches := pattern.FindAllString(line, -1)

			mappings := make(map[string]bool)

			for _, match := range matches {
				_, ok := mappings[match]
				if ok {
					continue
				}

				matchesChan <- match
				mappings[match] = true
			}
		}
	}()

	return matchesChan
}

func getAndMatch(uri string, pattern *regexp.Regexp) chan *uriMatchChanPair {
	// TODO: Read in the value of n
	retries := uint32(3)

	getter := expb.NewUrlGetter(uri, retries)
	chanOPair := make(chan *uriMatchChanPair)

	go func() {
		expb.ExponentialBackOff(getter, func(resp interface{}, err error) {
			defer close(chanOPair)

			lines := responseStringer(resp, err)

			matchesChan := extractAndMatch(lines, pattern)
			uriChan := extractAndMatch(lines, uriRegCompile)

			chanOPair <- &uriMatchChanPair{uriChan: uriChan, matchesChan: matchesChan}
		})
	}()

	return chanOPair
}

func unloadStringChan(from chan string, to chan string) {
	for res := range from {
		to <- res
	}
}
