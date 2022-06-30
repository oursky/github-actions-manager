package kube

import (
	"net/http"
	"strings"

	"go.uber.org/zap"
	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var authzContextKey = struct{}{}

const (
	serviceAccountGroupPrefix = "system:serviceaccounts:"
	podNameKey                = "authentication.kubernetes.io/pod-name"
	podUIDKey                 = "authentication.kubernetes.io/pod-uid"
	audience                  = "github-actions-manager"
)

func (p *ControllerProvider) getPod(rw http.ResponseWriter, r *http.Request) (*corev1.Pod, bool) {
	authz := r.Header.Get("Authorization")
	bearer, token, ok := strings.Cut(authz, " ")
	if !ok || !strings.EqualFold(bearer, "Bearer") {
		http.Error(rw, "invalid token", http.StatusUnauthorized)
		return nil, false
	}

	review := &authnv1.TokenReview{Spec: authnv1.TokenReviewSpec{
		Token:     token,
		Audiences: []string{audience},
	}}
	review, err := p.kube.AuthenticationV1().TokenReviews().
		Create(r.Context(), review, metav1.CreateOptions{})
	if err != nil {
		p.logger.Warn("failed to validate token", zap.Error(err))
		http.Error(rw, "invalid token", http.StatusUnauthorized)
		return nil, false
	}

	status := review.Status
	if status.Error != "" {
		http.Error(rw, status.Error, http.StatusUnauthorized)
		return nil, false
	} else if !status.Authenticated {
		http.Error(rw, "unauthenticated", http.StatusUnauthorized)
		return nil, false
	}

	podName, ok := getExtraValue(status.User.Extra, podNameKey)
	if !ok {
		http.Error(rw, "invalid token", http.StatusUnauthorized)
		return nil, false
	}
	podUID, ok := getExtraValue(status.User.Extra, podUIDKey)
	if !ok {
		http.Error(rw, "invalid token", http.StatusUnauthorized)
		return nil, false
	}

	podNamespace := ""
	for _, grp := range status.User.Groups {
		if strings.HasPrefix(grp, serviceAccountGroupPrefix) {
			podNamespace = strings.TrimPrefix(grp, serviceAccountGroupPrefix)
			break
		}
	}
	if podNamespace == "" {
		http.Error(rw, "invalid token", http.StatusUnauthorized)
		return nil, false
	}

	pod, err := p.pods.Pods(podNamespace).Get(podName)
	if err != nil {
		http.Error(rw, "invalid token", http.StatusUnauthorized)
		return nil, false
	} else if string(pod.UID) != podUID {
		http.Error(rw, "invalid token", http.StatusUnauthorized)
		return nil, false
	}

	return pod, true
}

func getExtraValue(extra map[string]authnv1.ExtraValue, key string) (string, bool) {
	value, ok := extra[key]
	if !ok {
		return "", false
	}
	if len(value) != 1 {
		return "", false
	}
	return value[0], true
}
