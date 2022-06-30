package agent

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/oursky/github-actions-manager/pkg/controller"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type executer struct {
	logger        *zap.Logger
	config        *Config
	controllerAPI *controllerAPI
	provider      Provider
	agentCh       chan<- controller.Agent
}

func newExecuter(logger *zap.Logger, config *Config, controllerAPI *controllerAPI, provider Provider, agentCh chan<- controller.Agent) *executer {
	return &executer{
		logger:        logger.Named("executer"),
		config:        config,
		controllerAPI: controllerAPI,
		provider:      provider,
		agentCh:       agentCh,
	}
}

func (m *executer) Start(ctx context.Context, g *errgroup.Group) error {
	g.Go(func() error {
		defer m.provider.Shutdown(ctx)
		m.execute(ctx)
		return nil
	})
	return nil
}

func (m *executer) execute(ctx context.Context) {
	runnerName, err := os.Hostname()
	if err != nil {
		m.logger.Error("failed to get hostname", zap.Error(err))
		return
	}
	runnerName = strings.TrimSuffix(runnerName, ".local")

	m.logger.Info("registering agent", zap.String("runnerName", runnerName))
	var resp *controller.AgentResponse
	for {
		resp, err = m.controllerAPI.RegisterAgent(ctx, runnerName)
		if err == nil {
			break
		}

		m.logger.Error("failed to register agent", zap.Error(err))

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}

	m.provider.OnAgentRegistered(resp.Agent)

	m.agentCh <- resp.Agent

	m.logger.Info("configuring runner",
		zap.String("target", resp.TargetURL),
		zap.String("group", resp.Group),
		zap.Strings("labels", resp.Labels),
	)
	err = m.configure(ctx, resp)
	if err != nil {
		m.logger.Error("failed to configure runner", zap.Error(err))
		return
	}

	m.logger.Info("starting runner")
	err = m.start(ctx)
	if err != nil {
		m.logger.Error("failed to start runner", zap.Error(err))
		return
	}

	m.logger.Info("runner exited")
	close(m.agentCh)
}

func (m *executer) setupRunnerCmd(cmd *exec.Cmd) {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = m.config.RunnerDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func (m *executer) configure(ctx context.Context, resp *controller.AgentResponse) error {
	retry := 10
	for {
		args := []string{
			"--unattended",
			"--replace",
			"--ephemeral",
			"--name", resp.Agent.RunnerName,
			"--url", resp.TargetURL,
			"--token", resp.Token,
			"--work", m.config.WorkDir,
		}
		if resp.Group != "" {
			args = append(args, "--runnergroup", resp.Group)
		}
		if len(resp.Labels) > 0 {
			args = append(args, "--labels", strings.Join(resp.Labels, ","))
		}
		cmd := exec.CommandContext(ctx, m.config.GetConfigureScript(), args...)
		m.setupRunnerCmd(cmd)

		m.logger.Debug("starting config.sh", zap.String("cmd", cmd.String()))
		err := cmd.Run()
		if err != nil && retry > 0 {
			m.logger.Warn("failed to configure runner", zap.Error(err))
			retry--
			continue
		}
		return err
	}
}

func (m *executer) start(ctx context.Context) error {
	cmd := exec.Command(m.config.GetRunScript())
	m.setupRunnerCmd(cmd)

	m.logger.Debug("starting run.sh", zap.String("cmd", cmd.String()))
	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		m.logger.Info("interrupting runner")
		syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
	}()

	return cmd.Wait()
}
