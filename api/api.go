package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

func (g Game) ScheduledUTC() time.Time {
	local, err := time.Parse("2006-01-02T15:04:05-07:00", g.Scheduled)
	if err != nil {
		return time.Time{}
	}
	local = local.Add(time.Hour * time.Duration(g.UtcOffset))

	return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), time.UTC)
}

func (g Game) ScheduledLocal() time.Time {
	return g.ScheduledUTC().Local()
}

type Game struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Reference  string `json:"reference"`
	Number     int    `json:"number"`
	Scheduled  string `json:"scheduled"`
	Attendance int    `json:"attendance"`
	UtcOffset  int    `json:"utc_offset"`
	EntryMode  string `json:"entry_mode"`
	Weather    string `json:"weather"`
	SrID       string `json:"sr_id"`
	Venue      struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		City     string `json:"city"`
		State    string `json:"state"`
		Country  string `json:"country"`
		Zip      string `json:"zip"`
		Address  string `json:"address"`
		Capacity int    `json:"capacity"`
		Surface  string `json:"surface"`
		RoofType string `json:"roof_type"`
		SrID     string `json:"sr_id"`
		Location struct {
			Lat string `json:"lat"`
			Lng string `json:"lng"`
		} `json:"location"`
	} `json:"venue"`
	Home struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Alias      string `json:"alias"`
		GameNumber int    `json:"game_number"`
		SrID       string `json:"sr_id"`
	} `json:"home"`
	Away struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Alias      string `json:"alias"`
		GameNumber int    `json:"game_number"`
		SrID       string `json:"sr_id"`
	} `json:"away"`
	Scoring struct {
		HomePoints int `json:"home_points"`
		AwayPoints int `json:"away_points"`
		Periods    []struct {
			PeriodType string `json:"period_type"`
			ID         string `json:"id"`
			Number     int    `json:"number"`
			Sequence   int    `json:"sequence"`
			HomePoints int    `json:"home_points"`
			AwayPoints int    `json:"away_points"`
		} `json:"periods"`
	} `json:"scoring"`
	Broadcast struct {
		Network   string `json:"network"`
		Satellite string `json:"satellite"`
		Internet  string `json:"internet"`
	} `json:"broadcast,omitempty"`
}

type WeekSchedule struct {
	ID   string `json:"id"`
	Year int    `json:"year"`
	Type string `json:"type"`
	Name string `json:"name"`
	Week struct {
		ID       string `json:"id"`
		Sequence int    `json:"sequence"`
		Title    string `json:"title"`
		Games    []Game `json:"games"`
	} `json:"week"`
	Comment string `json:"_comment"`
}

type Api struct {
	apikey string
	client *http.Client
}

func New(apikey string, client *http.Client) *Api {
	if client == nil {
		client = &http.Client{
			Timeout: time.Second * 10,
		}
	}

	return &Api{
		client: client,
		apikey: apikey,
	}
}

func (a Api) baseurl(fragment string) string {
	return "https://api.sportradar.us/nfl/official/trial/v6/en" + fragment + "?api_key=" + a.apikey
}
func (a Api) scheduleurl(year, week int) string {
	return a.baseurl("/games/" + strconv.Itoa(year) + "/REG/" + strconv.Itoa(week) + "/schedule.json")
}

func (a Api) GetWeekSchedule(ctx context.Context, year, week int) (WeekSchedule, error) {
	req, err := http.NewRequest("GET", a.scheduleurl(year, week), nil)
	if err != nil {
		return WeekSchedule{}, errors.WithStack(err)
	}
	req = req.WithContext(ctx)

	resp, err := a.client.Do(req)
	if err != nil {
		return WeekSchedule{}, errors.WithStack(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return WeekSchedule{}, errors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var ws WeekSchedule
	if err := json.NewDecoder(resp.Body).Decode(&ws); err != nil {
		return WeekSchedule{}, errors.WithStack(err)
	}

	return ws, err
}

func (a Api) GetGamesOnDay(ctx context.Context, year, week int, day time.Time) ([]Game, error) {
	ws, err := a.GetWeekSchedule(ctx, year, week)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var games []Game
	for _, g := range ws.Week.Games {
		local := g.ScheduledLocal()
		if local.Year() == day.Year() && local.Month() == day.Month() && local.Day() == day.Day() {
			games = append(games, g)
		}
	}

	return games, nil
}
