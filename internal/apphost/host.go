// Package apphost exposes the application's existing methods to authenticated
// loopback clients. It owns transport only; permissions remain in the backend.
package apphost

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"
)

type Host struct {
	URL string
	token string
	server *http.Server
	methods map[string]reflect.Value
	mu sync.Mutex
	subs map[chan string]struct{}
}

// Start binds only an OS-assigned IPv4 loopback port. The explicit method list
// prevents new exported Go methods accidentally becoming remotely callable.
func Start(api any, names []string, assets fs.FS) (*Host, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil { return nil, err }
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil { listener.Close(); return nil, err }
	h := &Host{URL: "http://"+listener.Addr().String(), token: hex.EncodeToString(secret), methods: map[string]reflect.Value{}, subs: map[chan string]struct{}{}}
	value := reflect.ValueOf(api)
	typ := value.Type()
	if len(names) == 0 {
		for i := 0; i < typ.NumMethod(); i++ {
			m := typ.Method(i)
			if m.IsExported() {
				h.methods[m.Name] = value.Method(i)
			}
		}
	} else {
		for _, name := range names {
			method := value.MethodByName(name)
			if !method.IsValid() { listener.Close(); return nil, fmt.Errorf("unknown application method %s", name) }
			h.methods[name] = method
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/session", h.session)
	mux.HandleFunc("POST /api/call", h.call)
	mux.HandleFunc("GET /api/events", h.events)
	if assets != nil {
		mux.Handle("/", http.FileServer(http.FS(assets)))
	}
	h.server = &http.Server{Handler: h.guard(mux), ReadHeaderTimeout: 5*time.Second, IdleTimeout: 60*time.Second, MaxHeaderBytes: 32<<10}
	go h.server.Serve(listener)
	return h, nil
}

func (h *Host) LaunchURL() string { return h.URL+"/#session="+h.token }
func (h *Host) Close(ctx context.Context) error { return h.server.Shutdown(ctx) }

func (h *Host) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		if r.Host != strings.TrimPrefix(h.URL,"http://") { http.Error(w,"Invalid host",http.StatusForbidden); return }
		if origin := r.Header.Get("Origin"); origin != "" && origin != h.URL { http.Error(w,"Invalid origin",http.StatusForbidden); return }
		if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" { http.Error(w,"Cross-site access denied",http.StatusForbidden); return }
		if strings.HasPrefix(r.URL.Path,"/api/") && r.URL.Path != "/api/session" && !h.authorized(r) { http.Error(w,"Open the dashboard from LaraDeck to sign in",http.StatusUnauthorized); return }
		if r.Method == "POST" && r.Header.Get("Origin") == "" && !h.bearer(r) { http.Error(w,"Origin required",http.StatusForbidden); return }
		next.ServeHTTP(w,r)
	})
}
func (h *Host) equal(value string) bool { return subtle.ConstantTimeCompare([]byte(value),[]byte(h.token)) == 1 }
func (h *Host) bearer(r *http.Request) bool { return h.equal(strings.TrimPrefix(r.Header.Get("Authorization"),"Bearer ")) }
func (h *Host) authorized(r *http.Request) bool {
	if h.bearer(r) { return true }
	cookie,err := r.Cookie("laradeck_session")
	return err == nil && h.equal(cookie.Value)
}
func (h *Host) session(w http.ResponseWriter,r *http.Request) {
	if !h.bearer(r) { http.Error(w,"Invalid session",http.StatusUnauthorized); return }
	http.SetCookie(w,&http.Cookie{Name:"laradeck_session",Value:h.token,Path:"/",HttpOnly:true,SameSite:http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}
func (h *Host) call(w http.ResponseWriter,r *http.Request) {
	var request struct { Method string `json:"method"`; Args []json.RawMessage `json:"args"` }
	decoder:=json.NewDecoder(http.MaxBytesReader(w,r.Body,2<<20))
	decoder.DisallowUnknownFields()
	if err:=decoder.Decode(&request); err!=nil { http.Error(w,"Invalid request",400);return }
	if err:=decoder.Decode(new(any)); err!=io.EOF { http.Error(w,"Expected one request",400);return }
	method,ok:=h.methods[request.Method]
	if !ok { http.Error(w,"Unsupported application method",404);return }
	if method.Type().NumIn()!=len(request.Args) { http.Error(w,"Wrong argument count",400);return }
	args:=make([]reflect.Value,len(request.Args))
	for i,raw:=range request.Args {
		arg:=reflect.New(method.Type().In(i))
		if err:=json.Unmarshal(raw,arg.Interface()); err!=nil { http.Error(w,"Invalid argument",400);return }
		args[i]=arg.Elem()
	}
	// Avoid turning unexpected application failures into a crashed local host.
	defer func(){ if recover()!=nil { http.Error(w,"Application operation failed",500) } }()
	results:=method.Call(args)
	var result any
	for _,value:=range results {
		if value.Type().Implements(reflect.TypeFor[error]()) {
			if !value.IsNil() { writeJSON(w, map[string]any{"error":value.Interface().(error).Error()});return }
		} else { result=value.Interface() }
	}
	writeJSON(w,map[string]any{"result":result})
}
func writeJSON(w http.ResponseWriter,value any) { w.Header().Set("Content-Type","application/json"); json.NewEncoder(w).Encode(value) }
func (h *Host) Emit(name string) {
	h.mu.Lock();defer h.mu.Unlock()
	for subscriber:=range h.subs { select { case subscriber<-name:default: } }
}
func (h *Host) events(w http.ResponseWriter,r *http.Request) {
	flusher,ok:=w.(http.Flusher); if !ok { http.Error(w,"Streaming unavailable",500);return }
	w.Header().Set("Content-Type","text/event-stream")
	w.Header().Set("Connection","keep-alive")
	subscriber:=make(chan string,1)
	h.mu.Lock();h.subs[subscriber]=struct{}{};h.mu.Unlock()
	defer func(){h.mu.Lock();delete(h.subs,subscriber);h.mu.Unlock()}()
	fmt.Fprint(w,": connected\n\n");flusher.Flush()
	ticker:=time.NewTicker(15*time.Second);defer ticker.Stop()
	for { select {
	case <-r.Context().Done():return
	case name:=<-subscriber:
		encoded,_:=json.Marshal(name); if _,err:=fmt.Fprintf(w,"data: %s\n\n",encoded);err!=nil{return};flusher.Flush()
	case <-ticker.C: if _,err:=fmt.Fprint(w,": heartbeat\n\n");err!=nil{return};flusher.Flush()
	} }
}

// Endpoint is published only inside the user's existing application directory.
type Endpoint struct { Version int `json:"version"`; URL string `json:"url"`; PID int `json:"pid"` }
var ErrAlreadyRunning = errors.New("LaraDeck is already running")
