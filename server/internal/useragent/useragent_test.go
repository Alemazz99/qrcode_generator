package useragent

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		ua   string
		want Info
	}{
		{"iPhone Safari",
			"Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1",
			Info{"Smartphone", "iOS", "Safari", false}},
		{"iPhone Chrome",
			"Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/130.0.6723.90 Mobile/15E148 Safari/604.1",
			Info{"Smartphone", "iOS", "Chrome", false}},
		{"Android Chrome",
			"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36",
			Info{"Smartphone", "Android", "Chrome", false}},
		{"Android Samsung",
			"Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/26.0 Chrome/122.0.0.0 Mobile Safari/537.36",
			Info{"Smartphone", "Android", "Samsung Internet", false}},
		{"Cubot non è un bot",
			"Mozilla/5.0 (Linux; Android 12; CUBOT KINGKONG 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
			Info{"Smartphone", "Android", "Chrome", false}},
		{"Tablet Android",
			"Mozilla/5.0 (Linux; Android 13; SM-X710) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
			Info{"Tablet", "Android", "Chrome", false}},
		{"Instagram in-app",
			"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 340.0.0.22.109",
			Info{"Smartphone", "iOS", "Instagram", false}},
		{"Windows Edge",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0",
			Info{"Desktop", "Windows", "Edge", false}},
		{"Linux Firefox",
			"Mozilla/5.0 (X11; Linux x86_64; rv:132.0) Gecko/20100101 Firefox/132.0",
			Info{"Desktop", "Linux", "Firefox", false}},
		{"Mac Safari",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15",
			Info{"Desktop", "macOS", "Safari", false}},
		{"WhatsApp anteprima", "WhatsApp/2.23.20.0 A", Info{"Bot", Unknown, Unknown, true}},
		{"Facebook anteprima",
			"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)",
			Info{"Bot", Unknown, Unknown, true}},
		{"Googlebot",
			"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			Info{"Bot", Unknown, Unknown, true}},
		{"Telegram", "TelegramBot (like TwitterBot)", Info{"Bot", Unknown, Unknown, true}},
		{"curl", "curl/8.9.1", Info{"Bot", Unknown, Unknown, true}},
		{"vuoto", "", Info{Unknown, Unknown, Unknown, false}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Parse(c.ua); got != c.want {
				t.Errorf("Parse(%q)\n got  %+v\n want %+v", c.ua, got, c.want)
			}
		})
	}
}
