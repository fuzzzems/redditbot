package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// @TODO Delete comment if -1
// @TODO "you are doing that too much. try again in 4 minutes." - Check limits on posting
// @TODO Put limit on length of reply
// @TODO Handle sections
// @TODO Add error handling for everything. instead of just panicing
// Add posted comment if post successful
func main() {

	// Load blacklists
	blacklist := getBlacklist("reddit-wikipediaposter-blacklist.txt")
	blacklistUsers := getBlacklist("reddit-wikipediaposter-blacklist-users.txt")

	log.Printf("Blacklisted Subreddits %v. \n Blacklisted users %v", blacklist, blacklistUsers)

	// Get the client for making requests
	client := getClient("reddit-wikipediaposter-config.json")

	// RegEx for finding wikipedia links
	r := regexp.MustCompile(`http(?:s)?://([a-zA-Z]{2}).(?:m\.)?wikipedia.org/wiki/([^\s|#]+(?:#(\w+))?)`)

	//Wikipedia API endpoint
	wikilink := "https://%s.wikipedia.org/w/api.php?format=json&action=query&prop=extracts&exintro&explaintext&formatversion=2&titles=%s"

	limit := 100

	searchparams := make(map[string]interface{})
	searchparams["limit"] = limit

	commentInfo := fmt.Sprint("^I ^am ^a ^bot. ^Please ^contact ^[/u/GregMartinez](https://www.reddit.com/user/GregMartinez) ^with ^any ^questions ^or ^feedback.")

	commented := make([]string, 0, limit)

	replaceUrl := strings.NewReplacer("(", "\\(", ")", "\\)")

	// Run
	for {
		// Get new comments from /r/all
		listings := searchNew(client, searchparams)

		for _, listing := range listings.Data.Children {

			if listing.Data.Author == "WikipediaPoster" {
				continue
			}

			if contains(blacklistUsers, listing.Data.Author) {
				continue
			}

			sub := strings.ToLower(listing.Data.Subreddit)

			if contains(blacklist, sub) {
				continue
			}

			id := listing.Data.Name

			if contains(commented, id) {
				continue
			}

			matches := r.FindStringSubmatch(listing.Data.Body)
			if len(matches) >= 2 {
				log.Printf("Found wiki link here: %s\n", sub)

				if len(commented) < limit {
					log.Printf("Adding %s to commented list \n", id)
					log.Printf("%d spots remaining", limit-len(commented))
					commented = append(commented, id)
				} else {
					log.Println("Clearing commented list")
					commented = make([]string, 0, 1)
				}

				lang, query := matches[1], matches[2]
				endpoint := fmt.Sprintf(wikilink, lang, query)

				wiki := wikiData(endpoint)

				commentBody := strings.TrimSpace(wiki.Query.Pages[0].Extract)

				// Format the output
				commentBody = strings.Replace(commentBody, "\n", "\n\n>", -1)

				// Only want 2 paragraphs
				paragraphs := strings.Split(commentBody, ">")

				if len(paragraphs) >= 2 {
					commentBody = fmt.Sprintf("%s >%s", paragraphs[0], paragraphs[1])
				}

				if len(commentBody) > 0 {

					commentTitle, commentLink := wiki.Query.Pages[0].Title, replaceUrl.Replace(matches[0])

					comment := fmt.Sprintf("**[%s](%s)** \n\n ---  \n\n>%s \n\n --- \n\n %s", commentTitle, commentLink, commentBody, commentInfo)

					commentparams := make(map[string]interface{})
					commentparams["text"] = comment
					commentparams["parent"] = id

					postNewComment(client, commentparams)
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
}

func wikiData(link string) WikipediaResponse {
	resp, err := http.Get(link)
	if err != nil {
		panic(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var wiki WikipediaResponse

	err = json.Unmarshal(body, &wiki)
	if err != nil {
		panic(err)
	}

	return wiki
}

func contains(s []string, b string) bool {
	sort.Strings(s)

	i := sort.SearchStrings(s, b)

	if i >= len(s) || s[i] != b {
		return false
	}
	return true
}

func getBlacklist(filename string) []string {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("Unable to read blacklist %s", filename)
	}

	b := bytes.ToLower(contents)

	s := string(b[:])

	list := strings.Split(s, "\n")

	return list
}
