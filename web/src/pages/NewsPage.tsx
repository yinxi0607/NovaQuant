import { useEffect, useState } from "react";
import { Panel } from "../components/Panel";
import { NewsItem, apiGet } from "../lib/api";
import { asArray } from "../lib/collections";

export function NewsPage() {
  const [news, setNews] = useState<NewsItem[]>([]);

  useEffect(() => {
    apiGet<{ news: NewsItem[] | null }>("/news?page_size=10").then((data) => setNews(asArray(data.news)));
  }, []);

  return (
    <Panel title="News Intelligence" subtitle="情绪标签与摘要">
      <div className="list">
        {news.map((item) => (
          <article key={item.id} className="list-item">
            <div className="row-between">
              <strong>{item.title}</strong>
              <span className={`risk-pill risk-${item.sentiment === "positive" ? "low" : item.sentiment === "negative" ? "high" : "medium"}`}>
                {item.sentiment}
              </span>
            </div>
            <p className="muted">{item.summary}</p>
          </article>
        ))}
      </div>
    </Panel>
  );
}
