// # don't link with libc (cgo is enabled by default here)
// export CGO_ENABLED=0
// go build
// scp eurekalert-rss web@web20.vxlabs.com:~/apps/cpbotha.net/feeds/

package main

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path"
	"time"

	"strings"

	"github.com/gorilla/feeds"
	"github.com/n0madic/site2rss"
)

func genEnclosure(image string) *feeds.Enclosure {
	// see in rss.go that we need length and type
	contentType := mime.TypeByExtension(path.Ext(image))
	if contentType == "" {
		resp, err := http.Head(image)
		if err == nil {
			contentType = resp.Header.Get("Content-Type")
		}
		defer resp.Body.Close()
	}
	return &feeds.Enclosure{
		Length: "-1",
		Type:   contentType,
		Url:    image,
	}
}

func handlePage(doc *site2rss.Document, opts *site2rss.FindOnPage) *site2rss.Item {
	item := &site2rss.Item{
		Link: &site2rss.Link{Href: doc.Url.String()},
		Id:   doc.Url.String(),
	}
	if opts.Author != "" {
		item.Author = &site2rss.Author{Name: strings.TrimSpace(doc.Find(opts.Author).First().Text())}
	}
	if opts.Title != "" {
		item.Title = strings.TrimSpace(doc.Find(opts.Title).First().Text())
	}
	if opts.Image != "" {
		imageStr := strings.TrimSpace(doc.Find(opts.Image).AttrOr("src", ""))
		if imageStr != "" {
			// we currently pass length as -1, but type as "" because there are no extensions
			item.Enclosure = genEnclosure(imageStr)
		}
	}
	if opts.Date != "" {
		dateStr := strings.TrimSpace(doc.Find(opts.Date).First().Text())
		if dateStr != "" {
			t, err := time.ParseInLocation("02-Jan-2006", dateStr, time.UTC)
			if err == nil {
				item.Created = t
			}
		}
	}
	if opts.Description != "" {
		// until we figure out some way to get good summary, just skip the description
		//item.Description, _ = doc.Find(opts.Description).Html()
		// this will automatically wrap the contents in CDATA which is what we want
		item.Content, _ = doc.Find(opts.Description).Html()
	}
	return item
}

func main() {
	rss, err := site2rss.NewFeed("https://www.eurekalert.org/news-releases/browse/all", "EurekAlert RSS generator by cpbotha.net").
		GetLinks("article.post > a").
		SetParseOptions(&site2rss.FindOnPage{
			Title:       "h1.page_title",
			Author:      "p.meta_institute",
			Date:        "div.release_date > time",
			DateFormat:  "02-Jan-2006",
			Description: "div.entry",
			Image:       "figure.thumbnail img",
		}).
		GetItemsFromLinks(handlePage).
		GetRSS()
	if err != nil {
		fmt.Println("Unable to convert EurekAlert to RSS:", err)
		os.Exit(1)
	} else {
		fmt.Println(rss)
	}
}
