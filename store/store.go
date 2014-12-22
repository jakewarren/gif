package store

import (
	"github.com/evoL/gif/config"
	. "github.com/evoL/gif/image"
	"io/ioutil"
	"os"
	"path"
)

type Store struct {
	path string
}

func Default() (*Store, error) {
	return New(config.StorePath())
}

func New(path string) (*Store, error) {
	store := &Store{path: path}
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}
	return store, nil
}

func (store *Store) Save(image *Image) error {
	if err := ioutil.WriteFile(store.PathFor(image), image.Data, 0644); err != nil {
		return err
	}
	return nil
}

func (store *Store) PathFor(image *Image) string {
	return path.Join(store.path, image.Id+".gif")
}

func (store *Store) Contains(image *Image) bool {
	_, err := os.Stat(store.PathFor(image))
	return err == nil
}

func (store *Store) Purge() error {
	return os.RemoveAll(store.path)
}
