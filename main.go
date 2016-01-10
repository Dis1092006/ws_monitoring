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

type CheckResult struct {
	CheckTime     string        `json:"time"`
	CheckDuration time.Duration `json:"duration"`
	Address       string        `json:"address"`
	StatusCode    int           `json:"status"`
	Error         string        `json:"error"`
}

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
	//	log.Printf("%#v", cfg)

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
// Проверка работоспособности указанного web-сервиса и отправка результата в data collector
//----------------------------------------------------------------------------------------------------------------------
func checkWebService(url string, login string, password string, interval time.Duration) {

	// Вечный цикл
	for {
		checkResult := check(url, login, password)

		// Отправить результат проверки сборщику данных
		dataCollectorURL := "http://10.126.200.4:3000/api/imd"
		response, err := makeRequest("POST", dataCollectorURL, checkResult)
		if err != nil {
			fmt.Println("Ошибка отправки данных в data collector ", err)
			//return respTodo, err
		}
		fmt.Println("Результат отправки данных в data collector ", response)
		//err = processResponseEntity(response, &respTodo, 201)

		time.Sleep(interval * time.Second)
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Проверка подключения к web-сервису
// возвращает true — если сервис доступен, false, если нет и текст сообщения
//----------------------------------------------------------------------------------------------------------------------
func check(url string, login string, password string) *CheckResult {
	// Подготовка результата работы функции проверки
	var checkResult *CheckResult = new(CheckResult)

	// Засечка времени
	checkTime := time.Now()

	// Попытка подключения
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(login, password)
	client := &http.Client{}
	resp, err := client.Do(req)

	// Контроль длительности замера
	checkDuration := time.Since(checkTime)

	// Анализ результатов попытки подключения
	fmt.Println(checkTime.Format("2006–01–02 15:04:05"), "Проверка подключения к адресу: ", url)
	if err != nil {
		checkResult.StatusCode = 0
		checkResult.Error = err.Error()
		fmt.Println("Ошибка!", err)
	} else {
		defer resp.Body.Close()
		checkResult.StatusCode = resp.StatusCode
		checkResult.Error = ""
		fmt.Println("Успешно. Длительность запроса: ", checkDuration)
	}

	// Заполнение результата проверки подключения
	checkResult.CheckTime = checkTime.Format("2006–01–02T15:04:05")
	checkResult.CheckDuration = checkDuration
	checkResult.Address = url
//	fmt.Printf("%+v\n", checkResult)

	return checkResult
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
