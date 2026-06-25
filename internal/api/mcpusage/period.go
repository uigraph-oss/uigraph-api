package mcpusage

import "time"

// parsePeriod converts a period query param ("1d", "7d", "30d", "1y") into its
// canonical string and the UTC timestamp it resolves to. Unrecognized or empty
// values default to "7d".
func parsePeriod(raw string) (period string, since time.Time) {
	now := time.Now().UTC()
	switch raw {
	case "1d":
		return "1d", now.Add(-24 * time.Hour)
	case "30d":
		return "30d", now.Add(-30 * 24 * time.Hour)
	case "1y":
		return "1y", now.Add(-365 * 24 * time.Hour)
	default:
		return "7d", now.Add(-7 * 24 * time.Hour)
	}
}
