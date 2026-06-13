package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daniildulin/crypto-test/internal/wallet"
)

const testMnemonic = "test test test test test test test test test test test junk"

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	signer, err := wallet.NewEthSigner(map[string]string{"ethereum": testMnemonic})
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return NewRouter(NewHandler(signer))
}

func post(t *testing.T, router http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateAddress(t *testing.T) {
	rec := post(t, testRouter(t), "/api/v1/createaddress",
		`{"gate":"ethereum","account":0,"change":0,"address_index":0}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp createAddressResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Address != "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266" {
		t.Errorf("address = %s", resp.Address)
	}
}

func TestValidateAddress(t *testing.T) {
	router := testRouter(t)

	ok := post(t, router, "/api/v1/validateaddress",
		`{"gate":"ethereum","address":"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"}`)
	var okResp validateAddressResponse
	_ = json.Unmarshal(ok.Body.Bytes(), &okResp)
	if !okResp.Valid {
		t.Error("expected valid=true for checksummed address")
	}

	bad := post(t, router, "/api/v1/validateaddress",
		`{"gate":"ethereum","address":"0x1234"}`)
	var badResp validateAddressResponse
	_ = json.Unmarshal(bad.Body.Bytes(), &badResp)
	if badResp.Valid {
		t.Error("expected valid=false for malformed address")
	}
}

func TestTx(t *testing.T) {
	body := `{
		"gate":"ethereum","account":0,"change":0,"address_index":0,
		"tx_params":{
			"to":"0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
			"value_wei":"1000000000000000","data":"0x",
			"nonce":7,"chain_id":11155111,"gas_limit":21000,
			"max_fee_per_gas_wei":"30000000000","max_priority_fee_per_gas_wei":"1500000000"
		}
	}`

	rec := post(t, testRouter(t), "/api/v1/tx", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp txResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp.SignedTx, "0x02") {
		t.Errorf("expected EIP-1559 typed tx (0x02...), got %s", resp.SignedTx)
	}
	if !strings.HasPrefix(resp.TxHash, "0x") || len(resp.TxHash) != 66 {
		t.Errorf("bad tx hash: %s", resp.TxHash)
	}
}

func TestUnknownGate(t *testing.T) {
	rec := post(t, testRouter(t), "/api/v1/createaddress",
		`{"gate":"bitcoin","account":0,"change":0,"address_index":0}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/createaddress", nil)
	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}
