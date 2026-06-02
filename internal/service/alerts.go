package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"NovaQuant/internal/metrics"
	"NovaQuant/internal/repository"

	"github.com/jackc/pgx/v5"
)

type Alerts struct {
	repo    *repository.Repository
	metrics *metrics.Registry
}

func NewAlerts(repo *repository.Repository, registry *metrics.Registry) *Alerts {
	return &Alerts{repo: repo, metrics: registry}
}

func (a *Alerts) RunOnce(ctx context.Context) error {
	rules, err := a.repo.ListAlertRules(ctx)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		value, severity, err := a.resolveRuleValue(ctx, rule.Symbol, rule.RuleType)
		if err != nil {
			continue
		}
		if !compare(value, rule.Operator, rule.Threshold) {
			continue
		}
		lastTriggered, err := a.repo.LatestAlertForRule(ctx, rule.ID)
		if err == nil && time.Since(lastTriggered) < time.Duration(rule.CooldownSeconds)*time.Second {
			continue
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		message := fmt.Sprintf("%s triggered: %s %.2f %s %.2f", rule.Name, rule.RuleType, value, rule.Operator, rule.Threshold)
		payload := map[string]any{"value": value, "threshold": rule.Threshold, "rule_type": rule.RuleType}
		if err := a.repo.InsertAlertEvent(ctx, rule.ID, rule.Symbol, severity, message, payload); err != nil {
			return err
		}
		if a.metrics != nil {
			a.metrics.Inc("alerts_triggered_total")
		}
	}
	if a.metrics != nil {
		a.metrics.Inc("alerts_evaluated_total")
	}
	return nil
}

func (a *Alerts) resolveRuleValue(ctx context.Context, symbol, ruleType string) (float64, string, error) {
	switch ruleType {
	case "rsi":
		analysis, err := a.repo.GetLatestAnalysis(ctx, symbol, "1h")
		if err != nil || analysis.RSI14 == nil {
			return 0, "warning", err
		}
		return *analysis.RSI14, "warning", nil
	case "risk_score":
		risk, err := a.repo.GetLatestRisk(ctx, symbol)
		if err != nil {
			return 0, "critical", err
		}
		return float64(risk.TotalScore), "critical", nil
	case "funding":
		rows, err := a.repo.ListRecentFunding(ctx, symbol, 1)
		if err != nil || len(rows) == 0 {
			return 0, "warning", err
		}
		return rows[0].FundingRate, "warning", nil
	case "oi":
		rows, err := a.repo.ListRecentOpenInterest(ctx, symbol, 2)
		if err != nil || len(rows) < 2 {
			return 0, "warning", err
		}
		if rows[1].OpenInterest == 0 {
			return 0, "warning", nil
		}
		change := ((rows[0].OpenInterest / rows[1].OpenInterest) - 1) * 100
		return change, "warning", nil
	default:
		return 0, "info", fmt.Errorf("unsupported rule type %q", ruleType)
	}
}

func compare(value float64, operator string, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "=":
		return value == threshold
	default:
		return false
	}
}
