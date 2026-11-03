package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/m11s-io/jina-proxy/internal/api"
	"github.com/m11s-io/jina-proxy/internal/backend"
	"github.com/m11s-io/jina-proxy/internal/config"
	"github.com/m11s-io/jina-proxy/internal/formatter"
)

type Handler struct {
	cfg       config.Config
	backend   backend.Backend
	formatter formatter.Formatter
}

func New(cfg config.Config, f formatter.Formatter, b backend.Backend) http.Handler {
	h := &Handler{cfg: cfg, backend: b, formatter: f}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", ok)
	mux.HandleFunc("GET /readyz", h.ready)
	return api.HandlerFromMux(h, mux)
}
func ok(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if err := h.backend.Ready(r.Context()); err != nil {
		writeError(w, 503, "upstream unavailable")
		return
	}
	ok(w, r)
}

// RerankV1RerankPost implements the operation generated from Jina's OpenAPI specification.
func (h *Handler) RerankV1RerankPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxRequestBytes)
	var body api.RerankV1RerankPostJSONBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	value, err := body.ValueByDiscriminator()
	if err != nil {
		writeError(w, 400, "unsupported reranker model")
		return
	}
	in, ok := value.(api.TextRerankerRequest)
	if !ok {
		writeError(w, 400, "only text reranking is supported")
		return
	}
	documents, err := textDocuments(in.Documents)
	if err != nil || in.Query == "" || len(documents) == 0 || len(documents) > h.cfg.MaxDocuments {
		writeError(w, 400, "invalid query or documents")
		return
	}
	for _, d := range documents {
		if d == "" || len([]byte(d)) > h.cfg.MaxDocumentBytes {
			writeError(w, 400, "document is empty or too large")
			return
		}
	}
	if in.TopN != nil && (*in.TopN <= 0 || *in.TopN > len(documents)) {
		writeError(w, 400, "top_n is out of range")
		return
	}

	out := backend.Request{Model: h.cfg.UpstreamModel, Query: h.formatter.Query(h.cfg.Instruction, in.Query), TopN: in.TopN, ReturnDocuments: false, Documents: make([]string, len(documents))}
	for i, d := range documents {
		out.Documents[i] = h.formatter.Document(d)
	}
	resultsUpstream, err := h.backend.Rerank(r.Context(), out)
	if err != nil {
		message := "upstream unavailable"
		if errors.Is(err, backend.ErrStatus) {
			message = "upstream error"
		} else if errors.Is(err, backend.ErrResponse) {
			message = "invalid upstream response"
		}
		writeError(w, 502, message)
		return
	}

	includeDocs := in.ReturnDocuments == nil || *in.ReturnDocuments
	results := make([]api.RerankingResult, len(resultsUpstream))
	for i, item := range resultsUpstream {
		if item.Index < 0 || item.Index >= len(documents) {
			writeError(w, 502, "invalid upstream result index")
			return
		}
		results[i] = api.RerankingResult{Index: item.Index, RelevanceScore: float32(item.RelevanceScore)}
		if includeDocs {
			doc := api.RerankingResult_Document{}
			_ = doc.FromTextDoc(api.TextDoc{Text: documents[item.Index]})
			results[i].Document = &doc
		}
	}
	writeJSON(w, 200, api.RerankingResponse{Model: string(in.Model), Usage: api.BaseUsage{TotalTokens: 0}, Results: results})
}

func textDocuments(items []api.TextRerankerRequest_Documents_Item) ([]string, error) {
	out := make([]string, len(items))
	for i, item := range items {
		if value, err := item.AsTextRerankerRequestDocuments0(); err == nil {
			out[i] = value
			continue
		}
		if value, err := item.AsTextDoc(); err == nil {
			out[i] = value.Text
			continue
		}
		return nil, io.ErrUnexpectedEOF
	}
	return out, nil
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, api.ErrorResponse{Detail: message})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
