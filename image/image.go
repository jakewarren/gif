package image

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	filetype "gopkg.in/h2non/filetype.v1"
)

type Image struct {
	Id      string
	Url     string
	Tags    []string
	Data    []byte
	AddedAt *time.Time
	Size    uint64
	Type    string
}

func (i *Image) IsHydrated() bool {
	return len(i.Data) > 0
}

func (i *Image) SetAddedAtFromString(addedAt string) (err error) {
	value, err := time.Parse(time.RFC3339, addedAt)
	if err == nil {
		i.AddedAt = &value
	}
	return
}

func FromUrl(url string) (*Image, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s", response.Proto, response.Status)
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	image := FromData(data)
	image.Url = url
	return image, nil
}

func FromFile(path string) (*Image, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	image := FromData(data)
	return image, nil
}

func FromData(data []byte) *Image {
	image := &Image{Data: data}
	image.Id = image.generateId()
	image.Size = uint64(len(data))

	kind, _ := filetype.Match(data)

	image.Type = kind.Extension

	return image
}

func (image *Image) generateId() string {
	h := sha1.New()
	h.Write(image.Data)
	return hex.EncodeToString(h.Sum(nil))
}
