package wallet

import "fmt"

// Доступные реализации Signer.
const (
	SignerWalletCore = "walletcore" // боевая, поверх trustwallet/wallet-core (cgo)
	SignerNative     = "native"     // запасная, чистый Go на go-ethereum (без cgo)
)

// NewSigner собирает Signer выбранной реализации.
//
// walletcore - основная реализация: подпись и деривация идут через нативную
// библиотеку trustwallet/wallet-core. Она компилируется только при сборке с
// -tags walletcore (см. Dockerfile); в обычной сборке без тега вернётся ошибка.
//
// native - запасная чистая Go-реализация (go-ethereum), собирается без cgo и
// удобна для локального запуска/тестов.
func NewSigner(kind string, mnemonics map[string]string) (Signer, error) {
	switch kind {
	case SignerWalletCore:
		return newWalletCoreSigner(mnemonics)
	case SignerNative:
		return NewEthSigner(mnemonics)
	default:
		return nil, fmt.Errorf("unknown signer %q (want %q or %q)", kind, SignerWalletCore, SignerNative)
	}
}
