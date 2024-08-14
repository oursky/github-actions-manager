package slack

import (
	"context"
	"fmt"
	"strings"

	"github.com/oursky/github-actions-manager/pkg/utils/array"
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

type Command struct {
	trigger     string
	arguments   []Argument
	description string
	execute     func(env CLIContext, args []string) CLIResult
}

type CLIResult struct {
	printToChannel bool
	message        string
}

func NewCLIResult(printToChannel bool, message string) CLIResult {
	return CLIResult{printToChannel: printToChannel, message: message}
}

type CLIContext struct {
	app    *App
	ctx    context.Context
	data   slack.SlashCommand
	logger *zap.Logger
}

type CLI struct {
	commands *[]Command
}

// type TypeError struct {
// 	argument Argument
// }

// func (t *TypeError) Error() string {
// 	return fmt.Sprintf("incorrect type for argument %s (could not be coerced to %s)", t.argument.name, t.argument.argtype)
// }

// func (cli CLI) execute(name string, args []string) error {
// 	command := cli.commands[name]
// if len(args) < len(command.arguments) {
// 	return fmt.Errorf("not enough arguments for command %s (%d < %d)", name, len(args), len(command.arguments))
// }
// if len(args) > len(command.arguments) + len(command.optional_arguments) {
// return fmt.Errorf("Too many arguments for command %s (%d < %d)", name)
// }

// 	err := command.execute(args)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

func (c Command) String() string {
	output := fmt.Sprintf("`%s`: %s", c.trigger, c.description)
	output += fmt.Sprintf("\nUsage of `%s`: `%s", c.trigger, c.trigger)
	for _, arg := range c.arguments {
		argname := arg.name
		if arg.acceptsMany {
			argname += "..."
		}
		if arg.required {
			output += " " + argname
		} else {
			output += " [" + argname + "]"
		}
	}
	output += "`"
	for _, arg := range c.arguments {
		output += fmt.Sprintf("\n\t`%s`: %s", arg.name, arg.description)
	}

	return output
}

func (cli *CLI) Execute(env CLIContext, subcommand string, args []string) CLIResult {
	commands := *cli.commands
	for _, command := range commands {
		if subcommand != command.trigger {
			continue
		}
		return command.execute(
			env,
			args,
		)
	}
	return NewCLIResult(false, fmt.Sprintf("Unknown command: %s", subcommand))
}

func DefaultCLI() *CLI {
	cli := CLI{
		commands: &[]Command{},
	}
	// return &CLI{
	cli.commands = &[]Command{
		{
			trigger: "help",
			arguments: []Argument{
				NewArgument("subcommand", false, false, "The subcommand to get help about."),
			},
			description: "Get help about a command.",
			execute: func(env CLIContext, args []string) CLIResult {
				output := ""
				commands := (*cli.commands)
				if len(args) == 0 {
					output = "The known commands are:"
					for _, command := range commands {
						output += fmt.Sprintf(" `%s`", command.trigger)
					}
					return NewCLIResult(false, output)
				}
				subcommand := args[0]
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
			execute: func(env CLIContext, args []string) CLIResult {
				if len(args) < 1 {
					return NewCLIResult(false, "Please specify repo")
				}

				repo := args[0]
				if !repoRegex.MatchString(repo) {
					return NewCLIResult(false, fmt.Sprintf("Invalid repo '%s'\n", repo))
				}

				channels, err := env.app.GetChannels(env.ctx, repo)
				if err != nil {
					env.logger.Warn("failed to list channels", zap.Error(err))
					return NewCLIResult(false, fmt.Sprintf("Failed to get list of subscribed repos: '%s'", err))
				} else {
					return NewCLIResult(true, fmt.Sprintf("The channels '%s' are receiving updates from %s\n", channels, repo))
				}
			},
		},
		{
			trigger: "subscribe",
			arguments: []Argument{
				NewArgument("repo", true, false, "The repo to subscribe to."),
				NewArgument("conclusions", false, true, "Only be notified for actions with one of the given conclusion values"),
			},
			description: "Subscribe this channel to a given repo.",
			execute: func(env CLIContext, args []string) CLIResult {
				if len(args) < 1 {
					return NewCLIResult(false, fmt.Sprintf("Please specify repo"))
				}

				repo := args[0]
				if !repoRegex.MatchString(repo) {
					return NewCLIResult(false, fmt.Sprintf("Invalid repo '%s'\n", repo))
				}

				conclusions := array.Unique(args[1:])
				channelInfo := ChannelInfo{
					channelID:   env.data.ChannelID,
					conclusions: conclusions,
				}
				err := env.app.AddChannel(env.ctx, repo, channelInfo)
				if err != nil {
					env.logger.Warn("failed to subscribe", zap.Error(err))
					return NewCLIResult(false, fmt.Sprintf("Failed to subscribe '%s': %s\n", repo, err))
				}
				if len(conclusions) > 0 {
					return NewCLIResult(true, fmt.Sprintf("Subscribed to '%s' with conclusions: %s\n", repo, strings.Join(conclusions, ", ")))
				} else {
					return NewCLIResult(true, fmt.Sprintf("Subscribed to '%s'\n", repo))
				}
			},
		},
		{
			trigger: "unsubscribe",
			arguments: []Argument{
				NewArgument("repo", true, false, "The repo to unsubscribe from."),
			},
			description: "Unsubscribe this channel from a given repo.",
			execute: func(env CLIContext, args []string) CLIResult {
				if len(args) < 1 {
					return NewCLIResult(false, fmt.Sprintf("Please specify repo"))
				}

				repo := args[0]
				if !repoRegex.MatchString(repo) {
					return NewCLIResult(false, fmt.Sprintf("Invalid repo '%s'\n", repo))
				}

				err := env.app.DelChannel(env.ctx, repo, env.data.ChannelID)
				if err != nil {
					env.logger.Warn("failed to unsubscribe", zap.Error(err))
					return NewCLIResult(false, fmt.Sprintf("Failed to unsubscribe '%s': %s\n", repo, err))
				} else {
					return NewCLIResult(true, fmt.Sprintf("Unsubscribed from '%s'\n", repo))
				}
			},
		},
		{
			trigger:     "meow",
			description: "Meow.",
			execute: func(env CLIContext, args []string) CLIResult {
				return NewCLIResult(false, "meow")
			},
		},
	}
	return &cli
}
