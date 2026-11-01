package store

import (
	"strconv"
	"time"
)

// Attribute names of the table key schema.
const (
	attrPK = "PK"
	attrSK = "SK"
)

// Key prefixes (ddl.md, section `history`).
const (
	soPKPrefix         = "SO#"
	statusSKPrefix     = "S#"
	workSKPrefix       = "W#"
	startedSKPrefix    = "R#"
	reportPK           = "REPORT#WORK"
	dedupePKPrefix     = "MSG#"
	dedupeSK           = "DEDUPE"
	reportWorkSKPrefix = "W#"
)

// timeLayout is a fixed-width UTC layout with nanoseconds. Sort keys compare as strings, so every timestamp
// in a key must have the same width; RFC3339Nano trims zeros and would order "00Z" after "00.1Z".
const timeLayout = "2006-01-02T15:04:05.000000000Z"

func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}

func formatSeconds(seconds float64) string {
	return strconv.FormatFloat(seconds, 'f', -1, 64)
}

func soPK(serviceOrderID string) string {
	return soPKPrefix + serviceOrderID
}

func statusSK(occurredAt time.Time, messageID string) string {
	return statusSKPrefix + formatTime(occurredAt) + "#" + messageID
}

func workSK(workID string, occurredAt time.Time, messageID string) string {
	return workSKPrefix + workID + "#" + formatTime(occurredAt) + "#" + messageID
}

func startedSK(workID string) string {
	return startedSKPrefix + workID
}

func reportSK(workID string) string {
	return reportWorkSKPrefix + workID
}

func dedupePK(messageID string) string {
	return dedupePKPrefix + messageID
}
