package config

import (
    "os"
    "fmt"
    "log"
    "io/ioutil"
    "gopkg.in/yaml.v2"
)

type SshdConfig struct {
    ListenHost string `yaml:"host"`
    ListenPort int32  `yaml:"port"`
    PrivateKey string `yaml:"private_key"`
}

type EnvironmentalConfig struct {
    Sshd SshdConfig
}

type Config struct {
    Development EnvironmentalConfig
    Production  EnvironmentalConfig
    Test        EnvironmentalConfig
}

var CurrentConfig *EnvironmentalConfig
var Candidates = []string {
    "/etc/pages.yml",
    "/etc/pages.yaml",
    "etc/pages.yml",
    "etc/pages.yaml",
}

func LoadConfig() {
    err := loadConfig()
    if err != nil {
        log.Fatal(err)
    }
}

func loadConfig() error {
    config := Config {}
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
        CurrentConfig = &config.Development
    case "test":
        CurrentConfig = &config.Test
    case "production":
        CurrentConfig = &config.Production
    default:
        return fmt.Errorf("Invalid environment `%v`", env)
    }
    return nil
}

func loadConfigFile() ([]byte, error) {
    for _, candidate := range Candidates {
        content, err := ioutil.ReadFile(candidate)
        if err != nil {
            continue
        }
        return content, nil
    }
    return nil, fmt.Errorf("Failed to load config file\n")
}

func getEnvironment() (env string) {
    env = os.Getenv("PAGES_ENV")
    if env == "" {
        env = "development"
    }
    return
}
