# demo-agent

AgentStore 기술 시연용 독립 Go resource server입니다. Spring 백엔드나 PostgreSQL에 접근하지 않으며, x402 결제 검증·정산은 공식 Go SDK와 configured
facilitator가 처리합니다.

## 실행

`.env.example`을 `.env`로 복사해 모든 필수 값을 명시합니다. 로컬 실행에서는 `.env`를 자동으로 읽으며, 이미 설정된 운영 환경변수는 덮어쓰지 않습니다. 프로그램은 환경변수에 기본값을 두지 않습니다.

```powershell
Copy-Item .env.example .env
go run ./cmd/demo-agent
```

fixture와 OpenAI 모드에서는 `investment`, `financial`, `news`, `risk` 네 Agent가 고정된 경로로 제공됩니다. Marketplace에서 공급자를 여러 개 등록하는 책임은 Spring AgentStore에 있으며, Go 서버 내부에 별도 slug 매핑을 설정하지 않습니다.

`DEMO_AGENT_MODE=fixture`는 네 개의 결정적 fixture 응답을 사용하고, `DEMO_AGENT_MODE=openai`는 `OPEN_AI_KEY`로 네 agent가 OpenAI Responses
API의 `gpt-5.6-luna` 모델을 호출합니다. OpenAI mode에서는 `financial`, `news`, `risk`가 웹 검색으로 최신 근거를 수집하고 `investment`가 이를 한국어
Markdown으로 종합합니다. 최종 투자 분석에는 검증된 HTTPS 출처 링크 3~5개가 붙습니다. 검색이 실패하거나 출처가 세 개 미만이면 근거 없는 빈 결과를 만들지 않고
호출을 실패 처리합니다. 웹 검색과 모델 호출에는 OpenAI API 비용이 발생합니다. 키는 반드시 명시해야 하며 로그나 응답에 노출하지 않습니다.

`DEMO_PAYMENT_MODE=simulated`는 결제 없이 agent를 호출합니다. `x402` mode에서는 `X402_FACILITATOR_URL`과 investment, financial, news,
risk의 atomic price/payTo/asset이 필요합니다. 등록한 slug에 해당하는 값만 검사하며 asset은 공식 Base Sepolia USDC
`0x036CbD53842c5426634e7929541eC2318f3dCF7e`만 허용합니다. 결제 설정은 네 고정 Agent 경로에 대해 `DEMO_INVESTMENT_*`, `DEMO_FINANCIAL_*`, `DEMO_NEWS_*`, `DEMO_RISK_*`로 제공합니다.

fixture/simulated 조합은 하나의 Go 서버에서 네 Agent를 모두 제공합니다.

```powershell
Set-Location ../agent-store-infra
Copy-Item ../agent-store-be/.env.example ../agent-store-be/.env
Copy-Item ../demo-agent/.env.example ../demo-agent/.env
docker compose --env-file ../agent-store-be/.env up --build -d
```

개발 Compose에서는 demo-agent가 Spring API 컨테이너의 네트워크 네임스페이스를 공유합니다. 따라서
`investment`, `financial`, `news`, `risk`는 호스트의 `127.0.0.1:8090`과 API 컨테이너의 같은 주소에서 제공됩니다.

서버를 실행한 뒤 다음 요청으로 실제 OpenAI agent를 호출할 수 있습니다.

```powershell
Invoke-RestMethod -Method Post `
    -Uri http://127.0.0.1:8090/agents/investment/invoke `
    -ContentType 'application/json' `
    -Body '{"input":{"ticker":"ACME"}}'
```

## HTTP 계약

- `GET /health` → `{ "status": "ok" }`
- `POST /agents/:agent/invoke` → fixture 또는 등록 agent의 output과 dependencyResults

현재 agent는 `internal/agent/model`의 `Agent` interface를 `internal/agent/service`에서 등록합니다. OpenAI adapter는
`internal/agent/client`에 두어 이후 다른 LLM provider도 같은 service 경계에 주입할 수 있습니다. HTTP, x402, callback 안전 경계는 provider와 독립적으로
유지됩니다.

runtime callback은 HTTP loopback만 허용하고, localhost를 IPv4 loopback으로 고정 연결합니다. redirect, 1 MiB를 넘는 request/response, 30초 초과
callback은 거절합니다.

## 디렉토리 구조

```text
cmd/demo-agent/       실행 진입점과 의존성 조립
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
