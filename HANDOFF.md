# Demo Agent 인수인계서

최종 갱신: 2026-09-06 — provider readiness 제거와 catalog 즉시 공개

## 역할과 실행

- 경로: 이 저장소 루트
- Go/Gin 기반 독립 x402 resource server다. Spring DB를 직접 읽거나 쓰지 않는다.
- `catalog/agents.yaml`이 demo Agent, Function Contract, 가격, 결제 지갑, dependency와 runtime fixture의
  단일 원본이다.
- `cmd/demo-agent`는 `--config`, `--host`, `--port`, `--mode`를 명시적으로 받아 실행한다. `fixture`는
  비용 없는 결정적 시연용이고 `openai`는 `OPEN_AI_KEY`가 필요하다.
- `cmd/catalog-bootstrap`은 Spring API에 Function Contract와 Agent manifest를 등록하고 ACTIVE Version을
  publish한다. 기존 데이터가 다르면 drift로 중단하며 ACTIVE 데이터를 덮어쓰지 않는다.

## Catalog 공개 계약 — 2026-09-06

- catalog에는 `verificationInput`이 없으며 bootstrap은 Function Contract와 manifest import 뒤 DRAFT Version을
  `publish`만 한다. publish는 즉시 ACTIVE가 되며 x402 요청·wallet/facilitator·testnet 결제를 만들지 않는다.
- 재실행은 ACTIVE manifest digest가 같은 경우에만 성공한다. contract, manifest, price, endpoint, dependency drift는
  기존처럼 중단하며 ACTIVE 데이터나 Spring DB를 직접 수정하지 않는다.
- 이 변경 뒤 `go test ./...`, `go vet ./...`, `go build ./...`가 통과했다. Bootstrap 회귀는 local HTTP fixture로
  최초 import/publish, 재실행 idempotency, drift 중단을 확인하며 mock framework를 사용하지 않는다.

## One-click Bearer bootstrap — 2026-09-05

- Cookie/CSRF session은 제거됐다. `cmd/catalog-bootstrap`은 사람이 AgentStore 랜딩에서 한 번 클릭해 발급받은
  6시간 Bearer token을 `AGENT_STORE_DEMO_ACCESS_TOKEN`으로 명시적으로 받아 모든 AgentStore API request에
  `Authorization: Bearer`로 보낸다. token이 없으면 network mutation 전에 실패하며 별도 challenge를 처리하지 않는다.
- Bootstrap unit test는 local HTTP fixture에서 Authorization header를 검증한다. Go test는 mock framework를 사용하지 않는다.

`DEMO_AGENT_MODE`는 `.env`에서 읽어 YAML의 `agent.mode`를 덮어쓴다. `cmd/demo-agent --mode`가 명시되면 CLI 값이
최우선이다. 따라서 OpenAI 데모는 `DEMO_AGENT_MODE=openai`와 `OPEN_AI_KEY`를 설정한 뒤 `--mode fixture` 없이
시작해야 하며, fixture 답변을 원할 때만 `--mode fixture`를 명시한다. 이 precedence는 `internal/config/config_test.go`에서
검증한다.

## HTTP catalog 복구 — 2026-09-05

- 실행 중인 Spring `:8080`과 Go fixture `:8090`에 실제 HTTP 요청을 보내 catalog를 복구했다.
  `AGENT_STORE_DEMO_ACCESS_TOKEN`은 bootstrap 프로세스 환경변수에만 두었고, SQL 직접 삽입이나
  provider readiness 상태 수동 변경은 하지 않았다.
- bootstrap 중 실패해 남은 정확한 DRAFT Agent 행은 로컬 개발 DB에서 사용자의
  명시적 승인 후에만 삭제하고 재실행했다. 기존 Function Contract·실행·결제 데이터는 보존했다.
- `catalog/agents.yaml`의 `price-comparison` 가격 문자열을 YAML이 하나의 `priceText` 값으로
  읽도록 수정했다. 기존의 `₩100` 뒤 줄바꿈 `000:`은 output schema의 unknown property를 만들었다.
