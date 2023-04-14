// simple Go command to convert current eurekalert news (all pages, usually two days) to RSS
// see https://emacs.ch/@cpbotha/110196947212868015

// to build and deploy:
// # don't link with libc (cgo is enabled by default here)
// export CGO_ENABLED=0
// go build
// scp eurekalert-rss web@web20.vxlabs.com:~/apps/cpbotha.net/feeds/
// then stick in cronjob -- I run it every two hours
// you should be able to find the results at https://cpbotha.net/feeds/eurekalert/rss.xml

package main

import (
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/feeds"
	"github.com/n0madic/site2rss"
)

func genEnclosure(image string) *feeds.Enclosure {
	// modified from site2rss to retrieve the content-type from the resource response
	// see in rss.go that we need both length and type for the enclosure
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

func getNewDocumentFromURL(sourceURL string) (*goquery.Document, error) {
	// copied from site2rss.go else we can't use here
	res, err := http.Get(sourceURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	doc.Url, err = url.Parse(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url")
	}

	return doc, nil
}

// Use pagination links to harvest press release links from all pages
//
// This one also ignores the MaxFeedItems setting, as we just want all of them thanks
func getLinksMulti(s *site2rss.Site2RSS, linkPattern string) *site2rss.Site2RSS {
	nextUrl := s.SourceURL
	for !(nextUrl == nil) {
		//fmt.Println("about to scrape", nextUrl)
		sourceDoc, err := getNewDocumentFromURL(nextUrl.String())
		// we start with the assumption that there is no next page
		nextUrl = nil
		if err == nil {
			links := sourceDoc.Find(linkPattern).Map(func(i int, sel *goquery.Selection) string {
				link, _ := sel.Attr("href")
				return s.AbsoluteURL(link)
			})

			// we concatenate links from all pages that we find
			s.Links = append([]string(s.Links), links...)

			// the site is pretty terrible; best way for us to find the "next" page link
			// is via the icon
			urlStr := sourceDoc.Find("ul.pagination li a i.fa-angle-right").First().Parent().AttrOr("href", "")
			if urlStr != "" {
				nextUrl, err = url.Parse(s.AbsoluteURL(urlStr))
				if err != nil {
					nextUrl = nil
				}
			}
		}
	}
	//fmt.Println("Converted N links:", len(s.Links))
	return s
}

// Custom function to extract metadata from page
//
// This was necessary to skip monday date parsing, and also to fix image handling
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
		// until we figure out some way to get good de-HTMLified summary, just skip the description
		//item.Description, _ = doc.Find(opts.Description).Html()
		// this will automatically wrap the HTML contents in CDATA which is what we want
		item.Content, _ = doc.Find(opts.Description).Html()
	}
	return item
}

func main() {
	s2rss := site2rss.NewFeed("https://www.eurekalert.org/news-releases/browse/all", "EurekAlert RSS generator by cpbotha.net")

	// can't chain getLinksMulti, because could not define extension method in this non-local package
	getLinksMulti(s2rss, "article.post > a").
		SetParseOptions(&site2rss.FindOnPage{
			Title:       "h1.page_title",
			Author:      "p.meta_institute",
			Date:        "div.release_date > time",
			DateFormat:  "02-Jan-2006",
			Description: "div.entry",
			Image:       "figure.thumbnail img",
		}).
		GetItemsFromLinks(handlePage)

	rss, err := s2rss.GetRSS()

	if err != nil {
		fmt.Println("Unable to convert EurekAlert to RSS:", err)
		os.Exit(1)
	} else {
		fmt.Println(rss)
	}
}
