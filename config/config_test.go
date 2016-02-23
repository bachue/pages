package config

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "config")
	assert.Nil(t, err)
	defer func() {
		err := os.RemoveAll(dir)
		assert.Nil(t, err)
	}()
	config := `
production:
    sshd:
        host: configdb
        port: 22
        private_key: PRIVATEKEYPRIVATEKEYPRIVATEKEY1
development:
    sshd:
        host: localhost
        port: 2200
        private_key: PRIVATEKEYPRIVATEKEYPRIVATEKEY2
test:
    sshd:
        host: localhost
        port: 2201
        private_key: PRIVATEKEYPRIVATEKEYPRIVATEKEY3
    `
	configPath := dir + "/config.yml"
	ioutil.WriteFile(configPath, []byte(config), 0600)
	Candidates = []string{configPath}

	os.Setenv("PAGES_ENV", "production")
	err = Load()
	assert.Nil(t, err)
	assert.EqualValues(t, Current.Sshd.ListenHost, "configdb")
	assert.EqualValues(t, Current.Sshd.ListenPort, 22)
	assert.EqualValues(t, Current.Sshd.PrivateKey, "PRIVATEKEYPRIVATEKEYPRIVATEKEY1")

	os.Setenv("PAGES_ENV", "development")
	err = Load()
	assert.Nil(t, err)
	assert.EqualValues(t, Current.Sshd.ListenHost, "localhost")
	assert.EqualValues(t, Current.Sshd.ListenPort, 2200)
	assert.EqualValues(t, Current.Sshd.PrivateKey, "PRIVATEKEYPRIVATEKEYPRIVATEKEY2")
}
