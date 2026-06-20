//go:build walletcore

package wallet

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func newWCSigner(t *testing.T) Signer {
	t.Helper()
	s, err := newWalletCoreSigner(map[string]string{"ethereum": testMnemonic})
	if err != nil {
		t.Fatalf("newWalletCoreSigner: %v", err)
	}
	return s
}

// wallet-core должен деривить ровно те же hardhat-адреса, что и native-реализация:
// это и есть гарантия, что контракт с Laravel/индексером не поедет при смене signer'а.
func TestWalletCore_DeriveMatchesVectors(t *testing.T) {
	want := []string{
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		"0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		"0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC",
	}

	s := newWCSigner(t)
	for i, exp := range want {
		got, err := s.DeriveAddress("ethereum", Path{Account: 0, Change: 0, AddressIndex: uint32(i)})
		if err != nil {
			t.Fatalf("derive index %d: %v", i, err)
		}
		if got != exp {
			t.Errorf("index %d: got %s, want %s", i, got, exp)
		}
	}
}

func TestWalletCore_ValidateAddress(t *testing.T) {
	s := newWCSigner(t)
	cases := []struct {
		addr string
		want bool
	}{
		{"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", true},
		{"0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266", true},
		{"0x1234", false},
		{"", false},
	}
	for _, c := range cases {
		got, err := s.ValidateAddress("ethereum", c.addr)
		if err != nil {
			t.Fatalf("ValidateAddress(%q): %v", c.addr, err)
		}
		if got != c.want {
			t.Errorf("ValidateAddress(%q) = %v, want %v", c.addr, got, c.want)
		}
	}
}

func TestWalletCore_SignRecoversToSender(t *testing.T) {
	const chainID = 11155111
	s := newWCSigner(t)

	from, err := s.DeriveAddress("ethereum", Path{})
	if err != nil {
		t.Fatal(err)
	}

	res, err := s.SignTx("ethereum", Path{}, TxParams{
		To:                      "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		ValueWei:                "1000000000000000",
		Data:                    "0x",
		Nonce:                   3,
		ChainID:                 chainID,
		GasLimit:                21000,
		MaxFeePerGasWei:         "30000000000",
		MaxPriorityFeePerGasWei: "1500000000",
	})
	if err != nil {
		t.Fatalf("SignTx: %v", err)
	}

	var tx types.Transaction
	if err := tx.UnmarshalBinary(common.FromHex(res.SignedTx)); err != nil {
		t.Fatalf("decode signed tx: %v", err)
	}
	if tx.Type() != types.DynamicFeeTxType {
		t.Errorf("expected EIP-1559 tx, got type %d", tx.Type())
	}
	if tx.Hash().Hex() != res.TxHash {
		t.Errorf("tx hash mismatch: %s vs %s", tx.Hash().Hex(), res.TxHash)
	}

	recovered, err := types.Sender(types.LatestSignerForChainID(big.NewInt(chainID)), &tx)
	if err != nil {
		t.Fatalf("recover sender: %v", err)
	}
	if recovered.Hex() != from {
		t.Errorf("recovered %s, want %s", recovered.Hex(), from)
	}
}

func TestWalletCore_Erc20Calldata(t *testing.T) {
	s := newWCSigner(t)
	const data = "0xa9059cbb00000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c800000000000000000000000000000000000000000000000000000000000f4240"

	res, err := s.SignTx("ethereum", Path{}, TxParams{
		To:                      "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238",
		ValueWei:                "0",
		Data:                    data,
		Nonce:                   0,
		ChainID:                 11155111,
		GasLimit:                100000,
		MaxFeePerGasWei:         "30000000000",
		MaxPriorityFeePerGasWei: "1500000000",
	})
	if err != nil {
		t.Fatalf("SignTx: %v", err)
	}

	var tx types.Transaction
	if err := tx.UnmarshalBinary(common.FromHex(res.SignedTx)); err != nil {
		t.Fatal(err)
	}
	if "0x"+common.Bytes2Hex(tx.Data()) != data {
		t.Errorf("calldata mismatch: 0x%s", common.Bytes2Hex(tx.Data()))
	}
	if tx.Value().Sign() != 0 {
		t.Errorf("expected zero value for erc20 transfer, got %s", tx.Value())
	}
}

func TestWalletCore_Deterministic(t *testing.T) {
	s := newWCSigner(t)
	p := TxParams{
		To:                      "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		ValueWei:                "1",
		Data:                    "0x",
		Nonce:                   1,
		ChainID:                 11155111,
		GasLimit:                21000,
		MaxFeePerGasWei:         "20000000000",
		MaxPriorityFeePerGasWei: "1000000000",
	}
	a, err := s.SignTx("ethereum", Path{}, p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.SignTx("ethereum", Path{}, p)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("signing not deterministic:\n%+v\n%+v", a, b)
	}
}

// Главная кросс-проверка: wallet-core и go-ethereum при одинаковых входных данных
// должны давать байт-в-байт одинаковую подпись. Обе используют детерминированный
// ECDSA (RFC6979, low-S) и стандартный RLP EIP-1559, так что signed_tx и tx_hash
// обязаны совпасть. Две независимые реализации проверяют друг друга.
func TestWalletCore_MatchesNative(t *testing.T) {
	wc := newWCSigner(t)
	native, err := NewEthSigner(map[string]string{"ethereum": testMnemonic})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		p    Path
		tx   TxParams
	}{
		{
			name: "native transfer",
			tx: TxParams{
				To:                      "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
				ValueWei:                "1000000000000000",
				Data:                    "0x",
				Nonce:                   7,
				ChainID:                 11155111,
				GasLimit:                21000,
				MaxFeePerGasWei:         "30000000000",
				MaxPriorityFeePerGasWei: "1500000000",
			},
		},
		{
			name: "erc20 transfer",
			p:    Path{Account: 0, Change: 0, AddressIndex: 5},
			tx: TxParams{
				To:                      "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238",
				ValueWei:                "0",
				Data:                    "0xa9059cbb00000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c800000000000000000000000000000000000000000000000000000000000f4240",
				Nonce:                   42,
				ChainID:                 11155111,
				GasLimit:                90000,
				MaxFeePerGasWei:         "25000000000",
				MaxPriorityFeePerGasWei: "2000000000",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := wc.SignTx("ethereum", c.p, c.tx)
			if err != nil {
				t.Fatalf("wallet-core SignTx: %v", err)
			}
			exp, err := native.SignTx("ethereum", c.p, c.tx)
			if err != nil {
				t.Fatalf("native SignTx: %v", err)
			}
			if got.SignedTx != exp.SignedTx {
				t.Errorf("signed_tx mismatch:\nwallet-core: %s\ngo-ethereum: %s", got.SignedTx, exp.SignedTx)
			}
			if got.TxHash != exp.TxHash {
				t.Errorf("tx_hash mismatch: %s vs %s", got.TxHash, exp.TxHash)
			}
		})
	}
}
