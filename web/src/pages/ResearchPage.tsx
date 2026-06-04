import { FormEvent, useState } from "react";
import { Panel } from "../components/Panel";
import { AgentResponse, apiPost } from "../lib/api";
import { asArray } from "../lib/collections";

export function ResearchPage() {
  const [question, setQuestion] = useState("分析 BTC 风险，现在是否过热？");
  const [response, setResponse] = useState<AgentResponse | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    try {
      const data = await apiPost<AgentResponse>("/agent/chat", {
        question,
        symbols: ["BTCUSDT"],
        time_horizon: "short",
      });
      setResponse(data);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="page-grid two-column">
      <Panel title="AI Research Center" subtitle="数据库证据驱动的问答">
        <form className="stack" onSubmit={onSubmit}>
          <textarea value={question} onChange={(event) => setQuestion(event.target.value)} rows={6} />
          <button type="submit" className="primary-button" disabled={loading}>
            {loading ? "分析中..." : "提交问题"}
          </button>
        </form>
      </Panel>
      <Panel title="Answer" subtitle="结构化结论与免责声明">
        <p className="summary-text">{response?.answer ?? "输入问题后，这里会返回 answer、evidence、risk 和 data timestamp。"}</p>
        {response ? (
          <>
            <div className="tag-row">
              <span className="price-tag bull-tag">Trend {response.trend}</span>
              <span className="price-tag bear-tag">Risk {response.risk_level}</span>
            </div>
            <div className="list">
              {asArray(response.evidence).map((item, index) => (
                <article key={`${item.type}-${index}`} className="list-item">
                  <strong>{item.type}</strong>
                  <p className="muted">{item.summary}</p>
                </article>
              ))}
            </div>
          </>
        ) : null}
      </Panel>
    </div>
  );
}
