package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"path/filepath"
	"strings"
	"sync"

	"herdworkbench/internal/storage"
	"herdworkbench/internal/workbench"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MCPTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// CapabilityProvider abstracts Workbench domain operations for the MCP server.
type CapabilityProvider interface {
	ListProjects() (any, error)
	GetProjectSnapshot(projectID string) (any, error)
	GetEnvDrift(projectID string) (any, error)
	AuditProject(projectID string) (any, error)
	GetRoutes(projectID string) (any, error)
	GetServices() (any, error)
	GetErrors(projectID string) (any, error)
	GetErdSchema(projectID string) (any, error)
	ExplainQuery(projectID, query string) (any, error)
	GetArchitectureMap(projectID string) (any, error)
	GetHealthHeartbeat(projectID string) (any, error)
	InspectNPlusOne(projectID string) (any, error)
	GetLogPail(projectID string) (any, error)
}

type Server struct {
	mu       sync.Mutex
	provider CapabilityProvider
	tools    map[string]func(params json.RawMessage) (any, error)
	toolDefs []MCPTool
}

func NewServer() *Server {
	return NewServerWithProvider(NewDefaultCapabilityProvider())
}

func NewServerWithProvider(provider CapabilityProvider) *Server {
	if provider == nil {
		provider = NewDefaultCapabilityProvider()
	}
	s := &Server{
		provider: provider,
		tools:    make(map[string]func(params json.RawMessage) (any, error)),
	}
	s.registerDefaultTools()
	return s
}

