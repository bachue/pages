package config

import (
    "os"
    "io/ioutil"
    "testing"
    "gopkg.in/stretchr/testify.v1/assert"
)

func TestLoadConfig(t *testing.T) {
    dir, err := ioutil.TempDir(os.TempDir(), "config")
    if err != nil {
        t.Fatal(err)
    }
    defer func() {
        err := os.RemoveAll(dir)
        if err != nil {
            t.Fatal(err)
        }
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
    Candidates = []string { configPath }

    os.Setenv("PAGES_ENV", "production")
    LoadConfig()
    assert.EqualValues(t, CurrentConfig.Sshd.ListenHost, "configdb")
    assert.EqualValues(t, CurrentConfig.Sshd.ListenPort, 22)
    assert.EqualValues(t, CurrentConfig.Sshd.PrivateKey, "PRIVATEKEYPRIVATEKEYPRIVATEKEY1")

    os.Setenv("PAGES_ENV", "development")
    LoadConfig()
    assert.EqualValues(t, CurrentConfig.Sshd.ListenHost, "localhost")
    assert.EqualValues(t, CurrentConfig.Sshd.ListenPort, 2200)
    assert.EqualValues(t, CurrentConfig.Sshd.PrivateKey, "PRIVATEKEYPRIVATEKEYPRIVATEKEY2")
}
