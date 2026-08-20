package web

import (
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"qrcd/internal/payload"
	"qrcd/internal/qrcode"
	"qrcd/internal/receive"
	"qrcd/internal/send"
)

// session 一次发送会话：预生成全部帧（用于发送端逐帧取 PNG）。
// 多会话分片（DEC-012）时 items 为各分片帧按序拼接（扁平索引），
// partSizes[i] 记录第 i 个分片的帧数，供发送端展示"分片 X/Y"。
type session struct {
	items     []*send.StreamItem
	partSizes []int
	current   int
}

// recvSession 网络接收会话：手机中继把扫到的 QR 帧字节 POST 到 /api/ingest。
type recvSession struct {
	proc  *receive.Processor
	done  bool
	res   *receive.Result
	count int
	total int
}

// Server 提供 send/receive 三端 Web 产物。
type Server struct {
	mu   sync.Mutex
	sess *session
	addr string
	srv  *http.Server

	rmu  sync.Mutex
	recv *recvSession
}

// NewServer 创建 Web 服务（addr 形如 ":8080"）。
func NewServer(addr string) *Server {
	return &Server{addr: addr}
}

// tlsPort 把 HTTP 监听地址映射到 HTTPS 端口（":8080" → ":8443"，偏移 +363）。
func tlsPort(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ":8443"
	}
	n, _ := strconv.Atoi(port)
	if n <= 0 {
		n = 8080
	}
	return net.JoinHostPort(host, strconv.Itoa(n+363))
}

// Start 启动 HTTP 服务（阻塞）。
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/sender", s.handleSender)
	mux.HandleFunc("/relay", s.handleRelay)
	mux.HandleFunc("/receiver", s.handleReceiver)
	mux.HandleFunc("/scantest", s.handleScanTest)
	mux.HandleFunc("/api/encode", s.handleEncode)
	mux.HandleFunc("/api/frame/", s.handleFrame)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/ingest", s.handleIngest)
	mux.HandleFunc("/api/recv/status", s.handleRecvStatus)
	mux.HandleFunc("/api/recv/file", s.handleRecvFile)
	mux.HandleFunc("/jsQR.js", s.handleJSQR)
	mux.HandleFunc("/dl/app.apk", s.handleAPKDownload)
	mux.HandleFunc("/api/qrcode", s.handleQRCode)

	s.srv = &http.Server{Addr: s.addr, Handler: mux}

	// HTTPS 端口（手机中继需要安全上下文才能开摄像头）：同 addr 基础上 +363
	// （:8080 → :8443），与 HTTP 共享同一 mux/state。
	httpsAddr := tlsPort(s.addr)
	if err := ensureCertFiles(); err != nil {
		fmt.Printf("  ⚠ 生成自签证书失败（手机中继将无法开摄像头）: %v\n", err)
		httpsAddr = ""
	}

	go func() {
		if ctx == nil {
			return
		}
		<-ctx.Done()
		s.srv.Shutdown(context.Background())
	}()

	// HTTPS 在后台 goroutine 起给手机用（安全上下文才能开摄像头）；HTTP 阻塞当前 goroutine。
	if httpsAddr != "" {
		httpsSrv := &http.Server{Addr: httpsAddr, Handler: mux}
		go func() {
			if err := httpsSrv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
				fmt.Printf("  ⚠ HTTPS(%s) 启动失败: %v\n", httpsAddr, err)
			}
		}()
	}

	fmt.Printf("\nqrcd Web 三端已启动:\n")
	fmt.Printf("  发送端(本机选文件→播放 QR): http://localhost%s/sender\n", s.addr)
	fmt.Printf("  接收端(等手机上传→还原):    http://localhost%s/receiver\n", s.addr)
	fmt.Printf("  手机App下载:                http://<本机IP>%s/dl/app.apk\n", s.addr)
	fmt.Printf("  首页(含App下载二维码):      http://localhost%s/\n", s.addr)
	if httpsAddr != "" {
		fmt.Printf("  手机中继(扫码→转发,需HTTPS): https://<本机IP>%s/relay\n", httpsAddr)
		fmt.Println("  手机首次访问会提示证书不安全，点「高级/继续访问」即可开摄像头。")
	}
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

