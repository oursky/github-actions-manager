package github

import "github.com/oursky/github-actions-manager/pkg/kv"

var KVNamespace = kv.RegisterNamespace("github-state")
