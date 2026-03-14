package server

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"bin2.io/internal/db"
)

var (
	storageRateUSDPerGiBMonth = big.NewRat(1, 50)
	pushRateUSDPerOp          = big.NewRat(1, 100000)
	pullRateUSDPerOp          = big.NewRat(1, 500000)
)

type usagePeriodSummary struct {
	From                time.Time
	To                  time.Time
	AsOf                time.Time
	PushOpCount         int64
	PullOpCount         int64
	StorageOpeningBytes int64
	StorageClosingBytes int64
	StorageByteNanos    *big.Int
}

func calculateUsagePeriodSummary(
	from, to time.Time,
	asOf time.Time,
	openingStorageBytes int64,
	storageDeltas []db.UsageEventDelta,
	pushOpCount int64,
	pullOpCount int64,
) (usagePeriodSummary, error) {
	if !to.After(from) {
		return usagePeriodSummary{}, fmt.Errorf("usage period end must be after start")
	}
	if openingStorageBytes < 0 {
		return usagePeriodSummary{}, fmt.Errorf("opening storage balance is negative")
	}
	if asOf.Before(from) || asOf.After(to) {
		return usagePeriodSummary{}, fmt.Errorf("usage period as-of must be within the requested period")
	}

	summary := usagePeriodSummary{
		From:                from.UTC(),
		To:                  to.UTC(),
		AsOf:                asOf.UTC(),
		PushOpCount:         pushOpCount,
		PullOpCount:         pullOpCount,
		StorageOpeningBytes: openingStorageBytes,
		StorageClosingBytes: openingStorageBytes,
		StorageByteNanos:    big.NewInt(0),
	}

	cursor := summary.From
	runningBytes := openingStorageBytes

	for _, delta := range storageDeltas {
		at := delta.CreatedAt.UTC()
		if at.Before(summary.From) || !at.Before(summary.AsOf) {
			return usagePeriodSummary{}, fmt.Errorf("storage delta outside requested period")
		}
		if at.Before(cursor) {
			return usagePeriodSummary{}, fmt.Errorf("storage deltas must be ordered by time")
		}
		if at.After(cursor) {
			accumulateStorageByteNanos(summary.StorageByteNanos, runningBytes, at.Sub(cursor))
			cursor = at
		}
		runningBytes += delta.Value
		if runningBytes < 0 {
			return usagePeriodSummary{}, fmt.Errorf("storage balance became negative at %s", at.Format(time.RFC3339Nano))
		}
	}

	if summary.AsOf.After(cursor) {
		accumulateStorageByteNanos(summary.StorageByteNanos, runningBytes, summary.AsOf.Sub(cursor))
	}

	summary.StorageClosingBytes = runningBytes
	return summary, nil
}

func accumulateStorageByteNanos(total *big.Int, bytes int64, duration time.Duration) {
	if bytes == 0 || duration <= 0 {
		return
	}
	product := new(big.Int).Mul(big.NewInt(bytes), big.NewInt(duration.Nanoseconds()))
	total.Add(total, product)
}

func usageSummaryWindow(fromRaw, toRaw string) (time.Time, time.Time, error) {
	fromRaw = strings.TrimSpace(fromRaw)
	toRaw = strings.TrimSpace(toRaw)
	if fromRaw == "" || toRaw == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("from and to are required")
	}

	from, err := time.Parse(time.RFC3339, fromRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid from value")
	}
	to, err := time.Parse(time.RFC3339, toRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid to value")
	}
	from = from.UTC()
	to = to.UTC()
	if !to.After(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("to must be after from")
	}
	if from.Day() != 1 || from.Hour() != 0 || from.Minute() != 0 || from.Second() != 0 || from.Nanosecond() != 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("from must be the first instant of a UTC month")
	}
	if !to.Equal(from.AddDate(0, 1, 0)) {
		return time.Time{}, time.Time{}, fmt.Errorf("from and to must describe a single UTC calendar month")
	}
	return from, to, nil
}

func usageSummaryAsOf(from, to, now time.Time) time.Time {
	from = from.UTC()
	to = to.UTC()
	now = now.UTC()

	if now.Before(from) {
		return from
	}
	if now.Before(to) {
		return now
	}
	return to
}

func (s usagePeriodSummary) storageByteSeconds() *big.Rat {
	return new(big.Rat).Quo(new(big.Rat).SetInt(s.StorageByteNanos), big.NewRat(int64(time.Second), 1))
}

func (s usagePeriodSummary) storageGiBHours() *big.Rat {
	denominator := new(big.Int).Mul(big.NewInt(1024*1024*1024), big.NewInt(int64(time.Hour)))
	return new(big.Rat).Quo(new(big.Rat).SetInt(s.StorageByteNanos), new(big.Rat).SetInt(denominator))
}

func (s usagePeriodSummary) storageGiBMonths() *big.Rat {
	periodNanos := s.To.Sub(s.From).Nanoseconds()
	if periodNanos <= 0 {
		return big.NewRat(0, 1)
	}
	denominator := new(big.Int).Mul(big.NewInt(1024*1024*1024), big.NewInt(periodNanos))
	return new(big.Rat).Quo(new(big.Rat).SetInt(s.StorageByteNanos), new(big.Rat).SetInt(denominator))
}

func (s usagePeriodSummary) storageChargeUSD() *big.Rat {
	return new(big.Rat).Mul(s.storageGiBMonths(), storageRateUSDPerGiBMonth)
}

func (s usagePeriodSummary) pushChargeUSD() *big.Rat {
	return new(big.Rat).Mul(big.NewRat(s.PushOpCount, 1), pushRateUSDPerOp)
}

func (s usagePeriodSummary) pullChargeUSD() *big.Rat {
	return new(big.Rat).Mul(big.NewRat(s.PullOpCount, 1), pullRateUSDPerOp)
}

func (s usagePeriodSummary) totalChargeUSD() *big.Rat {
	total := new(big.Rat).Add(s.storageChargeUSD(), s.pushChargeUSD())
	return total.Add(total, s.pullChargeUSD())
}

func formatUsageDecimal(value *big.Rat, scale int) string {
	if value == nil {
		return "0"
	}
	formatted := value.FloatString(scale)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	if formatted == "" || formatted == "-0" {
		return "0"
	}
	return formatted
}
