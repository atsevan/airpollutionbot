package main

import (
	"flag"
	"log"
	"os"

	"github.com/robfig/cron"
)

const (
	defaultDBPath = "./airpollutionbot.db"
)

var dFlag = flag.Bool("debug", false, "increase verbosity")

func getEnvVarOrPanic(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Panic("env variable not found ", key)
	}
	return v
}

func main() {
	flag.Parse()

	botAPIToken := getEnvVarOrPanic("TELEGRAM_API_TOKEN")
	owmAPIToken := getEnvVarOrPanic("OWM_API_TOKEN")
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = defaultDBPath
	}

	bot, cancel := NewBot(botAPIToken, owmAPIToken, dbPath, *dFlag)

	defer cancel()
	c := cron.New()
	c.AddFunc("@every 30m", bot.Cron)
	c.AddFunc("@every 12h", bot.CronCleanup)
	c.Start()

	bot.Run()
}
