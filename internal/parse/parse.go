package parse

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Kind string

const (
	KindCar  Kind = "car"
	KindApt  Kind = "apt"
	KindSkip Kind = "skip"
)

type Attrs struct {
	Kind       Kind
	Rooms      int
	Area       float64
	Year       int
	City       string
	Brand      string
	Model      string
	Confidence float64
}

var (
	reYear     = regexp.MustCompile(`\b((?:19|20)\d{2})\b`)
	reRooms    = regexp.MustCompile(`(?i)(\d+)\s*[-–]?\s*кімнат`)
	reArea     = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(?:кв\.?\s*м|м²|м2)`)
	reCity     = regexp.MustCompile(`(?i)(?:^|[\s,;(])(?:м\.|місто)\s*([А-ЯІЇЄҐа-яіїєґ'’\-]+)`)
	reVIN      = regexp.MustCompile(`(?i)\bVIN\b`)
	reVINCode  = regexp.MustCompile(`\b[A-HJ-NPR-Z0-9]{17}\b`)
	reApt      = regexp.MustCompile(`(?i)квартир|кімнатн`)
	reHouse    = regexp.MustCompile(`(?i)будин(ок|ку|ка|ком)`)
	reCar      = regexp.MustCompile(`(?i)автомобіль|легков(ий|ого)|транспортний засіб|(?:^|[^а-яіїєґА-ЯІЇЄҐa-zA-Z])авто(?:[^а-яіїєґА-ЯІЇЄҐa-zA-Z]|$)`)
	reSkip     = regexp.MustCompile(`(?i)право оренди|земельн(а|их|ої) ділян|брухт|металобрухт|право вимоги|майнов(і|их) прав`)
	reLandOnly = regexp.MustCompile(`(?i)земельн`)
)

// Common marks so classification works before RIA dicts load.
var brands = []string{
	"Toyota", "Volkswagen", "VW", "BMW", "Mercedes", "Mercedes-Benz", "Audi", "Renault",
	"Skoda", "Škoda", "Hyundai", "Kia", "Ford", "Nissan", "Mazda", "Honda", "Opel",
	"Peugeot", "Citroen", "Citroën", "Chevrolet", "Daewoo", "Lada", "ВАЗ", "ЗАЗ",
	"Mitsubishi", "Suzuki", "Volvo", "Lexus", "Porsche", "Tesla", "Chery", "Geely",
}

func Classify(title, description, classID string) Attrs {
	text := title + "\n" + description
	a := Attrs{Kind: KindSkip}

	if classID != "" {
		if strings.HasPrefix(classID, "34") {
			a.Kind = KindCar
			a.Confidence = 0.7
		}
		if strings.HasPrefix(classID, "04") || strings.HasPrefix(classID, "70") {
			a.Kind = KindApt
			a.Confidence = 0.65
		}
	}

	if reSkip.MatchString(text) && !reApt.MatchString(text) && !reHouse.MatchString(text) && !reCar.MatchString(text) {
		a.Kind = KindSkip
		a.Confidence = 0.9
		return a
	}
	if reLandOnly.MatchString(text) && !reApt.MatchString(text) && !reHouse.MatchString(text) && !reCar.MatchString(text) {
		a.Kind = KindSkip
		a.Confidence = 0.85
		return a
	}

	if reCar.MatchString(text) || brandIn(text) != "" || hasVIN(text) {
		a.Kind = KindCar
		if a.Confidence < 0.75 {
			a.Confidence = 0.8
		}
	}
	if reApt.MatchString(text) || reHouse.MatchString(text) {
		a.Kind = KindApt
		if a.Confidence < 0.75 {
			a.Confidence = 0.8
		}
	}

	if a.Kind == KindSkip {
		return a
	}

	if m := reYear.FindStringSubmatch(text); len(m) > 1 {
		if y, err := strconv.Atoi(m[1]); err == nil && y >= 1970 && y <= time.Now().Year()+1 {
			a.Year = y
			a.Confidence += 0.05
		}
	}
	if n := roomsIn(text); n > 0 {
		a.Rooms = n
		a.Confidence += 0.05
	}
	if m := reArea.FindStringSubmatch(text); len(m) > 1 {
		s := strings.ReplaceAll(m[1], ",", ".")
		if v, err := strconv.ParseFloat(s, 64); err == nil && v > 0 && v < 10000 {
			a.Area = v
			a.Confidence += 0.05
		}
	}
	if m := reCity.FindStringSubmatch(text); len(m) > 1 {
		a.City = titleCase(m[1])
		a.Confidence += 0.03
	}
	if a.Kind == KindCar {
		a.Brand = brandIn(text)
		if a.Brand != "" {
			a.Confidence += 0.07
		}
	}
	if a.Confidence > 1 {
		a.Confidence = 1
	}
	return a
}

// hasVIN keeps IBANs and account numbers out: a bare code counts only when it
// carries a letter, and ASCII \b stops a 17-char window inside a longer run.
func hasVIN(text string) bool {
	if reVIN.MatchString(text) {
		return true
	}
	for _, m := range reVINCode.FindAllString(text, -1) {
		if strings.ContainsAny(m, "ABCDEFGHJKLMNPRSTUVWXYZ") {
			return true
		}
	}
	return false
}

func roomsIn(text string) int {
	if m := reRooms.FindStringSubmatch(text); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 && n < 20 {
			return n
		}
	}
	low := strings.ToLower(text)
	for _, p := range []struct {
		w string
		n int
	}{
		{"однокімнат", 1},
		{"двокімнат", 2},
		{"трикімнат", 3},
		{"чотирикімнат", 4},
		{"п'ятикімнат", 5},
		{"пʼятикімнат", 5},
		{"пятикімнат", 5},
		{"шестикімнат", 6},
		{"семикімнат", 7},
		{"восьмикімнат", 8},
	} {
		if strings.Contains(low, p.w) {
			return p.n
		}
	}
	return 0
}

func brandIn(text string) string {
	low := strings.ToLower(text)
	best := ""
	for _, b := range brands {
		if strings.Contains(low, strings.ToLower(b)) && len(b) > len(best) {
			best = b
		}
	}
	if best == "VW" {
		return "Volkswagen"
	}
	return best
}

func titleCase(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
