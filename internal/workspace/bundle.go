package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// Bundle 是多窗口工作区。每个 Session 仍只记录路径，不记录编辑缓冲区。
type Bundle struct {
	Version  int       `json:"version"`
	Sessions []Session `json:"sessions"`
}

// Autosave 原子写入崩溃前的工作区。已有的非普通文件会被拒绝。
func Autosave(path string, sessions []Session) error {
	stored := make([]Session, len(sessions))
	for i, s := range sessions {
		if err := s.validate(); err != nil {
			return err
		}
		var err error
		if s.Left, err = filepath.Abs(s.Left); err != nil {
			return err
		}
		if s.Right, err = filepath.Abs(s.Right); err != nil {
			return err
		}
		if s.Base != "" {
			if s.Base, err = filepath.Abs(s.Base); err != nil {
				return err
			}
		}
		if s.Output != "" {
			if s.Output, err = filepath.Abs(s.Output); err != nil {
				return err
			}
		}
		stored[i] = s
	}
	data, err := json.MarshalIndent(Bundle{Version: 1, Sessions: stored}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeAtomic(path, data)
}

// Recover 读回工作区。版本必须是 1；相对路径按工作区文件所在目录解析。
func Recover(path string) (Bundle, error) {
	var b Bundle
	f, err := os.Open(path)
	if err != nil {
		return b, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return b, err
	}
	if len(data) > 1<<20 {
		return b, errors.New("workspace exceeds 1 MiB limit")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err = dec.Decode(&b); err != nil {
		return Bundle{}, err
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return Bundle{}, errors.New("unexpected trailing workspace data")
	}
	if b.Version != 1 {
		return Bundle{}, errors.New("unsupported workspace version")
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return Bundle{}, err
	}
	for i := range b.Sessions {
		if err = b.Sessions[i].validate(); err != nil {
			return Bundle{}, err
		}
		b.Sessions[i].Left = absJoin(base, b.Sessions[i].Left)
		b.Sessions[i].Right = absJoin(base, b.Sessions[i].Right)
		b.Sessions[i].Base = absJoin(base, b.Sessions[i].Base)
		b.Sessions[i].Output = absJoin(base, b.Sessions[i].Output)
	}
	return b, nil
}

func absJoin(base, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

func writeAtomic(path string, data []byte) error {
	if st, err := os.Lstat(path); err == nil && !st.Mode().IsRegular() {
		return errors.New("workspace target is not a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".gocompare-workspace-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
