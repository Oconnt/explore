package utils

import (
	"os"
	"path/filepath"
)

func CheckPid(pid string) bool {
	path := filepath.Join("/proc", pid)
	_, err := os.Stat(path)
	return err == nil
}
