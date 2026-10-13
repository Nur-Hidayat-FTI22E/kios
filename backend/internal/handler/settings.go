package handler

import (
	"net/http"
)

type settingsPayload struct {
	IzinStokNegatif string `json:"izin_stok_negatif"`
	ZonaWaktu       string `json:"zona_waktu"`
	BatasDiskonPct  string `json:"batas_diskon_pct"`
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	resp := settingsPayload{
		IzinStokNegatif: GetSetting(h.db, "izin_stok_negatif", "0"),
		ZonaWaktu:       GetSetting(h.db, "zona_waktu", h.cfg.APPTimezone),
		BatasDiskonPct:  GetSetting(h.db, "batas_diskon_pct", "100"),
	}
	okResp(w, resp)
}

func (h *Handler) PutSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsPayload
	if err := decode(r, &req); err != nil {
		errResp(w, 400, "VALIDASI", "Data tidak valid", nil)
		return
	}
	if req.IzinStokNegatif != "" {
		setSetting(h.db, "izin_stok_negatif", req.IzinStokNegatif)
	}
	if req.ZonaWaktu != "" {
		setSetting(h.db, "zona_waktu", req.ZonaWaktu)
	}
	if req.BatasDiskonPct != "" {
		setSetting(h.db, "batas_diskon_pct", req.BatasDiskonPct)
	}
	okResp(w, map[string]string{"status": "ok"})
}
