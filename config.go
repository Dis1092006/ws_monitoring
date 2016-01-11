package main

import (
	"errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"time"
)

var (
	configModTime  int64
	errNotModified = errors.New("Not modified")
)

// Service - структура настроек для web-сервиса, который будет мониториться
type Service struct {
	Address       string        `yaml:"address"`
	Login         string        `yaml:"login"`
	Password      string        `yaml:"password"`
	Enabled       bool          `yaml:"enabled"`
	CheckInterval time.Duration `yaml:"check_interval"`
}

// Config - структура для считывания конфигурационного файла
type Config struct {
	ReloadConfigInterval int       `yaml:"reload_config_interval"`
	LogLevel             string    `yaml:"log_level"`
	LogFilename          string    `yaml:"log_filename"`
	Services             []Service `yaml:"services"`
}

func readConfig(ConfigName string) (x *Config, err error) {
	var file []byte
	if file, err = ioutil.ReadFile(ConfigName); err != nil {
		return nil, err
	}
	x = new(Config)
	if err = yaml.Unmarshal(file, x); err != nil {
		return nil, err
	}
	if x.LogLevel == "" {
		x.LogLevel = "Debug"
	}
	return x, nil
}

// Проверяет время изменения конфигурационного файла и перезагружает его если он изменился
// Возвращает errNotModified если изменений нет
func reloadConfig(configName string) (cfg *Config, err error) {
	info, err := os.Stat(configName)
	if err != nil {
		return nil, err
	}
	if configModTime != info.ModTime().UnixNano() {
		configModTime = info.ModTime().UnixNano()
		cfg, err = readConfig(configName)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	return nil, errNotModified
}
