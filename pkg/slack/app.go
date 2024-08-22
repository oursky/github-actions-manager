package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/github/jobs"
	"github.com/oursky/github-actions-manager/pkg/kv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var repoRegex = regexp.MustCompile("[a-zA-Z0-9-]+(/[a-zA-Z0-9-]+)?")

type App struct {
	logger      *zap.Logger
	disabled    bool
	api         *slack.Client
	store       kv.Store
	commandName string
	commands    *[]Command
}

type ChannelInfo struct {
	ChannelID string        `json:"channelID"`
	Filter    MessageFilter `json:"filter"`
}

func (f ChannelInfo) String() string {
	if f.Filter.Length() == 0 {
		return f.ChannelID
	}
	return fmt.Sprintf("%s with %s", f.ChannelID, f.Filter)
}

func (f ChannelInfo) ShouldSend(run *jobs.WorkflowRun) (bool, error) {
	if f.Filter.Length() == 0 {
		return true, nil
	}
	result, err := f.Filter.Any(run)
	if err != nil {
		return false, err
	}
	return result, nil
}

func NewApp(logger *zap.Logger, config *Config, store kv.Store) *App {
	logger = logger.Named("slack-app")
	return &App{
		logger:   logger,
		disabled: config.Disabled,
		api: slack.New(
			config.BotToken,
			slack.OptionLog(zap.NewStdLog(logger)),
			slack.OptionAppLevelToken(config.AppToken),
		),
		store:       store,
		commandName: config.GetCommandName(),
		commands:    GetCommands(),
	}
}

func (a *App) Disabled() bool {
	return a.disabled
}

func (a *App) GetChannels(ctx context.Context, repo string) ([]ChannelInfo, error) {
	data, err := a.store.Get(ctx, kvNamespace, repo)
	if err != nil {
		return nil, err
	} else if data == "" {
		return nil, nil
	}

	var channelInfos []ChannelInfo
	err = json.Unmarshal([]byte(data), &channelInfos)

	if err != nil {
		// Maybe it's using the old format? Handle this case
		channelInfoStrings := strings.Split(data, ";")
		var channelInfos []ChannelInfo

		for _, channelString := range channelInfoStrings {
			channelID, conclusionsString, _ := strings.Cut(channelString, ":")
			var conclusions []Conclusion
			for _, conclusionString := range strings.Split(conclusionsString, ",") {
				if len(conclusionString) > 0 {
					conclusion, err := NewConclusionFromString(conclusionString)
					if err != nil {
						return nil, err
					}
					conclusions = append(conclusions, conclusion)
				}
			}
			conclusionRule, err := NewFilterRule("conclusions", []string{}, conclusions)
			if err != nil {
				return nil, err
			}

			filter := NewFilter([]MessageFilterRule{*conclusionRule})
			channelInfos = append(channelInfos, ChannelInfo{
				ChannelID: channelID,
				Filter:    filter,
			})
		}

		return channelInfos, nil
	}
	return channelInfos, nil
}

func (a *App) AddChannel(ctx context.Context, repo string, channelInfo ChannelInfo) error {
	channelInfos, err := a.GetChannels(ctx, repo)
	if err != nil {
		return err
	}

	var newChannelInfos []ChannelInfo
	for _, c := range channelInfos {
		if c.ChannelID == channelInfo.ChannelID {
			// Skip the old subscription and will replace with the new conclusion filter options
			continue
		}
		newChannelInfos = append(newChannelInfos, c)
	}
	newChannelInfos = append(newChannelInfos, channelInfo)

	data, err := json.Marshal(newChannelInfos)
	if err != nil {
		return err
	}

	return a.store.Set(ctx, kvNamespace, repo, string(data))
}

func (a *App) DelChannel(ctx context.Context, repo string, channelID string) error {
	channelInfos, err := a.GetChannels(ctx, repo)
	if err != nil {
		return err
	}

	var newChannelInfos []ChannelInfo
	found := false
	for _, c := range channelInfos {
		if c.ChannelID == channelID {
			found = true
			continue
		}
		newChannelInfos = append(newChannelInfos, c)
	}
	if !found {
		return fmt.Errorf("not subscribed to repo")
	}

	data, err := json.Marshal(newChannelInfos)
	if err != nil {
		return err
	}

	return a.store.Set(ctx, kvNamespace, repo, string(data))
}

func (a *App) SendMessage(ctx context.Context, channel string, options ...slack.MsgOption) error {
	_, _, _, err := a.api.SendMessageContext(ctx, channel, options...)
	return err
}

func (a *App) Start(ctx context.Context, g *errgroup.Group) error {
	if a.disabled {
		return nil
	}

	client := socketmode.New(
		a.api,
		socketmode.OptionLog(zap.NewStdLog(a.logger)),
	)

	g.Go(func() error {
		a.messageLoop(ctx, client)
		return nil
	})

	g.Go(func() error {
		err := client.RunContext(ctx)
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("slack: %w", err)
		}
		return nil
	})

	return nil
}

func (a *App) messageLoop(ctx context.Context, client *socketmode.Client) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-client.Events:
			switch data := e.Data.(type) {
			case *slack.ConnectingEvent:
				a.logger.Debug("connecting")
			case *slack.ConnectionErrorEvent:
				a.logger.Debug("connection error")
			case *socketmode.ConnectedEvent:
				a.logger.Debug("connected")

			case slack.SlashCommand:
				a.logger.Debug("slash command",
					zap.String("channel", data.ChannelName),
					zap.String("channelID", data.ChannelID),
					zap.String("user", data.UserName),
					zap.String("command", data.Command),
					zap.String("text", data.Text),
				)

				response := a.Handle(ctx, data)
				client.Ack(*e.Request, response)

			default:
				if e.Type == socketmode.EventTypeHello {
					continue
				}
				a.logger.Warn("unexpected event", zap.String("type", string(e.Type)))
			}
		}
	}
}
