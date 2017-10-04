package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/jakewarren/gif/image"
	"github.com/jakewarren/gif/store"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var extensionWhitelist = [...]string{"gif", "jpeg", "jpg", "png", "webp"}
var extensionWhitelistMap map[string]struct{} = make(map[string]struct{})

func computeWhitelist() {
	for _, ext := range extensionWhitelist {
		extensionWhitelistMap[ext] = struct{}{}
	}
}

func ImportCommand(c *cli.Context) {
	location := c.Args().First()

	ltype, err := parseLocation(location)

	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	s := getStore()
	defer s.Close()

	switch ltype {
	case urlLocation:
		importFromUrl(s, location)
	case fileLocation:
		importFromFile(s, location)
	case directoryLocation:
		importDirectory(s, filepath.Clean(location), c.Bool("recursive"))
	}
}

func importFromUrl(s *store.Store, location string) {
	response, err := http.Get(location)
	if err != nil {
		fmt.Println("Import Error: " + err.Error())
		os.Exit(1)
	}

	if response.StatusCode >= 300 {
		fmt.Printf("Import Error: %s %s\n", response.Proto, response.Status)
		os.Exit(1)
	}

	defer response.Body.Close()

	err = importFromReader(s, response.Body)
	if err != nil {
		fmt.Println("Import Error: " + err.Error())
		os.Exit(1)
	}
}

func importFromFile(s *store.Store, location string) {
	file, err := os.Open(location)
	if err != nil {
		fmt.Println("Import Error: " + err.Error())
		os.Exit(1)
	}
	defer file.Close()

	if err = importFromReader(s, file); err != nil {
		fmt.Println("Import Error: " + err.Error())
		os.Exit(1)
	}
}

func importDirectory(s *store.Store, location string, recursive bool) {
	if len(extensionWhitelistMap) == 0 {
		computeWhitelist()
	}

	writer := image.DefaultWriter()
	defer writer.Flush()

	entries, err := ioutil.ReadDir(location)
	if err != nil {
		fmt.Println("Import Error: " + err.Error())
		os.Exit(1)
	}

	for _, entry := range entries {
		currentPath := filepath.Join(location, entry.Name())

		if entry.IsDir() {
			if recursive {
				importDirectory(s, currentPath, recursive)
			}
		} else {
			extensionWithDot := filepath.Ext(currentPath)
			if extensionWithDot == "" {
				continue
			}

			extension := extensionWithDot[1:]

			if _, ok := extensionWhitelistMap[extension]; ok {
				img, err := image.FromFile(currentPath)
				if err != nil {
					fmt.Fprintf(writer, "[error]\t%s: %s\f", currentPath, err.Error())
					continue
				}

				img.Tags = []string{entry.Name()}

				AddInterface(s, writer, img, false)
			}
		}
	}
}

func importFromReader(s *store.Store, reader io.Reader) error {
	bufferedReader := bufio.NewReader(reader)

	// The gzip header has 10 bytes, so let's peek the next 10 bytes and check if the header is OK
	testBytes, err := bufferedReader.Peek(10)
	if err != nil {
		return err
	}
	testBuffer := bytes.NewBuffer(testBytes)
	_, err = gzip.NewReader(testBuffer)
	if err == nil {
		gzipReader, _ := gzip.NewReader(bufferedReader)
		return importBundle(s, gzipReader)
	}

	// Check if it's valid JSON
	if images, err := store.ParseMetadata(bufferedReader); err == nil {
		importUrls(s, images)
		return nil
	}

	return errors.New("Invalid import file")
}

