package timescar

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// slotCells renders a car's 48 quarter-hour cells; every fourth is the site's
// thicker hour-boundary cell, which must still count as a slot.
func slotCells(states []string) string {
	var b strings.Builder
	for i, s := range states {
		class := "timelinedot"
		if i%4 == 3 {
			class = "timelinespace"
		}
		fmt.Fprintf(&b, "<td class='%s timeline %s'>&nbsp;</td>", class, s)
	}
	return b.String()
}

func states(fill string, vacant ...int) []string {
	s := make([]string, 48)
	for i := range s {
		s[i] = fill
	}
	for _, i := range vacant {
		s[i] = string(slotVacant)
	}
	return s
}

func TestParseAvailability(t *testing.T) {
	from := time.Date(2026, 10, 12, 10, 0, 0, 0, Tokyo)
	to := from.Add(12 * time.Hour)
	html := "<h4 class='nomargin4'><a onclick=openPop('LA10')>新宿サブナード駐車場（２）</a></h4>" +
		"<div id='1' class='tableon'><p class='carname'><a>ベーシック／A</a><span><a href='../reserve/input.jsp?scd=LA10&carId=1'>x</a></span></p><table>" +
		slotCells(states("full", 0, 1, 2, 3)) + "</table></div>" +
		"<div id='2' class='tableoff'><p class='carname'><a>ミドル／B</a></p><table>" +
		slotCells(states("vacant")) + "</table></div>" +
		"<h4 class='nomargin4'><a onclick=openPop('BM08')>別のステーション</a></h4>" +
		"<div id='3' class='tableon'><p class='carname'><a>ベーシック／C</a></p><table>" +
		slotCells(states("impossible")) + "</table></div>"

	res, err := parseAvailability(html, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stations) != 2 {
		t.Fatalf("stations = %d, want 2: %+v", len(res.Stations), res.Stations)
	}
	first := res.Stations[0]
	if first.Code != "LA10" || first.Name != "新宿サブナード駐車場（２）" {
		t.Errorf("station = %+v", first)
	}
	if len(first.Cars) != 2 {
		t.Fatalf("cars = %d, want 2", len(first.Cars))
	}
	if got := first.Cars[0].Name; got != "ベーシック／A" {
		t.Errorf("car[0].Name = %q", got)
	}
	if want := []Range{{From: from, To: from.Add(time.Hour)}}; !equalRanges(first.Cars[0].Available, want) {
		t.Errorf("car[0].Available = %v, want %v", first.Cars[0].Available, want)
	}
	if want := []Range{{From: from, To: to}}; !equalRanges(first.Cars[1].Available, want) {
		t.Errorf("car[1].Available = %v, want %v", first.Cars[1].Available, want)
	}
	if got := res.Stations[1].Cars[0].Available; len(got) != 0 {
		t.Errorf("car with only 'impossible' slots should be free never, got %v", got)
	}
}

func TestParseAvailabilityClipsWindow(t *testing.T) {
	// Window starts mid-hour and ends before the timetable's end.
	from := time.Date(2026, 10, 12, 10, 30, 0, 0, Tokyo)
	to := time.Date(2026, 10, 12, 11, 30, 0, 0, Tokyo)
	html := "<h4 class='nomargin4'><a onclick=openPop('X')>S</a></h4>" +
		"<div id='1' class='tableon'><p class='carname'><a>C</a></p><table>" +
		slotCells(states("vacant")) + "</table></div>"

	res, err := parseAvailability(html, from, to)
	if err != nil {
		t.Fatal(err)
	}
	want := []Range{{From: from, To: to}}
	if got := res.Stations[0].Cars[0].Available; !equalRanges(got, want) {
		t.Errorf("Available = %v, want %v", got, want)
	}
}

func equalRanges(a, b []Range) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].From.Equal(b[i].From) || !a[i].To.Equal(b[i].To) {
			return false
		}
	}
	return true
}
