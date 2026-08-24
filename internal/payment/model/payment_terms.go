package model

const (
	BaseSepoliaNetwork = "eip155:84532"
	BaseSepoliaUSDC    = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"
)

type PaymentTerms struct {
	AmountAtomic string
	Asset        string
	PayTo        string
}
