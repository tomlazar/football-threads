package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"text/tabwriter"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/caarlos0/env"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/tomlazar/football-threads/api"

	_ "github.com/joho/godotenv/autoload"
)

type State struct {
	LastActivity  time.Time
	LastDay       time.Time
	Year          int
	FootballFacts []string
}

func WriteState(state State) error {
	f, err := os.Create(".state.json")
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(state); err != nil {
		return err
	}
	return nil
}

func ReadState() (State, error) {
	var state State
	f, err := os.Open(".state.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}

		return state, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&state); err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}

		return state, err
	}
	return state, nil
}

type GameWeek struct {
	WeekNo            int
	FirstDay, LastDay time.Time
}

func (gw GameWeek) ThreadName() string {
	var start, stop string

	start = gw.FirstDay.Format("Jan _2")
	if gw.FirstDay.Month() == gw.LastDay.Month() {
		stop = gw.LastDay.Format("_2")
	} else {
		stop = gw.LastDay.Format("Jan _2")
	}

	return fmt.Sprintf("NFL Game Week %v (%v - %v)", gw.WeekNo, start, stop)
}

func TimeInWeek(t time.Time, gw GameWeek) bool {
	return t.After(gw.FirstDay) && t.Before(gw.LastDay)
}

func GenerateGameWeeks(firstThrus, firstMon time.Time) map[int]GameWeek {
	r := map[int]GameWeek{
		1: {1, firstThrus, firstMon},
	}
	for i := 2; i <= 18; i++ {
		r[i] = GameWeek{
			WeekNo:   i,
			FirstDay: r[i-1].FirstDay.AddDate(0, 0, 7),
			LastDay:  r[i-1].LastDay.AddDate(0, 0, 7),
		}
	}

	return r
}

func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
func today() time.Time {
	now := time.Now()
	return date(now.Year(), int(now.Month()), now.Day())
}

type Config struct {
	Debug      bool   `env:"FB_THREADS_DEBUG"`
	BotToken   string `env:"FB_THREADS_BOT_TOKEN"`
	BotChannel string `env:"FB_THREADS_CHANNEL"`
	BotGuild   string `env:"FB_THREADS_GUILD"`
	ApiKey     string `env:"FB_THREADS_API_KEY"`
}

func MustLoadConfig() (bool, Config) {
	var (
		debug bool
	)
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.Parse()

	var config Config
	if err := env.Parse(&config); err != nil {
		fmt.Printf("%+v\n", err)
	}

	return debug, config
}

