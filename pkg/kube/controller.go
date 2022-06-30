package kube

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/oursky/github-actions-manager/pkg/controller"
	"github.com/oursky/github-actions-manager/pkg/github/runners"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listercorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	labelRunner            = "github-actions-manager.oursky.com/runner"
	annotationRunnerGroup  = "github-actions-manager.oursky.com/runner-group"
	annotationRunnerLabels = "github-actions-manager.oursky.com/runner-labels"
	annotationRunnerState  = "github-actions-manager.oursky.com/runner-state"
	finalizer              = "github-actions-manager.oursky.com/finalizer"
)

const (
	annotationDeletionCost = "controller.kubernetes.io/pod-deletion-cost"
	annotationSafeToEvict  = "cluster-autoscaler.kubernetes.io/safe-to-evict"
)

type ControllerProvider struct {
	logger   *zap.Logger
	ctx      context.Context
	cancel   func()
	state    *ControllerState
	kube     *kubernetes.Clientset
	informer informers.SharedInformerFactory
	pods     listercorev1.PodLister
}

func NewControllerProvider(logger *zap.Logger) (*ControllerProvider, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, nil)
		config, err = kubeConfig.ClientConfig()

		if err != nil {
			return nil, err
		}
	}

	kube, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	informer := informers.NewSharedInformerFactory(kube, 5*time.Minute)
	pods := informer.Core().V1().Pods().Lister()

	ctx, cancel := context.WithCancel(context.Background())
	logger = logger.Named("kube")
	return &ControllerProvider{
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		state:    NewControllerState(ctx, logger, kube, pods),
		kube:     kube,
		informer: informer,
		pods:     pods,
	}, nil
}

func (c *ControllerProvider) Start(ctx context.Context, g *errgroup.Group) error {
	c.informer.Start(c.ctx.Done())
	return nil
}

func (c *ControllerProvider) State() controller.State {
	return c.state
}

func (c *ControllerProvider) Shutdown() {
	c.cancel()
}

func (p *ControllerProvider) AuthenticateRequest(rw http.ResponseWriter, r *http.Request, next http.Handler) {
	pod, ok := p.getPod(rw, r)
	if !ok {
		return
	}

	if pod.Labels[labelRunner] != "true" {
		http.Error(rw, "unauthorized runner", http.StatusUnauthorized)
		return
	}

	r = r.WithContext(context.WithValue(r.Context(), authzContextKey, pod))
	next.ServeHTTP(rw, r)
}

func (p *ControllerProvider) RegisterAgent(
	r *http.Request,
	runnerName string,
	regToken string,
	targetURL string,
) (*controller.AgentResponse, error) {
	pod := r.Context().Value(authzContextKey).(*corev1.Pod)
	annotations := pod.Annotations
	group := annotations[annotationRunnerGroup]
	labels := strings.Split(annotations[annotationRunnerLabels], ",")

	agent, err := p.state.makeAgent(pod, runnerName)
	if err != nil {
		return nil, err
	}

	p.logger.Info("registered agent",
		zap.String("id", agent.ID),
		zap.String("runnerName", agent.RunnerName),
		zap.String("url", targetURL),
		zap.String("group", group),
		zap.Strings("labels", labels),
	)

	p.updateAgentPod(p.ctx, pod, agent.RunnerName, false)

	return &controller.AgentResponse{
		Agent:     *agent,
		TargetURL: targetURL,
		Token:     regToken,
		Group:     group,
		Labels:    labels,
	}, nil
}

func (p *ControllerProvider) CheckAgent(ctx context.Context, agent *controller.Agent, instance *runners.Instance) error {
	pod, err := p.state.GetPod(agent.ID)
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	if pod.DeletionTimestamp != nil && agent.State != controller.AgentStateTerminating {
		p.logger.Info("pod is terminating",
			zap.String("namespace", pod.Namespace),
			zap.String("name", pod.Name))

		return p.state.UpdateAgent(agent.ID, func(agent *controller.Agent) {
			agent.State = controller.AgentStateTerminating
			agent.LastTransitionTime = time.Now()
		})
	}

	if instance == nil {
		return nil
	}
	return p.updateAgentPod(ctx, pod, instance.Name, instance.IsBusy)
}

func (p *ControllerProvider) updateAgentPod(ctx context.Context, pod *corev1.Pod, runnerName string, isBusy bool) error {
	deletionCost := ""
	safeToEvict := ""
	if isBusy {
		deletionCost = "100"
		safeToEvict = "false"
	}

	var patches []jsonPatch
	if pod.Annotations[annotationDeletionCost] != deletionCost {
		patches = append(patches, annotationPatch(annotationDeletionCost, deletionCost))
	}
	if pod.Annotations[annotationSafeToEvict] != safeToEvict {
		patches = append(patches, annotationPatch(annotationSafeToEvict, safeToEvict))
	}
	patches = append(patches, addFinalizerPatch(pod.ObjectMeta, finalizer)...)

	return patchPod(p.ctx, p.kube, pod, patches)
}

func (p *ControllerProvider) TerminateAgent(ctx context.Context, agent controller.Agent) error {
	pod, err := p.state.GetPod(agent.ID)
	if err != nil {
		return err
	}

	p.logger.Info("deleting pod",
		zap.String("namespace", pod.Namespace),
		zap.String("name", pod.Name),
	)

	err = p.kube.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}