- 최종 Spring DB/API 확인: Function Contract 12개, Agent 13개, Version 13개, 모두
  ACTIVE. Marketplace API는 13개 catalog Agent를 반환했다.

## 구조

- `cmd`: 서버와 bootstrap 실행 진입점
- `internal/app`: 의존성 조립
- `internal/agent`: controller, DTO, model, registry와 fixture/OpenAI service
- `internal/runtime/client`: loopback/exact-origin callback, redirect 차단, 1 MiB와 call-path deadline
- `internal/payment/x402`: Base Sepolia USDC exact middleware
- `catalog`: embedded YAML 로더와 schema/결제/strategy 검증

Root Agent 구현만 dependency resolver를 호출한다. specialist는 callback을 호출하지 않는다. callback은
Spring이 보낸 `Authorization: Bearer ...`를 원 invocation header로 전달하고 새 Idempotency-Key를 생성한다.

Root Agent의 independent sibling callback은 병렬 실행한다. EIP-3009 `exact`의 nonce는 요청별 32-byte random
authorization nonce로 replay를 막는 값이며, 같은 payer의 서로 다른 authorization을 직렬화하는 체인 nonce가 아니다.
따라서 hot-wallet payer를 이유로 callback을 순차화하지 않는다. callback client 회귀 테스트는 세 independent
callback의 동시 실행을 검증한다.

노드 하나의 예산은 30초다. 그러나 runtime callback transport는 target call path의 남은 depth를 곱해 계산한다.
따라서 root는 150초, depth 2 callback은 120초, leaf depth 5는 30초다. Spring outbound x402 client도 같은
call-path 계산을 사용하고, Go는 `maxDependencyDepth`가 AgentStore contract의 정확히 5가 아니면 기동을 거부한다.
이보다 짧으면 Spring이 먼저 연결을 닫고 Go callback context가 취소되어 signed payment가
`PAYMENT_RECONCILIATION_REQUIRED`로 남는다. 이 경계는 `PAY-08B`, nested HTTP fixture, config mismatch regression으로 검증한다.

OpenAI mode는 Function Contract JSON Schema를 그대로 전송하지 않는다. OpenAI Structured Outputs가 지원하지 않는
JSON Schema `format` 키(예: `uri`)를 요청 payload에서 재귀적으로 제거하고, 원래 계약 schema는 보존한다.
OpenAI specialist에는 웹 검색과 최소 3개 HTTPS 출처 요구를, root Markdown Agent에는 첫 줄 `# 제목` 요구를
명시적으로 추가한다. 이 최소 출처 수는 `structuredResult`에서도 강제하며, 출처 title의 제어문자·마크다운
구분자는 transport 경계에서 정제한다. 응답이 계약을 만족하지 않으면 결제 후 성공으로 위조하지 않고 기존
실패·reconciliation 상태를 유지한다.

2026-09-05 실제 `:8080` → `:8090` → OpenAI Responses API → Base Sepolia x402 HTTP 실행에서 처음에는
`response_format`의 `format: uri` 때문에 400, 이후에는 출처 누락과 root Markdown 제목 누락이 각각 재현됐다.
Schema 정규화와 명시적 출력 지시를 적용한 뒤 실행 `a43c25f7-321b-4e16-9324-3b9e25e550b5`가
4/4 step `COMPLETED`, actual cost `3400`, 네 payment 모두 `SETTLED` 및 transaction hash로 종료됐다.
중간에 실패한 실행·결제 row는 reconciliation 규칙에 따라 삭제하지 않았다.

이전 OpenAI/callback 변경에 대한 fresh read-only verifier는 `PASS`였다. 이후 parallel callback·depth 기반 timeout
정책의 첫 verifier는 nested transport 30초 cancellation과 Go depth mismatch를 차단 결함으로 지적했다. 보정 뒤
`go test ./...`, `go vet ./...`, `go build ./...`, `gofmt`, `git diff --check`를 통과했으며, 새 fresh verifier와
실제 paid execution은 아직 재실행하지 않았다.

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
