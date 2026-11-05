package httpapi

import (
	"cmp"
	"net/http"
	"slices"
	"strconv"
	"time"

	"fearlix/qrcode_generator/server/internal/store"
)

type dayCount struct {
	Date  string `json:"date"` // AAAA-MM-GG nel fuso orario configurato
	Count int    `json:"count"`
}

type labelCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type historyJSON struct {
	OldURL    string    `json:"old_url"`
	NewURL    string    `json:"new_url"`
	ChangedAt time.Time `json:"changed_at"`
	ChangedBy string    `json:"changed_by"`
}

type statsJSON struct {
	Days        int           `json:"days"`
	Total       int           `json:"total"`
	Period      int           `json:"period"`
	BotsIgnored int           `json:"bots_ignored"`
	LastScanAt  *time.Time    `json:"last_scan_at"`
	Daily       []dayCount    `json:"daily"`
	Devices     []labelCount  `json:"devices"`
	OS          []labelCount  `json:"os"`
	Browsers    []labelCount  `json:"browsers"`
	History     []historyJSON `json:"history"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request, sess session) {
	id, ok := qrID(w, r)
	if !ok {
		return
	}
	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 365 {
			writeError(w, http.StatusBadRequest, "days deve essere tra 1 e 365")
			return
		}
		days = n
	}
	// Prima verifica che il QR sia dell'utente (l'admin vede quelli di tutti)
	if _, err := s.store.GetQR(r.Context(), readScope(sess), id); err != nil {
		s.storeError(w, r, err)
		return
	}

	from := startOfPeriod(s.now(), days, s.cfg.Location)
	sum, err := s.store.ScanSummary(r.Context(), id, from)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	history, err := s.store.TargetHistory(r.Context(), id)
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	out := aggregate(sum.Since, from, days, s.cfg.Location)
	out.Total, out.BotsIgnored, out.LastScanAt = sum.Total, sum.BotsSince, sum.LastScanAt
	out.History = make([]historyJSON, len(history))
	for i, h := range history {
		out.History[i] = historyJSON{OldURL: h.OldURL, NewURL: h.NewURL, ChangedAt: h.ChangedAt, ChangedBy: h.ChangedBy}
	}
	writeJSON(w, http.StatusOK, out)
}

// startOfPeriod è la mezzanotte (nel fuso loc) del primo dei `days` giorni che finiscono oggi.
func startOfPeriod(now time.Time, days int, loc *time.Location) time.Time {
	t := now.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day()-(days-1), 0, 0, 0, 0, loc)
}

// aggregate raggruppa le scansioni per giorno e per dispositivo, sistema e browser.
func aggregate(scans []store.Scan, from time.Time, days int, loc *time.Location) statsJSON {
	out := statsJSON{Days: days, Period: len(scans), Daily: make([]dayCount, days)}
	index := map[string]int{}
	for i := range days {
		d := time.Date(from.Year(), from.Month(), from.Day()+i, 0, 0, 0, 0, loc).Format(time.DateOnly)
		out.Daily[i].Date = d
		index[d] = i
	}
	devices, oses, browsers := map[string]int{}, map[string]int{}, map[string]int{}
	for _, sc := range scans {
		if i, ok := index[sc.At.In(loc).Format(time.DateOnly)]; ok {
			out.Daily[i].Count++
		}
		devices[sc.Device]++
		oses[sc.OS]++
		browsers[sc.Browser]++
	}
	out.Devices, out.OS, out.Browsers = sorted(devices), sorted(oses), sorted(browsers)
	return out
}

func sorted(m map[string]int) []labelCount {
	out := make([]labelCount, 0, len(m))
	for k, v := range m {
		out = append(out, labelCount{k, v})
	}
	slices.SortFunc(out, func(a, b labelCount) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Label, b.Label))
	})
	return out
}
