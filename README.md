# NovaQuant

NovaQuant 是一个按需求文档重构为 `Golang + React` 的数字资产研究平台。系统只做研究、分析、预警、AI 问答和回测，不做自动交易、不调用私钥下单。

## 技术栈

- 后端: Go 1.25, `net/http`, `pgx`
- 前端: React + TypeScript + Vite + ECharts
- 数据库: PostgreSQL / TimescaleDB，通过 `PG_DSN` 接入
- 辅助组件: Redis, Docker Compose

## 目录

- `cmd/`: API、Agent、Collector、Analyzer、Alerts、Migrate、Seed 入口
- `internal/`: 配置、仓储、HTTP API、指标、回测、Agent 逻辑
- `migrations/`: SQL 迁移
- `web/`: React 仪表盘
- `docs/openapi.yaml`: API 契约草案

## 快速开始

1. 复制环境变量

```bash
cp .env.example .env
```

2. 修改 `.env` 中的 `PG_DSN`

本地直接运行 Go 命令时使用 `PG_DSN` 连接你自己的 PostgreSQL / TimescaleDB。
`docker compose` 默认会启动内部 `db` 服务，不再依赖宿主机数据库，并通过命名卷持久化数据。

如果你的网络对交易所 API 有限制，可以同时配置多个上游，采集器会按顺序自动回退：

```env
MARKET_DATA_PROVIDERS=bitget,okx,binance
```

如果你要接入自己的大模型或第三方兼容模型接口，可以配置:

```env
LLM_BASE_URL=https://your-openai-compatible-host
LLM_API_KEY=your_api_key
LLM_MODEL=your_model_name
LLM_CHAT_PATH=/v1/chat/completions
```

如果不配置这些变量，`AI Briefing` 会退回到本地模板摘要模式。

默认也支持登录保护，登录口令会先在前端用 `RSA-OAEP-256` 加密再提交:

```env
AUTH_ENABLED=true
AUTH_USERNAME=admin
AUTH_PASSWORD=change-this-password
AUTH_TOKEN_SECRET=change-this-token-secret
VITE_AUTH_ENABLED=true
```

说明:

- 前端口令加密只覆盖登录提交本身
- 生产环境仍然应该启用 HTTPS，这才是完整的传输层保护

3. 初始化数据库并写入演示数据

```bash
make migrate
make seed
```

或者直接使用 Docker Compose 一步启动，所有后端服务都会从项目根目录 `.env` 读取配置:

```bash
docker compose --env-file .env up --build
```

其中:

- 本地直接运行 Go 命令时使用 `PG_DSN` / `REDIS_URL`
- Docker Compose 内部会自动改用内置 `db` 服务和 `DOCKER_REDIS_URL`
- 内置数据库账号可通过 `DOCKER_POSTGRES_DB` / `DOCKER_POSTGRES_USER` / `DOCKER_POSTGRES_PASSWORD` 调整

这条命令会按顺序启动:

- `db / redis`
- `migrate`
- `seed`
- `api / agent / collector / analyzer / alerts`
- `web`

4. 启动后端 API

```bash
go run ./cmd/api
```

5. 启动前端

```bash
cd web
export PATH="$HOME/.nvm/versions/node/v23.11.0/bin:$PATH"
npm install
npm run dev
```

默认地址:

- API: `http://localhost:50800`
- Agent: `http://localhost:50890`
- Web: `http://localhost:51740`

## 常用命令

```bash
make migrate
make seed
make check-data
make test
make test-go
make test-web
make build
make docker-build
make up
make down
make compose-config
```

这些命令里，`make migrate`、`make seed`、`make check-data`、`make mcp-db` 现在都通过 `docker compose run` 在容器内执行，直接走内部 `db` 服务。

`make test-python` 保留为兼容目标，但会转向 Go 版本的 agent/service 测试，因为本项目已经按你的要求移除了 Python Agent 实现。

## 已实现模块

- 市场总览: `/api/v1/market/overview`
- 单币详情: `/api/v1/market/{symbol}`, `/api/v1/klines/{symbol}`, `/api/v1/analysis/{symbol}`, `/api/v1/risk/{symbol}`
- AI Research: `/api/v1/agent/chat`
- 回测: `/api/v1/backtest`
- 资讯/ETF/Whale/Alerts: `/api/v1/news`, `/api/v1/etf`, `/api/v1/whale`, `/api/v1/alerts`
- React 页面: 总览、详情、AI、风险、鲸鱼、ETF、新闻、策略、设置
- 只读 MCP 服务: `make mcp-db`

ETF 数据当前通过 `SOSO Value` 的 `summary-history` 接口采集，相关环境变量:

- `SOSO_BASE_URL`
- `SOSO_ETF_API_KEY`
- `ETF_COUNTRY_CODE`

## 测试状态

当前仓库内已验证:

- `go test ./...`
- `cd web && npm test`
- `cd web && npm run build`

## 数据检查

如果你想快速确认数据库里是否已经有 `BTC/ETH` 的最新价格、`1m` K 线、分析和 ETF 数据，可以执行:

```bash
make check-data
```

它会输出一段 JSON，重点看:

- `latest_price_source`
- `latest_price_ts`
- `kline_1m_rows`
- `latest_analysis_1h_trend`
- `latest_analysis_4h_trend`
- `latest_analysis_1d_trend`

如果 `latest_price_source` 显示为 `bitget`、`okx` 或 `binance`，并且 `kline_1m_rows > 0`，说明实时采集已经落库，不再只是种子数据。

## MCP 读取

项目内置了一个只读的数据库 MCP 服务，启动方式:

```bash
make mcp-db
```

当前暴露的工具:

- `db_health`
- `market_overview`
- `symbol_analysis`
- `etf_flows`

这个服务通过 `stdio` 工作，适合后面挂到支持 MCP 的客户端里，直接读取 NovaQuant 数据库内容。

未在当前环境完成:

- 真实 `PG_DSN` 下的迁移与种子执行
- `Playwright` E2E
- `docker compose up` 全链路验收

## 故障排查

- 如果 `node -v` 显示的不是 `v23.11.0`，前端命令前显式执行:

```bash
export PATH="$HOME/.nvm/versions/node/v23.11.0/bin:$PATH"
```

- 如果 `make migrate` 报 `timescaledb` 扩展不存在，说明当前库不是 TimescaleDB；迁移脚本会跳过 hypertable 初始化，但推荐接入真正的 TimescaleDB。

- 如果 API 健康检查里 `redis` 是 `down`，核心读接口仍可工作，但缓存链路不会完整。
