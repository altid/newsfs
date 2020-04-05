package main

import (
	"flag"
	"log"
	"os"
	"path"

	"github.com/altid/libs/config"
	"github.com/altid/libs/config/types"
	"github.com/altid/libs/fs"
)

var (
	mtpt    = flag.String("p", "/tmp/altid", "Path for filesystem")
	srv     = flag.String("s", "news", "Name of service")
	cfgfile = flag.String("c", "", "Directory of configuration file")
	debug   = flag.Bool("d", false, "enable debug logging")
	setup   = flag.Bool("conf", false, "Run configuration setup")
)

func main() {
	flag.Parse()
	if flag.Lookup("h") != nil {
		flag.Usage()
		os.Exit(1)
	}

	shr, err := fs.UserShareDir()
	if err != nil {
		log.Fatal(err)
	}

	conf := &struct {
		Sleep  int                 `altid:"sleep,prompt:Hours to wait between fetching feeds"`
		Feeds  string              `altid:"feeds_file,prompt:Path to file containing list of feeds"`
		Listen types.ListenAddress `altid:"listen_address,no_prompt"`
		Logdir types.Logdir        `altid:"logdir,no_prompt"`
	}{1, path.Join(shr, "Altid", "feeds"), "none", "none"}

	if *setup {
		if e := config.Create(conf, *srv, *cfgfile, *debug); e != nil {
			log.Fatal(e)
		}

		os.Exit(0)
	}

	if e := config.Marshal(conf, *srv, *cfgfile, *debug); e != nil {
		log.Fatal(e)
	}

	s := &server{}

	ctrl, err := fs.New(s, string(conf.Logdir), *mtpt, *srv, "feed", *debug)
	if err != nil {
		log.Fatal(err)
	}

	defer ctrl.Cleanup()
	ctrl.SetCommands(Commands...)
	ctrl.CreateBuffer("main", "feed")

	if conf.Sleep < 1 {
		log.Fatal("Sub-hour timeouts for fetching new feeds is unsupported")
	}

	if e := s.setup(ctrl, conf.Feeds); e != nil {
		log.Fatal(e)
	}

	go s.listen(conf.Sleep)

	if e := ctrl.Listen(); e != nil {
		log.Fatal(e)
	}
}
