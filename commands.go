package main

import "github.com/altid/libs/fs"

var Commands = []*fs.Command{
	{
		Name: "subscribe",
		Args: []string{"<url>"},
		Heading: fs.DefaultGroup,
		Description: "Subscribe to a feed/channel",
	},
	{
		Name: "unsubscribe",
		Args: []string{"<url>"},
		Heading: fs.DefaultGroup,
		Description: "Unsubscribe from a feed/channel",
	},
}