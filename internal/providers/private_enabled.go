//go:build private

package providers

import (
	"net/http"

	"github.com/omarys/labrador/internal/provider"
	"github.com/omarys/labrador/internal/providers/private/gallery"
	"github.com/omarys/labrador/internal/providers/private/mangadistrict"
)

func registerPrivate(reg *provider.Registry, client *http.Client) {
	_ = reg.Register(mangadistrict.New(client))
	_ = reg.Register(gallery.New(client))
}
