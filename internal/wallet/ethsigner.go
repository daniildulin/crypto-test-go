package wallet

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/tyler-smith/go-bip32"
)

// EthSigner - чистый Go (без cgo) Signer на базе go-ethereum.
type EthSigner struct {
	// gate -> bip32 master key. Деривация seed'а - дорогой шаг, поэтому
	// делаем её один раз на gate при старте.
	masters map[string]*bip32.Key
}

// NewEthSigner строит master-ключи для каждого gate. Падает сразу на плохом mnemonic.
func NewEthSigner(mnemonics map[string]string) (*EthSigner, error) {
	masters := make(map[string]*bip32.Key, len(mnemonics))
	for name, mnemonic := range mnemonics {
		master, err := masterKeyFromMnemonic(mnemonic)
		if err != nil {
			return nil, fmt.Errorf("gate %q: %w", name, err)
		}
		masters[name] = master
	}

	return &EthSigner{masters: masters}, nil
}

func (s *EthSigner) DeriveAddress(gate string, p Path) (string, error) {
	priv, err := s.privateKey(gate, p)
	if err != nil {
		return "", err
	}

	return crypto.PubkeyToAddress(priv.PublicKey).Hex(), nil
}

func (s *EthSigner) ValidateAddress(gate, address string) (bool, error) {
	if _, ok := s.masters[gate]; !ok {
		return false, fmt.Errorf("unknown gate %q", gate)
	}

	return isValidEthAddress(address), nil
}

func (s *EthSigner) SignTx(gate string, p Path, tp TxParams) (SignedTx, error) {
	priv, err := s.privateKey(gate, p)
	if err != nil {
		return SignedTx{}, err
	}

	value, err := parseWei(tp.ValueWei, "value_wei")
	if err != nil {
		return SignedTx{}, err
	}
	tip, err := parseWei(tp.MaxPriorityFeePerGasWei, "max_priority_fee_per_gas_wei")
	if err != nil {
		return SignedTx{}, err
	}
	feeCap, err := parseWei(tp.MaxFeePerGasWei, "max_fee_per_gas_wei")
	if err != nil {
		return SignedTx{}, err
	}
	if !common.IsHexAddress(tp.To) {
		return SignedTx{}, fmt.Errorf("invalid to address %q", tp.To)
	}
	data, err := decodeData(tp.Data)
	if err != nil {
		return SignedTx{}, err
	}
	if tp.ChainID == 0 {
		return SignedTx{}, fmt.Errorf("chain_id is required")
	}

	to := common.HexToAddress(tp.To)
	chainID := new(big.Int).SetUint64(tp.ChainID)

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     tp.Nonce,
		GasTipCap: tip,
		GasFeeCap: feeCap,
		Gas:       tp.GasLimit,
		To:        &to,
		Value:     value,
		Data:      data,
	})

	signed, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), priv)
	if err != nil {
		return SignedTx{}, fmt.Errorf("sign tx: %w", err)
	}

	raw, err := signed.MarshalBinary()
	if err != nil {
		return SignedTx{}, fmt.Errorf("encode tx: %w", err)
	}

	return SignedTx{
		TxHash:   signed.Hash().Hex(),
		SignedTx: hexutil.Encode(raw),
	}, nil
}

func (s *EthSigner) privateKey(gate string, p Path) (*ecdsa.PrivateKey, error) {
	master, ok := s.masters[gate]
	if !ok {
		return nil, fmt.Errorf("unknown gate %q", gate)
	}

	keyBytes, err := derivePrivateKey(master, p)
	if err != nil {
		return nil, err
	}

	return crypto.ToECDSA(keyBytes)
}

func parseWei(s, field string) (*big.Int, error) {
	if s == "" {
		return nil, fmt.Errorf("%s is required", field)
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || v.Sign() < 0 {
		return nil, fmt.Errorf("invalid %s: %q", field, s)
	}
	return v, nil
}

func decodeData(s string) ([]byte, error) {
	if s == "" || strings.EqualFold(s, "0x") {
		return []byte{}, nil
	}
	b, err := hexutil.Decode(s)
	if err != nil {
		return nil, fmt.Errorf("invalid data: %w", err)
	}
	return b, nil
}

// isValidEthAddress проверяет формат и, для mixed-case, EIP-55 checksum.
func isValidEthAddress(address string) bool {
	if !common.IsHexAddress(address) {
		return false
	}

	body := strings.TrimPrefix(strings.TrimPrefix(address, "0x"), "0X")
	mixedCase := body != strings.ToLower(body) && body != strings.ToUpper(body)
	if !mixedCase {
		return true
	}

	withPrefix := address
	if !strings.HasPrefix(address, "0x") && !strings.HasPrefix(address, "0X") {
		withPrefix = "0x" + address
	}

	return common.HexToAddress(address).Hex() == withPrefix
}
