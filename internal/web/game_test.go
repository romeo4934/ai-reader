package web

import "testing"

func TestGameScoreCombo(t *testing.T) {
	qs := make([]gameQuestion, 5)
	fast := func(right bool) gameAnswer {
		if right {
			return gameAnswer{Choice: 0, MS: 100}
		}
		return gameAnswer{Choice: 1, MS: 100}
	}
	for _, c := range []struct {
		name    string
		answers []gameAnswer
		want    int
	}{
		{"all right", []gameAnswer{fast(true), fast(true), fast(true), fast(true), fast(true)}, 80},
		{"miss breaks the streak", []gameAnswer{fast(true), fast(true), fast(false), fast(true), fast(true)}, 40},
		{"combo from the third", []gameAnswer{fast(false), fast(true), fast(true), fast(true)}, 40},
		{"time up", []gameAnswer{{Choice: 0, MS: 10000}}, 0},
	} {
		if got, _ := gameScore(qs, c.answers); got != c.want {
			t.Errorf("%s: score %d, want %d", c.name, got, c.want)
		}
	}
	if gameMaxScore != 80 {
		t.Errorf("gameMaxScore = %d, want 80", gameMaxScore)
	}
}

func TestRankFor(t *testing.T) {
	T := map[string]string{"GameRankBronze": "B", "GameRankSilver": "S", "GameRankGold": "G", "GameRankPlatinum": "P", "GameRankOwl": "O"}
	if r := rankFor(T, 7); r.Name != "S" || r.Next != "G" || r.Need != 8 {
		t.Errorf("7 wins: %+v", r)
	}
	if r := rankFor(T, 60); r.Name != "O" || r.Next != "" {
		t.Errorf("60 wins: %+v", r)
	}
}
