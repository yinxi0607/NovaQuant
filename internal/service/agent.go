package service

import (
	"context"
	"fmt"
	"strings"

	"NovaQuant/internal/domain"
	"NovaQuant/internal/repository"
)

type Agent struct {
	repo *repository.Repository
	llm  *LLMClient
}

func NewAgent(repo *repository.Repository, llm *LLMClient) *Agent {
	return &Agent{repo: repo, llm: llm}
}

func (a *Agent) Chat(ctx context.Context, req domain.AgentChatRequest) (domain.AgentChatResponse, error) {
	symbols := req.Symbols
	if len(symbols) == 0 {
		symbols = []string{"BTCUSDT"}
	}
	symbol := strings.ToUpper(symbols[0])

	snapshot, err := a.repo.GetMarketSnapshot(ctx, symbol)
	if err != nil {
		return domain.AgentChatResponse{}, err
	}
	analysisDay, err := a.repo.GetLatestAnalysis(ctx, symbol, "1h")
	if err != nil {
		return domain.AgentChatResponse{}, err
	}
	analysisWeek, err := a.repo.GetLatestAnalysis(ctx, symbol, "4h")
	if err != nil {
		analysisWeek = analysisDay
	}
	analysisMonth, err := a.repo.GetLatestAnalysis(ctx, symbol, "1d")
	if err != nil {
		analysisMonth = analysisWeek
	}
	risk, err := a.repo.GetLatestRisk(ctx, symbol)
	if err != nil {
		return domain.AgentChatResponse{}, err
	}
	news, err := a.repo.ListNews(ctx, symbol, 2)
	if err != nil {
		return domain.AgentChatResponse{}, err
	}

	answer := renderAnswer(symbol, snapshot, analysisDay, analysisWeek, analysisMonth, risk, news)
	if a.llm != nil && a.llm.Enabled() {
		llmAnswer, err := a.renderLLMAnswer(ctx, req, snapshot, analysisDay, analysisWeek, analysisMonth, risk, news)
		if err == nil && strings.TrimSpace(llmAnswer) != "" {
			answer = llmAnswer
		}
	}
	evidence := []domain.AgentEvidence{
		{Type: "day_analysis", Symbol: symbol, Timestamp: analysisDay.TS, Summary: analysisDay.Summary},
		{Type: "week_analysis", Symbol: symbol, Timestamp: analysisWeek.TS, Summary: analysisWeek.Summary},
		{Type: "month_analysis", Symbol: symbol, Timestamp: analysisMonth.TS, Summary: analysisMonth.Summary},
		{Type: "risk", Symbol: symbol, Timestamp: risk.TS, Summary: fmt.Sprintf("risk score %d (%s)", risk.TotalScore, risk.RiskLevel)},
	}
	if len(news) > 0 {
		evidence = append(evidence, domain.AgentEvidence{
			Type:      "news",
			Symbol:    symbol,
			Timestamp: news[0].PublishedAt,
			Summary:   news[0].Title,
		})
	}

	sessionID, err := a.repo.EnsureAgentSession(ctx, req.SessionID, strings.TrimSpace(req.Question))
	if err != nil {
		return domain.AgentChatResponse{}, err
	}
	_ = a.repo.StoreAgentMessage(ctx, sessionID, "user", req.Question, nil)
	_ = a.repo.StoreAgentMessage(ctx, sessionID, "assistant", answer, evidence)

	return domain.AgentChatResponse{
		SessionID:     sessionID,
		Answer:        answer,
		Symbols:       []string{symbol},
		Trend:         analysisDay.Trend,
		RiskLevel:     risk.RiskLevel,
		Support:       analysisMonth.SupportLevels,
		Resistance:    analysisMonth.ResistanceLevels,
		Evidence:      evidence,
		DataTimestamp: analysisDay.TS,
		Disclaimer:    "This is not financial advice.",
	}, nil
}

