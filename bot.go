package main

//go:generate gotext -srclang=en update -lang=en,ru

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	_ "golang.org/x/text/message/catalog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	aboutText         = "Get the Air Quality Index (AQI) for the current location.\nContact: andrei+aqibot@tsevan.com"
	startMsg          = "/airQuality - get the Air Quality Index for the location,\n/about      - into about the bot."
	notifyMeCnfrmText = "OK. I will notify you if AQI changes in your location. /mySubsription"
	cleanupNotifBtn   = "Cleanup AQI Subscriptions"
	notifyMeDelText   = "OK. I won't notify you anymore"
	safeToRetryErrMsg = "Error! Please, retry!"
	numberSubsTmpl    = "You have %d subscription(s)"
	aqiGetsWorseMsg   = "😷 AQI gets worse"
	aqiGetsBetterMsg  = "😌 AQI gets better"
	aqiText           = "Air Quality Index"
	detailsText       = "Details"

	keyboardCmds = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButtonLocation("/airQualityIndex")),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/mySubsription"),
			tgbotapi.NewKeyboardButton("/about"),
		),
	)
	cleanupSubscriptionInline = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(cleanupNotifBtn, "cleanup"),
		),
	)
)

type AQIProvider interface {
	GetAirPollution(l *Location) (*ApiPollutionResponse, error)
}

type Bot struct {
	tApi  *tgbotapi.BotAPI
	store *Store
	wAPI  AQIProvider
}

// NewBot creates a PollutionBot. Returns Bot and cleanUp() function.
func NewBot(telegramAPIToken, owmApiToken string, debug bool) (*Bot, func()) {

	botapi, err := tgbotapi.NewBotAPI(telegramAPIToken)
	if err != nil {
		log.Panic("failed to create a tgbotapi client:", err)
	}

	owmapi, err := NewOpenWheatherMapApi(owmApiToken)
	if err != nil {
		log.Panic("failed to create an openwhethermapapi client:", err)
	}

	if debug {
		botapi.Debug = true
		owmapi.Debug = true
	}

	db, err := sql.Open("sqlite3", "./airpollutionbot.db")
	if err != nil {
		log.Panic("creating DB client: ", err)
	}

	store := &Store{
		DB:        db,
		CacheTime: 10 * time.Minute,
	}
	if err := store.Init(); err != nil {
		log.Panic("cannot init DB: ", err)
	}

	bot := &Bot{
		tApi:  botapi,
		store: store,
		wAPI:  owmapi,
	}

	log.Printf("Authorized on account %s", botapi.Self.UserName)

	return bot, func() {
		db.Close()
	}
}

// Run listens to Updates and process them by gorourines
func (bot *Bot) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.tApi.GetUpdatesChan(u)

	for update := range updates {
		go bot.handleUpdate(update)
	}
}

func (bot *Bot) handleUpdate(update tgbotapi.Update) {
	switch {
	case update.Message != nil:
		bot.handleMessage(update.Message)
	case update.CallbackQuery != nil:
		bot.handleCallbackQuery(update.CallbackQuery)
	}
}

func (bot *Bot) handleLocationMessage(msg *tgbotapi.Message) {
	if msg.Location == nil {
		return
	}

	var (
		chatID       = msg.Chat.ID
		userID       = msg.From.ID
		languageCode = msg.From.LanguageCode
		location     = &Location{
			msg.Location.Latitude,
			msg.Location.Longitude,
		}
	)

	lang, err := language.Parse(languageCode)
	if err != nil {
		log.Println(err)
	}
	p := message.NewPrinter(lang)

	us := &UserSession{
		ChatID:       chatID,
		UserID:       userID,
		LanguageCode: languageCode,
	}
	us.SetLocation(location)

	if err := bot.store.UpdateUserSession(us); err != nil {
		log.Panic("UpdateUserSession: ", err)
	}

	dp, err := bot.store.GetLastPD(chatID)
	if err != nil {
		log.Panic("GetLastPD: ", err)
	}

	// Caching pollution results for bot.store.CacheTime (10 min)
	if time.Since(time.Unix(dp.Dt, 0)) > bot.store.CacheTime {
		resp, err := bot.wAPI.GetAirPollution(location)
		if err != nil {
			log.Print("GetAirPollution: ", err)
			bot.Send(tgbotapi.NewMessage(chatID, p.Sprint(safeToRetryErrMsg)))
			return
		}
		if err := bot.store.AddDataPoint(chatID, &resp.DP); err != nil {
			log.Panic("AddDataPoint: ", err)
		}
		dp, err = bot.store.GetLastPD(chatID)
		if err != nil {
			log.Panic("GetLastPD: ", err)
		}
	}

	msgText := []string{
		p.Sprint(aqiText) + ": " + p.Sprint(dp.Main.Aqi),
		"",
		p.Sprint(dp.Main.Aqi.Description()),
	}

	tgMsg := tgbotapi.NewMessage(chatID, strings.Join(msgText, "\n"))

	// show inline buttons - details and notifyMe
	tgMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				p.Sprint("Notify Me on AQI changes"),
				"notifyMe",
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				p.Sprint("Details"),
				"details",
			),
		),
	)
	bot.Send(tgMsg)
}

func (bot *Bot) Send(tgMsg tgbotapi.MessageConfig) {
	if _, err := bot.tApi.Send(tgMsg); err != nil {
		log.Print("failed to send a telegram message: ", err)
	}
}

func (bot *Bot) handleMessage(msg *tgbotapi.Message) {
	if msg.IsCommand() {
		bot.handleCommand(msg)
		return
	}

	if msg.Location != nil { // User shares their location
		bot.handleLocationMessage(msg)
		return
	}

	tgMsg := tgbotapi.NewMessage(msg.Chat.ID, "Just share your location or try /start")
	bot.Send(tgMsg)
}

