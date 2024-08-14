package slack

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/kv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"k8s.io/utils/strings/slices"
)

var repoRegex = regexp.MustCompile("[a-zA-Z0-9-]+(/[a-zA-Z0-9-]+)?")

type App struct {
	logger      *zap.Logger
	disabled    bool
	api         *slack.Client
	store       kv.Store
	commandName string
	cli         *CLI
}

type ChannelInfo struct {
	channelID   string
	conclusions []string
}

func (c ChannelInfo) String() string {
	if len(c.conclusions) == 0 {
		return c.channelID
	}
	return fmt.Sprintf("%s (%s)", c.channelID, strings.Join(c.conclusions, ", "))
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
		cli:         DefaultCLI(),
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
	var channelInfos []ChannelInfo

	for _, channelString := range channelInfoStrings {
		channelID, conclusionsString, _ := strings.Cut(channelString, ":")
		var conclusions []string
		for _, conclusion := range strings.Split(conclusionsString, ",") {
			if len(conclusion) > 0 {
				conclusions = append(conclusions, conclusion)
			}
		}
		channelInfos = append(channelInfos, ChannelInfo{
			channelID:   channelID,
			conclusions: conclusions,
		})
	}

	return channelInfos, nil
}

func (a *App) AddChannel(ctx context.Context, repo string, channelInfo ChannelInfo) error {
	channelInfos, err := a.GetChannels(ctx, repo)
	if err != nil {
		return err
	}

	// Ref: https://docs.github.com/en/rest/checks/runs?apiVersion=2022-11-28#create-a-check-run--parameters
	supportedConclusions := []string{"action_required", "cancelled", "failure", "neutral", "success", "skipped", "stale", "timed_out"}
	var unsupportedConclusions []string
	for _, c := range channelInfo.conclusions {
		if !slices.Contains(supportedConclusions, c) {
			unsupportedConclusions = append(unsupportedConclusions, c)
		}
	}

	if len(unsupportedConclusions) > 0 {
		return fmt.Errorf("unsupported conclusions: %s", strings.Join(unsupportedConclusions, ", "))
	}

	var newChannelInfoStrings []string
	for _, c := range channelInfos {
		if c.channelID == channelInfo.channelID {
			// Skip the old subscription and will replace with the new conclusion filter options
			continue
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

	var newChannelInfoStrings []string
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

				if data.Command != "/"+a.commandName {
					client.Ack(*e.Request, map[string]interface{}{
						"text": fmt.Sprintf("Unknown command '%s'\n", data.Command)})
					continue
				}

				args := strings.Split(data.Text, " ")
				if len(args) < 1 {
					client.Ack(*e.Request, map[string]interface{}{
						"text": fmt.Sprintf("Please specify subcommand")})
					continue
				}

				cliContext := CLIContext{
					app:    a,
					ctx:    ctx,
					data:   data,
					logger: a.logger,
				}
				subcommand := args[0]

				result := a.cli.Execute(cliContext, subcommand, args[1:])
				if result.printToChannel {
					client.Ack(*e.Request, map[string]interface{}{
						"response_type": "in_channel",
						"text":          result.message,
					})
				}
				client.Ack(*e.Request, map[string]interface{}{
					"text": result.message,
				})

			default:
				if e.Type == socketmode.EventTypeHello {
					continue
				}
				a.logger.Warn("unexpected event", zap.String("type", string(e.Type)))
			}
		}
	}
}
