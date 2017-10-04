package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/codegangsta/cli"
	"github.com/jakewarren/gif/image"
	"github.com/jakewarren/gif/store"
)

func ListCommand(c *cli.Context) {
	s := getStore()
	defer s.Close()

	filter := listFilter(c)

	images, err := s.List(filter)
	if err != nil {
		fmt.Println("Error while fetching: " + err.Error())
		os.Exit(1)
	}

	fmt.Printf("%v images\n", len(images))

	image.PrintAll(images)
}

func UrlCommand(c *cli.Context) {
	s := getStore()
	defer s.Close()

	filter := orderAndLimit(store.RemoteFilter{Filter: typeFilter(c)}, c)

	images, err := s.List(filter)
	if err != nil {
		fmt.Println("Error while fetching: " + err.Error())
		os.Exit(1)
	}

	for _, image := range images {
		fmt.Println(image.Url)
	}
}

func PathCommand(c *cli.Context) {
	s := getStore()
	defer s.Close()

	filter := orderAndLimit(typeFilter(c), c)

	images, err := s.List(filter)
	if err != nil {
		fmt.Println("Error while fetching: " + err.Error())
		os.Exit(1)
	}

	for _, image := range images {
		fmt.Println(s.PathFor(&image))
	}
}

func OpenCommand(c *cli.Context) {
	s := getStore()
	defer s.Close()

	filter := orderAndLimit(typeFilter(c), c)

	images, err := s.List(filter)
	if err != nil {
		fmt.Println("Error while fetching: " + err.Error())
		os.Exit(1)
	}

	for _, image := range images {
		openImage(s.PathFor(&image))
	}
}

func openImage(path string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, path)
	return exec.Command(cmd, args...).Start()
}
