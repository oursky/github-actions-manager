package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oursky/github-actions-manager/pkg/controller"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	listercorev1 "k8s.io/client-go/listers/core/v1"
)

type ControllerState struct {
	logger *zap.Logger
	ctx    context.Context
	kube   *kubernetes.Clientset
	pods   listercorev1.PodLister
}

func NewControllerState(ctx context.Context, logger *zap.Logger, kube *kubernetes.Clientset, pods listercorev1.PodLister) *ControllerState {
	return &ControllerState{
		logger: logger.Named("state"),
		ctx:    ctx,
		kube:   kube,
		pods:   pods,
	}
}

func (s *ControllerState) decodeState(pod *corev1.Pod) *controller.Agent {
	data := pod.Annotations[annotationRunnerState]
	if data == "" {
		return nil
	}

	agent := &controller.Agent{}
	if err := json.Unmarshal([]byte(data), agent); err != nil {
		s.logger.Warn("invalid runner state",
			zap.String("namespace", pod.Namespace),
			zap.String("name", pod.Name),
			zap.Error(err),
		)
		return nil
	}

	return agent
}

func (s *ControllerState) encodeState(pod *corev1.Pod, agent *controller.Agent) (string, error) {
	data, err := json.Marshal(agent)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *ControllerState) GetPod(agentID string) (*corev1.Pod, error) {
	namespace, name, _ := strings.Cut(agentID, "/")
	return s.pods.Pods(namespace).Get(name)
}

func (s *ControllerState) makeAgent(pod *corev1.Pod, hostName string) (*controller.Agent, error) {
	runnerName := hostName
	ctrl := metav1.GetControllerOf(pod)
	if ctrl != nil && ctrl.Kind == "StatefulSet" {
		runnerName += "-" + rand.String(5)
	}

	id := pod.Namespace + "/" + pod.Name
	agent := &controller.Agent{
		ID:                 id,
		RunnerName:         runnerName,
		State:              controller.AgentStateConfiguring,
		LastTransitionTime: time.Now(),
		RunnerID:           nil,
	}

	if pod.Annotations[annotationRunnerState] != "" {
		agent := s.decodeState(pod)
		return nil, fmt.Errorf("pod is already registered as agent: %+v", agent)
	}

	data, err := s.encodeState(pod, agent)
	if err != nil {
		return nil, err
	}

	patches := []jsonPatch{annotationPatch(annotationRunnerState, data)}
	patches = append(patches, addFinalizerPatch(pod.ObjectMeta, finalizer)...)

	if err := patchPod(s.ctx, s.kube, pod, patches); err != nil {
		return nil, err
	}

	return agent, nil
}

func (s *ControllerState) Agents() ([]controller.Agent, error) {
	pods, err := s.pods.List(labels.SelectorFromSet(labels.Set{
		labelRunner: "true",
	}))
	if err != nil {
		return nil, err
	}

	var agents []controller.Agent
	for _, pod := range pods {
		if agent := s.decodeState(pod); agent != nil {
			agents = append(agents, *agent)
		}
	}
	return agents, nil
}

func (s *ControllerState) GetAgent(id string) (*controller.Agent, error) {
	pod, err := s.GetPod(id)
	if apierrors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return s.decodeState(pod), nil
}

func (s *ControllerState) DeleteAgent(id string) error {
	pod, err := s.GetPod(id)
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	// keep annotations to wait until pod deleted
	patches := removeFinalizerPatch(pod.ObjectMeta, finalizer)

	err = patchPod(s.ctx, s.kube, pod, patches)
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}

func (s *ControllerState) UpdateAgent(id string, updater func(*controller.Agent)) error {
	pod, err := s.GetPod(id)
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	agent := s.decodeState(pod)
	if agent == nil {
		return nil
	}
	updater(agent)
	data, err := s.encodeState(pod, agent)
	if err != nil {
		return err
	}

	err = patchPod(s.ctx, s.kube, pod, []jsonPatch{annotationPatch(annotationRunnerState, data)})
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}
