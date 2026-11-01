package remote

import (
	"bufio"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Endpoint 是界面里填写的连接信息，不含密码。Share 只用于 SMB。
type Endpoint struct {
	Scheme string `json:"scheme"`
	Host   string `json:"host"`
	Port   string `json:"port,omitempty"`
	User   string `json:"user,omitempty"`
	Domain string `json:"domain,omitempty"`
	Share  string `json:"share,omitempty"`
}

// DefaultPort 返回协议默认端口，用作输入框占位提示。
func DefaultPort(scheme string) string {
	switch scheme {
	case "sftp":
		return "22"
	case "smb":
		return "445"
	case "ftp", "ftps":
		return "21"
	case "webdav+https":
		return "443"
	case "webdav":
		return "80"
	}
	return ""
}

// URL 拼出指向 p（文件或目录）的地址；p 使用 Lister 返回的路径写法。端口为空或等于默认端口时省略。
func (e Endpoint) URL(p string) string {
	u := url.URL{Scheme: e.Scheme, Host: e.Host}
	if strings.Contains(e.Host, ":") && !strings.HasPrefix(e.Host, "[") {
		u.Host = "[" + e.Host + "]" // IPv6 字面量
	}
	if e.Port != "" && e.Port != DefaultPort(e.Scheme) {
		u.Host = net.JoinHostPort(strings.Trim(e.Host, "[]"), e.Port)
	}
	name := e.User
	if e.Scheme == "smb" && e.Domain != "" && name != "" {
		name = e.Domain + ";" + name
	}
	if name != "" {
		u.User = url.User(name)
	}
	// 空路径表示起点（SFTP 为登录目录），"/" 表示根目录：两者在 SFTP 下含义不同，必须区分。
	switch {
	case e.Scheme == "smb":
		if joined := strings.Trim(path.Join(e.Share, strings.TrimLeft(p, "/")), "/"); joined != "" {
			u.Path = "/" + joined
		}
	case p != "":
		u.Path = "/" + strings.TrimLeft(p, "/")
	}
	return u.String()
}

// Key 与 CredentialKey 一致，用来关联内存中的密码和私钥设置。
func (e Endpoint) Key() string { return CredentialKey(e.URL("")) }

// SSHHost 是 ~/.ssh/config 中的一个具体主机（不含通配符）。
type SSHHost struct {
	Alias        string
	HostName     string
	User         string
	Port         string
	IdentityFile string
}

// LoadSSHConfig 读取 OpenSSH 客户端配置中的具体主机，供界面一键填写。
// 只解析 Host、HostName、User、Port、IdentityFile；Match 块和 Include 被忽略，
// "Host *" 中的值补足未设置的字段，同一字段以先出现的为准（与 ssh 一致）。文件不存在时返回空。
func LoadSSHConfig(file string) ([]SSHHost, error) {
	home, _ := os.UserHomeDir()
	if file == "" {
		file = filepath.Join(home, ".ssh", "config")
	}
	f, err := os.Open(file)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var hosts []SSHHost
	var defaults SSHHost
	var current []int // 当前块对应的 hosts 下标
	inDefaults, skip := false, false
	set := func(field *string, v string) {
		if *field == "" {
			*field = v
		}
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, " ")
		if k, v, eq := strings.Cut(line, "="); eq && (!ok || len(k) < len(key)) {
			key, value = k, v
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.Trim(strings.TrimSpace(value), `"`)
		switch key {
		case "host":
			current, inDefaults, skip = nil, false, false
			for _, pattern := range strings.Fields(value) {
				if pattern == "*" {
					inDefaults = true
					continue
				}
				if strings.ContainsAny(pattern, "*?!") {
					continue
				}
				hosts = append(hosts, SSHHost{Alias: pattern})
				current = append(current, len(hosts)-1)
			}
			continue
		case "match":
			current, inDefaults, skip = nil, false, true
			continue
		}
		if skip {
			continue
		}
		if strings.HasPrefix(value, "~/") {
			value = filepath.Join(home, value[2:])
		}
		targets := []*SSHHost{}
		for _, i := range current {
			targets = append(targets, &hosts[i])
		}
		if inDefaults {
			targets = append(targets, &defaults)
		}
		for _, h := range targets {
			switch key {
			case "hostname":
				set(&h.HostName, value)
			case "user":
				set(&h.User, value)
			case "port":
				set(&h.Port, value)
			case "identityfile":
				set(&h.IdentityFile, value)
			}
		}
	}
	if err = sc.Err(); err != nil {
		return nil, err
	}
	for i := range hosts {
		h := &hosts[i]
		set(&h.HostName, defaults.HostName)
		set(&h.HostName, h.Alias)
		set(&h.User, defaults.User)
		set(&h.Port, defaults.Port)
		set(&h.IdentityFile, defaults.IdentityFile)
	}
	return hosts, nil
}
