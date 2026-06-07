/*
 * @module api/controllers/upload_controller
 * @description 通用图片上传接口 — 接收 multipart/form-data 上传的图片，返回访问URL
 * @dependencies net/http, chi/render, crypto/md5
 */

package controllers

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/render"
)

// UploadImage 通用图片上传
// POST /api/v1/upload/image
// Content-Type: multipart/form-data
// Fields: file (file)
func UploadImage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB
		render.JSON(w, r, map[string]string{"error": "解析表单失败: " + err.Error()})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		render.JSON(w, r, map[string]string{"error": "未找到上传文件 (field: file): " + err.Error()})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		render.JSON(w, r, map[string]string{"error": "读取文件失败: " + err.Error()})
		return
	}
	if len(data) == 0 {
		render.JSON(w, r, map[string]string{"error": "上传文件为空"})
		return
	}

	// 保存目录
	uploadDir := os.Getenv("IMAGE_UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./data/uploaded_images"
	}
	_ = os.MkdirAll(uploadDir, 0755)

	// 生成文件名
	hash := fmt.Sprintf("%x", md5.Sum(data))[:12]
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	filename := fmt.Sprintf("img_%s_%d%s", hash, time.Now().UnixNano(), ext)
	savePath := filepath.Join(uploadDir, filename)

	if err := os.WriteFile(savePath, data, 0644); err != nil {
		render.JSON(w, r, map[string]string{"error": "保存文件失败: " + err.Error()})
		return
	}

	// 生成访问URL
	baseURL := os.Getenv("IMAGE_BASE_URL")
	if baseURL == "" {
		// 默认用请求的 host
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, r.Host)
	}
	baseCtx := os.Getenv("BASE_CONTEXT")
	imageURL := fmt.Sprintf("%s%s/uploaded-images/%s", baseURL, baseCtx, filename)

	log.Printf("[UploadImage] 上传成功: %s (%d bytes)", filename, len(data))
	render.JSON(w, r, map[string]interface{}{
		"url":       imageURL,
		"filename":  filename,
		"size":      len(data),
		"image_url": imageURL,
	})
}
