//go:build tools

// Package tools pins the build-time tools of the repository.
package tools

import (
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
