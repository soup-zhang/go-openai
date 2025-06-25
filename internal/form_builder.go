package openai

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
)

type FormBuilder interface {
	CreateFormFile(fieldname string, file *os.File) error
	CreateFormFileContentType(fieldname string, file *os.File) error
	CreateFormFileReader(fieldname string, r io.Reader, filename string) error
	WriteField(fieldname, value string) error
	Close() error
	FormDataContentType() string
}

type DefaultFormBuilder struct {
	writer *multipart.Writer
}

func NewFormBuilder(body io.Writer) *DefaultFormBuilder {
	return &DefaultFormBuilder{
		writer: multipart.NewWriter(body),
	}
}

func (fb *DefaultFormBuilder) CreateFormFile(fieldname string, file *os.File) error {
	return fb.createFormFile(fieldname, file, file.Name())
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// CreateFormFileReader creates a form field with a file reader.
// The filename in Content-Disposition is required.
func (fb *DefaultFormBuilder) CreateFormFileReader(fieldname string, r io.Reader, filename string) error {
	if filename == "" {
		if f, ok := r.(interface{ Name() string }); ok {
			filename = f.Name()
		}
	}
	var contentType string
	if f, ok := r.(interface{ ContentType() string }); ok {
		contentType = f.ContentType()
	}

	h := make(textproto.MIMEHeader)
	h.Set(
		"Content-Disposition",
		fmt.Sprintf(
			`form-data; name="%s"; filename="%s"`,
			escapeQuotes(fieldname),
			escapeQuotes(filepath.Base(filename)),
		),
	)
	// content type is optional, but it can be set
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}

	fieldWriter, err := fb.writer.CreatePart(h)
	if err != nil {
		return err
	}

	_, err = io.Copy(fieldWriter, r)
	if err != nil {
		return err
	}

	return nil
}

func (fb *DefaultFormBuilder) createFormFile(fieldname string, r io.Reader, filename string) error {
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	fieldWriter, err := fb.writer.CreateFormFile(fieldname, filename)
	if err != nil {
		return err
	}

	_, err = io.Copy(fieldWriter, r)
	if err != nil {
		return err
	}

	return nil
}

func (fb *DefaultFormBuilder) WriteField(fieldname, value string) error {
	if fieldname == "" {
		return fmt.Errorf("fieldname cannot be empty")
	}
	return fb.writer.WriteField(fieldname, value)
}

func (fb *DefaultFormBuilder) Close() error {
	return fb.writer.Close()
}

func (fb *DefaultFormBuilder) FormDataContentType() string {
	return fb.writer.FormDataContentType()
}

func (fb *DefaultFormBuilder) CreateFormFileContentType(fieldname string, file *os.File) error {
	if file == nil {
		return fmt.Errorf("file cannot be nil")
	}

	// 获取文件名
	filename := filepath.Base(file.Name())
	if filename == "" {
		return fmt.Errorf("cannot get filename from file")
	}

	// 获取文件的 MIME 类型
	contentType, err := getFileContentType(file)
	if err != nil {
		return err
	}

	// 创建 multipart writer
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes(fieldname), escapeQuotes(filename)))
	h.Set("Content-Type", contentType)

	// 创建表单字段
	fieldWriter, err := fb.writer.CreatePart(h)
	if err != nil {
		return err
	}

	// 确保文件指针回到开始位置
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// 复制文件内容
	_, err = io.Copy(fieldWriter, file)
	if err != nil {
		return err
	}

	return nil
}

// getFileContentType 检测文件的 MIME 类型
func getFileContentType(file *os.File) (string, error) {
	// 保存当前文件位置
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", err
	}
	defer file.Seek(currentPos, io.SeekStart)

	// 首先通过文件扩展名判断
	ext := strings.ToLower(filepath.Ext(file.Name()))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg", nil
	case ".png":
		return "image/png", nil
	case ".gif":
		return "image/gif", nil
	case ".webp":
		return "image/webp", nil
	case ".bmp":
		return "image/bmp", nil
	case ".svg":
		return "image/svg+xml", nil
	case ".tiff", ".tif":
		return "image/tiff", nil
	}

	// 通过文件头检测
	// 读取文件头部分
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	// 检测内容类型
	contentType := http.DetectContentType(buffer)

	// 如果检测到的是通用二进制流，且有文件扩展名，使用扩展名判断
	if contentType == "application/octet-stream" && ext != "" {
		// 可以添加更多的扩展名映射
		mimeTypes := map[string]string{
			".webp": "image/webp",
			".svg":  "image/svg+xml",
			".tiff": "image/tiff",
			".tif":  "image/tiff",
		}
		if mime, ok := mimeTypes[ext]; ok {
			return mime, nil
		}
	}

	return contentType, nil
}

// escapeQuotes 转义引号
func escapeQuotes(s string) string {
	return strings.Replace(s, `"`, `\"`, -1)
}
