package main

//go:generate gotext -srclang=en update -lang=en,ru

import (
	"database/sql"
	"log"
	"strings"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	_ "golang.org/x/text/message/catalog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	authorContact     = "andrei+aqibot@tsevan.com"
	aboutTextTmpl     = "Get the Air Quality Index (AQI) for the current location.\nContact: %s"
	startMsg          = "/airQualityIndex - get the Air Quality Index for the location,\n/about      - into about the bot."
	notifyMeCnfrmText = "OK. I will notify you if AQI changes in your location. /mySubsription"
	cleanupNotifBtn   = "Cleanup AQI Subscriptions"
	notifyMeDelText   = "OK. I won't notify you anymore"
	safeToRetryErrMsg = "Error! Please, retry!"
	numberSubsTmpl    = "You have %d subscription(s)"
	aqiGetsWorseMsg   = "😷 AQI gets worse"
	aqiGetsBetterMsg  = "😌 AQI gets better"
	aqiText           = "Air Quality Index"
	detailsText       = "Details"
	unknownCmdMsg     = "Just share your location or try /start"
)

var (
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

func newLangPrinter(languageCode string) *message.Printer {
	lang, err := language.Parse(languageCode)
	if err != nil {
		log.Println(err)
		lang = language.English
	}
	return message.NewPrinter(lang)
}

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

	p := newLangPrinter(languageCode)

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
			bot.Send(tgbotapi.NewMessage(chatID, p.Sprintf(safeToRetryErrMsg)))
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
		p.Sprintf(aqiText) + ": " + p.Sprintf(dp.Main.Aqi),
		"",
		p.Sprintf(dp.Main.Aqi.Description()),
	}

	tgMsg := tgbotapi.NewMessage(chatID, strings.Join(msgText, "\n"))

	// show inline buttons - details and notifyMe
	tgMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				p.Sprintf("Notify Me on AQI changes"),
				"notifyMe",
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				p.Sprintf("Details"),
				"details",
			),
		),
	)
	bot.Send(tgMsg)
}

func (bot *Bot) Send(tgMsg tgbotapi.MessageConfig) {
	if _, err := bot.tApi.Send(tgMsg); err != nil {
		log.Printf("failed to send a telegram message: ", err)
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

	tgMsg := tgbotapi.NewMessage(msg.Chat.ID, startMsg)
	bot.Send(tgMsg)
}

func (bot *Bot) handleCommand(msg *tgbotapi.Message) {
	var (
		chatID       = msg.Chat.ID
		languageCode = msg.From.LanguageCode
	)

	p := newLangPrinter(languageCode)

	tgMsg := tgbotapi.NewMessage(chatID, "")
	tgMsg.ReplyToMessageID = msg.MessageID

	switch msg.Command() { // Extract the command from the Message.
	case "airQualityIndex", "air":
		tgMsg.Text = p.Sprintf("Share location!")
		btn := tgbotapi.KeyboardButton{
			RequestLocation: true,
			Text:            p.Sprintf("Share location!"),
		}
		tgMsg.ReplyMarkup = tgbotapi.NewOneTimeReplyKeyboard([]tgbotapi.KeyboardButton{btn})
	case "start":
		tgMsg.Text = p.Sprintf(startMsg)
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
		tgMsg.Text = p.Sprintf(aboutTextTmpl, authorContact)
	default:
		tgMsg.Text = p.Sprintf(unknownCmdMsg)
		tgMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	}
	bot.Send(tgMsg)
}

func (bot *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	var (
		chatID       = query.Message.Chat.ID
		messageID    = query.Message.MessageID
		languageCode = query.Message.From.LanguageCode
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
	p := newLangPrinter(languageCode)

	tgMsg := tgbotapi.NewMessage(chatID, "")
	tgMsg.ReplyToMessageID = messageID

	switch query.Data {
	case "notifyMe":
		tgMsg.Text = notifyMeCnfrmText
		err := bot.store.AddAQISubscription(chatID)
		if err != nil {
			log.Println("AddAQISubscription: ", err)
			tgMsg.Text = p.Sprintf("Error: %v", err)
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
			msgText = append(msgText, p.Sprintf("%s=%.2f", k, v))
		}
		tgMsg.Text = strings.Join(msgText, "\n")
	case "cleanup":
		err := bot.store.DeleteAQISubscriptions(chatID)
		if err != nil {
			log.Println("DeleteAQISubscriptions: ", err)
			tgMsg.Text = p.Sprint(safeToRetryErrMsg)
		}
		tgMsg.Text = p.Sprint(notifyMeDelText)
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

			p := newLangPrinter(s.LanguageCode)

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
