/*
 * @Author: Malin Xie
 * @Description:
 * @Date: 2021-07-07 17:39:12
 */
package web

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

type WebDir struct {
	Prefix      string
	EmbedPrefix string
	Content     embed.FS
	Embbed      bool
}

type webfile struct {
	io.Seeker
	fs.File
}

// Readdir implments Readdir mehtod for http.File
func (*webfile) Readdir(count int) ([]fs.FileInfo, error) {
	return nil, nil
}

// func (f *webfile) Seek(offset int64, whence int) (int64, error) {
// 	return -1, nil
// }

// Open implments Open method
func (d WebDir) Open(name string) (http.File, error) {
	if !d.Embbed { // if not embed mode
		ff := http.Dir(d.Prefix)
		f, err := ff.Open(name)
		if err != nil {
			return nil, err
		}
		return f, nil
	}

	if filepath.Separator != '/' && strings.ContainsRune(name, filepath.Separator) {
		return nil, errors.New("http: invalid character in file path")
	}
	dir := d.EmbedPrefix
	if dir == "" {
		dir = "."
	}
	fullName := filepath.Join(dir, filepath.FromSlash(path.Clean("/"+name)))
	fullName = strings.ReplaceAll(fullName, "\\", "/")
	f, err := d.Content.Open(fullName) // open from embed.FS
	if err != nil {
		return nil, err
	}
	wf := &webfile{File: f}
	return wf, nil
}

// template FS mock
type TemplateFS struct {
	Content     embed.FS
	Embbed      bool
	Current     string
	DelimsLeft  string
	DelimsRigth string
}

func (t TemplateFS) Parse(engine *gin.Engine, nonEmbedPath string, patterns ...string) error {
	if t.Embbed {
		templ := template.Must(template.New("").Delims(t.DelimsLeft, t.DelimsRigth).Funcs(engine.FuncMap).ParseFS(t.Content, patterns...))
		engine.SetHTMLTemplate(templ)
		return nil
	}
	var filenames []string
	for _, pattern := range patterns {
		list, err := fs.Glob(t.Content, pattern)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return fmt.Errorf("template: pattern matches no files: %#q", pattern)
		}
		for _, path := range list {
			vpath := nonEmbedPath + "/" + path
			filenames = append(filenames, vpath)
		}
	}
	engine.Delims(t.DelimsLeft, t.DelimsRigth)
	engine.LoadHTMLFiles(filenames...)
	return nil
}
