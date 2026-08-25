# demo-agent

AgentStore 기술 시연용 독립 Go resource server입니다. Spring 백엔드나 PostgreSQL에 접근하지 않으며, x402 결제 검증·정산은 공식 Go SDK와 configured
facilitator가 처리합니다.

## 실행

`.env.example`을 `.env`로 복사해 모든 필수 값을 명시합니다. 로컬 실행에서는 `.env`를 자동으로 읽으며, 이미 설정된 운영 환경변수는 덮어쓰지 않습니다. 프로그램은 환경변수에 기본값을 두지 않습니다.

```powershell
Copy-Item .env.example .env
go run ./cmd/demo-agent
```

fixture와 OpenAI 모드는 하나의 catalog에서 13개 Agent를 제공합니다. 쉬운 사용 Marketplace에는 Root Agent 세 개만 보이고,
개발자 모드에는 모든 Agent가 보입니다.

| 분야 | Root Agent | 전문 Agent |
|---|---|---|
| 투자 | `investment-analysis` | `financial-analysis`, `market-news-fast`, `market-news-deep`, `investment-risk` |
| 쇼핑 | `shopping-assistant` | `product-search`, `review-analysis`, `price-comparison` |
| 여행 | `travel-planner` | `destination-research`, `weather-forecast`, `travel-safety` |

Root는 세 전문 Agent를 callback으로 호출해 Markdown을 종합합니다. `market-news-fast`와 `market-news-deep`은 같은
`market-news-analysis` Function Contract의 대체 공급자이며, 투자 Root는 marketplace + `lowest_price`로 전자를 선택합니다.

`DEMO_AGENT_MODE=fixture`는 Schema를 만족하는 결정적 결과와 각 Agent별 HTTPS 출처 세 개를 사용합니다.
`DEMO_AGENT_MODE=openai`는 `OPEN_AI_KEY`로 전문 Agent가 OpenAI Responses API의 `gpt-5.6-luna` 모델과 웹 검색을 사용하고,
Root가 검증된 dependency 출처를 붙여 Markdown을 완성합니다. 검색·모델 호출에는 OpenAI API 비용이 발생합니다.

`DEMO_PAYMENT_MODE=simulated`는 결제 없이 호출합니다. `x402` mode는 `X402_FACILITATOR_URL`만 추가로 필요합니다.
Agent별 price, 고유 `payTo`, Base Sepolia USDC asset은 catalog에 고정되어 있어 `.env`에서 바꾸지 않습니다.

```powershell
Set-Location ../agent-store-infra
Copy-Item ../agent-store-be/.env.example ../agent-store-be/.env
Copy-Item ../demo-agent/.env.example ../demo-agent/.env
docker compose --env-file ../agent-store-be/.env up --build -d
```

개발 Compose에서는 demo-agent가 Spring API 컨테이너의 네트워크 네임스페이스를 공유합니다. 빈 DB의 demo catalog는 Spring `dev`
initializer가 직접 생성하며, Go 서비스는 invocation만 담당합니다.

서버를 실행한 뒤 다음 요청으로 실제 OpenAI agent를 호출할 수 있습니다.

```powershell
Invoke-RestMethod -Method Post `
    -Uri http://127.0.0.1:8090/agents/investment-analysis/invoke `
    -ContentType 'application/json' `
    -Body '{"input":{"ticker":"ACME"}}'
```

## HTTP 계약

- `GET /health` → `{ "status": "ok" }`
- `POST /agents/:code/invoke` → fixture 또는 catalog Agent의 output과 dependencyResults

현재 agent는 `internal/agent/model`의 `Agent` interface를 `internal/agent/service`에서 등록합니다. OpenAI adapter는
`internal/agent/client`에 두어 이후 다른 LLM provider도 같은 service 경계에 주입할 수 있습니다. HTTP, x402, callback 안전 경계는 provider와 독립적으로
유지됩니다.

runtime callback은 HTTP loopback만 허용하고, localhost를 IPv4 loopback으로 고정 연결합니다. redirect, 1 MiB를 넘는 request/response, 30초 초과
callback은 거절합니다.

## 디렉토리 구조

```text
cmd/demo-agent/       서버 실행 진입점과 의존성 조립
internal/catalog/     Go runtime Agent 동작 정의
internal/app/         애플리케이션 구성
internal/config/      명시적 환경변수 로딩과 검증
internal/agent/       agent controller, service, dto, model
internal/runtime/     runtime DTO와 loopback callback client
internal/payment/     결제 모델과 x402 middleware
internal/server/      Gin router와 공통 HTTP middleware
```

## 검증

```powershell
gofmt -w cmd internal
go test ./...
go vet ./...
go build ./...
Set-Location ../agent-store-infra
docker compose --env-file ../agent-store-be/.env config
```