func (s *Server) RegisterTool(name, description string, schema interface{}, handler func(params json.RawMessage) (any, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[name] = handler
	s.toolDefs = append(s.toolDefs, MCPTool{
		Name:        name,
		Description: description,
		InputSchema: schema,
	})
}

func (s *Server) registerDefaultTools() {
	s.RegisterTool("list_tools", "List all available Workbench diagnostic tools", map[string]any{"type": "object"}, func(p json.RawMessage) (any, error) {
		return s.toolDefs, nil
	})

	s.RegisterTool("workbench_list_projects", "List all discovered Herd Laravel projects", map[string]any{
		"type": "object",
	}, func(p json.RawMessage) (any, error) {
		return s.provider.ListProjects()
	})

	s.RegisterTool("workbench_get_snapshot", "Get project runtime and configuration snapshot", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or path"},
		},
		"required": []string{"project_id"},
	}, func(p json.RawMessage) (any, error) {
		var args struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return s.provider.GetProjectSnapshot(args.ProjectID)
	})

	s.RegisterTool("workbench_get_env_drift", "Compare project .env against .env.example without exposing sensitive values", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or path"},
		},
		"required": []string{"project_id"},
	}, func(p json.RawMessage) (any, error) {
		var args struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return s.provider.GetEnvDrift(args.ProjectID)
	})

	s.RegisterTool("workbench_audit_project", "Audit project configuration and security rules against Best Practices Doctor guidelines", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or path"},
		},
		"required": []string{"project_id"},
	}, func(p json.RawMessage) (any, error) {
		var args struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return s.provider.AuditProject(args.ProjectID)
	})

	s.RegisterTool("workbench_get_routes", "List all routes discovered from Laravel route:list", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or path"},
		},
		"required": []string{"project_id"},
	}, func(p json.RawMessage) (any, error) {
		var args struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return s.provider.GetRoutes(args.ProjectID)
	})

	s.RegisterTool("workbench_get_services", "List native development services and current statuses", map[string]any{
		"type": "object",
	}, func(p json.RawMessage) (any, error) {
		return s.provider.GetServices()
	})

	s.RegisterTool("workbench_get_errors", "Get recent application error logs with sensitive secrets redacted", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or path"},
		},
		"required": []string{"project_id"},
	}, func(p json.RawMessage) (any, error) {
		var args struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return s.provider.GetErrors(args.ProjectID)
	})

	s.RegisterTool("workbench_get_erd_schema", "Extract Eloquent model schema and entity-relationship connections", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or path"},
		},
		"required": []string{"project_id"},
	}, func(p json.RawMessage) (any, error) {
		var args struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return s.provider.GetErdSchema(args.ProjectID)
	})

	s.RegisterTool("workbench_explain_query", "Run SQL EXPLAIN query plan optimizer and index diagnostics", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or path"},
			"query":      map[string]any{"type": "string", "description": "SQL query to explain"},
		},
		"required": []string{"project_id", "query"},
	}, func(p json.RawMessage) (any, error) {
		var args struct {
			ProjectID string `json:"project_id"`
			Query     string `json:"query"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return s.provider.ExplainQuery(args.ProjectID, args.Query)
	})

	s.RegisterTool("workbench_get_architecture_map", "Get 4-tier Living Architecture Canvas and Boundary Linter report", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or path"},
		},
		"required": []string{"project_id"},
	}, func(p json.RawMessage) (any, error) {
		var args struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return s.provider.GetArchitectureMap(args.ProjectID)
	})

	s.RegisterTool("workbench_get_health_heartbeat", "Probe Laravel storage, database PDO boot latency, and system heartbeat", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or path"},
		},
		"required": []string{"project_id"},
	}, func(p json.RawMessage) (any, error) {
		var args struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return s.provider.GetHealthHeartbeat(args.ProjectID)
	})

	s.RegisterTool("workbench_inspect_nplusone", "Detect N+1 query loops and duplicate query execution patterns", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or path"},
		},
		"required": []string{"project_id"},
	}, func(p json.RawMessage) (any, error) {
		var args struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return s.provider.InspectNPlusOne(args.ProjectID)
	})

	s.RegisterTool("workbench_get_log_pail", "Parse recent exceptions grouped with deduplication fingerprint and AI prompt", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id": map[string]any{"type": "string", "description": "Project ID or path"},
		},
		"required": []string{"project_id"},
	}, func(p json.RawMessage) (any, error) {
		var args struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.Unmarshal(p, &args); err != nil {
			return nil, err
		}
		return s.provider.GetLogPail(args.ProjectID)
	})
}

func (s *Server) HandleRequest(req JSONRPCRequest) JSONRPCResponse {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	if req.JSONRPC != "2.0" || req.Method == "" {
		resp.Error = &RPCError{Code: -32600, Message: "Invalid Request"}
		return resp
	}
	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if json.Unmarshal(req.Params, &params) != nil || params.ProtocolVersion == "" {
			resp.Error = &RPCError{Code: -32602, Message: "protocolVersion is required"}
			return resp
		}
		version := "2025-11-25"
		switch params.ProtocolVersion {
		case "2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25":
			version = params.ProtocolVersion
		}
		resp.Result = map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "herd-workbench", "version": "0.1.0"}}
		return resp
	case "ping":
		resp.Result = map[string]any{}
		return resp
	case "notifications/initialized", "notifications/cancelled":
		return resp
	case "tools/list":
		s.mu.Lock()
		defs := append([]MCPTool{}, s.toolDefs...)
		s.mu.Unlock()
		resp.Result = map[string]any{"tools": defs}
		return resp

	case "tools/call":
		var callArgs struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callArgs); err != nil {
			resp.Error = &RPCError{Code: -32602, Message: "Invalid arguments"}
			return resp
		}

		s.mu.Lock()
		handler, ok := s.tools[callArgs.Name]
		s.mu.Unlock()

		if !ok {
			resp.Error = &RPCError{Code: -32601, Message: fmt.Sprintf("Tool not found: %s", callArgs.Name)}
			return resp
		}

		res, err := handler(callArgs.Arguments)
		if err != nil {
			resp.Error = &RPCError{Code: -32603, Message: err.Error()}
			return resp
		}
		resp.Result = map[string]any{"content": []map[string]any{{"type": "text", "text": toJSON(res)}}}
		return resp

	default:
		// Also support direct tool invocation matching tool name
		s.mu.Lock()
		handler, ok := s.tools[req.Method]
		s.mu.Unlock()

		if ok {
			res, err := handler(req.Params)
			if err != nil {
				resp.Error = &RPCError{Code: -32603, Message: err.Error()}
				return resp
			}
			resp.Result = res
			return resp
		}

		resp.Error = &RPCError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
		return resp
	}
}

func (s *Server) Serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			errResp := JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: -32700, Message: "Parse error"},
			}
			data, _ := json.Marshal(errResp)
			if _, err := w.Write(append(data, '\n')); err != nil {
				return err
			}
			continue
		}

		resp := s.HandleRequest(req)
		if req.ID == nil {
			continue
		} // JSON-RPC notifications never receive a response.
		data, err := json.Marshal(resp)
		if err == nil {
			if _, err := w.Write(append(data, '\n')); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func (s *Server) ServeStdio() error {
	return s.Serve(os.Stdin, os.Stdout)
}

func toJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// -------------------------------------------------------------
// Default Capability Provider Implementation
// Reuse desktop discovery, runtime isolation and project trust decisions.
type DefaultCapabilityProvider struct {
	mu      sync.Mutex
	backend *workbench.Backend
}

func NewDefaultCapabilityProvider() *DefaultCapabilityProvider { return &DefaultCapabilityProvider{} }
func (p *DefaultCapabilityProvider) getBackend() (*workbench.Backend, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.backend == nil {
		p.backend = workbench.NewBackend(nil)

	}
	return p.backend, nil
}
func (p *DefaultCapabilityProvider) resolve(id string) (*workbench.Backend, string, error) {
	b, err := p.getBackend()
	if err != nil {
		return nil, "", err
	}
	if len(b.Snapshot().Projects) == 0 {
		if err := b.Discover(); err != nil {
			return nil, "", err
		}
	}
	for _, project := range b.Snapshot().Projects {
		if id == project.ID || strings.EqualFold(filepath.Clean(id), filepath.Clean(project.Path)) {
			return b, project.ID, nil
		}
	}
	return nil, "", fmt.Errorf("project %q does not exist in discovered Herd projects", id)
}
func (p *DefaultCapabilityProvider) ListProjects() (any, error) {
	b, err := p.getBackend()
	if err != nil {
		return nil, err
	}
	if err := b.Discover(); err != nil {
		return nil, err
	}
	return b.Snapshot().Projects, nil
}
func (p *DefaultCapabilityProvider) GetProjectSnapshot(id string) (any, error) {
	b, id, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	return b.GetProjectSnapshot(id)
}
func (p *DefaultCapabilityProvider) GetEnvDrift(id string) (any, error) {
	b, id, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	return b.GetEnvDrift(id)
}
func (p *DefaultCapabilityProvider) AuditProject(id string) (any, error) {
	b, id, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	return b.AuditProject(id)
}
func (p *DefaultCapabilityProvider) GetRoutes(id string) (any, error) {
	b, id, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	return b.Routes(id)
}
func (p *DefaultCapabilityProvider) GetServices() (any, error) {
	b, err := p.getBackend()
	if err != nil {
		return nil, err
	}
	return b.Snapshot().Services, nil
}
func (p *DefaultCapabilityProvider) GetErrors(id string) (any, error) {
	b, id, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	entries, err := b.GetRawLogs(id)
	if err != nil {
		return nil, err
	}
	// Apply the same persisted/export redaction policy to read-only diagnostics.
	data, err := json.Marshal(entries)
	if err != nil {
		return nil, err
	}
	return map[string]any{"project_id": id, "content": storage.RedactSensitive(string(data))}, nil
}
func (p *DefaultCapabilityProvider) GetErdSchema(id string) (any, error) {
	b, id, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	return b.GetErdSchema(id)
}
func (p *DefaultCapabilityProvider) ExplainQuery(id, query string) (any, error) {
	b, id, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	return b.ExplainQuery(id, query)
}
func (p *DefaultCapabilityProvider) GetArchitectureMap(id string) (any, error) {
	b, id, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	return b.GetArchitectureCanvas(id)
}
func (p *DefaultCapabilityProvider) GetHealthHeartbeat(id string) (any, error) {
	b, id, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	return b.GetHealthHeartbeat(id)
}
func (p *DefaultCapabilityProvider) InspectNPlusOne(id string) (any, error) {
	b, id, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	return b.GetNPlusOneReport(id, "")
}
func (p *DefaultCapabilityProvider) GetLogPail(id string) (any, error) {
	b, id, err := p.resolve(id)
	if err != nil {
		return nil, err
	}
	return b.GetLogPailReport(id, 50)
}
func (p *DefaultCapabilityProvider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.backend != nil {
		p.backend.Shutdown()
		p.backend = nil
	}
}
func (s *Server) Close() {
	if p, ok := s.provider.(interface{ Close() }); ok {
		p.Close()
	}
}
