// Package server provides the MCP-compatible HTTP/JSON-RPC 2.0 server
// exposing query_knowledge and submit_knowledge tools.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"knowledge-base/internal/fs"
	"knowledge-base/internal/search"
	"knowledge-base/internal/tag"
)

// --- JSON-RPC 2.0 Types ---

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id"`
}

type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// --- Tool parameter types ---

type queryParams struct {
	Query string `json:"query"`
}

type draftParams struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

// Server is the MCP service layer.
type Server struct {
	store  *fs.Store
	engine *search.Engine
	tagMgr *tag.Manager
	mux    *http.ServeMux
}

// New creates a new server with given store, engine, and tag manager.
func New(store *fs.Store, engine *search.Engine, tagMgr *tag.Manager) *Server {
	s := &Server{
		store:  store,
		engine: engine,
		tagMgr: tagMgr,
		mux:    http.NewServeMux(),
	}

	s.mux.HandleFunc("/query", s.handleQuery)
	s.mux.HandleFunc("/draft", s.handleDraft)
	s.mux.HandleFunc("/drafts", s.handleListDrafts)
	s.mux.HandleFunc("/drafts/approve", s.handleApproveDraft)
	s.mux.HandleFunc("/drafts/reject", s.handleRejectDraft)
	s.mux.HandleFunc("/debug", s.handleDebug)
	s.mux.HandleFunc("/debug/index", s.handleDebugIndex)

	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	s.mux.ServeHTTP(w, r)
}

// --- JSON-RPC handler ---

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, -32600, "method not allowed, use POST")
		return
	}

	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, -32700, "parse error: "+err.Error())
		return
	}

	if req.Method != "query_knowledge" {
		jsonError(w, http.StatusBadRequest, -32601, fmt.Sprintf("method %q not found", req.Method))
		return
	}

	var params queryParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			jsonError(w, http.StatusBadRequest, -32602, "invalid params: "+err.Error())
			return
		}
	}

	if params.Query == "" {
		jsonError(w, http.StatusBadRequest, -32602, "query must not be empty")
		return
	}

	results := s.engine.Search(params.Query)

	// Build Markdown response
	var buf strings.Builder
	if len(results) == 0 {
		buf.WriteString("未找到相关结果。\n")
	} else {
		fmt.Fprintf(&buf, "找到 %d 条相关结果：\n\n", len(results))
		for i, r := range results {
			doc := r.Document
			fmt.Fprintf(&buf, "### %d. %s\n\n", i+1, doc.Title)
			if len(doc.Tags) > 0 {
				buf.WriteString("标签：`" + strings.Join(doc.Tags, "`, `") + "`\n\n")
			}
			// Truncate content to 200 chars
			content := doc.Content
			runes := []rune(content)
			if len(runes) > 200 {
				content = string(runes[:200]) + "..."
			}
			buf.WriteString(content + "\n\n---\n\n")
		}
	}

	jsonResult(w, req.ID, map[string]interface{}{
		"results": resultsToMap(results),
		"markdown": buf.String(),
	})
}

func resultsToMap(results []search.Result) []map[string]interface{} {
	out := make([]map[string]interface{}, len(results))
	for i, r := range results {
		doc := r.Document
		out[i] = map[string]interface{}{
			"title":    doc.Title,
			"content":  truncateContent(doc.Content, 200),
			"tags":     doc.Tags,
			"filepath": doc.FilePath,
			"score":    r.Document.Score,
		}
	}
	return out
}

func truncateContent(content string, maxLen int) string {
	runes := []rune(content)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return content
}

// --- Draft submission ---

