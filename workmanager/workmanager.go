package workmanager

import (
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"
	"ws_monitoring/helper"
	"ws_monitoring/log"
	//"github.com/Sirupsen/logrus"
)

// workManager is responsible for starting and shutting down the program.
type workManager struct {
	Shutdown        int32
	ShutdownChannel chan string
}

// Тип - идентификатор рабочего потока
type WorkerID int

// Тип - команда управления рабочим потоком
type Command bool

// Тип - рабочий поток
type Worker struct {
	ID            WorkerID
	LastStateTime time.Time
	State         bool
	commandChan   chan Command
}

// Тип - cписок рабочих потоков
type WorkersList []*Worker

type CheckResult struct {
	CheckTime     string        `json:"time"`
	CheckDuration time.Duration `json:"duration"`
	Address       string        `json:"address"`
	StatusCode    int           `json:"status"`
	Error         string        `json:"error"`
}

var (
	wm               workManager // Reference to the singleton
	aliveWorkerChan  chan WorkerID
	workerIDSequence WorkerID = 0
)

//----------------------------------------------------------------------------------------------------------------------
// Startup brings the manager to a running state.
//----------------------------------------------------------------------------------------------------------------------
func Startup(cfg *helper.Config) error {
	var err error
	defer CatchPanic(&err, "main", "workmanager.Startup")

	log.Info("main, workmanager.Startup, Started")

	// Create the work manager to get the program going
	wm = workManager{
		Shutdown:        0,
		ShutdownChannel: make(chan string),
	}

	// Start the work timer routine.
	// When workManager returns the program terminates.
	go wm.WorkingLoop(cfg)

	log.Info("main, workmanager.Startup, Completed")
	return err
}

//----------------------------------------------------------------------------------------------------------------------
// Shutdown brings down the manager gracefully.
//----------------------------------------------------------------------------------------------------------------------
func Shutdown() error {
	var err error
	defer CatchPanic(&err, "main", "workmanager.Shutdown")

	log.Info("main, workmanager.Shutdown, Started")

	// Shutdown the program
	log.Info("main, workmanager.Shutdown, Info : Shutting Down")
	atomic.CompareAndSwapInt32(&wm.Shutdown, 0, 1)

	log.Info("main, workmanager.Shutdown, Info : Shutting Down Work Timer")
	wm.ShutdownChannel <- "Down"
	<-wm.ShutdownChannel

	close(wm.ShutdownChannel)

	log.Info("main, workmanager.Shutdown, Completed")
	return err
}

