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
		fmt.Errorf("%s not supported", cmd.Args[0])
	}

	return nil
}

func (s *server) Quit() {
}

func (s *server) setup(ctrl *fs.Control, feeds string) error {
	s.feeds = feeds
	s.ctrl = ctrl
	s.run = make(chan struct{})

	return nil
}

func (s *server) listen(timeout int) {
	s.last = make(map[string]time.Time)
	p := gofeed.NewParser()

	ew, err := s.ctrl.ErrorWriter()
	if err != nil {
		log.Fatal(err)
	}

	defer ew.Close()

	if e := listen(s, ew, p, timeout); e != nil {
		log.Fatal(e)
	}
}

func listen(s *server, ew io.Writer, p *gofeed.Parser, timeout int) error {
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

		for feed := range listNewFeeds(ctx, bufio.NewReader(fp), ew) {
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
