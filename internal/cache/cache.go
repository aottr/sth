package cache

import (
	"encoding/gob"
	"os"
	"time"
)

func CacheIsFresh(path string, ttl time.Duration) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(fi.ModTime()) < ttl

}

func LoadCache(path string, out any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := gob.NewDecoder(f)
	return dec.Decode(out)
}

func SaveCache(path string, v any) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
