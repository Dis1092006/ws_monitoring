package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
	"os"
	"os/signal"
	"syscall"
)

const (
	configFileName = "ws_monitoring.yaml"
	//statfile       = "tmp/stat.json"
)

func main() {

	// Загрузка конфигурации
	cfg, err := reloadConfig(configFileName)
	if err != nil {
		if err != errNotModified {
			log.Fatalf("Не удалось загрузить %s: %s", configFileName, err)
		}
	}
	log.Printf("%#v", cfg)

	// Запуск рабочего цикла
	go workingLoop(cfg)

	// Контроль завершения программы по Ctrl-C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	signal.Notify(sigChan, syscall.SIGTERM)
	for {
		select {
		case <-sigChan:
			log.Println("CTRL-C: Завершаю работу.")
			return
		}
	}
}

// Вечный рабочий цикл
func workingLoop(cfg *Config) {
	// Цикл по списку web-сервисов
	for _, service := range cfg.Services {
		if service.Enabled != true {
			continue
		}
		go check(service.Address)
	}
}

func checkWebService(url string) {
	// Вечный цикл
	for {
		tm := time.Now().Format("2006–01–02 15:04:05")
		// статус, который возвращает check, пока не используем, поэтому ставим _
		_, msg := check(url)
		//log_to_file(tm, msg)
		fmt.Println(tm, msg)
		time.Sleep(1 * time.Minute)
	}
}

func check(url string) (bool, string) {
	// возвращает true — если сервис доступен, false, если нет и текст сообщения
	fmt.Println("Проверяем адрес ", url)
	resp, err := http.Get(url)

	if err != nil {
		return false, fmt.Sprintf("Ошибка соединения. % s", err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false, fmt.Sprintf("Ошибка.http - статус: %s", resp.StatusCode)
	}
	return true, fmt.Sprintf("Онлайн. http-статус: %d", resp.StatusCode)
}

func log_to_file(tm, s string) {
	// Сохраняет сообщения в файл
	f, err := os.OpenFile("ws_monitoring.log", os.O_RDWR | os.O_APPEND | os.O_CREATE, 0644)
	if err != nil {
		fmt.Println(tm, err)
		return
	}
	defer f.Close()
	if _, err = f.WriteString(fmt.Sprintln(tm, s)); err != nil {
		fmt.Println(tm, err)
	}
}