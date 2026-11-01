package remote

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf16"
)

// WebDAV 使用 GET、PUT 和 DELETE。User 非空时使用 Basic 认证。
type WebDAV struct {
	Base   string
	User   string
	Pass   string
	Client *http.Client
}

func (w *WebDAV) Read(ctx context.Context, path string) ([]byte, error) {
	return w.do(ctx, http.MethodGet, path, nil)
}

func (w *WebDAV) Write(ctx context.Context, path string, data []byte) error {
	_, err := w.do(ctx, http.MethodPut, path, data)
	return err
}

func (w *WebDAV) Delete(ctx context.Context, path string) error {
	_, err := w.do(ctx, http.MethodDelete, path, nil)
	return err
}

func (w *WebDAV) do(ctx context.Context, method, path string, data []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, joinURL(w.Base, path), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if data != nil {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	if w.User != "" {
		req.SetBasicAuth(w.User, w.Pass)
	}
	body, code, err := readLimited(ctx, w.Client, req)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, statusError("webdav", path, code, body)
	}
	return body, nil
}

// S3 使用 path-style 请求和 AWS Signature Version 4。
type S3 struct {
	Base      string
	Region    string
	AccessKey string
	SecretKey string
	Client    *http.Client
	Now       func() time.Time
}

func (s *S3) Read(ctx context.Context, path string) ([]byte, error) {
	return s.do(ctx, http.MethodGet, path, nil)
}

func (s *S3) Write(ctx context.Context, path string, data []byte) error {
	_, err := s.do(ctx, http.MethodPut, path, data)
	return err
}

func (s *S3) Delete(ctx context.Context, path string) error {
	_, err := s.do(ctx, http.MethodDelete, path, nil)
	return err
}

func (s *S3) do(ctx context.Context, method, path string, data []byte) ([]byte, error) {
	if data == nil {
		data = []byte{}
	}
	// 请求路径按 SigV4 规则编码，签名与实际发送的路径一致；Go 默认编码会保留 !*'() 等字符导致验签失败。
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(s.Base, "/")+"/"+awsEscapePath(strings.TrimLeft(path, "/")), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	region := s.Region
	if region == "" {
		region = "us-east-1"
	}
	amz := now.Format("20060102T150405Z")
	sum := sha256.Sum256(data)
	payload := hex.EncodeToString(sum[:])
	req.Header.Set("x-amz-date", amz)
	req.Header.Set("x-amz-content-sha256", payload)
	req.Header.Set("Authorization", s.authorization(method, req.URL, req.Host, amz, region, payload))
	body, code, err := readLimited(ctx, s.Client, req)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, statusError("s3", path, code, body)
	}
	if method == http.MethodGet {
		return body, nil
	}
	return nil, nil
}

func (s *S3) authorization(method string, u *url.URL, host, amz, region, payload string) string {
	date := amz[:8]
	scope := date + "/" + region + "/s3/aws4_request"
	canonURI := u.EscapedPath()
	if canonURI == "" {
		canonURI = "/"
	}
	canonHeaders := "host:" + host + "\n" + "x-amz-content-sha256:" + payload + "\n" + "x-amz-date:" + amz + "\n"
	signed := "host;x-amz-content-sha256;x-amz-date"
	canon := method + "\n" + canonURI + "\n" + u.RawQuery + "\n" + canonHeaders + "\n" + signed + "\n" + payload
	hash := sha256.Sum256([]byte(canon))
	toSign := "AWS4-HMAC-SHA256\n" + amz + "\n" + scope + "\n" + hex.EncodeToString(hash[:])
	kDate := hmacSHA256([]byte("AWS4"+s.SecretKey), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, "s3")
	kSign := hmacSHA256(kService, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(kSign, toSign))
	return "AWS4-HMAC-SHA256 Credential=" + s.AccessKey + "/" + scope + ", SignedHeaders=" + signed + ", Signature=" + sig
}

// awsEscapePath 按 SigV4 对 S3 对象键编码：保留 unreserved 字符和 "/"，其余按 UTF-8 字节写成大写 %XX。
func awsEscapePath(path string) string {
	var b strings.Builder
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '/' || c == '-' || c == '_' || c == '.' || c == '~' ||
			(c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteByte(c)
			continue
		}
		b.WriteString(fmt.Sprintf("%%%02X", c))
	}
	return b.String()
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	_, _ = io.WriteString(m, data)
	return m.Sum(nil)
}

// Dropbox 使用 content API 的 download/upload 和 RPC API 的 delete_v2。
// 真实服务的两类接口在不同主机上；ContentBase 为空时回退到 Base，便于单一夹具。
type Dropbox struct {
	Base        string
	ContentBase string
	Token       string
	Client      *http.Client
}

