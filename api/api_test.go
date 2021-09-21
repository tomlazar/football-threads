package api

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestApi_GetWeekSchedule(t *testing.T) {
	var (
		api = New(os.Getenv("FB_THREADS_API_KEY"), nil)
		is  = is.New(t)
	)

	res, err := api.GetWeekSchedule(context.Background(), 2021, 2)
	is.NoErr(err)
	is.True(res.ID != "")

	now := time.Now()
	t.Logf("%30v\t%v", "today", now.Format("2006-01-02"))

	for _, game := range res.Week.Games {
		local := game.ScheduledLocal()
		t.Logf("%30v\t%v", fmt.Sprintf("%v vs %v", game.Home.Alias, game.Away.Alias), local.Format("2006-01-02"))

		// t.Logf("game today: %v vs %v (%v)", game.Home.Name, game.Away.Name, local)

		if local.Year() == now.Year() && local.Month() == now.Month() && local.Day() == now.Day() {
			t.Logf("game today: %v vs %v (%v)", game.Home.Name, game.Away.Name, local)
		}
	}
}
