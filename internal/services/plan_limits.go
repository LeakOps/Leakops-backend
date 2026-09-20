package services

import (
	"errors"
	"fmt"
	"time"

	"Leakops-backend/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrPlanRevenueLimitReached = errors.New("monthly plan revenue limit reached")

type PlanLimits struct {
	MaxGateways              int
	MonthlyRevenueLimitCents int64
}

type RevenueLimitError struct {
	Plan       models.PlanTier
	LimitCents int64
	UsedCents  int64
}

func (e *RevenueLimitError) Error() string {
	return fmt.Sprintf("%s plan monthly revenue limit reached", e.Plan)
}

func (e *RevenueLimitError) Is(target error) bool {
	return target == ErrPlanRevenueLimitReached
}

func LimitsForPlan(plan models.PlanTier) PlanLimits {
	switch plan {
	case models.PlanStarter:
		return PlanLimits{MaxGateways: 1, MonthlyRevenueLimitCents: 500_000}
	case models.PlanGrowth:
		return PlanLimits{MaxGateways: 1, MonthlyRevenueLimitCents: 2_500_000}
	case models.PlanScale:
		return PlanLimits{MaxGateways: 0, MonthlyRevenueLimitCents: 10_000_000}
	case models.PlanEnterprise:
		return PlanLimits{MaxGateways: 0, MonthlyRevenueLimitCents: 0}
	default:
		return PlanLimits{MaxGateways: 1, MonthlyRevenueLimitCents: 200_000}
	}
}

func UserPlan(db *gorm.DB, userID uuid.UUID) (models.PlanTier, error) {
	var subscription models.Subscription
	err := db.Where("user_id = ?", userID).First(&subscription).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.PlanFree, nil
	}
	if err != nil {
		return models.PlanFree, err
	}

	if subscription.Status != models.SubStatusActive {
		return models.PlanFree, nil
	}
	return subscription.Plan, nil
}

func CheckMonthlyRevenueLimit(db *gorm.DB, userID uuid.UUID, amountCents int64) error {
	var user models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
		return err
	}

	plan, err := UserPlan(db, userID)
	if err != nil {
		return err
	}

	limits := LimitsForPlan(plan)
	if amountCents < 0 || limits.MonthlyRevenueLimitCents <= 0 {
		return nil
	}

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	nextMonth := monthStart.AddDate(0, 1, 0)

	var usedCents int64
	result := db.Model(&models.FailedPayment{}).
		Joins("JOIN gateway_accounts ON gateway_accounts.id = failed_payments.gateway_account_id").
		Where("gateway_accounts.user_id = ? AND failed_payments.created_at >= ? AND failed_payments.created_at < ?", userID, monthStart, nextMonth).
		Select("COALESCE(SUM(failed_payments.amount_cents), 0)").
		Scan(&usedCents)
	if result.Error != nil {
		return result.Error
	}

	if usedCents+amountCents > limits.MonthlyRevenueLimitCents {
		return &RevenueLimitError{
			Plan:       plan,
			LimitCents: limits.MonthlyRevenueLimitCents,
			UsedCents:  usedCents,
		}
	}
	return nil
}
