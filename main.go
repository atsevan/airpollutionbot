package main

import (
	"flag"
	"log"
	"os"

	"github.com/robfig/cron"
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

	botApiToken := getEnvVarOrPanic("TELEGRAM_API_TOKEN")
	owmApiToken := getEnvVarOrPanic("OWM_API_TOKEN")

	bot, cancel := NewBot(botApiToken, owmApiToken, *dFlag)

	defer cancel()
	c := cron.New()
	c.AddFunc("@every 30m", bot.Cron)
	c.AddFunc("@every 12h", bot.CronCleanup)
	c.Start()

	bot.Run()
}
