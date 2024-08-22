package slack

import (
	"context"
	"fmt"
	"testing"

	"github.com/oursky/github-actions-manager/pkg/kv"
	"github.com/slack-go/slack"
	. "github.com/smartystreets/goconvey/convey"

	"go.uber.org/zap"
)

func TestSpec(t *testing.T) {
	testCommand := "test-gha"

	NewTestSlackChannel := func(channelID string) (string, func(string) slack.SlashCommand) {
		return channelID, func(command string) slack.SlashCommand {
			return slack.SlashCommand{
				ChannelID: channelID,
				Command:   "/" + testCommand,
				Text:      command,
			}
		}
	}
	channelID1, commandFromChannel1 := NewTestSlackChannel("TestChannelID1")
	channelID2, commandFromChannel2 := NewTestSlackChannel("TestChannelID2")

	Convey("When receiving commands, the bot", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		testApp := &App{
			logger:      zap.NewNop(),
			store:       kv.NewInMemoryStore(),
			commandName: testCommand,
			commands:    GetCommands(),
		}

		Convey("responds", func() {
			response := testApp.Handle(ctx, commandFromChannel1("meow"))
			So(response["text"], ShouldEqual, "meow")
		})
		Convey("rejects unrecognised commmands", func() {
			response := testApp.Handle(ctx, commandFromChannel1("fhqwhgads"))
			So(response["response_type"], ShouldBeNil)
			So(response["text"], ShouldContainSubstring, "fhqwhgads")
		})
		Convey("When asked to subscribe", func() {
			Convey("rejects an insufficient number of arguments", func() {
				response := testApp.Handle(ctx, commandFromChannel1("subscribe"))
				So(response["printToChannel"], ShouldBeNil)
				So(response["text"], ShouldContainSubstring, "repo")
			})
			Convey("rejects an unrecognised conclusion", func() {
				response := testApp.Handle(ctx, commandFromChannel1("subscribe owner/repo foo"))
				So(response["printToChannel"], ShouldBeNil)
				So(response["text"], ShouldContainSubstring, "conclusion")
			})
			Convey("rejects a malformed filter", func() {
				response := testApp.Handle(ctx, commandFromChannel1("subscribe owner/repo foo:bar"))
				So(response["printToChannel"], ShouldBeNil)
				So(response["text"], ShouldContainSubstring, "filter")

				response = testApp.Handle(ctx, commandFromChannel1("subscribe owner/repo foo:bar:success"))
				So(response["printToChannel"], ShouldBeNil)
				So(response["text"], ShouldContainSubstring, "filter")
			})
			Convey("rejects a duplicated filter", func() {
				response := testApp.Handle(ctx, commandFromChannel1("subscribe owner/repo success failure"))
				So(response["printToChannel"], ShouldBeNil)
				So(response["text"], ShouldContainSubstring, "duplicated")

				response = testApp.Handle(ctx, commandFromChannel1("subscribe owner/repo workflows:bar:success workflows:bar:failure"))
				So(response["printToChannel"], ShouldBeNil)
				So(response["text"], ShouldContainSubstring, "duplicated")
			})
			Convey("accepts a well-formed filter", func() {
				response := testApp.Handle(ctx, commandFromChannel1("subscribe owner/repo workflows:workflow1:success failure"))
				So(response["response_type"], ShouldEqual, "in_channel")
				So(response["text"], ShouldContainSubstring, "Subscribed")
				So(response["text"], ShouldContainSubstring, "workflow1")
			})
			Convey("overrides an existing subscription", func() {
				response := testApp.Handle(ctx, commandFromChannel1("subscribe owner/repo workflows:workflow2:success failure"))
				So(response["response_type"], ShouldEqual, "in_channel")
				So(response["text"], ShouldContainSubstring, "Subscribed")
				So(response["text"], ShouldContainSubstring, "workflow2")

				// Anachronistic usage of list command, this needs to be fixed
				response = testApp.Handle(ctx, commandFromChannel1("list owner/repo"))
				So(response["response_type"], ShouldEqual, "in_channel")
				So(response["text"], ShouldNotContainSubstring, "workflow1")
			})
		})
		Convey("When asked to list", func() {
			Convey("rejects an insufficient number of arguments", func() {
				response := testApp.Handle(ctx, commandFromChannel1("list"))
				So(response["response_type"], ShouldBeNil)
				So(response["text"], ShouldContainSubstring, "repo")
			})
			Convey("correct lists subscribed channels", func() {
				testApp.Handle(ctx, commandFromChannel1("subscribe owner/repo"))
				testApp.Handle(ctx, commandFromChannel2("subscribe owner/repo workflows:workflow1 failure"))
				response := testApp.Handle(ctx, commandFromChannel1("list owner/repo"))
				So(response["response_type"], ShouldEqual, "in_channel")
				So(response["text"], ShouldContainSubstring, channelID1)
				So(response["text"], ShouldContainSubstring, channelID2)
				So(response["text"], ShouldContainSubstring, "workflow1")
				So(response["text"], ShouldContainSubstring, "failure")
			})
			Convey("responds correctly if no channels are subscribed", func() {
				response := testApp.Handle(ctx, commandFromChannel1("list owner/repo"))
				So(response["response_type"], ShouldEqual, "in_channel")
				So(response["text"], ShouldContainSubstring, " no")
			})
		})
		Convey("When asked to unsubscribe", func() {
			Convey("rejects an insufficient number of arguments", func() {
				response := testApp.Handle(ctx, commandFromChannel1("unsubscribe"))
				So(response["response_type"], ShouldBeNil)
				So(response["text"], ShouldContainSubstring, "repo")
			})
			Convey("notifies if the channel is not subscribed to the repo", func() {
				response := testApp.Handle(ctx, commandFromChannel1("unsubscribe owner/repo"))
				So(response["response_type"], ShouldBeNil)
				So(response["text"], ShouldContainSubstring, "subscribed")
			})
			Convey("correctly unsubscribes from a channel", func() {
				testApp.Handle(ctx, commandFromChannel1("subscribe owner/repo"))
				testApp.Handle(ctx, commandFromChannel1("unsubscribe owner/repo"))
				response := testApp.Handle(ctx, commandFromChannel1("list owner/repo"))
				So(response["response_type"], ShouldEqual, "in_channel")
				So(response["text"], ShouldContainSubstring, " no")
			})
			Convey("correctly unsubscribes from only the requested channel", func() {
				testApp.Handle(ctx, commandFromChannel1("subscribe owner/repo"))
				testApp.Handle(ctx, commandFromChannel2("subscribe owner/repo workflows:workflow1 failure"))
				testApp.Handle(ctx, commandFromChannel1("unsubscribe owner/repo"))
				response := testApp.Handle(ctx, commandFromChannel1("list owner/repo"))
				So(response["response_type"], ShouldEqual, "in_channel")
				So(response["text"], ShouldContainSubstring, channelID2)
				So(response["text"], ShouldContainSubstring, "workflow1")
				So(response["text"], ShouldContainSubstring, "failure")
			})
			Convey("is able to resubscribe", func() {
				testApp.Handle(ctx, commandFromChannel1("subscribe owner/repo"))
				testApp.Handle(ctx, commandFromChannel1("unsubscribe owner/repo"))
				testApp.Handle(ctx, commandFromChannel1("subscribe owner/repo"))
				response := testApp.Handle(ctx, commandFromChannel1("list owner/repo"))
				So(response["response_type"], ShouldEqual, "in_channel")
				So(response["text"], ShouldContainSubstring, channelID1)
			})
		})
	})
	Convey("When receiving webhooks, the bot", t, func() {
		Convey("has no tests at the moment", func() {
		})
	})
	Convey("When reading the previous (deprecated) format, the bot", t, func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		testStore := kv.NewInMemoryStore()
		nsSlackSubscriptions := "slack-subscriptions"
		testApp := &App{
			logger:      zap.NewNop(),
			store:       testStore,
			commandName: testCommand,
			commands:    GetCommands(),
		}

		Convey("correctly converts from filterless", func() {
			testStore.Set(ctx, kv.Namespace(nsSlackSubscriptions), "owner/repo", channelID1)

			response := testApp.Handle(ctx, commandFromChannel1("list owner/repo"))
			So(response["response_type"], ShouldEqual, "in_channel")
			So(response["text"], ShouldContainSubstring, channelID1)
		})
		Convey("correctly converts from filtered", func() {
			testStore.Set(ctx, kv.Namespace(nsSlackSubscriptions), "owner/repo", fmt.Sprintf("%s:success,failure", channelID1))

			response := testApp.Handle(ctx, commandFromChannel1("list owner/repo"))
			So(response["response_type"], ShouldEqual, "in_channel")
			So(response["text"], ShouldContainSubstring, "success")
			So(response["text"], ShouldContainSubstring, "failure")
			So(response["text"], ShouldContainSubstring, channelID1)
		})
		Convey("correctly converts from fusion", func() {
			testStore.Set(ctx, kv.Namespace(nsSlackSubscriptions), "owner/repo", fmt.Sprintf("%s;%s:success,failure", channelID1, channelID2))

			response := testApp.Handle(ctx, commandFromChannel1("list owner/repo"))
			So(response["response_type"], ShouldEqual, "in_channel")
			So(response["text"], ShouldContainSubstring, "success")
			So(response["text"], ShouldContainSubstring, "failure")
			So(response["text"], ShouldContainSubstring, channelID1)
			So(response["text"], ShouldContainSubstring, channelID2)
		})
	})
}