func (d *Dropbox) contentRoot() string {
	if d.ContentBase != "" {
		return strings.TrimRight(d.ContentBase, "/")
	}
	return strings.TrimRight(d.Base, "/")
}

func (d *Dropbox) Read(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.contentRoot()+"/2/files/download", nil)
	if err != nil {
		return nil, err
	}
	d.auth(req, path, false)
	body, code, err := readLimited(ctx, d.Client, req)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, statusError("dropbox", path, code, body)
	}
	return body, nil
}

func (d *Dropbox) Write(ctx context.Context, path string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.contentRoot()+"/2/files/upload", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	d.auth(req, path, true)
	body, code, err := readLimited(ctx, d.Client, req)
	if err != nil {
		return err
	}
	if code >= 400 {
		return statusError("dropbox", path, code, body)
	}
	return nil
}

func (d *Dropbox) Delete(ctx context.Context, path string) error {
	arg, _ := json.Marshal(map[string]string{"path": cleanPath(path)})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(d.Base, "/")+"/2/files/delete_v2", bytes.NewReader(arg))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+d.Token)
	req.Header.Set("Content-Type", "application/json")
	body, code, err := readLimited(ctx, d.Client, req)
	if err != nil {
		return err
	}
	if code >= 400 {
		return statusError("dropbox", path, code, body)
	}
	return nil
}

func (d *Dropbox) auth(req *http.Request, path string, upload bool) {
	payload := map[string]string{"path": cleanPath(path)}
	if upload {
		payload["mode"] = "overwrite"
	}
	arg, _ := json.Marshal(payload)
	req.Header.Set("Authorization", "Bearer "+d.Token)
	req.Header.Set("Dropbox-API-Arg", asciiJSON(arg))
}

// asciiJSON 把非 ASCII 字符写成 \uXXXX。Dropbox-API-Arg 放在 HTTP 头里，服务端要求纯 ASCII。
func asciiJSON(b []byte) string {
	var out strings.Builder
	for _, r := range string(b) {
		if r < 0x80 {
			out.WriteRune(r)
			continue
		}
		for _, u := range utf16.Encode([]rune{r}) {
			fmt.Fprintf(&out, "\\u%04x", u)
		}
	}
	return out.String()
}

// OneDrive 使用 Microsoft Graph 的 drive content 子集。
type OneDrive struct {
	Base   string
	Token  string
	Client *http.Client
}

func (o *OneDrive) Read(ctx context.Context, path string) ([]byte, error) {
	return o.do(ctx, http.MethodGet, path, nil)
}

func (o *OneDrive) Write(ctx context.Context, path string, data []byte) error {
	_, err := o.do(ctx, http.MethodPut, path, data)
	return err
}

func (o *OneDrive) Delete(ctx context.Context, path string) error {
	_, err := o.do(ctx, http.MethodDelete, path, nil)
	return err
}

func (o *OneDrive) do(ctx context.Context, method, path string, data []byte) ([]byte, error) {
	root := strings.TrimRight(o.Base, "/") + "/v1.0/me/drive/root:/" + escapeSegments(strings.TrimLeft(path, "/"))
	if method != http.MethodDelete {
		root += ":/content"
	}
	req, err := http.NewRequestWithContext(ctx, method, root, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+o.Token)
	body, code, err := readLimited(ctx, o.Client, req)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, statusError("onedrive", path, code, body)
	}
	return body, nil
}

// SVN 是 HTTP 子集：GET、PUT、DELETE，并带上 X-SVN-Request。它不是完整的 delta editor。
// User 非空时使用 Basic 认证。
type SVN struct {
	Base   string
	User   string
	Pass   string
	Client *http.Client
}

func (s *SVN) Read(ctx context.Context, path string) ([]byte, error) {
	return s.do(ctx, http.MethodGet, path, nil)
}

func (s *SVN) Write(ctx context.Context, path string, data []byte) error {
	_, err := s.do(ctx, http.MethodPut, path, data)
	return err
}

func (s *SVN) Delete(ctx context.Context, path string) error {
	_, err := s.do(ctx, http.MethodDelete, path, nil)
	return err
}

func (s *SVN) do(ctx context.Context, method, path string, data []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, joinURL(s.Base, path), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-SVN-Request", "subset")
	if s.User != "" {
		req.SetBasicAuth(s.User, s.Pass)
	}
	body, code, err := readLimited(ctx, s.Client, req)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, statusError("svn", path, code, body)
	}
	return body, nil
}

// joinURL 逐段转义路径，保留层级分隔符；路径中的 "#"、"?"、空格不会改变 URL 结构。
func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + escapeSegments(strings.TrimLeft(path, "/"))
}

func escapeSegments(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func cleanPath(path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}
