package api

import (
	"encoding/json"
	"net/http"

	"github.com/daniildulin/crypto-test/internal/wallet"
)

// Handler держит HTTP-эндпоинты wallet-сервиса.
type Handler struct {
	signer wallet.Signer
}

func NewHandler(signer wallet.Signer) *Handler {
	return &Handler{signer: signer}
}

func (h *Handler) CreateAddress(w http.ResponseWriter, r *http.Request) {
	var req createAddressRequest
	if !decode(w, r, &req) {
		return
	}

	address, err := h.signer.DeriveAddress(req.Gate, req.path())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, createAddressResponse{Address: address})
}

func (h *Handler) ValidateAddress(w http.ResponseWriter, r *http.Request) {
	var req validateAddressRequest
	if !decode(w, r, &req) {
		return
	}

	valid, err := h.signer.ValidateAddress(req.Gate, req.Address)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, validateAddressResponse{Valid: valid})
}

func (h *Handler) Tx(w http.ResponseWriter, r *http.Request) {
	var req txRequest
	if !decode(w, r, &req) {
		return
	}

	signed, err := h.signer.SignTx(req.Gate, req.path(), req.walletParams())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, txResponse{
		TxHash:   signed.TxHash,
		SignedTx: signed.SignedTx,
	})
}

// decode читает JSON-тело; при ошибке сам пишет error-ответ и
// возвращает false, чтобы вызывающий мог выйти.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
