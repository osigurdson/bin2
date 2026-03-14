package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bin2.io/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestCalculateUsagePeriodSummaryTimeWeightedStorage(t *testing.T) {
	from := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	tenGiB := int64(10 * 1024 * 1024 * 1024)

	summary, err := calculateUsagePeriodSummary(
		from,
		to,
		to,
		0,
		[]db.UsageEventDelta{
			{CreatedAt: from, Value: tenGiB},
			{CreatedAt: from.Add(10 * 24 * time.Hour), Value: -tenGiB},
		},
		3,
		5,
	)
	if err != nil {
		t.Fatalf("calculateUsagePeriodSummary returned error: %v", err)
	}

	if summary.StorageOpeningBytes != 0 {
		t.Fatalf("StorageOpeningBytes = %d, want 0", summary.StorageOpeningBytes)
	}
	if summary.StorageClosingBytes != 0 {
		t.Fatalf("StorageClosingBytes = %d, want 0", summary.StorageClosingBytes)
	}
	if summary.PushOpCount != 3 {
		t.Fatalf("PushOpCount = %d, want 3", summary.PushOpCount)
	}
	if summary.PullOpCount != 5 {
		t.Fatalf("PullOpCount = %d, want 5", summary.PullOpCount)
	}

	if got := formatUsageDecimal(summary.storageGiBMonths(), 12); got != "3.225806451613" {
		t.Fatalf("storageGiBMonths = %s, want 3.225806451613", got)
	}
	if got := formatUsageDecimal(summary.storageChargeUSD(), 12); got != "0.064516129032" {
		t.Fatalf("storageChargeUSD = %s, want 0.064516129032", got)
	}
	if got := formatUsageDecimal(summary.totalChargeUSD(), 12); got != "0.064556129032" {
		t.Fatalf("totalChargeUSD = %s, want 0.064556129032", got)
	}
}

func TestCalculateUsagePeriodSummaryRejectsNegativeBalance(t *testing.T) {
	from := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	_, err := calculateUsagePeriodSummary(
		from,
		to,
		to,
		0,
		[]db.UsageEventDelta{{CreatedAt: from.Add(time.Hour), Value: -1}},
		0,
		0,
	)
	if err == nil {
		t.Fatal("calculateUsagePeriodSummary returned nil error, want error")
	}
}

func TestUsageSummaryWindow(t *testing.T) {
	from, to, err := usageSummaryWindow("2026-03-01T00:00:00Z", "2026-04-01T00:00:00Z")
	if err != nil {
		t.Fatalf("usageSummaryWindow returned error: %v", err)
	}

	if !from.Equal(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("from = %s", from)
	}
	if !to.Equal(time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("to = %s", to)
	}

	if _, _, err := usageSummaryWindow("", "2026-04-01T00:00:00Z"); err == nil {
		t.Fatal("usageSummaryWindow missing from returned nil error")
	}
	if _, _, err := usageSummaryWindow("2026-03-01T00:00:00Z", "2026-03-01T00:00:00Z"); err == nil {
		t.Fatal("usageSummaryWindow equal range returned nil error")
	}
	if _, _, err := usageSummaryWindow("2026-03-01T00:00:00-07:00", "2026-04-01T00:00:00-06:00"); err == nil {
		t.Fatal("usageSummaryWindow non-UTC month returned nil error")
	}
	if _, _, err := usageSummaryWindow("2026-03-02T00:00:00Z", "2026-04-01T00:00:00Z"); err == nil {
		t.Fatal("usageSummaryWindow non-month-boundary from returned nil error")
	}
}

func TestUsageSummaryAsOfClampsToNowWithinMonth(t *testing.T) {
	from := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.March, 14, 12, 0, 0, 0, time.UTC)

	got := usageSummaryAsOf(from, to, now)
	if !got.Equal(now) {
		t.Fatalf("usageSummaryAsOf = %s, want %s", got, now)
	}
}

func TestCalculateUsagePeriodSummaryUsesAsOfForCurrentMonthAccrual(t *testing.T) {
	from := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	asOf := from.Add(14 * 24 * time.Hour)
	oneGiB := int64(1024 * 1024 * 1024)

	summary, err := calculateUsagePeriodSummary(
		from,
		to,
		asOf,
		0,
		[]db.UsageEventDelta{
			{CreatedAt: asOf.Add(-1 * time.Hour), Value: oneGiB},
		},
		0,
		0,
	)
	if err != nil {
		t.Fatalf("calculateUsagePeriodSummary returned error: %v", err)
	}

	if got := formatUsageDecimal(summary.storageGiBMonths(), 12); got != "0.001344086022" {
		t.Fatalf("storageGiBMonths = %s, want 0.001344086022", got)
	}
}

func TestUsageSummaryHandlerRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := &Server{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/usage/summary", nil)

	server.usageSummaryHandler(ctx)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestUsageSummaryHandlerRejectsInvalidWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := &Server{}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/usage/summary?from=&to=", nil)
	ctx.Set("user", user{tenantID: newTestUUID(t)})

	server.usageSummaryHandler(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func newTestUUID(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.New()
}
