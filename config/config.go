package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

type Fuse struct {
	GitRepoDir string `yaml:"repo_dir"`
	Debug      bool
}

type Sshd struct {
	ListenHost string `yaml:"host"`
	ListenPort int32  `yaml:"port"`
	PrivateKey string `yaml:"private_key"`
	MaxClient  int32  `yaml:"max_client"`
	ShellPath  string `yaml:"shell"`
}

type Syslog struct {
	Protocol string
	Host     string
	Level    string
	Tag      string
}

type Log struct {
	Local  string
	Level  string
	Syslog Syslog
}

type Environmental struct {
	Sshd Sshd
	Fuse Fuse
	Log  Log
}

type Config struct {
	Development Environmental
	Production  Environmental
	Test        Environmental
}

var Current *Environmental
var Candidates = []string{
	os.Getenv("PAGES_CONFIG"),
	"/etc/pages.yml",
	"/etc/pages.yaml",
	"etc/pages.yml",
	"etc/pages.yaml",
}

func Load() error {
	config := Config{}
	env := getEnvironment()
	content, err := loadConfigFile()
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(content, &config)
	if err != nil {
		return err
	}
	switch env {
	case "development":
		Current = &config.Development
	case "test":
		Current = &config.Test
	case "production":
		Current = &config.Production
	default:
		return fmt.Errorf("Invalid environment `%v`", env)
	}
	if Current.Sshd.ListenPort == 0 {
		Current.Sshd.ListenPort = 22
	}
	if Current.Sshd.PrivateKey == "" {
		return fmt.Errorf("Config Error: private key must be set")
	}
	if Current.Sshd.MaxClient == 0 {
		Current.Sshd.MaxClient = 256
	}
	if Current.Sshd.ShellPath == "" {
		Current.Sshd.ShellPath = "/bin/bash"
	}
	if Current.Log.Local == "" {
		Current.Log.Local = "stderr"
	}
	if Current.Log.Level == "" {
		Current.Log.Level = "DEBUG"
	}
	Current.Log.Level = strings.ToUpper(Current.Log.Level)
	if Current.Log.Syslog.Level == "" {
		Current.Log.Syslog.Level = "DEBUG"
	}
	Current.Log.Syslog.Level = strings.ToUpper(Current.Log.Syslog.Level)

	return nil
}

func loadConfigFile() ([]byte, error) {
	for _, candidate := range Candidates {
		if len(candidate) == 0 {
			continue
		}
		content, err := ioutil.ReadFile(candidate)
		if err != nil {
			continue
		}
		return content, nil
	}
	return nil, fmt.Errorf("Failed to load config file")
}

func getEnvironment() (env string) {
	env = os.Getenv("PAGES_ENV")
	if env == "" {
		env = "development"
	}
	return
}
