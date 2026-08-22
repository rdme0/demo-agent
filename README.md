# demo-agent

AgentStore 기술 시연용 독립 Go resource server입니다. Spring 백엔드나 PostgreSQL에 접근하지 않으며, x402 결제 검증·정산은 공식 Go SDK와 configured facilitator가 처리합니다.

## 실행

`.env.example`을 `.env`로 복사해 모든 필수 값을 명시합니다. 프로그램은 환경변수에 기본값을 두지 않습니다.

```powershell
Copy-Item .env.example .env
Get-Content .env | Where-Object { $_ -match '^[A-Z0-9_]+=' } | ForEach-Object {
    $name, $value = $_ -split '=', 2
    Set-Item -Path "Env:$name" -Value $value
}
go run .
```

`DEMO_PAYMENT_MODE=simulated`는 fixture 응답만 반환합니다. `x402` mode에서는 `X402_FACILITATOR_URL`과 investment, financial, news, risk의 atomic price/payTo가 모두 필요합니다. 선택 asset을 설정하면 공식 Base Sepolia USDC `0x036CbD53842c5426634e7929541eC2318f3dCF7e`만 허용합니다.

## HTTP 계약

- `GET /health` → `{ "status": "ok" }`
- `POST /agents/:agent/invoke` → fixture 또는 등록 agent의 output과 dependencyResults

현재 fixture agent는 `internal/agent`의 `Agent` interface로 등록됩니다. 실제 LLM 연결은 같은 interface를 구현한 agent를 등록하면 되므로 HTTP, x402, callback 안전 경계를 변경하지 않습니다.

runtime callback은 HTTP loopback만 허용하고, localhost를 IPv4 loopback으로 고정 연결합니다. redirect, 1 MiB를 넘는 request/response, 30초 초과 callback은 거절합니다.

## 검증

```powershell
gofmt -w .
go test ./...
go vet ./...
go build ./...
```
