// Package useragent classifica in modo grezzo uno User-Agent: dispositivo,
// sistema operativo, browser e se si tratta di un bot (o di un'anteprima di link).
package useragent

import (
	"regexp"
	"strings"
)

type Info struct {
	Device  string
	OS      string
	Browser string
	Bot     bool
}

const Unknown = "Altro"

// Programmi che aprono il link senza che una persona abbia scansionato il QR:
// crawler, anteprime delle chat e client da riga di comando.
var botMarkers = []string{
	"googlebot", "bingbot", "yandex", "baiduspider", "duckduckbot", "applebot", "petalbot",
	"facebookexternalhit", "facebookcatalog", "meta-externalagent", "whatsapp", "telegrambot",
	"slackbot", "slack-imgproxy", "discordbot", "twitterbot", "linkedinbot", "skypeuripreview",
	"redditbot", "embedly", "vkshare", "bitlybot",
	"crawler", "spider", "headlesschrome", "lighthouse", "pagespeed", "preview",
	"curl/", "wget/", "python-requests", "python-urllib", "go-http-client", "okhttp", "java/",
	"httpclient", "axios/", "node-fetch", "postmanruntime", "uptime", "monitor",
}

// "bot" come parola a sé (es. "SomeBot/1.0"), ma non dentro nomi come "Cubot"
var botWord = regexp.MustCompile(`(?i)\bbot\b|bot/`)

func Parse(ua string) Info {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return Info{Device: Unknown, OS: Unknown, Browser: Unknown}
	}
	l := strings.ToLower(ua)

	if isBot(l) {
		return Info{Device: "Bot", OS: Unknown, Browser: Unknown, Bot: true}
	}
	return Info{Device: device(l), OS: operatingSystem(l), Browser: browser(l)}
}

func isBot(l string) bool {
	for _, m := range botMarkers {
		if strings.Contains(l, m) {
			return true
		}
	}
	return botWord.MatchString(l)
}

func operatingSystem(l string) string {
	switch {
	case strings.Contains(l, "iphone"), strings.Contains(l, "ipad"), strings.Contains(l, "ipod"):
		return "iOS"
	case strings.Contains(l, "android"):
		return "Android"
	case strings.Contains(l, "windows"):
		return "Windows"
	case strings.Contains(l, "cros"):
		return "ChromeOS"
	case strings.Contains(l, "macintosh"), strings.Contains(l, "mac os x"):
		return "macOS"
	case strings.Contains(l, "linux"):
		return "Linux"
	}
	return Unknown
}

func device(l string) string {
	switch {
	case strings.Contains(l, "ipad"), strings.Contains(l, "tablet"),
		strings.Contains(l, "android") && !strings.Contains(l, "mobile"):
		return "Tablet"
	case strings.Contains(l, "iphone"), strings.Contains(l, "ipod"), strings.Contains(l, "mobile"):
		return "Smartphone"
	case strings.Contains(l, "windows"), strings.Contains(l, "macintosh"),
		strings.Contains(l, "linux"), strings.Contains(l, "cros"):
		return "Desktop"
	}
	return Unknown
}

func browser(l string) string {
	// L'ordine conta: quasi tutti i browser dichiarano anche "Safari" o "Chrome"
	switch {
	case strings.Contains(l, "instagram"):
		return "Instagram"
	case strings.Contains(l, "fban"), strings.Contains(l, "fbav"):
		return "Facebook"
	case strings.Contains(l, "edg/"), strings.Contains(l, "edga/"), strings.Contains(l, "edgios/"):
		return "Edge"
	case strings.Contains(l, "opr/"), strings.Contains(l, "opera"):
		return "Opera"
	case strings.Contains(l, "samsungbrowser"):
		return "Samsung Internet"
	case strings.Contains(l, "firefox"), strings.Contains(l, "fxios"):
		return "Firefox"
	case strings.Contains(l, "crios"), strings.Contains(l, "chrome"):
		return "Chrome"
	case strings.Contains(l, "safari"):
		return "Safari"
	}
	return Unknown
}