func runmain(logger zerolog.Logger, cfg Config, state State, api *api.Api) (State, error) {
	gw := GenerateGameWeeks(date(2021, int(time.September), 9), date(2021, int(time.September), 14))

	// ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	// defer cancel()

	var currentweek GameWeek
	now := today()

	for week := 1; week <= 18; week++ {
		currentweek = gw[week]

		if now.Before(currentweek.LastDay) {
			break
		}
	}

	if currentweek == (GameWeek{}) {
		return state, errors.WithStack(errors.New("no current week found"))
	}

	if time.Now().Before(currentweek.FirstDay) {
		logger.Debug().Int("gw", currentweek.WeekNo).Msg("waiting for first day ofweek")
		return state, nil
	}

	logger.Debug().Int("gw", currentweek.WeekNo).Time("first", currentweek.FirstDay).Time("last", currentweek.LastDay).Msg("starting")

	d, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return state, errors.Wrap(errors.WithStack(err), "failed to create discord session")
	}
	defer d.Close()

	tr, err := d.ThreadsListActive(cfg.BotGuild)
	if err != nil {
		return state, errors.Wrap(errors.WithStack(err), "failed to get active threads")
	}

	var target *discordgo.Channel
	for _, th := range tr.Threads {
		logger.Debug().Str("thread", th.ID).Str("name", th.Name).Str("topic", th.Topic).Str("channel", th.ParentID).Msg("found thread")

		if th.ParentID == cfg.BotChannel && th.Name == currentweek.ThreadName() {
			target = th
			break
		}
	}
	if target == nil {
		target, err = d.ThreadStartWithoutMessage(cfg.BotChannel, &discordgo.ThreadCreateData{
			Name:                currentweek.ThreadName(),
			Type:                discordgo.ChannelTypeGuildPublicThread,
			AutoArchiveDuration: discordgo.ArchiveDurationOneDay,
		})
		if err != nil {
			return state, errors.Wrap(errors.WithStack(err), "failed to create thread")
		}

		_, err = d.ChannelMessageSend(target.ID, "Welcome to the thread for the current game week.\n\n")
		if err != nil {
			return state, errors.Wrap(errors.WithStack(err), "failed to send message to thread")
		}

		logger.Info().Msg("created thread")
		state.LastActivity = time.Now()
	}

	if today() != state.LastDay {
		var (
			title = fmt.Sprintf("Today's games %v.", today().Format("Monday, Jan 2"))
			body  = bytes.Buffer{}
			tab   = tabwriter.NewWriter(&body, 0, 0, 2, ' ', 0)
		)

		games, err := api.GetGamesOnDay(context.Background(), state.Year, currentweek.WeekNo, today())
		if err != nil {
			return state, errors.Wrap(errors.WithStack(err), "failed to get games")
		}

		fmt.Fprintln(tab, "AWAY\tHOME\tSCHEDULED")
		for _, game := range games {
			fmt.Fprintf(tab, "%v\t%v\t%v\n", game.Away.Name, game.Home.Name, game.ScheduledLocal().Format("3:04 PM MST"))
		}
		tab.Flush()

		_, err = d.ChannelMessageSendEmbed(target.ID, &discordgo.MessageEmbed{
			Title:       title,
			Description: fmt.Sprintf("```\n%v\n```", body.String()),
			URL:         "https://www.espn.com/nfl/schedule",
			Footer: &discordgo.MessageEmbedFooter{
				Text: "Powered by GO!",
			},
		})
		if err != nil {
			return state, errors.Wrap(errors.WithStack(err), "failed to send message to thread")
		}

		logger.Info().Msg("sent daily schedule to thread")
		state.LastActivity = time.Now()
		state.LastDay = today()
	}

	if time.Since(state.LastActivity) > time.Hour*6 {
		// choose the message, the default bot stuff, or the custom stuff
		message := "Hi! I'm the NFL Threads bot. I'm here to help you keep track of the games in the NBA."
		if len(state.FootballFacts) > 0 {
			message = state.FootballFacts[rand.Int()%len(state.FootballFacts)]
		}

		_, err = d.ChannelMessageSend(target.ID, message)
		if err != nil {
			return state, errors.Wrap(errors.WithStack(err), "failed to send message")
		}
		logger.Info().Msg("sent message to thread")
		state.LastActivity = time.Now()
	}

	return state, nil
}

func main() {
	var (
		debug, cfg = MustLoadConfig()
		api        = api.New(cfg.ApiKey, nil)

		logger = zerolog.New(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05.000",
		}).With().Timestamp().Logger()
	)

	if !debug {
		logger = logger.Level(zerolog.InfoLevel)
	}
	logger.Debug().Msg("| == [START] loading configuration == ")
	logger.Debug().Msgf("|   token = '%v'", cfg.BotToken)
	logger.Debug().Msgf("| channel = '%v'", cfg.BotChannel)
	logger.Debug().Msgf("|   guild = '%v'", cfg.BotGuild)
	logger.Debug().Msg("| ==  [DONE] loading configuration == ")

	// read the state
	state, err := ReadState()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to read state")
		os.Exit(1)
	}
	logger.Debug().Msg("starting")

	// run the main loop
	state, err = runmain(logger, cfg, state, api)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to read state")
		os.Exit(1)
	}

	// write the state
	if err := WriteState(state); err != nil {
		logger.Fatal().Err(err).Msg("failed to write out the state")
		os.Exit(1)
	}
}
