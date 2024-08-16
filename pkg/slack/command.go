package slack

import (
	"context"
	"fmt"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/utils/array"
	"github.com/samber/lo"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

type Argument struct {
	name        string
	required    bool
	acceptsMany bool
	description string
}

func NewArgument(name string, required bool, acceptsMany bool, description string) Argument {
	return Argument{name: name, required: required, acceptsMany: acceptsMany, description: description}
}

type CommandContext struct {
	ctx       context.Context
	a         *App
	channelID string
	args      []string
}

type Command struct {
	trigger     string
	arguments   []Argument
	description string
	execute     func(CommandContext) CommandResult
}

type CommandResult struct {
	printToChannel bool
	message        string
}

func NewCLIResult(printToChannel bool, message string) CommandResult {
	return CommandResult{printToChannel: printToChannel, message: message}
}

func (arg Argument) String() string {
	argname := arg.name
	if arg.acceptsMany {
		argname += "..."
	}
	if arg.required {
		return argname
	} else {
		return " [" + argname + "]"
	}
}

func (c Command) String() string {
	output := fmt.Sprintf("`%s`: %s", c.trigger, c.description)
	output += fmt.Sprintf("\nUsage of `%s`:", c.trigger)
	output += fmt.Sprintf("`%s %s`", c.trigger, lo.Reduce(c.arguments,
		func(o string, x Argument, _ int) string {
			return fmt.Sprintf("%s %s", o, x.String())
		}, ""))
	for _, arg := range c.arguments {
		output += fmt.Sprintf("\n\t`%s`: %s", arg.name, arg.description)
	}

	return output
}

func (a *App) Execute(ctx context.Context, channelID string, subcommand string, args []string) CommandResult {
	for _, command := range *a.commands {
		if subcommand != command.trigger {
			continue
		}
		return command.execute(CommandContext{ctx: ctx, a: a, channelID: channelID, args: args})
	}
	return NewCLIResult(false, fmt.Sprintf("Unknown command: %s", subcommand))
}

func (a *App) Handle(ctx context.Context, data slack.SlashCommand) map[string]interface{} {
	if data.Command != "/"+a.commandName {
		return map[string]interface{}{"text": fmt.Sprintf("Unknown command '%s'\n", data.Command)}
	}

	args := strings.Split(data.Text, " ")
	if len(args) < 1 {
		return map[string]interface{}{"text": fmt.Sprintf("Please specify subcommand")}
	}

	result := a.Execute(ctx, data.ChannelID, args[0], args[1:])
	response := map[string]interface{}{"text": result.message}
	if result.printToChannel {
		response["response_type"] = "in_channel"
	}
	return response
}

func GetCommands() *[]Command {
	commands := &[]Command{}
	commands = &[]Command{
		{
			trigger: "help",
			arguments: []Argument{
				NewArgument("subcommand", false, false, "The subcommand to get help about."),
			},
			description: "Get help about a command.",
			execute: func(env CommandContext) CommandResult {
				output := ""
				commands := (*commands)
				if len(env.args) == 0 {
					output = "The known commands are:"
					for _, command := range commands {
						output += fmt.Sprintf(" `%s`", command.trigger)
					}
					return NewCLIResult(false, output)
				}
				subcommand := env.args[0]
				for _, command := range commands {
					if command.trigger != subcommand {
						continue
					}
					return NewCLIResult(false, command.String())
				}
				return NewCLIResult(false, fmt.Sprintf("No such command: %s", subcommand))
			},
		},
		{
			trigger: "list",
			arguments: []Argument{
				NewArgument("repo", true, false, "The repo to get the subscription data for."),
			},
			description: "List the channels subscribed to a given repo.",
			execute: func(env CommandContext) CommandResult {
				if len(env.args) < 1 {
					return NewCLIResult(false, "Please specify repo")
				}

				repo := env.args[0]
				if !repoRegex.MatchString(repo) {
					return NewCLIResult(false, fmt.Sprintf("Invalid repo *%s*\n", repo))
				}

				channels, err := env.a.GetChannels(env.ctx, repo)
				if err != nil {
					env.a.logger.Warn("failed to list channels", zap.Error(err))
					return NewCLIResult(false, fmt.Sprintf("Failed to get list of subscribed channels: '%s'", err))
				} else {
					if len(channels) == 0 {
						return NewCLIResult(true, fmt.Sprintf("*%s* is sending updates to no channels", repo))
					}
					channelStrings := lo.Map(channels, func(x ChannelInfo, _ int) string { return x.String() })
					return NewCLIResult(true, fmt.Sprintf("*%s* is sending updates to: %s\n", repo, strings.Join(channelStrings, "; ")))
				}
			},
		},
		{
			trigger: "subscribe",
			arguments: []Argument{
				NewArgument("repo", true, false, "The repo to subscribe to."),
				NewArgument("filters", false, true, "In the format of filter_key:value1,value2,...:conclusion1,conclusion2,..., one of the supported filter keys (workflows, branches)"),
			},
			description: "Subscribe this channel to a given repo.",
			execute: func(env CommandContext) CommandResult {
				if len(env.args) < 1 {
					return NewCLIResult(false, fmt.Sprintf("Please specify repo"))
				}

				repo := env.args[0]
				if !repoRegex.MatchString(repo) {
					return NewCLIResult(false, fmt.Sprintf("Invalid repo *%s*\n", repo))
				}

				filterLayers := array.Unique(env.args[1:])
				filter, err := NewFilter(filterLayers)
				if err != nil {
					env.a.logger.Warn("failed to subscribe", zap.Error(err))
					return NewCLIResult(false, fmt.Sprintf("Failed to subscribe to *%s*: '%s'\n", repo, err))
				}

				channelInfo := ChannelInfo{
					ChannelID: env.channelID,
					Filter:    filter,
				}
				err = env.a.AddChannel(env.ctx, repo, channelInfo)
				if err != nil {
					env.a.logger.Warn("failed to subscribe", zap.Error(err))
					return NewCLIResult(false, fmt.Sprintf("Failed to subscribe to *%s*: '%s'\n", repo, err))
				}
				if len(filterLayers) > 0 {
					return NewCLIResult(true, fmt.Sprintf("Subscribed to *%s* with filter layers %s", repo, filter.Whitelists))
				} else {
					return NewCLIResult(true, fmt.Sprintf("Subscribed to *%s*\n", repo))
				}
			},
		},
		{
			trigger: "unsubscribe",
			arguments: []Argument{
				NewArgument("repo", true, false, "The repo to unsubscribe from."),
			},
			description: "Unsubscribe this channel from a given repo.",
			execute: func(env CommandContext) CommandResult {
				if len(env.args) < 1 {
					return NewCLIResult(false, fmt.Sprintf("Please specify repo"))
				}

				repo := env.args[0]
				if !repoRegex.MatchString(repo) {
					return NewCLIResult(false, fmt.Sprintf("Invalid repo *%s*\n", repo))
				}

				err := env.a.DelChannel(env.ctx, repo, env.channelID)
				if err != nil {
					env.a.logger.Warn("failed to unsubscribe", zap.Error(err))
					return NewCLIResult(false, fmt.Sprintf("Failed to unsubscribe from *%s*: '%s'\n", repo, err))
				} else {
					return NewCLIResult(true, fmt.Sprintf("Unsubscribed from *%s*\n", repo))
				}
			},
		},
		{
			trigger:     "meow",
			description: "Meow.",
			execute: func(env CommandContext) CommandResult {
				return NewCLIResult(false, "meow")
			},
		},
	}
	return commands
}
