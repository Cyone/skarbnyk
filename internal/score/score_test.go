package score

import "testing"

func TestDiscount(t *testing.T) {
	d := Discount(500_000, 1_000_000)
	if d < 0.499 || d > 0.501 {
		t.Fatalf("got %v", d)
	}
	if Discount(10, 0) != 0 {
		t.Fatal("zero market")
	}
	if Discount(-1, 100) != 0 {
		t.Fatal("negative start")
	}
}

func TestFamily(t *testing.T) {
	if Family("bankRuptcy-english") != "english" {
		t.Fatal("english")
	}
	if Family("arrestedAssets-dutch") != "dutch" {
		t.Fatal("dutch")
	}
}

func TestPassN(t *testing.T) {
	if PassN(3, "") != 3 {
		t.Fatal("attempts")
	}
	if PassN(0, "BRE001-UA-1") != 2 {
		t.Fatal("previous")
	}
	if PassN(0, "") != 1 {
		t.Fatal("first")
	}
}
