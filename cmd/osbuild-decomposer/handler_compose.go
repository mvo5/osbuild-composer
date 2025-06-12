package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
)

// test for real via:
// curl --unix-socket /run/osbuild-decomposer-httpd.sock -d '{"distribution":"centos-9","image_request":{"image_type":"qcow2"}}' -H "Content-Type: application/json"  -X POST http://localhost/api/image-builder-composer/v2/compose
func handleCompose(state *State) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			slog.Debug("handlerBuild called on", "url", r.URL.Path)

			if r.Method != http.MethodPost {
				http.Error(w, "compose endpoint only supports POST", http.StatusMethodNotAllowed)
				return
			}

			var compReq v2.ComposeRequest
			if err := json.NewDecoder(r.Body).Decode(&compReq); err != nil {
				http.Error(w, fmt.Sprintf("cannot decode compose request: %v", err), http.StatusBadRequest)
				return
			}
			// add to the build queue
			buildReq := BuildRequest{
				Id:  uuid.New(),
				Req: &compReq,
			}
			state.AddBuildRequest(r.Context(), buildReq)

			w.WriteHeader(http.StatusCreated)
			compId := v2.ComposeId{
				Id:   buildReq.Id,
				Kind: "ComposeID",
			}
			if err := json.NewEncoder(w).Encode(compId); err != nil {
				slog.Error("cannot encode compose reply", "error", err)
				return
			}
		},
	)
}

// test for real via:
// sudo curl --unix-socket /run/osbuild-decomposer-httpd.sock  http://localhost/api/image-builder-composer/v2/composes/873d6b4d-5b38-4a69-93a2-be0fe1a6f3da
func handleComposeStatus(state *State) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			slog.Debug("handler composeStatus called", "url", r.URL.Path)
			if r.Method != http.MethodGet {
				http.Error(w, "result endpoint only supports Get", http.StatusMethodNotAllowed)
				return
			}
			id := r.PathValue("id")
			status := state.BuildStatus(r.Context(), id)
			if err := json.NewEncoder(w).Encode(status); err != nil {
				slog.Error("cannot encode compose status", "id", id, "error", err)
				return
			}

		},
	)
}

// test for real via:
// sudo curl --unix-socket /run/osbuild-decomposer-httpd.sock  http://localhost/api/image-builder-composer/v2/composes
func handleComposes(state *State) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			slog.Debug("handler composes called", "url", r.URL.Path)
			if r.Method != http.MethodGet {
				http.Error(w, "result endpoint only supports Get", http.StatusMethodNotAllowed)
				return
			}
			builds := state.Builds(r.Context())
			if err := json.NewEncoder(w).Encode(builds); err != nil {
				slog.Error("cannot encode composes", "error", err)
				return
			}

		},
	)
}
