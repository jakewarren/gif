package main

import (
	"bufio"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/jakewarren/gif/store"
	"os"
	"regexp"
)

func ExportCommand(c *cli.Context) {
	s := getStore()
	defer s.Close()

	var targetFile *os.File
	var err error

	output := c.String("output")
	if output == "-" {
		targetFile = os.Stdout
	} else {
		targetFile, err = os.Create(output)
		if err != nil {
			fmt.Println("Could not create file: " + err.Error())
			os.Exit(1)
		}
	}

	// Detect file extension and enable full export
	exportFiles := c.Bool("bundle") || regexp.MustCompile(`(?:\.tar\.gz|\.gifb)\z`).MatchString(output)

	// Exporting local gifs makes no sense when you don't include files
	var filter store.Filter
	if exportFiles {
		filter = store.NullFilter{}
	} else {
		filter = store.RemoteFilter{Filter: store.NullFilter{}}
	}

	writer := bufio.NewWriter(targetFile)
	defer writer.Flush()

	err = s.Export(writer, filter, exportFiles)
	if err != nil {
		fmt.Println("Export error: " + err.Error())
		os.Exit(1)
	}
}
