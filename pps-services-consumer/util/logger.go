package util

import (
	"log"
	"os"
	"pps-services-consumer/constanta"
	"time"

	"github.com/go-co-op/gocron"
)

func Println(a ...any) {
	logFile, err := createLogFile()
	if err != nil {
		log.Println(err.Error())
	}
	defer logFile.Close()

	// Set log out put and enjoy :)
	log.SetOutput(logFile)

	// optional: log date-time, filename, and line number
	log.SetFlags(log.LstdFlags)

	log.Println(a...)
}

func Printf(format string, a ...any) {
	// open log file
	logFile, err := createLogFile()
	if err != nil {
		log.Println(err.Error())
	}
	defer logFile.Close()

	// Set log out put and enjoy :)
	log.SetOutput(logFile)

	// optional: log date-time, filename, and line number
	log.SetFlags(log.LstdFlags)

	log.Printf(format, a...)
}

func createLogFile() (*os.File, error) {
	currentTime := time.Now()
	LOG_FILE := os.Getenv(constanta.OS_ENV_LOG_DIR) + currentTime.Format("2006-01-02") + ".log"

	// open log file
	logFile, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0777)
	return logFile, err
}

func DeleteLogFile() {
	Println("======Start job clear log======")
	previousTime := time.Now().Add(-744 * time.Hour) //(delete file hari ke -31)

	fileLog := os.Getenv(constanta.OS_ENV_LOG_DIR) + previousTime.Format("2006-01-02") + ".log"

	// delete file *.log
	_, err := os.Stat(fileLog)
	if err == nil {
		Println(fileLog + " ==> exist")
		e := os.Remove(fileLog)
		if e != nil {
			Println(fileLog + " error ==> " + e.Error())
		} else {
			Println("remove " + fileLog + " ==> success")
		}
	} else {
		Println(fileLog + " ==> does not exist")
	}

	Println("======End job clear log======")
}

func DoJob() interface{} {
	var doTask interface{} = func() {
		DeleteLogFile()
	}

	return doTask
}

func StartJobDeleteLog() {
	s := gocron.NewScheduler(time.Local)

	// set time
	s.Every(1).Day().At(os.Getenv(constanta.OS_ENV_TIME_JOB_CLEAR_LOG)).Do(DoJob())
	s.StartAsync()
}
