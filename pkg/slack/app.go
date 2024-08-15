package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/kv"
	"github.com/oursky/github-actions-manager/pkg/utils/array"
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
}

type ChannelInfo struct {
	ChannelID   string   `json:"channelID"`
	Conclusions []string `json:"conclusions"`
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
		return nil, err
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

				if data.Command != "/"+a.commandName {
					client.Ack(*e.Request, map[string]interface{}{
						"text": fmt.Sprintf("Unknown command '%s'\n", data.Command)})
					continue
				}

				args := strings.Split(data.Text, " ")
				if len(args) < 2 {
					client.Ack(*e.Request, map[string]interface{}{
						"text": fmt.Sprintf("Please specify subcommand and repo")})
					continue
				}

				repo := args[1]
				subcommand := args[0]
				conclusions := array.Unique(args[2:])
				if !repoRegex.MatchString(repo) {
					client.Ack(*e.Request, map[string]interface{}{
						"text": fmt.Sprintf("Invalid repo '%s'\n", repo),
					})
					continue
				}

				switch subcommand {
				case "subscribe":
					channelInfo := ChannelInfo{
						ChannelID:   data.ChannelID,
						Conclusions: conclusions,
					}
					err := a.AddChannel(ctx, repo, channelInfo)
					if err != nil {
						a.logger.Warn("failed to subscribe", zap.Error(err))
						client.Ack(*e.Request, map[string]interface{}{
							"text": fmt.Sprintf("Failed to subscribe '%s': %s\n", repo, err),
						})
					} else {
						if len(conclusions) > 0 {
							client.Ack(*e.Request, map[string]interface{}{
								"response_type": "in_channel",
								"text":          fmt.Sprintf("Subscribed to '%s' with conclusions: %s\n", repo, strings.Join(conclusions, ", ")),
							})
						} else {
							client.Ack(*e.Request, map[string]interface{}{
								"response_type": "in_channel",
								"text":          fmt.Sprintf("Subscribed to '%s'\n", repo),
							})
						}
					}

				case "unsubscribe":
					err := a.DelChannel(ctx, repo, data.ChannelID)
					if err != nil {
						a.logger.Warn("failed to unsubscribe", zap.Error(err))
						client.Ack(*e.Request, map[string]interface{}{
							"text": fmt.Sprintf("Failed to unsubscribe '%s': %s\n", repo, err),
						})
					} else {
						client.Ack(*e.Request, map[string]interface{}{
							"response_type": "in_channel",
							"text":          fmt.Sprintf("Unsubscribed from '%s'\n", repo),
						})
					}

				default:
					client.Ack(*e.Request, map[string]interface{}{
						"text": fmt.Sprintf("Unknown subcommand '%s'\n", subcommand),
					})
				}

			default:
				if e.Type == socketmode.EventTypeHello {
					continue
				}
				a.logger.Warn("unexpected event", zap.String("type", string(e.Type)))
			}
		}
	}
}
