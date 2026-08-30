package ria

import "testing"

func TestMatchMark(t *testing.T) {
	marks := []Mark{{ID: 9, Name: "BMW"}, {ID: 79, Name: "Toyota"}}
	if MatchMark("BMW", marks) != 9 {
		t.Fatal("exact")
	}
	if MatchMark("toyota", marks) != 79 {
		t.Fatal("fold")
	}
	if MatchMark("", marks) != 0 {
		t.Fatal("empty")
	}
}

func TestSpecHashStable(t *testing.T) {
	if SpecHash("car", 9, 0, 2014, 0) != SpecHash("car", 9, 0, 2014, 0) {
		t.Fatal("stable")
	}
	if SpecHash("car", 9, 0, 2014, 0) == SpecHash("car", 9, 0, 2015, 0) {
		t.Fatal("year")
	}
}
