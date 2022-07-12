package jobs

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/oursky/github-actions-manager/pkg/utils/channels"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type webhookObject[T any] struct {
	Key
	Object T
}

type webhookServer struct {
	logger *zap.Logger
	addr   string
	secret []byte
}

func newWebhookServer(logger *zap.Logger, addr string, secret string) *webhookServer {
	server := &webhookServer{
		logger: logger.Named("webhook-server"),
		addr:   addr,
		secret: []byte(secret),
	}

	return server
}

func (s *webhookServer) Start(
	ctx context.Context,
	g *errgroup.Group,
	runs chan<- webhookObject[*github.WorkflowRun],
	jobs chan<- webhookObject[*github.WorkflowJob],
) error {
	g.Go(func() error {
		server := &http.Server{
			Addr:         s.addr,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		server.Handler = http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			s.handle(ctx, rw, r, runs, jobs)
		})

		go func() {
			<-ctx.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			server.Shutdown(ctx)
		}()

		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	return nil
}

func (s *webhookServer) handle(
	ctx context.Context,
	rw http.ResponseWriter,
	r *http.Request,
	runs chan<- webhookObject[*github.WorkflowRun],
	jobs chan<- webhookObject[*github.WorkflowJob],
) {
	payload, err := github.ValidatePayload(r, s.secret)
	if err != nil {
		rw.WriteHeader(400)
		rw.Write([]byte(err.Error()))
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		rw.WriteHeader(400)
		rw.Write([]byte(err.Error()))
	}

	s.logger.Info("received webhook",
		zap.String("type", github.WebHookType(r)),
		zap.String("id", github.DeliveryID(r)),
	)

	switch event := event.(type) {
	case *github.WorkflowRunEvent:
		key := Key{
			ID:        event.GetWorkflowRun().GetID(),
			RepoOwner: event.GetRepo().GetOwner().GetLogin(),
			RepoName:  event.GetRepo().GetName(),
		}
		channels.Send(ctx, runs, webhookObject[*github.WorkflowRun]{
			Key:    key,
			Object: event.GetWorkflowRun(),
		})

	case *github.WorkflowJobEvent:
		key := Key{
			ID:        event.GetWorkflowJob().GetID(),
			RepoOwner: event.GetRepo().GetOwner().GetLogin(),
			RepoName:  event.GetRepo().GetName(),
		}
		channels.Send(ctx, jobs, webhookObject[*github.WorkflowJob]{
			Key:    key,
			Object: event.GetWorkflowJob(),
		})
	}
}

func NewWebhookObject[T any](key Key, object T) webhookObject[T] {
	return webhookObject[T]{
		key,
		object,
	}
}
