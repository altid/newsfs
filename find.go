package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/mmcdole/gofeed"
)

func listNewFeeds(ctx context.Context, rd *bufio.Reader, ew io.Writer) chan *gofeed.Feed {
	p := gofeed.NewParser()
	results := make(chan *gofeed.Feed)

	go func() {
		for {
			// Stagger our fetches - for incredibly big lists, this may overlap
			// with the timeout
			n, _ := rand.Int(rand.Reader, big.NewInt(1000))
			time.Sleep(time.Duration(n.Int64()) * time.Millisecond)

			url, err := rd.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(ew, "%v\n", err)
				}

				break
			}

			// Just ignore the short reads
			if len(url) < 3 {
				continue
			}

			url = url[:len(url)-1]
			c, cancel := context.WithTimeout(ctx, time.Second*3)

			feed, err := p.ParseURLWithContext(url, c)
			if err != nil {
				fmt.Fprintf(ew, "%v\n", err)
				continue
			}

			cancel()
			results <- feed
		}

		close(results)
	}()

	return results
}

func (s *server) writeFeeds(feed *gofeed.Feed) error {
	current, ok := s.last[feed.Title]
	if !ok {
		s.last[feed.Title] = time.Time{}
	}

	// Catch older posts
	// Timestamps here can be empty, which will fatal the service
	// So we check for nil, then skip anything older than our last timestamp
	if feed.PublishedParsed != nil && current.After(*feed.PublishedParsed) {
		if feed.UpdatedParsed == nil {
			return nil
		}

		// Feed updated before our last check, skip
		if current.After(*feed.UpdatedParsed) {
			return nil
		}
	}

	// Total Drama Island was a sorta weird show
	// I think I watched it enough with my son to say
	// it really wasn't good. It was just silly for silly's sake.
	// Compared to kids shows like, Teen Titans: Go! which have
	// incredibly creative writers, talented voice actors, and
	// skilled animators it just seems like timeslot filler.
	for _, item := range feed.Items {
		if item.PublishedParsed.Before(current) {
			if item.UpdatedParsed.Before(current) {
				continue
			}
		}

		mw, err := s.ctrl.MainWriter("main", "feed")
		if err != nil {
			return err
		}

		fmt.Fprintf(mw, "[%s](%s)\n", item.Title, item.Link)
		mw.Close()

		if item.UpdatedParsed == nil {
			s.last[feed.Title] = *item.PublishedParsed
			s.last[feed.Title].Add(time.Second)
		} else {
			s.last[feed.Title] = *item.UpdatedParsed
			s.last[feed.Title].Add(time.Second)
		}
	}

	return nil
}
