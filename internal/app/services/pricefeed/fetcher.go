package pricefeed

import (
	"context"
	"math/rand"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Fetcher retrieves prices for a feed.
type Fetcher interface {
	Fetch(ctx context.Context, feed pricefeed.Feed) (float64, string, error)
}

// FetcherFunc adapts a function to the Fetcher interface.
type FetcherFunc func(ctx context.Context, feed pricefeed.Feed) (float64, string, error)

func (f FetcherFunc) Fetch(ctx context.Context, feed pricefeed.Feed) (float64, string, error) {
	if f == nil {
		return 0, "", nil
	}
	return f(ctx, feed)
}

// RandomFetcher returns pseudo-random prices for demonstration purposes.
type RandomFetcher struct {
	rand *rand.Rand
	log  *logger.Logger
}

func NewRandomFetcher(log *logger.Logger) *RandomFetcher {
	if log == nil {
		log = logger.NewDefault("pricefeed-fetcher")
	}
	return &RandomFetcher{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
		log:  log,
	}
}

func (f *RandomFetcher) Fetch(ctx context.Context, feed pricefeed.Feed) (float64, string, error) {
	price := f.rand.Float64()*10 + 1
	return price, "random", nil
}
