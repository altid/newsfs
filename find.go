package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"sort"
	"time"

	"github.com/mmcdole/gofeed"
)

func listOldFeeds(ctx context.Context, rd *bufio.Reader, ew io.Writer) chan *gofeed.Feed {
	results := make(chan *gofeed.Feed)

	go func(results chan *gofeed.Feed) {
		for {
			feed, err := listFeeds(ctx, rd)
			switch err {
			// Make sure we catch the very last item
			case io.EOF:
				results <- feed
				close(results)
				return
			case nil:
				if feed != nil {
					results <- feed
				}
			default:
				// We shouldn't error out on a single malformed feed
				// but we should log it
				fmt.Fprintf(ew, "%v\n", err)
			}
		}
	}(results)

	return results
}

func listNewFeeds(ctx context.Context, rd *bufio.Reader, ew io.Writer, timeout int) chan *gofeed.Feed {
	results := make(chan *gofeed.Feed)
	current := time.Now().Add(time.Hour * time.Duration(timeout*-2))

	go func(results chan *gofeed.Feed, current time.Time) {
		for {
			feed, err := listFeeds(ctx, rd)
			switch err {
			case io.EOF:
				// The last entry is older than twice our timeout we really won't have
				// to process. The only time this would become an issue is on very short
				// (sub 1-hour timeouts) with high high amounts of feeds
				if feed != nil {
					last := feed.Items[feed.Len()-1].PublishedParsed
					if last != nil && last.After(current) {
						results <- feed
					}
				}

				close(results)
				return
			case nil:
				if feed != nil {
					last := feed.Items[feed.Len()-1].PublishedParsed
					if last != nil && last.After(current) {
						results <- feed
					}
				}
			default:
				// We shouldn't error out on a single malformed feed
				// but we should log it
				fmt.Fprintf(ew, "%v\n", err)
			}
		}
	}(results, current)

	return results
}

// Return a feed from a line in our file
// Will return EOF when encountered, as well as good data; or nil and any other error encountered
func listFeeds(ctx context.Context, rd *bufio.Reader) (*gofeed.Feed, error) {
	p := gofeed.NewParser()

	// Stagger our fetches - for incredibly big lists, this may overlap
	// with the timeout
	n, _ := rand.Int(rand.Reader, big.NewInt(1000))
	time.Sleep(time.Duration(n.Int64()) * time.Millisecond)

	url, err := rd.ReadString('\n')
	if err != nil && err != io.EOF {
		if err != io.EOF {
			return nil, err
		}
	}

	// Just ignore the short reads
	if len(url) < 3 {
		return nil, err
	}

	url = url[:len(url)-1]
	c, cancel := context.WithTimeout(ctx, time.Second*3)

	feed, parseErr := p.ParseURLWithContext(url, c)
	if parseErr != nil {
		cancel()
		return nil, parseErr
	}

	cancel()

	// No feed entries, short circuit
	if feed.Len() < 1 {
		return nil, err
	}

	sort.Sort(feed)
	return feed, err
}
