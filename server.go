package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"github.com/altid/libs/fs"
	"github.com/mmcdole/gofeed"
)

type server struct {
	feeds string
	ctrl  *fs.Control
	last  map[string]time.Time
	run   chan struct{}
	sync.Mutex
}

func (s *server) Run(ctrl *fs.Control, cmd *fs.Command) error {
	switch cmd.Name {
	case "open":
		// TODO: Create document for given RSS/ATOM feed
	case "close":
		return ctrl.DeleteBuffer(cmd.Args[0], "document")
	case "refresh":
		s.run <- struct{}{}
		return nil
	case "subscribe":
		fp, err := os.OpenFile(s.feeds, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}

		fp.WriteString(cmd.Args[0] + "\n")
		return fp.Close()
	case "unsubscribe":
		current, err := ioutil.ReadFile(s.feeds)
		if err != nil {
			return err
		}

		// Use the chance here to clean up erroneous newlines
		after := bytes.ReplaceAll(current, []byte(cmd.Args[0]), []byte(""))
		after = bytes.ReplaceAll(after, []byte("\n\n"), []byte("\n"))
		if len(after) > 0 && after[0] == '\n' {
			after = after[1:]
		}

		return ioutil.WriteFile(s.feeds, after, 0644)
	default:
		return fmt.Errorf("%s not supported", cmd.Args[0])
	}

	return nil
}

func (s *server) Quit() {
}

func (s *server) setup(ctrl *fs.Control, feeds string) error {
	s.feeds = feeds
	s.ctrl = ctrl
	s.run = make(chan struct{})
	s.last = make(map[string]time.Time)

	return s.populate()
}

func (s *server) populate() error {
	ctx := s.ctrl.Context()

	ew, err := s.ctrl.ErrorWriter()
	if err != nil {
		return err
	}

	fp, err := os.Open(s.feeds)
	if err != nil {
		return err
	}

	for feed := range listOldFeeds(ctx, bufio.NewReader(fp), ew) {
		if e := s.writeFeeds(feed); e != nil {
			fmt.Fprintf(ew, "%v\n", e)
		}
	}

	fp.Close()
	return nil
}

func (s *server) listen(timeout int) {
	ew, err := s.ctrl.ErrorWriter()
	if err != nil {
		log.Fatal(err)
	}

	defer ew.Close()

	if e := listen(s, ew, timeout); e != nil {
		log.Fatal(e)
	}
}

func (s *server) writeFeeds(feed *gofeed.Feed) error {
	// Oddly, we can only check for nillable feeds
	// here and not in find. Oh well, it's a quick
	// short circuit still
	if feed == nil {
		return nil
	}

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

		// Add a time to our map for the item
		s.last[feed.Title] = time.Now()
	}

	return nil
}

func listen(s *server, ew io.Writer, timeout int) error {
	ctx := s.ctrl.Context()

	// Break into functions
	for {
		if *debug {
			log.Println("Fetching feeds...")
		}

		fp, err := os.Open(s.feeds)
		if err != nil {
			log.Fatal(err)
		}

		for feed := range listNewFeeds(ctx, bufio.NewReader(fp), ew, timeout) {
			if e := s.writeFeeds(feed); e != nil {
				return e
			}
		}

		fp.Close()

		// Allow unlocking for subscribe
		// Otherwise just chill for an hour inbetween fetches
		select {
		case <-time.After(time.Duration(timeout) * time.Hour):
		case <-s.ctrl.Context().Done():
			break
		case <-s.run:
		}
	}
}
