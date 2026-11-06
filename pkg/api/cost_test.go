package api_test

import (
	"testing"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdsdk"
	"github.com/stretchr/testify/require"
)

func TestAppCostNoOptions(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		c1 := &structs.AppCost{App: "app1", SpendUsd: 1.5}
		var c2 *structs.AppCost
		p.On("AppCostWithOptions", "app1", structs.AppCostOptions{}).Return(c1, nil)
		err := c.Get("/apps/app1/cost", stdsdk.RequestOptions{}, &c2)
		require.NoError(t, err)
		require.Equal(t, c1, c2)
	})
}

func TestAppCostRange(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		c1 := &structs.AppCost{App: "app1", SpendUsd: 0.5, RangeStart: "2026-10-01", RangeEnd: "2026-10-02"}
		var c2 *structs.AppCost
		opts := structs.AppCostOptions{Start: options.String("2026-10-01"), End: options.String("2026-10-02")}
		ro := stdsdk.RequestOptions{Query: stdsdk.Query{"start": "2026-10-01", "end": "2026-10-02"}}
		p.On("AppCostWithOptions", "app1", opts).Return(c1, nil)
		err := c.Get("/apps/app1/cost", ro, &c2)
		require.NoError(t, err)
		require.Equal(t, c1, c2)
	})
}

func TestAppCostBadRequest(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var c1 *structs.AppCost
		opts := structs.AppCostOptions{Start: options.String("10/01/2026")}
		ro := stdsdk.RequestOptions{Query: stdsdk.Query{"start": "10/01/2026"}}
		p.On("AppCostWithOptions", "app1", opts).Return(nil, structs.ErrBadRequest("start must be YYYY-MM-DD"))
		err := c.Get("/apps/app1/cost", ro, &c1)
		require.EqualError(t, err, "start must be YYYY-MM-DD")
		require.Nil(t, c1)
	})
}
