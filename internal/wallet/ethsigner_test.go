package wallet

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// публично известный тестовый mnemonic (дефолт hardhat/anvil) - НЕ для реальных средств
const testMnemonic = "test test test test test test test test test test test junk"

func newSigner(t *testing.T) *EthSigner {
	t.Helper()
	s, err := NewEthSigner(map[string]string{"ethereum": testMnemonic})
	if err != nil {
		t.Fatalf("NewEthSigner: %v", err)
	}
	return s
}

func TestDeriveAddress_HardhatVectors(t *testing.T) {
	// m/44'/60'/0'/0/i для hardhat mnemonic - эти аккаунты общеизвестны
	want := []string{
		"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		"0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		"0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC",
	}

	s := newSigner(t)
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

func TestDeriveAddress_UnknownGate(t *testing.T) {
	if _, err := newSigner(t).DeriveAddress("nope", Path{}); err == nil {
		t.Fatal("expected error for unknown gate")
	}
}

func TestValidateAddress(t *testing.T) {
	s := newSigner(t)
	cases := []struct {
		name string
		addr string
		want bool
	}{
		{"checksummed", "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", true},
		{"lowercase", "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266", true},
		{"wrong checksum", "0xF39Fd6e51aad88F6F4ce6aB8827279cffFb92266", false},
		{"too short", "0x1234", false},
		{"not hex", "0xZZZZd6e51aad88F6F4ce6aB8827279cffFb92266", false},
		{"empty", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := s.ValidateAddress("ethereum", c.addr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("ValidateAddress(%q) = %v, want %v", c.addr, got, c.want)
			}
		})
	}
}

func TestSignTx_RecoversToSender(t *testing.T) {
	const chainID = 11155111
	s := newSigner(t)

	from, err := s.DeriveAddress("ethereum", Path{})
	if err != nil {
		t.Fatal(err)
	}

	res, err := s.SignTx("ethereum", Path{}, TxParams{
		To:                      "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		ValueWei:                "1000000000000000", // 0.001 ETH
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

	raw := common.FromHex(res.SignedTx)
	var tx types.Transaction
	if err := tx.UnmarshalBinary(raw); err != nil {
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

func TestSignTx_Erc20Calldata(t *testing.T) {
	s := newSigner(t)
	// transfer(0x7099..., 1000000)
	data := "0xa9059cbb00000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c800000000000000000000000000000000000000000000000000000000000f4240"

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
	if common.Bytes2Hex(tx.Data()) != "a9059cbb00000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c800000000000000000000000000000000000000000000000000000000000f4240" {
		t.Errorf("calldata mismatch: %s", common.Bytes2Hex(tx.Data()))
	}
	if tx.Value().Sign() != 0 {
		t.Errorf("expected zero value for erc20 transfer, got %s", tx.Value())
	}
}

func TestSignTx_Deterministic(t *testing.T) {
	s := newSigner(t)
	params := TxParams{
		To:                      "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		ValueWei:                "1",
		Data:                    "0x",
		Nonce:                   1,
		ChainID:                 11155111,
		GasLimit:                21000,
		MaxFeePerGasWei:         "20000000000",
		MaxPriorityFeePerGasWei: "1000000000",
	}

	a, err := s.SignTx("ethereum", Path{}, params)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.SignTx("ethereum", Path{}, params)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("signing not deterministic:\n%+v\n%+v", a, b)
	}
}

func TestSignTx_InvalidInputs(t *testing.T) {
	s := newSigner(t)
	base := TxParams{
		To:                      "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		ValueWei:                "1",
		Data:                    "0x",
		ChainID:                 11155111,
		GasLimit:                21000,
		MaxFeePerGasWei:         "20000000000",
		MaxPriorityFeePerGasWei: "1000000000",
	}

	mutate := map[string]func(*TxParams){
		"bad value":   func(p *TxParams) { p.ValueWei = "abc" },
		"neg value":   func(p *TxParams) { p.ValueWei = "-1" },
		"bad to":      func(p *TxParams) { p.To = "0x123" },
		"bad data":    func(p *TxParams) { p.Data = "zz" },
		"no chain id": func(p *TxParams) { p.ChainID = 0 },
		"empty fee":   func(p *TxParams) { p.MaxFeePerGasWei = "" },
	}

	for name, m := range mutate {
		t.Run(name, func(t *testing.T) {
			p := base
			m(&p)
			if _, err := s.SignTx("ethereum", Path{}, p); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