func (a *Agent) renderLLMAnswer(ctx context.Context, req domain.AgentChatRequest, snapshot domain.MarketSnapshot, analysisDay, analysisWeek, analysisMonth domain.Analysis, risk domain.Risk, news []domain.NewsItem) (string, error) {
	if a.llm == nil || !a.llm.Enabled() {
		return "", nil
	}
	systemPrompt := "You are a crypto market research assistant. Use only the supplied structured data. Do not fabricate prices, indicators, or events. Keep the answer concise, practical, and in Chinese."
	var builder strings.Builder
	builder.WriteString("Question: ")
	builder.WriteString(strings.TrimSpace(req.Question))
	builder.WriteString("\n\nStructured context:\n")
	builder.WriteString(fmt.Sprintf("Symbol: %s\n", snapshot.Symbol))
	builder.WriteString(fmt.Sprintf("Price: %.4f\n24h change: %.4f%%\nFunding rate: %.6f\nOpen interest: %.4f\nRisk score: %d\nRisk level: %s\n", snapshot.Price, snapshot.Change24H, snapshot.FundingRate, snapshot.OpenInterest, risk.TotalScore, risk.RiskLevel))
	builder.WriteString(fmt.Sprintf("1h trend: %s | regime: %s | summary: %s\n", analysisDay.Trend, analysisDay.MarketRegime, analysisDay.Summary))
	builder.WriteString(fmt.Sprintf("4h trend: %s | regime: %s | summary: %s\n", analysisWeek.Trend, analysisWeek.MarketRegime, analysisWeek.Summary))
	builder.WriteString(fmt.Sprintf("1d trend: %s | regime: %s | summary: %s\n", analysisMonth.Trend, analysisMonth.MarketRegime, analysisMonth.Summary))
	builder.WriteString(fmt.Sprintf("Monthly support: %v\nMonthly resistance: %v\n", analysisMonth.SupportLevels, analysisMonth.ResistanceLevels))
	if len(news) > 0 {
		builder.WriteString("Recent news:\n")
		for _, item := range news {
			builder.WriteString("- ")
			builder.WriteString(item.Title)
			builder.WriteString(" | sentiment: ")
			builder.WriteString(item.Sentiment)
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\nReturn a compact Chinese briefing with: current state, day/week/month trend, risk, key support/resistance, and one clear caution.")
	return a.llm.Chat(ctx, systemPrompt, builder.String())
}

func renderAnswer(symbol string, snapshot domain.MarketSnapshot, analysisDay, analysisWeek, analysisMonth domain.Analysis, risk domain.Risk, news []domain.NewsItem) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%s 当前价格 %.2f，24 小时涨跌 %.2f%%。", symbol, snapshot.Price, snapshot.Change24H))
	builder.WriteString(fmt.Sprintf("日内趋势 %s，周级趋势 %s，月级趋势 %s。", analysisDay.Trend, analysisWeek.Trend, analysisMonth.Trend))
	builder.WriteString(fmt.Sprintf("当前市场状态分别为 日线 %s、周线 %s、月线 %s。", analysisDay.MarketRegime, analysisWeek.MarketRegime, analysisMonth.MarketRegime))
	builder.WriteString(fmt.Sprintf("综合风险分 %d，属于 %s 风险。", risk.TotalScore, risk.RiskLevel))
	if analysisDay.RSI14 != nil {
		builder.WriteString(fmt.Sprintf("日线 RSI14 为 %.1f；", *analysisDay.RSI14))
	}
	if len(analysisMonth.SupportLevels) > 0 {
		builder.WriteString(fmt.Sprintf("月级主要支撑位参考 %.2f。", analysisMonth.SupportLevels[0]))
	}
	if len(analysisMonth.ResistanceLevels) > 0 {
		builder.WriteString(fmt.Sprintf("月级主要压力位参考 %.2f。", analysisMonth.ResistanceLevels[0]))
	}
	if len(news) > 0 {
		builder.WriteString(fmt.Sprintf("最近一条相关资讯是“%s”。", news[0].Title))
	}
	builder.WriteString(" 当前系统只提供研究和风险提示，不提供买卖保证；若接近压力位或风险分继续抬升，应控制仓位并等待新的确认。")
	return builder.String()
}
