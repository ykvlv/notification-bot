package telegram

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// UI texts in English
const (
	startText = "👋 Hi! I'm your motivation reminder bot.\n\n" +
		"Set your interval, active hours and custom message — and I will keep you on track."
	statusTitle = "Your current settings:"
	statusFmt   = "• Interval: %s\n• Active hours: %s–%s\n• TZ: %s\n• Enabled: %s\n• Next: %s\n"
)

func mainMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("/status"),
			tgbotapi.NewKeyboardButton("/settings"),
		),
	)
}

func settingsInlineKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Interval", "set_interval"),
			tgbotapi.NewInlineKeyboardButtonData("Active hours", "set_hours"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Timezone", "set_tz"),
			tgbotapi.NewInlineKeyboardButtonData("Message", "set_msg"),
		),
	)
}

func intervalPresetsKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("30m", "interval:30m"),
			tgbotapi.NewInlineKeyboardButtonData("1h", "interval:1h"),
			tgbotapi.NewInlineKeyboardButtonData("2h", "interval:2h"),
			tgbotapi.NewInlineKeyboardButtonData("3h", "interval:3h"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("4h", "interval:4h"),
			tgbotapi.NewInlineKeyboardButtonData("6h", "interval:6h"),
			tgbotapi.NewInlineKeyboardButtonData("8h", "interval:8h"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("12h", "interval:12h"),
			tgbotapi.NewInlineKeyboardButtonData("24h", "interval:24h"),
			tgbotapi.NewInlineKeyboardButtonData("Custom…", "interval:custom"),
		),
	)
}
