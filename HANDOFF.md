# Demo Agent 인수인계서

최종 갱신: 2026-09-04

## 역할과 실행

- 경로: 이 저장소 루트
- Go/Gin 기반 독립 x402 resource server다. Spring DB를 직접 읽거나 쓰지 않는다.
- `catalog/agents.yaml`이 demo Agent, Function Contract, 가격, 결제 지갑, dependency와 runtime fixture의
  단일 원본이다.
- `cmd/demo-agent`는 `--config`, `--host`, `--port`, `--mode`를 명시적으로 받아 실행한다. `fixture`는
  비용 없는 결정적 시연용이고 `openai`는 `OPEN_AI_KEY`가 필요하다.
- `cmd/catalog-bootstrap`은 Spring API에 Function Contract와 Agent manifest를 등록하고 ACTIVE Version을
  publish한다. 기존 데이터가 다르면 drift로 중단하며 ACTIVE 데이터를 덮어쓰지 않는다.

## Demo cookie / readiness bootstrap — 2026-09-04 (superseded)

- Bootstrap은 `POST /api/demo/session`으로 server-issued demo cookie와 CSRF cookie를 먼저 받고, 이후 모든
  unsafe API request에 `X-XSRF-TOKEN`을 보낸다. catalog의 developer ID는 manifest content의 식별값일 뿐
  인증 근거가 아니다.
- 13개 catalog Agent에는 schema-valid deterministic `verificationInput: {}`이 있다. 기존 ACTIVE + UNVERIFIED
  Version이 이 input만 비어 있으면 manifest drift로 덮어쓰지 않고 one-time backfill API 뒤 `verify`를 호출한다.
  VERIFIED Version 재실행은 결제를 만들지 않으며 다른 manifest/price/endpoint/contract/dependency drift는 중단한다.
- verify는 실제 testnet 결제 경로이므로 bootstrap을 실행하기 전에 wallet/facilitator 준비와 사용자 확인이 필요하다.
  이번 검증은 local fixture만 사용했으며 실제 settlement를 수행하지 않았다.

## One-click Bearer bootstrap — 2026-09-05

- Cookie/CSRF session은 제거됐다. `cmd/catalog-bootstrap`은 사람이 AgentStore 랜딩에서 한 번 클릭해 발급받은
  365일 Bearer token을 `AGENT_STORE_DEMO_ACCESS_TOKEN`으로 명시적으로 받아 모든 AgentStore API request에
  `Authorization: Bearer`로 보낸다. token이 없으면 network mutation 전에 실패하며 별도 challenge를 처리하지 않는다.
- Bootstrap unit test는 local HTTP fixture에서 Authorization header를 검증한다. Go test는 mock framework를 사용하지 않는다.

## 구조

- `cmd`: 서버와 bootstrap 실행 진입점
- `internal/app`: 의존성 조립
- `internal/agent`: controller, DTO, model, registry와 fixture/OpenAI service
- `internal/runtime/client`: loopback/exact-origin callback, redirect 차단, 1 MiB와 30초 제한
- `internal/payment/x402`: Base Sepolia USDC exact middleware
- `catalog`: embedded YAML 로더와 schema/결제/strategy 검증

Root Agent 구현만 dependency resolver를 호출한다. specialist는 callback을 호출하지 않는다. callback은
Spring이 보낸 `Authorization: Bearer ...`를 원 invocation header로 전달하고 새 Idempotency-Key를 생성한다.

## 설정과 검증

- 공개 host/port·fixture 기본값은 `config/application.yaml`에 둔다. 비밀값은 `.env`의 `OPEN_AI_KEY`와
  사람이 발급받은 단기 `AGENT_STORE_DEMO_ACCESS_TOKEN`만 사용한다.
- Compose에서는 명시적 `DEMO_AGENT_MODE`와 command flag로 실행하며 fallback을 추가하지 않는다.
- catalog schema는 input object root, output format, 64 KiB, 깊이 32, local fragment `$ref`만 허용한다.

```powershell
gofmt -w .
go test ./...
go vet ./...
go build ./...
```

2026-08-27 검증에서 로컬 Go 대신 `golang:1.26` 컨테이너로 `go test ./...`, `go vet ./...`,
`go build ./...`를 통과했다. `DEMO_AGENT_MODE=fixture` Compose health와 catalog bootstrap 최초·재실행도
통과했다. 변경 후 `git diff --check`와 Spring/FE 계약 parity를 확인한다. secret, `.env`, `.idea`는 커밋하지 않는다.
