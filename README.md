# Air Pollution Telegram Bot

This is a Telegram bot that notifies users when the Air Quality Index (AQI) changes.

## Features

- AQI updates: The bot continuously monitors the AQI and sends notifications to users whenever there is a change.
- Location-based tracking: The bot can track the AQI of multiple locations and provide personalized notifications based on user preferences.
- User-friendly interface: The bot provides a simple and intuitive interface for users to interact with and manage their notification settings.
- Support of locales. At the moment partially added russian.

## Getting Started

To use the Air Pollution Telegram Bot, follow these steps:

1. Install Telegram on your device if you haven't already.
2. Open https://t.me/AirPollution_Bot or search for the "@AirPollution_Bot" in the Telegram app.
3. Start a conversation with the bot and share your location.
4. Create a subsription and get AQI updates to be informed about air pollution in your area!

## Contributing

Contributions are welcome! If you have any ideas, bug reports, or feature requests, please open an issue on the GitHub repository.

## License

This project is licensed under the [MIT License](LICENSE).

## Translation
```
go install golang.org/x/text/cmd/gotext@latest
go generate translations/translations.go
```