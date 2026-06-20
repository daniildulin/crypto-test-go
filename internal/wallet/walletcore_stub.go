//go:build !walletcore

package wallet

import "errors"

// newWalletCoreSigner - заглушка для сборок без cgo. Боевой signer на
// trustwallet/wallet-core доступен только при сборке с -tags walletcore
// (нужна нативная библиотека, см. Dockerfile). Здесь же возвращаем понятную
// ошибку, чтобы --signer=walletcore не падал молча в native-сборке.
func newWalletCoreSigner(map[string]string) (Signer, error) {
	return nil, errors.New("wallet-core support not compiled in (build with -tags walletcore)")
}