// handleEncode 接收文件/文本，BuildSessionStreams 预生成全部帧 PNG 序列。
// 大文件自动多会话分片（DEC-012），各分片帧按序拼成扁平 items，partSizes 记录分片边界。
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
	streams, err := send.BuildSessionStreams(load, send.Options{
		Version: 15, ECC: "Q", Redundancy: 0.25, BlockSize: 1024, FPS: 8, MaxSymbol: 151,
	})
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var items []*send.StreamItem
	var partSizes []int
	totalSymbols := 0
	for _, st := range streams {
		n0 := len(items)
		for {
			it, ok := st.Next()
			if !ok {
				break
			}
			cp := it
			items = append(items, &cp)
		}
		partSizes = append(partSizes, len(items)-n0)
		totalSymbols += st.Meta().TotalSymbols
	}
	s.mu.Lock()
	s.sess = &session{items: items, partSizes: partSizes}
	s.mu.Unlock()

	// 响应带整体信息：多会话时 name/hash 取 part 0 的 overall 字段
	m0 := streams[0].Meta()
	respName, respHash := name, m0.Hash
	if m0.OverallName != "" {
		respName = m0.OverallName
	}
	if m0.OverallHash != "" {
		respHash = m0.OverallHash
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"frames":    len(items),
		"symbols":   totalSymbols,
		"name":      respName,
		"size":      len(data),
		"hash":      respHash,
		"parts":     len(streams),
		"partSizes": partSizes,
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
	// 直接用原始帧字节入 QR。手机端原生 App(ML Kit)能可靠解二进制 byte-mode QR。
	// 数据帧用 v15 上限（payload 151B 自动降到 v12，稀疏易扫）；
	// meta 帧（含文件名/hash 等）大小随文件名变化，用 v40 上限确保不超容量（见 DEC-009）。
	// meta 帧虽可能较密但只占少数帧（每周期重播），手机多扫几次能捕获。
	v := 15
	if item.IsMeta {
		v = 40
	}
	code, err := qrcode.Encode(item.Bytes, qrcode.Options{Version: v, ECC: "Q"})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, code.Image(8))
}

// handleQRCode /api/qrcode?d=<文本> 返回任意文本的标准 QR PNG（如 App 下载链接二维码）。
func (s *Server) handleQRCode(w http.ResponseWriter, r *http.Request) {
	d := r.URL.Query().Get("d")
	if d == "" {
		http.Error(w, "缺少 d 参数", 400)
		return
	}
	code, err := qrcode.Encode([]byte(d), qrcode.Options{Version: 10, ECC: "M"})
	if err != nil {
		// 短链一般 v10 足够；失败再尝试更高版本
		code, err = qrcode.Encode([]byte(d), qrcode.Options{Version: 20, ECC: "M"})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, code.Image(8))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	sess := s.sess
	s.mu.Unlock()
	ready := sess != nil && len(sess.items) > 0
	resp := map[string]interface{}{
		"ready":  ready,
		"frames": func() int { if sess == nil { return 0 }; return len(sess.items) }(),
		"parts":  func() int { if sess == nil { return 1 }; return len(sess.partSizes) }(),
	}
	if sess != nil && len(sess.partSizes) > 0 {
		resp["partSizes"] = sess.partSizes
	}
	json.NewEncoder(w).Encode(resp)
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

// handleScanTest /scantest：静态 QR 自检页。把当前第 0 帧 PNG 画到 canvas，
// 用 jsQR 直接解码（不经摄像头），用于判断"QR 本身 jsQR 能否解出" vs "摄像头能否拍到"。
func (s *Server) handleScanTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, scanTestPage)
}

// handleIngest 接收中继手机 POST 上来的单帧原始字节（= QR 解码出的 QRCD 帧字节），
// 喂给 receive.Processor 重组；收齐后落盘到 downloads/ 目录。
func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "空帧", 400)
		return
	}

	s.rmu.Lock()
	rs := s.recv
	if rs == nil || rs.done {
		os.MkdirAll("downloads", 0o755)
		proc, _ := receive.NewProcessor(receive.Options{Output: "downloads/", Overwrite: true})
		rs = &recvSession{proc: proc}
		s.recv = rs
	}
	s.rmu.Unlock()

	err = rs.proc.Process(body)
	count, total, _ := rs.proc.Stats()
	rs.count = count
	rs.total = total
	if rs.proc.Done() {
		if res, _ := rs.proc.Result(); res != nil {
			rs.res = res
			rs.done = true
		}
	}
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error(), "count": count, "total": total, "done": rs.done})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "count": count, "total": total, "done": rs.done})
}

// handleRecvStatus 网络接收进度。
func (s *Server) handleRecvStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.rmu.Lock()
	rs := s.recv
	s.rmu.Unlock()
	if rs == nil {
		json.NewEncoder(w).Encode(map[string]any{"ready": false})
		return
	}
	resp := map[string]any{
		"ready": true,
		"count": rs.count,
		"total": rs.total,
		"done":  rs.done,
	}
	if rs.done && rs.res != nil {
		resp["name"] = rs.res.Name
		resp["size"] = rs.res.Size
		resp["sha256"] = rs.res.SHA256
		resp["path"] = rs.res.OutputPath
	}
	json.NewEncoder(w).Encode(resp)
}

// handleRecvFile 下载已还原的文件。
func (s *Server) handleRecvFile(w http.ResponseWriter, r *http.Request) {
	s.rmu.Lock()
	rs := s.recv
	s.rmu.Unlock()
	if rs == nil || !rs.done || rs.res == nil {
		http.Error(w, "文件未就绪", 404)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+rs.res.Name+"\"")
	http.ServeFile(w, r, rs.res.OutputPath)
}
