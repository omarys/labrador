package providers

import (
	"net/http"
	"time"

	"github.com/omarys/labrador/internal/httpclient"
	"github.com/omarys/labrador/internal/provider"
	"github.com/omarys/labrador/internal/providers/mangadex"
	"github.com/omarys/labrador/internal/providers/mangakatana"
	"github.com/omarys/labrador/internal/providers/manganato"
	"github.com/omarys/labrador/internal/providers/manganelo"
	"github.com/omarys/labrador/internal/providers/mangapill"
	"github.com/omarys/labrador/internal/providers/readcomicsonline"
	"github.com/omarys/labrador/internal/providers/weebcentral"
)

// RegisterAll registers all public and local private providers into the registry.
func RegisterAll(reg *provider.Registry, client *http.Client) {
	if client == nil {
		client = httpclient.NewStealthClient(30 * time.Second)
	}
	_ = reg.Register(mangapill.New(client))
	_ = reg.Register(mangadex.New(client))
	_ = reg.Register(mangakatana.New(client))
	_ = reg.Register(manganato.New(client))
	_ = reg.Register(manganelo.New(client))
	_ = reg.Register(readcomicsonline.New(client))
	_ = reg.Register(weebcentral.New(client))

	registerPrivate(reg, client)
}
