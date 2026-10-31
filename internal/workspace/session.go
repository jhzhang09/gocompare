// Package workspace 负责可移植会话文件，独立于原生窗口。
package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Session 保存比较输入和显示选项；不隐式保存编辑缓冲区。
type Session struct {
	Kind            string `json:"kind,omitempty"`
	Version         int    `json:"version"`
	Left            string `json:"left"`
	Right           string `json:"right"`
	Base            string `json:"base,omitempty"`
	Output          string `json:"output,omitempty"`
	Wrap            bool   `json:"wrap"`
	IgnoreSpace     bool   `json:"ignoreSpace,omitempty"`
	DirectoryFilter string `json:"directoryFilter,omitempty"`
	CompareMode     bool   `json:"compareMode,omitempty"`
	CompareTime     bool   `json:"compareTime,omitempty"`
	FollowLinks     bool   `json:"followLinks,omitempty"`
}

func (s Session) validate() error {
	if s.Kind != "" && s.Kind != "text" && s.Kind != "directory" && s.Kind != "merge" {
		return errors.New("unknown session kind")
	}
	if s.Version != 1 {
		return fmt.Errorf("unsupported session version: %d", s.Version)
	}
	if s.Left == "" || s.Right == "" {
		return errors.New("session needs two input paths")
	}
	return nil
}

// Read 解析会话，按会话目录解析相对路径，并拒绝未知版本及尾随数据。
func Read(path string) (Session, error) {
	var s Session
	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return s, err
	}
	if len(data) > 1<<20 {
		return s, errors.New("session exceeds 1 MiB limit")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err = dec.Decode(&s); err != nil {
		return s, err
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return s, errors.New("unexpected trailing session data")
	}
	if err = s.validate(); err != nil {
		return s, err
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return s, err
	}
	if !filepath.IsAbs(s.Left) {
		s.Left = filepath.Join(base, s.Left)
	}
	if !filepath.IsAbs(s.Right) {
		s.Right = filepath.Join(base, s.Right)
	}
	if s.Base != "" && !filepath.IsAbs(s.Base) {
		s.Base = filepath.Join(base, s.Base)
	}
	if s.Output != "" && !filepath.IsAbs(s.Output) {
		s.Output = filepath.Join(base, s.Output)
	}
	return s, nil
}

// Write 使用同目录临时文件保存会话。仅记录路径和视图选项，不写入文件内容。
func Write(path string, s Session) error {
	if err := s.validate(); err != nil {
		return err
	}
	// 保存绝对路径，避免工作目录或会话文件位置改变时静默指向另一个输入。
	var err error
	s.Left, err = filepath.Abs(s.Left)
	if err != nil {
		return err
	}
	s.Right, err = filepath.Abs(s.Right)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if st, err := os.Lstat(path); err == nil && !st.Mode().IsRegular() {
		return errors.New("session target is not a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".gocompare-session-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err == nil {
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