func (s *Server) handleDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, -32600, "method not allowed, use POST")
		return
	}

	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, -32700, "parse error: "+err.Error())
		return
	}

	if req.Method != "submit_knowledge" {
		jsonError(w, http.StatusBadRequest, -32601, fmt.Sprintf("method %q not found", req.Method))
		return
	}

	var params draftParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			jsonError(w, http.StatusBadRequest, -32602, "invalid params: "+err.Error())
			return
		}
	}

	if params.Title == "" {
		jsonError(w, http.StatusBadRequest, -32602, "title must not be empty")
		return
	}
	if params.Content == "" {
		jsonError(w, http.StatusBadRequest, -32602, "content must not be empty")
		return
	}

	// Validate tags
	if len(params.Tags) > 0 {
		vr := tag.Validate(params.Tags)
		if !vr.Valid {
			jsonError(w, http.StatusBadRequest, -32602, "tag validation failed: "+strings.Join(vr.Errors, "; "))
			return
		}
	}

	path, err := fs.WriteDraft(s.store.DraftsDir(), params.Title, params.Content, params.Tags)
	if err != nil {
		log.Printf("write draft error: %v", err)
		jsonError(w, http.StatusInternalServerError, -32000, "failed to write draft: "+err.Error())
		return
	}

	jsonResult(w, req.ID, map[string]string{
		"status":   "ok",
		"draft_id": path,
		"message":  "草稿已提交，等待人工审核。",
	})
}

// --- Draft review endpoints (REST-style, not JSON-RPC) ---

func (s *Server) handleListDrafts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, -32600, "method not allowed, use GET")
		return
	}

	drafts, err := s.store.ListDrafts()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, -32000, "list drafts: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"drafts": resultsToSearchResults(drafts),
	})
}

func (s *Server) handleApproveDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, -32600, "method not allowed, use POST")
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		jsonError(w, http.StatusBadRequest, -32602, "path query parameter required")
		return
	}

	doc, err := fs.ParseFile(filePath)
	if err != nil {
		jsonError(w, http.StatusBadRequest, -32602, "parse draft: "+err.Error())
		return
	}

	if err := s.store.ApproveDraft(doc); err != nil {
		jsonError(w, http.StatusInternalServerError, -32000, "approve draft: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "草稿已审核通过并入库。",
	})
}

func (s *Server) handleRejectDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, -32600, "method not allowed, use POST")
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		jsonError(w, http.StatusBadRequest, -32602, "path query parameter required")
		return
	}

	doc, err := fs.ParseFile(filePath)
	if err != nil {
		jsonError(w, http.StatusBadRequest, -32602, "parse draft: "+err.Error())
		return
	}

	if err := s.store.RejectDraft(doc); err != nil {
		jsonError(w, http.StatusInternalServerError, -32000, "reject draft: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "草稿已废弃删除。",
	})
}

// handleDebug dumps engine index for diagnostics.
func (s *Server) handleDebug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, -32600, "method not allowed, use GET")
		return
	}

	docs := s.store.AllDocs()
	type docInfo struct {
		Title   string   `json:"title"`
		Tags    []string `json:"tags"`
		Content string   `json:"content_preview"`
	}
	docInfos := make([]docInfo, len(docs))
	for i, d := range docs {
		preview := d.Content
		runes := []rune(preview)
		if len(runes) > 100 {
			preview = string(runes[:100])
		}
		docInfos[i] = docInfo{Title: d.Title, Tags: d.Tags, Content: preview}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"doc_count": len(docs),
		"docs":      docInfos,
	})
}

// handleDebugIndex dumps the search engine's inverted index keys.
func (s *Server) handleDebugIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, -32600, "method not allowed, use GET")
		return
	}
	keys := s.engine.IndexKeys()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"key_count": len(keys),
		"keys":      keys,
	})
}

func resultsToSearchResults(docs []*fs.Document) []map[string]interface{} {
	out := make([]map[string]interface{}, len(docs))
	for i, d := range docs {
		out[i] = map[string]interface{}{
			"title":    d.Title,
			"content":  truncateContent(d.Content, 200),
			"tags":     d.Tags,
			"filepath": d.FilePath,
			"status":   d.Status,
		}
	}
	return out
}

func jsonError(w http.ResponseWriter, httpStatus, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(rpcResponse{
		JSONRPC: "2.0",
		Error:   &rpcError{Code: code, Message: msg},
	})
}

func jsonResult(w http.ResponseWriter, id interface{}, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(rpcResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
