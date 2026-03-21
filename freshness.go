package main

import (
	"fmt"
	"strings"
	"time"
)

// freshnessEnabled controls the --freshness feature.
// freshnessYears/Months/Days hold the optional threshold.
var (
	freshnessEnabled bool
	freshnessYears   int
	freshnessMonths  int
	freshnessDays    int
)

// formatVersionAge returns a compact duration string representing the time
// between the current version's publish date and the latest version's
// publish date (e.g. "2y4m"). Returns "-" when current == latest version,
// and "" when data is unavailable.
func formatVersionAge(m Module) string {
	if m.LatestVersion == "" {
		return ""
	}
	if m.LatestVersion == m.Version {
		return "-"
	}
	if m.VersionTime.IsZero() || m.LatestTime.IsZero() {
		return ""
	}
	return versionAgeDuration(m.VersionTime, m.LatestTime)
}

// exceedsFreshnessThreshold returns true if the module's version publish date
// is older than the freshness threshold relative to now. Returns false if
// no threshold is set (all zeros) or if version time is unavailable.
func exceedsFreshnessThreshold(m Module) bool {
	if freshnessYears == 0 && freshnessMonths == 0 && freshnessDays == 0 {
		return true // no threshold → show all
	}
	if m.VersionTime.IsZero() {
		return false
	}
	return exceedsThreshold(m.VersionTime, freshnessYears, freshnessMonths, freshnessDays)
}

// formatFreshnessThreshold formats the threshold as a compact string for display.
func formatFreshnessThreshold() string {
	var parts []string
	if freshnessYears > 0 {
		parts = append(parts, fmt.Sprintf("%dy", freshnessYears))
	}
	if freshnessMonths > 0 {
		parts = append(parts, fmt.Sprintf("%dm", freshnessMonths))
	}
	if freshnessDays > 0 {
		parts = append(parts, fmt.Sprintf("%dd", freshnessDays))
	}
	return strings.Join(parts, "")
}

// versionAgeDuration computes a compact duration string between two times.
func versionAgeDuration(from, to time.Time) string {
	if to.Before(from) || to.Equal(from) {
		return "-"
	}
	y, m, d := calcDurationBetween(from, to)
	var parts []string
	if y > 0 {
		parts = append(parts, fmt.Sprintf("%dy", y))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if d > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%dd", d))
	}
	return strings.Join(parts, "")
}

// calcDurationBetween computes calendar duration between two dates,
// similar to calcDuration but without the +1 day inclusiveness.
func calcDurationBetween(from, to time.Time) (years, months, days int) {
	f := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	t := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)

	years = t.Year() - f.Year()
	months = int(t.Month()) - int(f.Month())
	days = t.Day() - f.Day()

	if days < 0 {
		months--
		days += time.Date(t.Year(), t.Month(), 0, 0, 0, 0, 0, time.UTC).Day()
	}
	if months < 0 {
		years--
		months += 12
	}
	return years, months, days
}
