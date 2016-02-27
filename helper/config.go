package helper

import (
	"errors"
	"io/ioutil"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

const ConfigFileName = "ws_monitoring.yaml"

var (
	configModTime  int64
	ErrNotModified = errors.New("Not modified")
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
	DataCollectorURL     string    `yaml:"data_collector_url"`
	Services             []Service `yaml:"services"`
}

//----------------------------------------------------------------------------------------------------------------------
// Загрузка конфигурации из указанного файла
//----------------------------------------------------------------------------------------------------------------------
func ReadConfig(ConfigName string) (x *Config, err error) {
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

//----------------------------------------------------------------------------------------------------------------------
// Проверка времени изменения конфигурационного файла и перезагрузка его, если он изменился
// Возврат errNotModified если изменений нет
//----------------------------------------------------------------------------------------------------------------------
func ReloadConfig(configName string) (cfg *Config, err error) {
	info, err := os.Stat(configName)
	if err != nil {
		return nil, err
	}
	if configModTime != info.ModTime().UnixNano() {
		configModTime = info.ModTime().UnixNano()
		cfg, err = ReadConfig(configName)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	return nil, ErrNotModified
}
