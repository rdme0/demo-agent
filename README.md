# demo-agent

AgentStore의 x402 reference scenario용 독립 Go resource server입니다. Spring API나 PostgreSQL에 직접 쓰지 않습니다. 결제 검증·정산은 x402 v2 `exact`/EIP-3009과 configured facilitator가 담당합니다.

`catalog/agents.yaml`은 demo 계약의 단일 원본입니다. 13개 Agent의 Function Contract Schema, 가격·`payTo`·Base Sepolia USDC 조건, dependency 선택 정책, runtime role·prompt·fixture를 함께 정의합니다. Go runtime과 `catalog-bootstrap`은 같은 embedded catalog를 사용합니다.

## 실행

checked-in [config/application.yaml](config/application.yaml)이 host, port, fixture 기본 mode, facilitator, callback allowlist를 소유합니다. `.env`의 `DEMO_AGENT_MODE`는 기본 mode를 덮어쓰며, CLI `--mode`가 있으면 CLI가 최우선입니다. `.env`에는 OpenAI 비밀값도 둘 수 있습니다.

```powershell
Copy-Item .env.example .env
go run ./cmd/demo-agent --config config/application.yaml --host 127.0.0.1 --port 8090
```

`DEMO_AGENT_MODE=openai`와 `OPEN_AI_KEY`를 `.env`에 설정하면 같은 명령이 OpenAI mode로 실행됩니다. 명시적으로 바꾸려면 다음처럼 실행합니다.

```powershell
go run ./cmd/demo-agent --config config/application.yaml --mode openai
```

OpenAI Responses 요청에는 애플리케이션이 정한 `max_output_tokens` 상한을 보내지 않습니다. 출력 길이는 선택한 모델과 OpenAI provider의 정책을 따르며, provider가 `incomplete`를 반환하면 해당 Agent 호출은 실패로 처리합니다.

Root Agent 세 개(`investment-analysis`, `shopping-assistant`, `travel-planner`)만 declared dependency를 병렬 callback으로 호출해 결과를 종합합니다. EIP-3009은 요청별 32-byte random authorization nonce를 사용하므로 독립 payment는 같은 payer라도 병렬 settlement할 수 있습니다. specialist는 resolver를 호출하지 않습니다. callback URL은 config의 exact origin만 허용합니다: local Spring `http://127.0.0.1:8080`/`http://localhost:8080`, Compose `http://api:8080`. local loopback은 IPv4 loopback으로 고정되고 redirect와 1 MiB 초과 body는 거절합니다. 노드 하나의 예산은 30초지만 callback transport는 target call path의 남은 depth를 곱한다. 즉 depth 2는 120초, leaf depth 5는 30초이고 root의 aggregate는 `30 × 5 = 150초`이다. `payment.maxDependencyDepth`가 AgentStore contract의 5와 다르면 Go runtime은 기동하지 않는다.

## catalog bootstrap

API와 demo-agent health 뒤 catalog를 등록합니다. Bootstrap은 Function Contract를 먼저 만들고 manifest를 import하여 Version을 publish합니다. 이미 ACTIVE 데이터가 있으면 canonical Contract와 manifest digest가 같은 경우에만 성공하며, 다르면 덮어쓰지 않고 `catalog drift`로 중단합니다. 사람은 먼저 AgentStore 랜딩의 `데모 시작`을 눌러 6시간 demo Bearer access token을 발급받고 `AGENT_STORE_DEMO_ACCESS_TOKEN`으로 전달합니다. bootstrap은 cookie/CSRF session을 만들지 않습니다.

```powershell
$env:AGENT_STORE_DEMO_ACCESS_TOKEN = '<AgentStore demo access token>'
go run ./cmd/catalog-bootstrap `
  --agent-store-base-url http://127.0.0.1:8080 `
  --demo-agent-base-url http://127.0.0.1:8090
```

Compose는 같은 CLI에 `http://api:8080`, `http://demo-agent:8090`를 넘깁니다. `DEMO_AGENT_MODE`은 Compose가 `--mode` flag로 전달하는 명시적 interpolation 값입니다.

## HTTP 계약

- `GET /health` → `{ "status": "ok" }`
- `POST /agents/:code/invoke` → Agent output과 Root의 dependency results

## 검증

```powershell
gofmt -w catalog cmd internal
go test ./...
go vet ./...
go build ./...
git diff --check
```