func importBundle(s *store.Store, reader *gzip.Reader) error {
	tarReader := tar.NewReader(reader)

	writer := image.DefaultWriter()
	defer writer.Flush()

	dirname, err := ioutil.TempDir("", "gif-import")
	if err != nil {
		return err
	}

	imageMap := map[string]store.ExportedImage{}
	metadataRead := false
	idRegexp := regexp.MustCompile(`\A[0-9a-fA-F]{40}\z`)
	imageQueue := []string{}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Read metadata
		if header.Name == "gif.json" {
			images, err := store.ParseMetadata(tarReader)
			if err != nil {
				return err
			}
			metadataRead = true

			// Put the images into a map for faster access
			for _, exported := range images {
				imageMap[exported.Id] = exported
			}

			// Empty the queue, if any
			for _, queuedImageId := range imageQueue {
				tempPath := path.Join(dirname, queuedImageId+".gif")
				img, err := image.FromFile(tempPath)
				if err != nil {
					fmt.Fprintf(writer, "[error]\t%s\t%s\f", queuedImageId[:8], err.Error())
					continue
				}

				if queuedImageId != img.Id {
					fmt.Fprintf(writer, "[warn]\t%s\tID mismatch, new ID: %s\f", queuedImageId[:8], img.Id)
				}

				queuedImage := imageMap[queuedImageId]
				img.Url = queuedImage.Url
				img.Tags = queuedImage.Tags

				if queuedImage.AddedAt != "" {
					if err = img.SetAddedAtFromString(queuedImage.AddedAt); err != nil {
						fmt.Fprintf(writer, "[warn]\t%s\tCould not set addition date: %s\f", queuedImage.Id[:8], queuedImage.AddedAt)
					}
				}

				AddInterface(s, writer, img, false)
			}
			imageQueue = nil
			os.RemoveAll(dirname)

			continue
		}

		imageId := strings.TrimSuffix(header.Name, ".gif")
		if !idRegexp.MatchString(imageId) {
			// Skip files that don't match
			continue
		}

		// TODO: Check if ID exists

		if metadataRead {
			// Read and add the current image
			exported := imageMap[imageId]

			buffer := new(bytes.Buffer)
			if _, err := io.Copy(buffer, tarReader); err != nil {
				fmt.Fprintf(writer, "[error]\t%s\t%s\f", imageId[:8], err.Error())
				continue
			}

			img := image.FromData(buffer.Bytes())

			if exported.Id != img.Id {
				fmt.Fprintf(writer, "[warn]\t%s\tID mismatch, new ID: %s\f", exported.Id[:8], img.Id)
			}

			img.Url = exported.Url
			img.Tags = exported.Tags

			if exported.AddedAt != "" {
				_ = img.SetAddedAtFromString(exported.AddedAt)

			}

			AddInterface(s, writer, img, false)
		} else {
			// Put the file into temporary storage which will be merged into the store later
			tempPath := path.Join(dirname, header.Name)
			tempFile, err := os.Create(tempPath)
			if err != nil {
				fmt.Fprintf(writer, "[error]\t%s\t%s\f", imageId[:8], err.Error())
				continue
			}
			defer tempFile.Close()

			if _, err := io.Copy(tempFile, tarReader); err != nil {
				fmt.Fprintf(writer, "[error]\t%s\t%s\f", imageId[:8], err.Error())
				continue
			}

			imageQueue = append(imageQueue, imageId)
		}
	}

	if !metadataRead {
		os.RemoveAll(dirname)
		return errors.New("Archive does not contain a gif.json file.")
	}

	return nil
}

func importUrls(s *store.Store, images []store.ExportedImage) {
	writer := image.DefaultWriter()
	defer writer.Flush()

	for _, exported := range images {
		img, err := image.FromUrl(exported.Url)
		if err != nil {
			fmt.Fprintf(writer, "[error]\t%s\t%s\f", exported.Id[:8], err.Error())
			continue
		}

		if exported.Id != img.Id {
			fmt.Fprintf(writer, "[warn]\t%s\tID mismatch, new ID: %s\f", exported.Id[:8], img.Id)
		}

		img.Tags = exported.Tags

		if exported.AddedAt != "" {
			if err = img.SetAddedAtFromString(exported.AddedAt); err != nil {
				fmt.Fprintf(writer, "[warn]\t%s\tCould not set addition date: %s\f", exported.Id[:8], exported.AddedAt)
			}
		}

		AddInterface(s, writer, img, false)
	}
}
