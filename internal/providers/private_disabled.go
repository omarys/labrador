//go:build no_private

package providers

import (
	"net/http"

	"github.com/omarys/labrador/internal/provider"
)

func registerPrivate(_ *provider.Registry, _ *http.Client) {}
