package config

import (
	"os"
	"regexp"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestEnvironement(t *testing.T) {
	assert := require.New(t)

	// parse each config file and ensure environment is well set.
	// we also ensure we can parse the config file without error.
	// look for all files with config-XXX.toml in current dir and capture XXX with a regexp.

	// For example for these file names, the regexp captures the following:
	// config-integration-development.toml 	--> integration-development
	// config-integration-full.toml 		--> integration-full
	re := regexp.MustCompile(`config-(.*)\.toml`)

	// get all files in current dir
	files, err := os.ReadDir(".")
	assert.NoError(err)

	count := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		matches := re.FindStringSubmatch(file.Name())
		if len(matches) == 0 {
			continue
		}
		count++
		t.Logf("loading config file %s - %s", file.Name(), matches[1])

		t.Run(matches[1], func(t *testing.T) {
			viper.Set("assets_dir", "../prover-assets")
			config, err := NewConfigFromFile(file.Name())
			assert.NoError(err, "when processing %s", file.Name())

			// check that the environment is set
			assert.Equal(matches[1], config.Environment)
		})
	}

	assert.NotEqual(0, count, "no config file found")
}