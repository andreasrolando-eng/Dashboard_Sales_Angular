package handler

import "net/http"

// Healthz reports the process is up. Used by deploy.sh and docker compose
// healthchecks — keep it dependency-free (no DB ping) so it reflects
// process liveness, not downstream state.
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
