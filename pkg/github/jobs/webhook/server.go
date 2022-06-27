package webhook

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/oursky/github-actions-manager/pkg/utils/channels"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	addr   string
	secret []byte
}

func NewServer(addr string, secret string) *Server {
	server := &Server{
		addr:   addr,
		secret: []byte(secret),
	}

	return server
}

func (s *Server) Start(ctx context.Context, g *errgroup.Group, runs chan<- Key, jobs chan<- Key) error {
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

func (s *Server) handle(
	ctx context.Context,
	rw http.ResponseWriter,
	r *http.Request,
	runs chan<- Key,
	jobs chan<- Key,
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

	switch event := event.(type) {
	case *github.WorkflowRunEvent:
		key := Key{
			ID:        event.GetWorkflowRun().GetID(),
			RepoOwner: event.GetRepo().GetOwner().GetLogin(),
			RepoName:  event.GetRepo().GetName(),
		}
		channels.Send(ctx, runs, key)

	case *github.WorkflowJobEvent:
		key := Key{
			ID:        event.GetWorkflowJob().GetID(),
			RepoOwner: event.GetRepo().GetOwner().GetLogin(),
			RepoName:  event.GetRepo().GetName(),
		}
		channels.Send(ctx, jobs, key)
	}
}
