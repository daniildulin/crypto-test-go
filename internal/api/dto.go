package api

import "github.com/daniildulin/crypto-test/internal/wallet"

type createAddressRequest struct {
	Gate         string `json:"gate"`
	Account      uint32 `json:"account"`
	Change       uint32 `json:"change"`
	AddressIndex uint32 `json:"address_index"`
}

func (r createAddressRequest) path() wallet.Path {
	return wallet.Path{Account: r.Account, Change: r.Change, AddressIndex: r.AddressIndex}
}

type createAddressResponse struct {
	Address string `json:"address"`
}

type validateAddressRequest struct {
	Gate    string `json:"gate"`
	Address string `json:"address"`
}

type validateAddressResponse struct {
	Valid bool `json:"valid"`
}

type txParams struct {
	To                      string `json:"to"`
	ValueWei                string `json:"value_wei"`
	Data                    string `json:"data"`
	Nonce                   uint64 `json:"nonce"`
	ChainID                 uint64 `json:"chain_id"`
	GasLimit                uint64 `json:"gas_limit"`
	MaxFeePerGasWei         string `json:"max_fee_per_gas_wei"`
	MaxPriorityFeePerGasWei string `json:"max_priority_fee_per_gas_wei"`
}

type txRequest struct {
	Gate         string   `json:"gate"`
	Account      uint32   `json:"account"`
	Change       uint32   `json:"change"`
	AddressIndex uint32   `json:"address_index"`
	TxParams     txParams `json:"tx_params"`
}

func (r txRequest) path() wallet.Path {
	return wallet.Path{Account: r.Account, Change: r.Change, AddressIndex: r.AddressIndex}
}

func (r txRequest) walletParams() wallet.TxParams {
	return wallet.TxParams{
		To:                      r.TxParams.To,
		ValueWei:                r.TxParams.ValueWei,
		Data:                    r.TxParams.Data,
		Nonce:                   r.TxParams.Nonce,
		ChainID:                 r.TxParams.ChainID,
		GasLimit:                r.TxParams.GasLimit,
		MaxFeePerGasWei:         r.TxParams.MaxFeePerGasWei,
		MaxPriorityFeePerGasWei: r.TxParams.MaxPriorityFeePerGasWei,
	}
}

type txResponse struct {
	TxHash   string `json:"tx_hash"`
	SignedTx string `json:"signed_tx"`
}

type errorResponse struct {
	Error string `json:"error"`
}
