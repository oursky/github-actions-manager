package kube

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type jsonPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

func escapeJSONPointer(ptr string) string {
	return strings.ReplaceAll(strings.ReplaceAll(ptr, "~", "~0"), "/", "~1")
}

func annotationPatch(key, value string) jsonPatch {
	if value == "" {
		return jsonPatch{
			Op:   "remove",
			Path: "/metadata/annotations/" + escapeJSONPointer(key),
		}
	}
	return jsonPatch{
		Op:    "replace",
		Path:  "/metadata/annotations/" + escapeJSONPointer(key),
		Value: value,
	}
}

func addFinalizerPatch(
	meta metav1.ObjectMeta,
	finalizer string,
) []jsonPatch {
	for _, f := range meta.Finalizers {
		if f == finalizer {
			return nil
		}
	}

	var patches []jsonPatch
	if len(meta.Finalizers) == 0 {
		patches = append(patches, jsonPatch{
			Op:    "test",
			Path:  "/metadata/finalizers",
			Value: nil,
		}, jsonPatch{
			Op:    "add",
			Path:  "/metadata/finalizers",
			Value: make([]any, 0),
		})
	}
	patches = append(patches, jsonPatch{
		Op:    "add",
		Path:  "/metadata/finalizers/-",
		Value: finalizer,
	})
	return patches
}

func removeFinalizerPatch(
	meta metav1.ObjectMeta,
	finalizer string,
) []jsonPatch {
	index := -1
	for i, f := range meta.Finalizers {
		if f == finalizer {
			index = i
			break
		}
	}
	if index == -1 {
		return nil
	}
	return []jsonPatch{{
		Op:    "test",
		Path:  "/metadata/finalizers/" + strconv.Itoa(index),
		Value: finalizer,
	}, {
		Op:   "remove",
		Path: "/metadata/finalizers/" + strconv.Itoa(index),
	}}
}

func patchPod(
	ctx context.Context,
	kube *kubernetes.Clientset,
	pod *corev1.Pod,
	patches []jsonPatch,
) error {
	if len(patches) == 0 {
		return nil
	}

	data, err := json.Marshal(patches)
	if err != nil {
		return err
	}

	_, err = kube.CoreV1().Pods(pod.Namespace).Patch(
		ctx,
		pod.Name,
		types.JSONPatchType,
		data,
		metav1.PatchOptions{})
	return err
}
