package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
)

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
	}{
		{"defaults", "", 50, 0},
		{"normal values", "?limit=20&offset=10", 20, 10},
		{"limit too high", "?limit=999999&offset=0", 100, 0},
		{"limit negative", "?limit=-5&offset=0", 50, 0},
		{"limit zero", "?limit=0&offset=0", 50, 0},
		{"offset negative", "?limit=50&offset=-100", 50, 0},
		{"offset too high (DoS prevention)", "?limit=50&offset=99999999", 50, MaxOffset},
		{"offset at max", "?limit=50&offset=10000", 50, 10000},
		{"invalid strings", "?limit=abc&offset=xyz", 50, 0},
		{"limit at boundary", "?limit=100&offset=0", 100, 0},
		{"limit over boundary", "?limit=101&offset=0", 100, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/test", func(c fiber.Ctx) error {
				p := ParsePagination(c)
				return c.JSON(fiber.Map{"limit": p.Limit, "offset": p.Offset})
			})

			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			// Parse response and verify
			app2 := fiber.New()
			var gotLimit, gotOffset int
			app2.Get("/test", func(c fiber.Ctx) error {
				p := ParsePagination(c)
				gotLimit = p.Limit
				gotOffset = p.Offset
				return nil
			})
			req2 := httptest.NewRequest("GET", "/test"+tt.query, nil)
			app2.Test(req2)

			assert.Equal(t, tt.wantLimit, gotLimit, "limit mismatch")
			assert.Equal(t, tt.wantOffset, gotOffset, "offset mismatch")
		})
	}
}

func TestParseDays(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		param    string
		defVal   int
		maxVal   int
		want     int
	}{
		{"default", "", "days", 7, 365, 7},
		{"normal", "?days=30", "days", 7, 365, 30},
		{"too high", "?days=9999", "days", 7, 365, 365},
		{"negative", "?days=-5", "days", 7, 365, 7},
		{"zero", "?days=0", "days", 7, 365, 7},
		{"at max", "?days=365", "days", 7, 365, 365},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			var got int
			app.Get("/test", func(c fiber.Ctx) error {
				got = ParseDays(c, tt.param, tt.defVal, tt.maxVal)
				return nil
			})

			req := httptest.NewRequest("GET", "/test"+tt.query, nil)
			app.Test(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseWeeks(t *testing.T) {
	app := fiber.New()
	var got int
	app.Get("/test", func(c fiber.Ctx) error {
		got = ParseWeeks(c, "weeks", 4, 52)
		return nil
	})

	// Test max capping
	req := httptest.NewRequest("GET", "/test?weeks=100", nil)
	app.Test(req)
	assert.Equal(t, 52, got, "should cap at 52 weeks")

	// Test normal value
	req = httptest.NewRequest("GET", "/test?weeks=8", nil)
	app.Test(req)
	assert.Equal(t, 8, got, "should accept valid value")
}

func TestParseMonths(t *testing.T) {
	app := fiber.New()
	var got int
	app.Get("/test", func(c fiber.Ctx) error {
		got = ParseMonths(c, "months", 6, 24)
		return nil
	})

	// Test max capping
	req := httptest.NewRequest("GET", "/test?months=100", nil)
	app.Test(req)
	assert.Equal(t, 24, got, "should cap at 24 months")

	// Test normal value
	req = httptest.NewRequest("GET", "/test?months=12", nil)
	app.Test(req)
	assert.Equal(t, 12, got, "should accept valid value")
}
