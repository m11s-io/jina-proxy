# jina-proxy

`jina-proxy` exposes Jina's `/v1/rerank` HTTP contract and adapts text
requests for Qwen3-Reranker running in OpenVINO Model Server (OVMS). It exists
so clients such as LibreChat can use a self-hosted Qwen reranker through their
existing Jina integration.

The complete upstream Jina OpenAPI 3.1 document is pinned at
[`api/jina.openapi.json`](api/jina.openapi.json). `oapi-codegen` generates only
the `rerank_v1_rerank_post` operation; other Jina endpoints and multimodal
reranking are intentionally not implemented.

## API

```http
POST /v1/rerank
Content-Type: application/json
Authorization: Bearer any-non-empty-placeholder

{
  "model": "jina-reranker-v2-base-multilingual",
  "query": "What is OpenVINO?",
  "documents": ["OpenVINO is an inference toolkit.", "Unrelated text."],
  "top_n": 1,
  "return_documents": true
}
```

The proxy does not validate or forward the bearer token. Restrict the Service
to trusted cluster clients. Query and document text are never logged.

Health endpoints are `GET /healthz` and `GET /readyz`.

## Configuration

| Variable | Default |
| --- | --- |
| `LISTEN_ADDR` | `:8080` |
| `UPSTREAM_URL` | `http://127.0.0.1:8000/v3/rerank` |
| `UPSTREAM_MODEL` | `OpenVINO/Qwen3-Reranker-0.6B-seq-cls-fp16-ov` |
| `RERANK_INSTRUCTION` | `Given a web search query, retrieve relevant passages that answer the query` |
| `UPSTREAM_TIMEOUT` | `30s` |
| `MAX_REQUEST_BYTES` | `1048576` |
| `MAX_DOCUMENTS` | `100` |
| `MAX_DOCUMENT_BYTES` | `65536` |

## Development

Requires Go 1.27.

```sh
go generate ./...
go test -race ./...
go vet ./...
docker build -t jina-proxy:test .
```

The generated source is committed. When updating Jina's contract, replace the
pinned document from `https://api.jina.ai/openapi.json`, run `go generate
./...`, and review both the specification and generated diff.

## Plugins

Select `MODEL_FORMATTER=qwen3` and `SERVING_BACKEND=ovms` (the defaults). These are the only implementations currently supported. Unknown or explicitly empty selectors fail startup. Formatters own model input formatting; backends own inference transport and readiness.
