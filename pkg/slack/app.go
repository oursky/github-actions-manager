package slack

import (
	"context"
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
	"k8s.io/utils/strings/slices"
)

var repoRegex = regexp.MustCompile("[a-zA-Z0-9-]+(/[a-zA-Z0-9-]+)?")

type App struct {
	logger   *zap.Logger
	disabled bool
	api      *slack.Client
	store    kv.Store
}

type ChannelInfo struct {
	channelID   string
	conclusions []string
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
		store: store,
	}
}

func (a *App) Disabled() bool {
	return a.disabled
}

// Format of channel info string: "<channel_Id>:<conclusion_1>,<conclusion_2>"
func toChannelInfoString(channelInfo ChannelInfo) string {
	if len(channelInfo.conclusions) == 0 {
		return channelInfo.channelID
	}
	conclusionsString := strings.Join(channelInfo.conclusions, ",")
	return channelInfo.channelID + ":" + conclusionsString
}

func (a *App) GetChannels(ctx context.Context, repo string) ([]ChannelInfo, error) {
	data, err := a.store.Get(ctx, kvNamespace, repo)
	if err != nil {
		return nil, err
	} else if data == "" {
		return nil, nil
	}
	channelInfoStrings := strings.Split(data, ";")
	channelInfos := []ChannelInfo{}

	for _, channelString := range channelInfoStrings {
		channelID, conclusionsString, _ := strings.Cut(channelString, ":")
		channelInfos = append(channelInfos, ChannelInfo{
			channelID:   channelID,
			conclusions: strings.Split(conclusionsString, ","),
		})
	}

	return channelInfos, nil
}

func (a *App) AddChannel(ctx context.Context, repo string, channelInfo ChannelInfo) error {
	channelInfos, err := a.GetChannels(ctx, repo)
	if err != nil {
		return err
	}

	supportedConclusions := []string{"action_required", "cancelled", "failure", "neutral", "success", "skipped", "stale", "timed_out"}
	unsupportedConclusions := []string{}
	for _, c := range channelInfo.conclusions {
		if !slices.Contains(supportedConclusions, c) {
			unsupportedConclusions = append(unsupportedConclusions, c)
		}
	}

	if len(unsupportedConclusions) > 0 {
		return fmt.Errorf("unsupported conclusions: %s", strings.Join(unsupportedConclusions, ", "))
	}

	newChannelInfoStrings := []string{}
	for _, c := range channelInfos {
		if c.channelID == channelInfo.channelID {
			return fmt.Errorf("already subscribed to repo")
		}
		newChannelInfoStrings = append(newChannelInfoStrings, toChannelInfoString(c))
	}
	newChannelInfoStrings = append(newChannelInfoStrings, toChannelInfoString(channelInfo))
	data := strings.Join(newChannelInfoStrings, ";")

	return a.store.Set(ctx, kvNamespace, repo, data)
}

func (a *App) DelChannel(ctx context.Context, repo string, channelID string) error {
	channelInfos, err := a.GetChannels(ctx, repo)
	if err != nil {
		return err
	}

	newChannelInfoStrings := []string{}
	found := false
	for _, c := range channelInfos {
		if c.channelID == channelID {
			found = true
			continue
		}
		newChannelInfoStrings = append(newChannelInfoStrings, toChannelInfoString(c))
	}
	if !found {
		return fmt.Errorf("not subscribed to repo")
	}
	data := strings.Join(newChannelInfoStrings, ";")

	return a.store.Set(ctx, kvNamespace, repo, data)
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

				if data.Command != "/gha" {
					client.Ack(*e.Request, map[string]interface{}{
						"text": fmt.Sprintf("Unknown command '%s'\n", data.Command)})
					return
				}

				args := strings.Split(data.Text, " ")
				if len(args) < 2 {
					client.Ack(*e.Request, map[string]interface{}{
						"text": fmt.Sprintf("Please specify subcommand and repo")})
					return
				}

				repo := args[1]
				subcommand := args[0]
				conclusions := array.Unique(args[2:])
				if !repoRegex.MatchString(repo) {
					client.Ack(*e.Request, map[string]interface{}{
						"text": fmt.Sprintf("Invalid repo '%s'\n", repo),
					})
					return
				}

				switch subcommand {
				case "subscribe":
					channelInfo := ChannelInfo{
						channelID:   data.ChannelID,
						conclusions: conclusions,
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