func (bot *Bot) handleCommand(msg *tgbotapi.Message) {
	var (
		chatID       = msg.Chat.ID
		languageCode = msg.From.LanguageCode
	)

	lang, err := language.Parse(languageCode)
	if err != nil {
		log.Println(err)
	}
	p := message.NewPrinter(lang)

	tgMsg := tgbotapi.NewMessage(chatID, "")
	tgMsg.ReplyToMessageID = msg.MessageID

	switch msg.Command() { // Extract the command from the Message.
	case "airQualityIndex", "air":
		tgMsg.Text = p.Sprint("Share location!")
		btn := tgbotapi.KeyboardButton{
			RequestLocation: true,
			Text:            p.Sprint("Share location!"),
		}
		tgMsg.ReplyMarkup = tgbotapi.NewOneTimeReplyKeyboard([]tgbotapi.KeyboardButton{btn})
	case "start":
		tgMsg.Text = p.Sprint(startMsg)
		tgMsg.ReplyMarkup = keyboardCmds
	case "mySubsription":
		subs, err := bot.store.ListAQISubscriptions(chatID)
		if err != nil {
			log.Print("ListAQISubscriptions", err)
		}

		msgText := []string{p.Sprintf(numberSubsTmpl, len(*subs)), ""}

		if len(*subs) > 0 {
			for _, s := range *subs {
				msgText = append(msgText, p.Sprintf("Location: %f;%f. Last AQI: %s",
					s.Longitude, s.Latitude, s.AirQualityIndex.String()))
			}
			tgMsg.ReplyMarkup = cleanupSubscriptionInline
		}

		tgMsg.Text = strings.Join(msgText, "\n")
	case "about":
		tgMsg.Text = p.Sprint(aboutText)
	default:
		tgMsg.Text = p.Sprint("Notify Me on AQI changes")
		tgMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	}
	bot.Send(tgMsg)
}

func (bot *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	var (
		chatID    = query.Message.Chat.ID
		messageID = query.Message.MessageID
	)

	// Respond to the callback query, telling Telegram to show the user
	// a message with the data received.
	callback := tgbotapi.NewCallback(
		query.ID,
		query.Data,
	)
	if _, err := bot.tApi.Request(callback); err != nil {
		log.Panic(err)
	}

	tgMsg := tgbotapi.NewMessage(chatID, "")
	tgMsg.ReplyToMessageID = messageID

	switch query.Data {
	case "notifyMe":
		tgMsg.Text = notifyMeCnfrmText
		err := bot.store.AddAQISubscription(chatID)
		if err != nil {
			log.Println("AddAQISubscription: ", err)
			tgMsg.Text = fmt.Sprintf("Error: %v", err)
		}
	case "details":
		dp, err := bot.store.GetLastPD(chatID)
		if err != nil {
			log.Panic(err)
		}

		var msgText []string
		msgText = append(msgText,
			detailsText,
			time.Unix(dp.Dt, 0).String(),
			"",
		)
		for k, v := range dp.Components {
			msgText = append(msgText, fmt.Sprintf("%s=%.2f", k, v))
		}
		tgMsg.Text = strings.Join(msgText, "\n")
	case "cleanup":
		err := bot.store.DeleteAQISubscriptions(chatID)
		if err != nil {
			log.Println("DeleteAQISubscriptions: ", err)
			tgMsg.Text = safeToRetryErrMsg
		}
		tgMsg.Text = notifyMeDelText
	default:
		tgMsg.Text = query.Data
	}

	bot.Send(tgMsg)
}

func (bot *Bot) Cron() {
	subs, err := bot.store.ListEnabledSubscriptions()
	if err != nil {
		log.Printf("ListEnabledSubscriptions: %v", err)
		return
	}
	log.Printf("%d subsription(s) to process", len(*subs))
	i := 0
	for _, s := range *subs {

		location := &Location{
			s.Latitude,
			s.Longitude,
		}

		resp, err := bot.wAPI.GetAirPollution(location)
		if err != nil {
			log.Print("GetAirPollution: ", err)
			continue
		}
		if err := bot.store.AddDataPoint(s.ChatID, &resp.DP); err != nil {
			log.Print("AddDataPoint: ", err)
			continue
		}
		dp, err := bot.store.GetLastPD(s.ChatID)
		if err != nil {
			log.Print("GetLastPD: ", err)
			continue
		}

		if dp.GetAQI() != s.AirQualityIndex {
			err := bot.store.UpdateSubscriptionAQI(s.ID, dp.GetAQI())
			if err != nil {
				log.Print("UpdateSubscriptionAQI: ", err)
				continue
			}

			headMsg := aqiGetsWorseMsg
			if dp.GetAQI() < s.AirQualityIndex {
				headMsg = aqiGetsBetterMsg
			}

			lang, err := language.Parse(s.LanguageCode)
			if err != nil {
				log.Println(err)
			}
			p := message.NewPrinter(lang)

			msgText := []string{
				p.Sprint(headMsg),
				"",
				p.Sprint(aqiText) + ": " + p.Sprint(dp.Main.Aqi.String()),
				"",
				p.Sprint(dp.Main.Aqi.Description()),
			}

			tgMsg := tgbotapi.NewMessage(s.ChatID, strings.Join(msgText, "\n"))

			tgMsg.ReplyMarkup = cleanupSubscriptionInline
			bot.Send(tgMsg)
			i++
		}
	}
	log.Printf("Sent %d messages", i)
}

func (bot *Bot) CronCleanup() {
	err := bot.store.ClenupAQISubscriptions()
	if err != nil {
		log.Println("CronCleanup:", err)
		return
	}

	log.Println("CronCleanup complete")
}