//----------------------------------------------------------------------------------------------------------------------
// CatchPanic is used to catch and display panics.
//----------------------------------------------------------------------------------------------------------------------
func CatchPanic(err *error, goRoutine string, function string) {
	if r := recover(); r != nil {
		// Capture the stack trace
		buf := make([]byte, 10000)
		runtime.Stack(buf, false)

		log.Errorf(goRoutine, function, "PANIC Defered [%v] : Stack Trace : %v", r, string(buf))

		if err != nil {
			*err = fmt.Errorf("%v", r)
		}
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Вечный рабочий цикл
//----------------------------------------------------------------------------------------------------------------------
func (workManager *workManager) WorkingLoop(cfg *helper.Config) {
	var workers WorkersList
	log.Debugf("workingLoop, ожидание %d секунд", time.Duration(cfg.ReloadConfigInterval))

	// Поток для контроля работоспособности рабочих потоков.
	aliveWorkerChan = make(chan WorkerID)

	// Первоначальная инициализация списка рабочих потоков
	log.Debugf("len(cfg.Services) = %d", len(cfg.Services))
	workers = make(WorkersList, len(cfg.Services))
	for i, service := range cfg.Services {
		if service.Enabled != true {
			continue
		}
		workerIDSequence = workerIDSequence + 1
		workers[i] = new(Worker)
		workers[i].ID = workerIDSequence
		workers[i].commandChan = make(chan Command)
		go workManager.CheckWebService(workers[i].ID, workers[i].commandChan, aliveWorkerChan, service.Address, service.Login, service.Password, service.CheckInterval * time.Second)
	}

	// Включение тикера
	t := time.Tick(time.Duration(cfg.ReloadConfigInterval) * time.Second)

	for {
		log.Debug("workingLoop, очередной цикл")
		select {
		case <-workManager.ShutdownChannel:
			log.Info("workingLoop, закрытие рабочих потоков")
			for i := 0; i < len(workers); i++ {
				if workers[i].State {
					workers[i].commandChan <- true
					<-workers[i].commandChan
					close(workers[i].commandChan)
				}
			}
			log.Info("workingLoop, выключение контрольного потока")
			workManager.ShutdownChannel <- "Down"
			return

		case <-t: // Срабатывание таймера.
			log.Debug("workingLoop, срабатывание таймера")
			// Перезагрузка конфигурации
			cfgTmp, err := helper.ReloadConfig(helper.ConfigFileName)
			if err != nil {
				if err != helper.ErrNotModified {
					log.Fatalf("Не удалось загрузить %s: %s", helper.ConfigFileName, err)
				} else {
					log.Debugf("workingLoop, конфигурация не изменилась")
					// ToDo - контроль рабочих потоков от которых давно не было подтверждения работоспособности
				}
			} else {
				log.Info("Перезагружен конфигурационный файл")
				if err := log.InitLogger(cfgTmp); err != nil {
					log.Error(err)
				} else {
					cfg = cfgTmp
				}

				// ToDo - пересоздать тикер при изменении cfg.ReloadConfigInterval

				// Закрыть предыдущие рабочие потоки.
				for i := 0; i < len(workers); i++ {
					if workers[i].State {
						workers[i].commandChan <- true
					}
				}

				// Создать новый набор рабочих потоков
				// ToDo - выделить в отдельную процедуру, т.к. дубль с блоком в начале функции
				workers = make(WorkersList, len(cfg.Services))
				for i, service := range cfg.Services {
					if service.Enabled != true {
						continue
					}
					workerIDSequence = workerIDSequence + 1
					workers[i] = new(Worker)
					workers[i].ID = workerIDSequence
					workers[i].commandChan = make(chan Command)
					go workManager.CheckWebService(workers[i].ID, workers[i].commandChan, aliveWorkerChan, service.Address, service.Login, service.Password, service.CheckInterval * time.Second)
				}
			}

		// Контрольный сигнал от рабочего потока.
		case workerID := <-aliveWorkerChan:
			log.Debugf("Контрольный сигнал от рабочего потока: %+v", workerID)
			// Обновить данные о рабочем потоке.
			for i := 0; i < len(workers); i++ {
				if workers[i].ID == workerID {
					// Сохранить время получения контрольного сигнала.
					workers[i].LastStateTime = time.Now()
				}
			}
		}
	}
}

//----------------------------------------------------------------------------------------------------------------------
// Проверка работоспособности указанного web-сервиса и отправка результата в data collector
//----------------------------------------------------------------------------------------------------------------------
func (workManager *workManager) CheckWebService(id WorkerID, innerChan chan Command, outerChan chan WorkerID, url string, login string, password string, interval time.Duration) {

	log.Debugf("checkWebService, запущен рабочий поток с номером: %d, интервал %d секунд", id, int(interval.Seconds()))

	wait := interval

	// Рабочий цикл
	for {
		log.Debugf("checkWebService, ожидание %.3f секунд", wait.Seconds())

		select {
		case <-innerChan:
			log.Infof("checkWebService, выключение рабочего потока с номером %d", id)
			innerChan <- true
			return

		case <-time.After(wait):
			log.Debug("checkWebService, завершение ожидания")
			break
		}

		// Mark the starting time
		startTime := time.Now()

		// Рабочая проверка
		checkResult := check(url, login, password)

		// Отправить результат проверки сборщику данных
		dataCollectorURL := "http://10.126.200.4:3000/api/imd"
		response, err := makeRequest("POST", dataCollectorURL, checkResult)
		if err != nil {
			log.Errorf("Ошибка отправки данных в data collector: %v", err)
		}
		log.Debugf("Результат отправки данных в data collector: %+v", response)

		// Отправить контрольный сигнал
		//outerChan <- id

		// Mark the ending time
		endTime := time.Now()

		// Calculate the amount of time to wait to start workManager again.
		duration := endTime.Sub(startTime)
		log.Debugf("Длительность выполнения рабочей проверки: %.3f секунд", duration.Seconds())
		wait = interval - duration
		log.Debugf("Следующее ожидание: %.3f секунд", wait.Seconds())
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
		log.Infof("Успешно. Длительность запроса: %.3f секунд", checkDuration.Seconds())
	}

	// Заполнение результата проверки подключения
	checkResult.CheckTime = checkTime.Format(time.RFC3339)
	//checkResult.CheckTime = checkTime.Format("2006–01–02T15:04:05")
	checkResult.CheckDuration = checkDuration
	checkResult.Address = url
	log.Debugf("%+v", checkResult)

	return checkResult
}
