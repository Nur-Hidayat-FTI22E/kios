package handler

import "net/http"

type capturedWriter struct {
	hdr      http.Header
	code     int
	buf      []byte
	captured map[string]interface{}
}

func (cw *capturedWriter) Header() http.Header { return hdrDefault() }
func (cw *capturedWriter) Write(b []byte) (int, error) {
	cw.buf = append(cw.buf, b...)
	return len(b), nil
}
func (cw *capturedWriter) WriteHeader(code int) { cw.code = code }

func hdrDefault() http.Header { return http.Header{} }
