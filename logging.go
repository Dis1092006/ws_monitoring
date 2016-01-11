package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"os"
	"strings"
	"sync"
)

var (
	// Глобальная переменная для логгера
	log      = logrus.New()
	initOnce sync.Once
)

//----------------------------------------------------------------------------------------------------------------------
// Инициализация логгера
//----------------------------------------------------------------------------------------------------------------------
func initLogger(cfg *Config) error {
	// Инициализация файла лога - выполняется однократно
	initOnce.Do(func() {
		file, err := os.OpenFile(cfg.LogFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalln(err)
		}
		log.Out = file
	})
	// Настройка уровня логирования
	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		log.Level = logrus.DebugLevel
	case "INFO":
		log.Level = logrus.InfoLevel
	case "ERROR":
		log.Level = logrus.ErrorLevel
	default:
		return fmt.Errorf("Неизвестный уровень лога, %s", cfg.LogLevel)
	}
	// Настройка формата времени
	log.Formatter = &logrus.TextFormatter{TimestampFormat: "2006-01-02 15:04:05"}
	// Информационное сообщение
	log.Infoln("Уровень логирования", cfg.LogLevel)
	// Ошибок не было
	return nil
}
