// Package solar implements the SolarService gRPC handler.
// It computes sunrise, sunset, and solar noon for today and tomorrow using
// a simple day-length approximation (latitude 37.7° N, San Francisco as default).
package solar

import (
	"context"
	"math"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	solarv1 "github.com/fdatoo/switchyard/gen/switchyard/solar/v1"
	"github.com/fdatoo/switchyard/gen/switchyard/solar/v1/solarv1connect"
)

// Service implements solarv1connect.SolarServiceHandler.
type Service struct{}

var _ solarv1connect.SolarServiceHandler = (*Service)(nil)

// NewService returns a new SolarService.
func NewService() *Service { return &Service{} }

// GetTable returns solar events for today and tomorrow.
func (s *Service) GetTable(ctx context.Context, req *connect.Request[solarv1.GetTableRequest]) (*connect.Response[solarv1.GetTableResponse], error) {
	lat := req.Msg.GetLatitude()
	if lat == 0 {
		lat = 37.7749 // default: San Francisco
	}
	lon := req.Msg.GetLongitude()
	if lon == 0 {
		lon = -122.4194
	}

	now := time.Now()
	today := computeSolarDay(now, lat, lon)
	tomorrow := computeSolarDay(now.AddDate(0, 0, 1), lat, lon)

	return connect.NewResponse(&solarv1.GetTableResponse{
		Today:    today,
		Tomorrow: tomorrow,
	}), nil
}

// computeSolarDay computes approximate solar events for a given date and location.
// Uses the NOAA simplified solar position algorithm.
func computeSolarDay(t time.Time, latDeg, lonDeg float64) *solarv1.SolarDay {
	// Work in local time zone so the date is correct for the caller.
	year, month, day := t.Date()
	loc := t.Location()

	// Julian Day Number (noon UT)
	jdn := julianDayNumber(year, int(month), day)
	jd := float64(jdn) + 0.5

	// Day of year
	doy := t.YearDay()

	// Equation of time (minutes) — simplified
	B := 2 * math.Pi * float64(doy-1) / 365.0
	eqTime := 229.18 * (0.000075 + 0.001868*math.Cos(B) - 0.032077*math.Sin(B) -
		0.014615*math.Cos(2*B) - 0.04089*math.Sin(2*B))

	// Hour angle for sunrise/sunset
	latRad := latDeg * math.Pi / 180.0
	decl := 0.006918 - 0.399912*math.Cos(B) + 0.070257*math.Sin(B) -
		0.006758*math.Cos(2*B) + 0.000907*math.Sin(2*B) -
		0.002697*math.Cos(3*B) + 0.00148*math.Sin(3*B)

	cosHA := (math.Cos(90.833*math.Pi/180) - math.Sin(latRad)*math.Sin(decl)) /
		(math.Cos(latRad) * math.Cos(decl))

	// Clamp to [-1, 1] to handle polar regions
	cosHA = math.Max(-1, math.Min(1, cosHA))
	haRad := math.Acos(cosHA)
	haDeg := haRad * 180.0 / math.Pi

	// Solar noon, sunrise, sunset in UTC minutes from midnight
	noonMin := 720 - 4*lonDeg - eqTime
	sunriseMin := noonMin - 4*haDeg
	sunsetMin := noonMin + 4*haDeg

	_ = jd // suppress unused warning

	toTime := func(minutesFromMidnightUTC float64) time.Time {
		base := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		return base.Add(time.Duration(minutesFromMidnightUTC * float64(time.Minute))).In(loc)
	}

	return &solarv1.SolarDay{
		Sunrise:   timestamppb.New(toTime(sunriseMin)),
		SolarNoon: timestamppb.New(toTime(noonMin)),
		Sunset:    timestamppb.New(toTime(sunsetMin)),
		Date:      t.Format("2006-01-02"),
	}
}

// julianDayNumber converts a Gregorian calendar date to Julian Day Number.
func julianDayNumber(year, month, day int) int {
	a := (14 - month) / 12
	y := year + 4800 - a
	m := month + 12*a - 3
	return day + (153*m+2)/5 + 365*y + y/4 - y/100 + y/400 - 32045
}
