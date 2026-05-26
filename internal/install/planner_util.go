package install

import "os"

func statFile(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
