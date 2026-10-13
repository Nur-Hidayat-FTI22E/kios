package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errResp(w http.ResponseWriter, status int, kode, pesan string, detail map[string]interface{}) {
	if detail == nil {
		detail = map[string]interface{}{}
	}
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{"kode": kode, "pesan": pesan, "detail": detail},
	})
}

func okResp(w http.ResponseWriter, v interface{}) {
	writeJSON(w, http.StatusOK, v)
}

func decode(r *http.Request, v interface{}) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 2<<20)).Decode(v)
}

func pageNum(r *http.Request) (page, per int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	per, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	if per < 1 || per > 200 {
		per = 25
	}
	return page, per
}

func paginate(w http.ResponseWriter, r *http.Request, total int, data interface{}) {
	page, per := pageNum(r)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": data,
		"meta": map[string]int{"total": total, "page": page, "per_page": per},
	})
}

func idFrom(r *http.Request) (int64, bool) {
	v := r.PathValue("id") // chi pakai URLParam
	if v == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
