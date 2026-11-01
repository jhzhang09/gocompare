package remote

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
	"github.com/pkg/sftp"
)

// Entry 是目录中的一项，供界面浏览远程文件。
type Entry struct {
	Name    string
	Dir     bool
	Link    bool // 符号链接本身（不跟随）；目前只有 SFTP 能识别
	Size    int64
	Mode    os.FileMode // 权限位；协议不提供时为 0
	ModTime time.Time
}

// Lister 列出远程目录。dir 为空表示起始目录；返回解析后的目录路径，用于面包屑和拼接子路径。
// 路径写法与 ParseURL 返回的路径一致：SFTP、WebDAV 为绝对路径，FTP 相对登录目录，SMB 相对共享根。
type Lister interface {
	List(ctx context.Context, dir string) (string, []Entry, error)
}

// ErrListUnsupported 表示该协议或服务端不支持浏览，界面改为手动输入路径。
var ErrListUnsupported = errors.New("listing is not supported for this remote")

// sortEntries 目录在前，同类按名称不区分大小写排序。
func sortEntries(entries []Entry) []Entry {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries
}

func (s *SFTP) List(ctx context.Context, dir string) (string, []Entry, error) {
	var resolved string
	var entries []Entry
	err := s.withClient(ctx, func(c *sftp.Client) error {
		var err error
		resolved, entries, err = sftpList(ctx, c, dir)
		return err
	})
	return resolved, entries, err
}

// sftpList 解析 dir（空表示登录目录）并列出其中的项；目录项来自 READDIR，符号链接不被跟随。
func sftpList(ctx context.Context, c *sftp.Client, dir string) (string, []Entry, error) {
	if dir == "" {
		dir = "."
	}
	resolved, err := c.RealPath(dir)
	if err != nil {
		return "", nil, fmt.Errorf("sftp list %s: %w", dir, err)
	}
	infos, err := c.ReadDirContext(ctx, resolved)
	if err != nil {
		return "", nil, fmt.Errorf("sftp list %s: %w", resolved, err)
	}
	return resolved, entriesOf(infos), nil
}

func entriesOf(infos []os.FileInfo) []Entry {
	entries := make([]Entry, 0, len(infos))
	for _, fi := range infos {
		entries = append(entries, Entry{Name: fi.Name(), Dir: fi.IsDir(), Link: fi.Mode()&os.ModeSymlink != 0, Size: fi.Size(), Mode: fi.Mode().Perm(), ModTime: fi.ModTime()})
	}
	return sortEntries(entries)
}

// List 在未指定共享名时列出服务端的共享（作为目录返回），否则列出共享内的目录。
func (s *SMB) List(ctx context.Context, dir string) (string, []Entry, error) {
	var entries []Entry
	if s.Share == "" {
		err := s.withSession(ctx, func(session *smb2.Session) error {
			names, err := session.WithContext(ctx).ListSharenames()
			if err != nil {
				return fmt.Errorf("smb list shares: %w", err)
			}
			for _, n := range names {
				// 以 $ 结尾的是管理共享（IPC$、C$ 等），普通用户无法浏览。
				if !strings.HasSuffix(n, "$") {
					entries = append(entries, Entry{Name: n, Dir: true})
				}
			}
			return nil
		})
		return "", sortEntries(entries), err
	}
	dir = strings.Trim(dir, "/")
	err := s.withShare(ctx, func(fs *smb2.Share) error {
		infos, err := fs.ReadDir(smbPath(dir))
		if err != nil {
			return fmt.Errorf("smb list %s: %w", dir, err)
		}
		entries = entriesOf(infos)
		return nil
	})
	return dir, entries, err
}

// List 使用 MLSD（RFC 3659）；服务端不支持时返回 ErrListUnsupported，不去猜测 LIST 的非标准格式。
func (f *FTP) List(ctx context.Context, dir string) (string, []Entry, error) {
	dir = strings.Trim(dir, "/")
	data, err := f.transfer(ctx, "MLSD", dir, nil)
	if err != nil {
		// 500/502 表示服务端不认识 MLSD 命令。
		if msg := err.Error(); strings.Contains(msg, ": 500 ") || strings.Contains(msg, ": 502 ") {
			return "", nil, fmt.Errorf("%w: %v", ErrListUnsupported, err)
		}
		return "", nil, err
	}
	return dir, parseMLSD(data), nil
}

// parseMLSD 解析 MLSD 输出，跳过当前目录与上级目录项。
func parseMLSD(data []byte) []Entry {
	var entries []Entry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		facts, name, ok := strings.Cut(line, " ")
		if !ok || name == "" {
			continue
		}
		e := Entry{Name: name}
		skip := false
		for _, fact := range strings.Split(facts, ";") {
			k, v, _ := strings.Cut(fact, "=")
			switch strings.ToLower(k) {
			case "type":
				switch strings.ToLower(v) {
				case "dir":
					e.Dir = true
				case "cdir", "pdir":
					skip = true
				}
			case "size":
				e.Size, _ = strconv.ParseInt(v, 10, 64)
			case "modify":
				e.ModTime, _ = time.Parse("20060102150405", v[:min(len(v), 14)])
			}
		}
		if !skip {
			entries = append(entries, e)
		}
	}
	return sortEntries(entries)
}

// List 发送 Depth: 1 的 PROPFIND，解析 multistatus 中的子项。
func (w *WebDAV) List(ctx context.Context, dir string) (string, []Entry, error) {
	dir = "/" + strings.Trim(dir, "/")
	target := joinURL(w.Base, dir)
	if !strings.HasSuffix(target, "/") {
		target += "/"
	}
	body := `<?xml version="1.0"?><propfind xmlns="DAV:"><prop><resourcetype/><getcontentlength/><getlastmodified/></prop></propfind>`
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", target, strings.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")
	if w.User != "" {
		req.SetBasicAuth(w.User, w.Pass)
	}
	data, code, err := readLimited(ctx, w.Client, req)
	if err != nil {
		return "", nil, err
	}
	if code == http.StatusMethodNotAllowed || code == http.StatusNotImplemented {
		return "", nil, fmt.Errorf("%w: webdav %d", ErrListUnsupported, code)
	}
	if code >= 400 {
		return "", nil, statusError("webdav list", dir, code, data)
	}
	var ms struct {
		Responses []struct {
			Href string `xml:"href"`
			Prop struct {
				Collection *struct{} `xml:"resourcetype>collection"`
				Length     int64     `xml:"getcontentlength"`
				Modified   string    `xml:"getlastmodified"`
			} `xml:"propstat>prop"`
		} `xml:"response"`
	}
	if err = xml.Unmarshal(data, &ms); err != nil {
		return "", nil, fmt.Errorf("webdav list %s: %w", dir, err)
	}
	self := strings.TrimRight(dir, "/")
	var entries []Entry
	for _, r := range ms.Responses {
		u, err := url.Parse(r.Href)
		if err != nil {
			continue
		}
		p := strings.TrimRight(u.Path, "/")
		if p == self || p == "" && self == "" {
			continue // 第一项是目录自身
		}
		e := Entry{Name: path.Base(p), Dir: r.Prop.Collection != nil, Size: r.Prop.Length}
		e.ModTime, _ = http.ParseTime(r.Prop.Modified)
		entries = append(entries, e)
	}
	return dir, sortEntries(entries), nil
}
