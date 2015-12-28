package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	configFileName = "ws_monitoring.yaml"
	//statfile       = "tmp/stat.json"
)

//----------------------------------------------------------------------------------------------------------------------
// Основная функция программы
//----------------------------------------------------------------------------------------------------------------------
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

//----------------------------------------------------------------------------------------------------------------------
// Вечный рабочий цикл
//----------------------------------------------------------------------------------------------------------------------
func workingLoop(cfg *Config) {
	// Цикл по списку web-сервисов
	for _, service := range cfg.Services {
		if service.Enabled != true {
			continue
		}
		go checkWebService(service.Address, service.Login, service.Password, service.CheckInterval)
	}
}

//----------------------------------------------------------------------------------------------------------------------
//----------------------------------------------------------------------------------------------------------------------
func checkWebService(url string, login string, password string, interval time.Duration) {
	// Вечный цикл
	for {
		tm1 := time.Now().Format("2006–01–02 15:04:05")
		// статус, который возвращает check, пока не используем, поэтому ставим _
		fmt.Println(tm1, "Проверка подключения к адресу: ", url)
		_, msg := check(url, login, password)
		tm2 := time.Now().Format("2006–01–02 15:04:05")
		log_to_file(tm2, msg)
		fmt.Println(tm2, msg)
		time.Sleep(interval * time.Second)
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Проверка подключения к web-сервису
// возвращает true — если сервис доступен, false, если нет и текст сообщения
//----------------------------------------------------------------------------------------------------------------------
func check(url string, login string, password string) (bool, string) {

	// Попытка подключения
	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(login, password)
	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()

	// Анализ результата попытки подключения
	if err != nil {
		return false, fmt.Sprintf("Ошибка соединения. % s", err)
	}
	if resp.StatusCode != 200 {
		return false, fmt.Sprintf("Ошибка.http - статус: %s", resp.StatusCode)
	}
	return true, fmt.Sprintf("Онлайн. http-статус: %d", resp.StatusCode)
}

//----------------------------------------------------------------------------------------------------------------------
// Запись в лог.
//----------------------------------------------------------------------------------------------------------------------
func log_to_file(tm, s string) {
	// Сохраняет сообщения в файл
	f, err := os.OpenFile("ws_monitoring.log", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println(tm, err)
		return
	}
	defer f.Close()
	if _, err = f.WriteString(fmt.Sprintln(tm, s)); err != nil {
		fmt.Println(tm, err)
	}
}
