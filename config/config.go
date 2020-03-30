// Config is put into a different package to prevent cyclic imports in case
// it is needed in several locations

package config

import (
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Period             time.Duration `config:"period"`
	ExcludedSubsystems []string      `config:"excluded.subsystems"`
	CacheDir           string        `config:"cache.dir"`
}

var DefaultConfig = Config{
	Period:             1 * time.Second,
	ExcludedSubsystems: []string{},
	CacheDir:           getDefaultCacheDir(),
}

func getDefaultCacheDir() string {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(executable)
}
