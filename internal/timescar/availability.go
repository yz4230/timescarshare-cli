package timescar

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	_ "time/tzdata" // embed the IANA database so Asia/Tokyo works on any OS
)

// Tokyo is the timezone the site schedules in.
var Tokyo = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return time.FixedZone("JST", 9*60*60)
	}
	return loc
}()

// Range is a contiguous period during which a car is free.
type Range struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// CarAvailability is one car at a station and the periods it is free.
type CarAvailability struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Available []Range `json:"available"`
}

// StationAvailability is one station and the availability of its cars.
type StationAvailability struct {
	Code string            `json:"code"`
	Name string            `json:"name"`
	Cars []CarAvailability `json:"cars"`
}

// Availability is the availability of every car at the requested stations
// within [From, To).
type Availability struct {
	From     time.Time             `json:"from"`
	To       time.Time             `json:"to"`
	Stations []StationAvailability `json:"stations"`
}

// slotState maps the site's timeline classes to whether a 15-minute slot is
// bookable.
type slotState string

const (
	slotVacant     slotState = "vacant"     // 空きあり
	slotFull       slotState = "full"       // 空きなし
	slotImpossible slotState = "impossible" // 予約不可
	slotAccident   slotState = "accident"   // メンテナンス
	slotAlready    slotState = "already"    // 予約済み（本人）
)

// Availability fetches the timetable for the given station codes and returns
// each car's free periods clipped to [from, to).
func (c *Client) Availability(ctx context.Context, codes []string, from, to time.Time) (*Availability, error) {
	if len(codes) == 0 {
		return nil, fmt.Errorf("no station codes given")
	}
	from = from.In(Tokyo)
	to = to.In(Tokyo)
	if !to.After(from) {
		return nil, fmt.Errorf("end %s is not after start %s", to.Format(time.RFC3339), from.Format(time.RFC3339))
	}
	dur := to.Sub(from)

	zeros := strings.TrimSuffix(strings.Repeat("0,", len(codes)), ",")
	form := url.Values{}
	form.Set("component", "station_stationMapPage")
	form.Set("action", "ajaxCreateTimeTable")
	form.Set("time", time.Now().Format("Mon Jan 02 2006 15:04:05 GMT-0700"))
	form.Set("stationCdAjax", strings.Join(codes, ","))
	form.Set("latAjax", zeros)
	form.Set("lonAjax", zeros)
	form.Set("centerLon", "0")
	form.Set("centerLat", "0")
	form.Set("startSearch", from.Format("2006-01-02 15:04:05"))
	form.Set("dateSpace", from.Format("2006-01-02")+" 00:00:00.0")
	form.Set("hourSpace", fmt.Sprint(from.Hour()))
	form.Set("useHour", fmt.Sprint(int(dur.Hours())))
	form.Set("useMinute", fmt.Sprint(int(dur.Minutes())%60))
	form.Set("sDate", fmt.Sprint(from.UnixMilli()))
	form.Set("eDate", fmt.Sprint(to.UnixMilli()))
	form.Set("mainteCard", "false")
	form.Set("bookingPeriod", "21")
	form.Set("tcsStationCdAjax", "")

	body, err := c.post(ctx, baseURL+"/view/station/teeda.ajax", form)
	if err != nil {
		return nil, err
	}
	if isLoginPage(body) {
		return nil, errNotLoggedIn
	}
	res, err := parseAvailability(body, from, to)
	if err != nil {
		return nil, err
	}
	res.From, res.To = from, to
	return res, nil
}

func parseAvailability(body string, from, to time.Time) (*Availability, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse timetable: %w", err)
	}
	// The timetable always starts on the hour that `from` falls in.
	windowStart := time.Date(from.Year(), from.Month(), from.Day(), from.Hour(), 0, 0, 0, from.Location())

	res := &Availability{}
	var station *StationAvailability
	for _, sel := range doc.Find("h4.nomargin4, div.tableon, div.tableoff").EachIter() {
		if goquery.NodeName(sel) == "h4" {
			res.Stations = append(res.Stations, StationAvailability{
				Code: codeFromOnclick(sel),
				Name: strings.TrimSpace(sel.Text()),
			})
			station = &res.Stations[len(res.Stations)-1]
			continue
		}
		if station == nil {
			continue
		}
		car := CarAvailability{ID: sel.AttrOr("id", "")}
		car.Name = strings.TrimSpace(sel.Find("p.carname a").First().Text())
		car.Available = freeRanges(sel, windowStart, from, to)
		station.Cars = append(station.Cars, car)
	}
	return res, nil
}

// codeFromOnclick reads the station code out of `onclick=openPop('LA10')`.
func codeFromOnclick(sel *goquery.Selection) string {
	onclick, _ := sel.Find("a").First().Attr("onclick")
	const marker = "openPop('"
	_, after, ok := strings.Cut(onclick, marker)
	if !ok {
		return ""
	}
	rest := after
	if before, _, ok := strings.Cut(rest, "'"); ok {
		return before
	}
	return ""
}

// freeRanges merges the vacant 15-minute slots into ranges within [from, to).
func freeRanges(car *goquery.Selection, windowStart, from, to time.Time) []Range {
	ranges := []Range{}
	var cur *Range
	flush := func() {
		if cur != nil {
			ranges = append(ranges, *cur)
			cur = nil
		}
	}
	for i, td := range car.Find("td.timelinedot, td.timelinespace").EachIter() {
		start := windowStart.Add(time.Duration(i) * 15 * time.Minute)
		end := start.Add(15 * time.Minute)
		if start.Before(from) || end.After(to) {
			flush()
			continue
		}
		if stateOf(td.AttrOr("class", "")) != slotVacant {
			flush()
			continue
		}
		if cur == nil {
			cur = &Range{From: start, To: end}
		} else {
			cur.To = end
		}
	}
	flush()
	return ranges
}

func stateOf(class string) slotState {
	for tok := range strings.FieldsSeq(class) {
		switch slotState(tok) {
		case slotVacant, slotFull, slotImpossible, slotAccident, slotAlready:
			return slotState(tok)
		}
	}
	return ""
}
