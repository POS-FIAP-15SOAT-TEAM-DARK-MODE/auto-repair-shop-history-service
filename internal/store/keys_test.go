package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatTimeIsFixedWidthUTC(t *testing.T) {
	whole := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	fraction := whole.Add(100 * time.Millisecond)
	local := time.Date(2026, 11, 1, 9, 0, 0, 0, time.FixedZone("BRT", -3*3600))

	assert.Equal(t, "2026-11-01T12:00:00.000000000Z", formatTime(whole))
	assert.Equal(t, "2026-11-01T12:00:00.100000000Z", formatTime(fraction))
	assert.Equal(t, "2026-11-01T12:00:00.000000000Z", formatTime(local))
	// Sort keys compare as strings: a whole second must sort before its fractions.
	assert.Less(t, formatTime(whole), formatTime(fraction))
}

func TestParseTimeRoundTrip(t *testing.T) {
	in := time.Date(2026, 11, 1, 12, 30, 0, 123456789, time.UTC)
	out, err := parseTime(formatTime(in))
	require.NoError(t, err)
	assert.True(t, in.Equal(out))
}

func TestParseTimeRejectsGarbage(t *testing.T) {
	_, err := parseTime("not-a-time")
	assert.Error(t, err)
}

func TestKeyBuilders(t *testing.T) {
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, "SO#01JD", soPK("01JD"))
	assert.Equal(t, "S#2026-11-01T12:00:00.000000000Z#m1", statusSK(at, "m1"))
	assert.Equal(t, "W#w1#2026-11-01T12:00:00.000000000Z#m1", workSK("w1", at, "m1"))
	assert.Equal(t, "R#w1", startedSK("w1"))
	assert.Equal(t, "W#w1", reportSK("w1"))
	assert.Equal(t, "MSG#m1", dedupePK("m1"))
}

func TestFormatSeconds(t *testing.T) {
	assert.Equal(t, "3600", formatSeconds(3600))
	assert.Equal(t, "0.5", formatSeconds(0.5))
}
