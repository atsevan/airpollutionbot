# Air Pollution Telegram Bot
This is a Telegram bot that notifies users when the Air Quality Index (AQI) changes.

## Features

- AQI updates: The bot continuously monitors the AQI and sends notifications to users whenever there is a change.
- Location-based tracking: The bot can track the AQI of multiple locations and provide personalized notifications based on user preferences.
- User-friendly interface: The bot provides a simple and intuitive interface for users to interact with and manage their notification settings.
- Support of locales. At the moment partially added russian and belarusian.

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
How to Modify the messages.gotext.json File for a Specific Language (e.g., Belarusian):

1. Locate the Language File. Navigate to the folder containing the language files. For Belarusian, the file path should be something like translations/be/messages.gotext.json.
1. Open the JSON File: Open the messages.gotext.json file using a text editor.
1. Edit Translations: Inside the file, you'll see key-value pairs where each key is an identifier in English, and the value is the corresponding translation in Belarusian.
1. Save the File: Once you've made the necessary changes, save the file and create a PR.

Update the translations:
```
go install golang.org/x/text/cmd/gotext@latest
go generate translations/translations.go
```