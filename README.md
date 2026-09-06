# demo-agent

AgentStore의 x402 reference scenario용 독립 Go resource server입니다. Spring API나 PostgreSQL에 직접 쓰지 않습니다. 결제 검증·정산은 x402 v2 `exact`/EIP-3009과 configured facilitator가 담당합니다.

`catalog/agents.yaml`은 demo 계약의 단일 원본입니다. 13개 Agent의 Function Contract Schema, 가격·`payTo`·Base Sepolia USDC 조건, dependency 선택 정책, runtime role·prompt·fixture를 함께 정의합니다. Go runtime과 `catalog-bootstrap`은 같은 embedded catalog를 사용합니다.

## 실행

checked-in [config/application.yaml](config/application.yaml)이 host, port, fixture 기본 mode, facilitator, callback allowlist를 소유합니다. `.env`의 `DEMO_AGENT_MODE`는 기본 mode를 덮어쓰며, CLI `--mode`가 있으면 CLI가 최우선입니다. `.env`에는 OpenAI 비밀값도 둘 수 있습니다. 운영 프록시 뒤에서 x402 리소스 URL을 HTTPS로 고정해야 할 때만 `DEMO_AGENT_PUBLIC_BASE_URL`을 설정합니다.

```powershell
Copy-Item .env.example .env
go run ./cmd/demo-agent --config config/application.yaml --host 127.0.0.1 --port 8090
```

`DEMO_AGENT_MODE=openai`와 `OPEN_AI_KEY`를 `.env`에 설정하면 같은 명령이 OpenAI mode로 실행됩니다. 명시적으로 바꾸려면 다음처럼 실행합니다.

```powershell
go run ./cmd/demo-agent --config config/application.yaml --mode openai
```

OpenAI Responses 요청에는 애플리케이션이 정한 `max_output_tokens` 상한을 보내지 않습니다. 출력 길이는 선택한 모델과 OpenAI provider의 정책을 따르며, provider가 `incomplete`를 반환하면 해당 Agent 호출은 실패로 처리합니다.

Root Agent 세 개(`investment-analysis`, `shopping-assistant`, `travel-planner`)만 declared dependency를 병렬 callback으로 호출해 결과를 종합합니다. EIP-3009은 요청별 32-byte random authorization nonce를 사용하므로 독립 payment는 같은 payer라도 병렬 settlement할 수 있습니다. specialist는 resolver를 호출하지 않습니다. callback URL은 config의 exact origin만 허용합니다: local Spring `http://127.0.0.1:8080`/`http://localhost:8080`, Compose `http://api:8080`, 배포 Spring `https://api-agent-store.sr-domain.win`. local loopback은 IPv4 loopback으로 고정되고 redirect와 1 MiB 초과 body는 거절합니다. 노드 하나의 예산은 30초지만 callback transport는 target call path의 남은 depth를 곱한다. 즉 depth 2는 120초, leaf depth 5는 30초이고 root의 aggregate는 `30 × 5 = 150초`이다. `payment.maxDependencyDepth`가 AgentStore contract의 5와 다르면 Go runtime은 기동하지 않는다.

## Docker 배포

이 저장소의 `Dockerfile`과 `compose.yaml`은 Go `demo-agent`만 실행합니다. Spring API, PostgreSQL, 프론트엔드는 별도 배포 대상이며 Go 컨테이너에 포함하지 않습니다. `catalog-bootstrap`은 운영 서버로 계속 실행하는 서비스가 아니라 catalog를 Spring에 등록할 때 한 번 실행하는 관리용 CLI이므로 이미지에도 포함하지 않습니다.

NAS의 Container Manager에서는 이 저장소를 별도 Project로 올리고 `.env`에 다음 값을 입력합니다.

```dotenv
OPEN_AI_KEY=<OpenAI API key>
DEMO_AGENT_MODE=openai
# 운영 프록시의 공개 원본 주소 (로컬에서는 생략)
DEMO_AGENT_PUBLIC_BASE_URL=https://demo-agent.sr-domain.win
```

그 다음 실행합니다.

```bash
docker compose build
docker compose up -d
docker compose ps
```

컨테이너는 내부 `8090`을 호스트 `27999`로 공개합니다. Nginx Proxy Manager에서 `demo-agent.sr-domain.win`을 `http://192.168.0.2:27999`로 연결하고 HTTPS 인증서를 적용합니다. 외부에 27999를 직접 공개하지 말고 Nginx Proxy Manager를 통해서만 접근합니다. 공개 endpoint는 catalog bootstrap을 실행할 때 `https://demo-agent.sr-domain.win`으로 사용합니다.

Nginx Proxy Manager가 TLS를 종료하면 Go 프로세스가 보는 연결은 HTTP가 되므로, 운영 `.env`에는 `DEMO_AGENT_PUBLIC_BASE_URL=https://demo-agent.sr-domain.win`을 넣습니다. 그러면 x402 402 응답의 `resource.url`이 catalog에 등록된 HTTPS endpoint와 일치합니다. 로컬에서는 이 변수를 비워 두어 요청의 실제 HTTP 주소를 그대로 사용합니다.

## catalog bootstrap

API와 demo-agent health 뒤 catalog를 등록합니다. Bootstrap은 Function Contract를 먼저 만들고 manifest를 import하여 Version을 publish합니다. 이미 ACTIVE 데이터가 있으면 canonical Contract와 manifest digest가 같은 경우에만 성공하며, 다르면 덮어쓰지 않고 `catalog drift`로 중단합니다. 사람은 먼저 AgentStore 랜딩의 `데모 시작`을 눌러 6시간 demo Bearer access token을 발급받고 `AGENT_STORE_DEMO_ACCESS_TOKEN`으로 전달합니다. bootstrap은 cookie/CSRF session을 만들지 않습니다.

```powershell
$env:AGENT_STORE_DEMO_ACCESS_TOKEN = '<AgentStore demo access token>'
go run ./cmd/catalog-bootstrap `
  --agent-store-base-url http://127.0.0.1:8080 `
  --demo-agent-base-url http://127.0.0.1:8090
```

Compose의 `DEMO_AGENT_MODE`은 `--mode` flag로 전달하는 명시적 interpolation 값입니다. 운영에서 공개 URL을 사용하는 경우 `.env`의 `DEMO_AGENT_PUBLIC_BASE_URL`도 함께 읽습니다.

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
