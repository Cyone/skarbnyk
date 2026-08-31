package parse

import (
	"fmt"
	"testing"
	"time"
)

func TestClassifyApartment(t *testing.T) {
	a := Classify(
		"Двокімнатна квартира загальною площею 44,97 кв.м. м. Кривий Ріг",
		"",
		"",
	)
	if a.Kind != KindApt {
		t.Fatalf("kind=%s", a.Kind)
	}
	if a.Rooms != 2 {
		t.Fatalf("rooms=%d", a.Rooms)
	}
	if a.Area < 44 || a.Area > 45 {
		t.Fatalf("area=%v", a.Area)
	}
	if a.City != "Кривий" {
		t.Fatalf("city=%q", a.City)
	}
	if a.Confidence < 0.6 {
		t.Fatalf("confidence=%v", a.Confidence)
	}
}

func TestClassifyCar(t *testing.T) {
	a := Classify(
		"Легковий автомобіль BMW X5 2014 року",
		"VIN WBAKS410900000000",
		"34110000-1",
	)
	if a.Kind != KindCar {
		t.Fatalf("kind=%s", a.Kind)
	}
	if a.Brand != "BMW" {
		t.Fatalf("brand=%s", a.Brand)
	}
	if a.Year != 2014 {
		t.Fatalf("year=%d", a.Year)
	}
}

func TestClassifySkipLandLease(t *testing.T) {
	a := Classify(
		"Аукціон з продажу права оренди на 16 земельних ділянок",
		"",
		"",
	)
	if a.Kind != KindSkip {
		t.Fatalf("kind=%s", a.Kind)
	}
}

func TestClassifyCityNotSquareMetreUnit(t *testing.T) {
	a := Classify("Квартира 58,3 кв.м. м. Полтава", "", "")
	if a.City != "Полтава" {
		t.Fatalf("city=%q", a.City)
	}
}

func TestClassifyHouse(t *testing.T) {
	a := Classify("Житловий будинок площею 96,4 кв.м. м. Ніжин", "", "")
	if a.Kind != KindApt {
		t.Fatalf("kind=%s", a.Kind)
	}
}

func TestClassifyHouseWithLandNotSkipped(t *testing.T) {
	for _, title := range []string{
		"Житловий будинок з земельною ділянкою",
		"Житловий будинок та земельна ділянка",
	} {
		if a := Classify(title, "", ""); a.Kind != KindApt {
			t.Fatalf("%q kind=%s", title, a.Kind)
		}
	}
}

func TestClassifyYearCeiling(t *testing.T) {
	ok := time.Now().Year() + 1
	a := Classify(fmt.Sprintf("Легковий автомобіль BMW %d року", ok), "", "")
	if a.Year != ok {
		t.Fatalf("year=%d want %d", a.Year, ok)
	}
	if a := Classify(fmt.Sprintf("Легковий автомобіль BMW %d року", ok+1), "", ""); a.Year != 0 {
		t.Fatalf("too-new year=%d", a.Year)
	}
}

func TestClassifyIBANNotCar(t *testing.T) {
	a := Classify(
		"Нежитлове приміщення площею 55 кв.м",
		"Оплата на рахунок UA213223130000026007233566001",
		"",
	)
	if a.Kind == KindCar {
		t.Fatalf("kind=%s", a.Kind)
	}
}

func TestClassifyBareVINCar(t *testing.T) {
	a := Classify("Лот № 5", "номер кузова WBAKS410900000000", "")
	if a.Kind != KindCar {
		t.Fatalf("kind=%s", a.Kind)
	}
}

func TestClassifyBusNotCar(t *testing.T) {
	a := Classify("Автобус ЛАЗ-695 1998 року випуску", "", "")
	if a.Kind == KindCar {
		t.Fatalf("kind=%s", a.Kind)
	}
}

func TestSettlement(t *testing.T) {
	cases := []struct{ loc, official, want string }{
		{"Ємільчинський район/смт ємільчине", "ЄМІЛЬЧИНСЬКИЙ РАЙОН/СМТ ЄМІЛЬЧИНЕ", "Ємільчине"},
		{"смт. ємільчине", "", "Ємільчине"},
		{"Громадське", "", "Громадське"},
		{"Київ", "КИЇВ", "Київ"},
		{"м. Одеса", "ОДЕСА", "Одеса"},
		{"ПОВОРСЬКА/С.ПОВОРСЬК", "ПОВОРСЬКА/С.ПОВОРСЬК", "Поворськ"},
		{"Ремчицька/с.ремчиці", "", "Ремчиці"},
		{"Великожолудська/с.великий жолудськ", "", "Великий Жолудськ"},
		{"Корсунь-шевченківський район/м.корсунь-шевченківський", "", "Корсунь-Шевченківський"},
		{"Слобожанська міська територіальна громада", "ХАРКІВСЬКА ОБЛАСТЬ/М.ХАРКІВ", "Слобожанська"},
		{"Суботцівська сільська громада", "СУБОТЦІВСЬКА/С.СУБОТЦІ", "Суботці"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := Settlement(c.loc, c.official); got != c.want {
			t.Fatalf("Settlement(%q,%q)=%q want %q", c.loc, c.official, got, c.want)
		}
	}
}

func TestClassifySkipRules(t *testing.T) {
	for _, title := range []string{
		"Металобрухт чорних металів, 12 тонн",
		"Право вимоги за кредитним договором",
		"Майнові права на об'єкт незавершеного будівництва",
	} {
		if a := Classify(title, "", ""); a.Kind != KindSkip {
			t.Fatalf("%q kind=%s", title, a.Kind)
		}
	}
}
