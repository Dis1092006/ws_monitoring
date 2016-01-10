package main

import (
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

var (
	cfg            *Config
	startTime      = time.Now().Round(time.Second)
	//эти переменные заполняются линкером.
	//чтобы их передать надо компилировать программу с ключами
	//go build -ldflags "-X main.buildtime '2015-12-22' -X main.version 'v1.0'"
	version   = "debug build"
	buildtime = "n/a"
)

//----------------------------------------------------------------------------------------------------------------------
// Основная функция программы
//----------------------------------------------------------------------------------------------------------------------
func main() {
	var err error

	// Загрузка конфигурации
	cfg, err = reloadConfig(configFileName)
	if err != nil {
		if err != errNotModified {
			log.Fatalf("Не удалось загрузить %s: %s", configFileName, err)
		}
	}
	log.Debugf("%#v", cfg)

	// Инициализация логгера
	if err := initLogger(cfg); err != nil {
		log.Fatalln(err)
	}
	log.Infof("Версия: %s. Собрано %s", version, buildtime)

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
	for {
		// Цикл по списку web-сервисов
		for _, service := range cfg.Services {
			if service.Enabled != true {
				continue
			}
			go checkWebService(service.Address, service.Login, service.Password, service.CheckInterval)
		}
		// Перезагрузка конфигурации
		cfgTmp, err := reloadConfig(configFileName)
		if err != nil {
			if err != errNotModified {
				log.Fatalf("Не удалось загрузить %s: %s", configFileName, err)
			}
		} else {
			log.Infoln("Перезагружен конфигурационный файл")
			if err := initLogger(cfgTmp); err != nil {
				log.Errorln(err)
			} else {
				cfg = cfgTmp
			}
		}
		// Пауза
		time.Sleep(time.Duration(cfg.ReloadConfigInterval) * time.Second)
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
			log.Errorf("Ошибка отправки данных в data collector: %v", err)
		}
		log.Debugf("Результат отправки данных в data collector: %+v", response)

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
	log.Infof("Проверка подключения к адресу: %s", url)
	if err != nil {
		checkResult.StatusCode = 0
		checkResult.Error = err.Error()
		log.Errorf("Ошибка! %v", err)
	} else {
		defer resp.Body.Close()
		checkResult.StatusCode = resp.StatusCode
		checkResult.Error = ""
		log.Infof("Успешно. Длительность запроса: %d", checkDuration)
	}

	// Заполнение результата проверки подключения
	checkResult.CheckTime = checkTime.Format("2006–01–02T15:04:05")
	checkResult.CheckDuration = checkDuration
	checkResult.Address = url
	log.Debugf("%+v", checkResult)

	return checkResult
}
