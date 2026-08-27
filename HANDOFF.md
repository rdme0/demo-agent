# Demo Agent 인수인계서

최종 갱신: 2026-08-27

## 역할과 실행

- Go/Gin 기반 독립 x402 resource server다. Spring DB를 직접 읽거나 쓰지 않는다.
- `catalog/agents.yaml`이 demo Agent, Function Contract, 가격, 결제 지갑, dependency와 runtime fixture의
  단일 원본이다.
- `cmd/demo-agent`는 `--config`, `--host`, `--port`, `--mode`를 명시적으로 받아 실행한다. `fixture`는
  비용 없는 결정적 시연용이고 `openai`는 `OPEN_AI_KEY`가 필요하다.
- `cmd/catalog-bootstrap`은 Spring API에 Function Contract와 Agent manifest를 등록하고 ACTIVE Version을
  publish한다. 기존 데이터가 다르면 drift로 중단하며 ACTIVE 데이터를 덮어쓰지 않는다.

## 구조

- `cmd`: 서버와 bootstrap 실행 진입점
- `internal/app`: 의존성 조립
- `internal/agent`: controller, DTO, model, registry와 fixture/OpenAI service
- `internal/runtime/client`: loopback/exact-origin callback, redirect 차단, 1 MiB와 30초 제한
- `internal/payment/x402`: Base Sepolia USDC exact middleware
- `catalog`: embedded YAML 로더와 schema/결제/strategy 검증

Root Agent 구현만 dependency resolver를 호출한다. specialist는 callback을 호출하지 않는다. callback은
원 invocation Authorization을 전달하고 새 Idempotency-Key를 생성한다.

## 설정과 검증

- 공개 host/port·fixture 기본값은 `config/application.yaml`에 둔다. 비밀값은 `.env`의 `OPEN_AI_KEY`만
  사용한다.
- Compose에서는 명시적 `DEMO_AGENT_MODE`와 command flag로 실행하며 fallback을 추가하지 않는다.
- catalog schema는 input object root, output format, 64 KiB, 깊이 32, local fragment `$ref`만 허용한다.

```powershell
gofmt -w .
go test ./...
go vet ./...
go build ./...
```

변경 후 `git diff --check`와 Spring/FE 계약 parity를 확인한다. secret, `.env`, `.idea`는 커밋하지 않는다.
