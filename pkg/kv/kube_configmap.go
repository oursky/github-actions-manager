package kv

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type kubeConfigMap struct {
	cm     *v1.ConfigMap
	values map[string]string
}

type KubeConfigMapStore struct {
	logger        *zap.Logger
	cli           *kubernetes.Clientset
	lock          *sync.RWMutex
	values        map[string]kubeConfigMap
	kubeNamespace string
}

func NewKubeConfigMapStore(logger *zap.Logger, kubeNamespace string) (*KubeConfigMapStore, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, nil)
		config, err = kubeConfig.ClientConfig()

		if err != nil {
			return nil, err
		}
	}

	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubeConfigMapStore{
		logger:        logger.Named("kube-configmap"),
		cli:           cli,
		lock:          new(sync.RWMutex),
		values:        make(map[string]kubeConfigMap),
		kubeNamespace: kubeNamespace,
	}, nil
}

func (s *KubeConfigMapStore) loadConfig(cm *v1.ConfigMap) {
	s.lock.Lock()
	defer s.lock.Unlock()

	values := map[string]string{}
	for k, v := range cm.Data {
		values[k] = v
	}
	s.values[cm.Name] = kubeConfigMap{cm: cm, values: values}

	s.logger.Info("config loaded", zap.String("namespace", cm.Name), zap.Int("len", len(values)))
}

func (s *KubeConfigMapStore) Start(ctx context.Context, g *errgroup.Group) error {
	cms := s.cli.CoreV1().ConfigMaps(s.kubeNamespace)
	for ns := range namespaces {
		cm, err := cms.Create(ctx, &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
			Data:       map[string]string{},
		}, metav1.CreateOptions{})

		if errors.IsAlreadyExists(err) {
			cm, err = cms.Get(ctx, ns, metav1.GetOptions{})
		}
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				return fmt.Errorf("cannot setup config %s: %w", ns, err)
			}
		}

		s.loadConfig(cm)
	}

	watchlist := cache.NewListWatchFromClient(
		s.cli.CoreV1().RESTClient(),
		string(v1.ResourceConfigMaps),
		s.kubeNamespace,
		fields.Everything(),
	)
	_, controller := cache.NewInformer(
		watchlist,
		&v1.ConfigMap{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				cm := obj.(*v1.ConfigMap)
				if _, ok := namespaces[cm.Name]; ok {
					s.loadConfig(cm)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				cm := newObj.(*v1.ConfigMap)
				if _, ok := namespaces[cm.Name]; ok {
					s.loadConfig(cm)
				}
			},
			DeleteFunc: nil,
		},
	)

	g.Go(func() error {
		controller.Run(ctx.Done())
		return nil
	})

	return nil
}

func (s *KubeConfigMapStore) Get(ctx context.Context, ns Namespace, key string) (string, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.values[string(ns)].values[escapeKey(key)], nil
}

func (s *KubeConfigMapStore) Set(ctx context.Context, ns Namespace, key string, value string) error {
	cm, err := s.cli.CoreV1().ConfigMaps(s.kubeNamespace).Patch(
		ctx,
		string(ns),
		types.StrategicMergePatchType,
		makePatch(key, value),
		metav1.PatchOptions{})
	s.loadConfig(cm)
	return err
}

var escaper = regexp.MustCompile(`[^a-zA-Z0-9-_]+`)

func escapeKey(key string) string {
	return escaper.ReplaceAllStringFunc(key, func(c string) string {
		return "." + base64.RawURLEncoding.EncodeToString([]byte(c)) + "."
	})
}

func makePatch(key string, value string) []byte {
	type patch struct {
		Data map[string]string `json:"data"`
	}
	json, err := json.Marshal(patch{Data: map[string]string{
		escapeKey(key): value,
	}})
	if err != nil {
		panic(err)
	}
	return json
}
