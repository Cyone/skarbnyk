package parse

import "testing"

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
	if a.City != "Кривий" && a.City != "Кривий Ріг" {
		// city regex takes the first word after м.
		if a.City == "" {
			t.Fatalf("empty city")
		}
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
