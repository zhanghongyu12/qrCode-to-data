package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"strings"
	"sync"

	"qrcd/internal/payload"
	"qrcd/internal/qrcode"
	"qrcd/internal/send"
)

// session 一次发送会话：预生成全部帧（用于发送端逐帧取 PNG）。
type session struct {
	items   []*send.StreamItem
	current int
}

// Server 提供 send/receive 三端 Web 产物。
type Server struct {
	mu   sync.Mutex
	sess *session
	addr string
	srv  *http.Server
}

// NewServer 创建 Web 服务（addr 形如 ":8080"）。
func NewServer(addr string) *Server {
	return &Server{addr: addr}
}

// Start 启动 HTTP 服务（阻塞）。
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/sender", s.handleSender)
	mux.HandleFunc("/relay", s.handleRelay)
	mux.HandleFunc("/receiver", s.handleReceiver)
	mux.HandleFunc("/api/encode", s.handleEncode)
	mux.HandleFunc("/api/frame/", s.handleFrame)
	mux.HandleFunc("/api/status", s.handleStatus)

	s.srv = &http.Server{Addr: s.addr, Handler: mux}
	go func() {
		<-ctx.Done()
		s.srv.Shutdown(context.Background())
	}()
	fmt.Printf("\nqrcd Web 三端已启动:\n")
	fmt.Printf("  发送端(本机选文件→播放 QR): http://localhost%s/sender\n", s.addr)
	fmt.Printf("  手机中继(扫 QR→重放):       http://localhost%s/relay\n", s.addr)
	fmt.Printf("  接收端(扫 QR→还原文件):     http://localhost%s/receiver\n", s.addr)
	fmt.Println("  手机与电脑需同网；手机访问 http://<本机IP>" + s.addr + "/relay")
	return s.srv.ListenAndServe()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, indexPage)
}

// handleEncode 接收文件/文本，BuildStream 预生成全部帧 PNG 序列。
func (s *Server) handleEncode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	var data []byte
	name := "text.txt"
	mime := "text/plain"
	pt := "text"

	if err := r.ParseMultipartForm(32 << 20); err == nil {
		if f, fh, ferr := r.FormFile("file"); ferr == nil {
			data, _ = io.ReadAll(f)
			f.Close()
			name = fh.Filename
			mime = fh.Header.Get("Content-Type")
			pt = "file"
		}
	}
	if data == nil {
		if t := r.FormValue("text"); t != "" {
			data = []byte(t)
		}
	}
	if data == nil {
		http.Error(w, "请上传 file 或填 text", 400)
		return
	}

	load := &payload.Load{Data: data, Name: name, PayloadType: pt, MimeType: mime}
	st, err := send.BuildStream(load, send.Options{
		Version: 20, ECC: "L", Redundancy: 0.15, BlockSize: 1024, FPS: 8,
	})
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var items []*send.StreamItem
	for {
		it, ok := st.Next()
		if !ok {
			break
		}
		cp := it
		items = append(items, &cp)
	}
	s.mu.Lock()
	s.sess = &session{items: items}
	s.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"frames": len(items),
		"name":   name,
		"size":   len(data),
		"hash":   st.Meta().Hash,
	})
}

// handleFrame /api/frame/<index> 返回该帧的 QR PNG。
func (s *Server) handleFrame(w http.ResponseWriter, r *http.Request) {
	idx := strings.TrimPrefix(r.URL.Path, "/api/frame/")
	var i int
	fmt.Sscanf(idx, "%d", &i)
	s.mu.Lock()
	sess := s.sess
	s.mu.Unlock()
	if sess == nil || i < 0 || i >= len(sess.items) {
		http.NotFound(w, r)
		return
	}
	item := sess.items[i]
	code, err := qrcode.Encode(item.Bytes, qrcode.Options{Version: 20, ECC: "L"})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, code.Image(6))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	sess := s.sess
	s.mu.Unlock()
	ready := sess != nil && len(sess.items) > 0
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ready":  ready,
		"frames": func() int { if sess == nil { return 0 }; return len(sess.items) }(),
	})
}

func (s *Server) handleSender(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, senderPage)
}
func (s *Server) handleRelay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, relayPage)
}
func (s *Server) handleReceiver(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, receiverPage)
}

// 供中继页 JS 上传扫到的帧字节（base64），仅用于诊断/调试，重放走手机本地 canvas。
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	var req struct{ Frames []string `json:"frames"` }
	json.NewDecoder(r.Body).Decode(&req)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"received":%d}`, len(req.Frames))
	_ = base64.StdEncoding
}
