/*
 * @module api/controllers/mock_face_controller
 * @description 人脸比对模拟接口，供 box-app 测试 face_compare 节点使用
 * @dependencies net/http
 */

package controllers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/render"
)

// MockFaceCompareResponse 模拟人脸比对响应
type MockFaceCompareResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    []interface{} `json:"data"`
}

// MockFaceCompare 模拟人脸比对接口
// POST /compareFaceImg
// Content-Type: multipart/form-data
// Fields: score (string), imgs (file[])
func MockFaceCompare(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20MB
		render.JSON(w, r, map[string]string{"error": "解析表单失败: " + err.Error()})
		return
	}

	score := r.FormValue("score")

	// 模拟无匹配：score > 0.99（优先于文件检查，方便测试）
	if s, err := strconv.ParseFloat(score, 64); err == nil && s > 0.99 {
		log.Printf("[MockFaceCompare] score=%.2f > 0.99, 返回 no match", s)
		render.JSON(w, r, map[string]string{"message": "no match"})
		return
	}

	// 异常情况 1: 没有上传图片
	files := r.MultipartForm.File["imgs"]
	if len(files) == 0 {
		log.Printf("[MockFaceCompare] 未上传图片, 返回错误")
		render.JSON(w, r, map[string]interface{}{
			"error": "compare_face_img OpenCV(4.11.0) ... !buf.empty() in function 'cv::imdecode_'",
		})
		return
	}

	// 异常情况 2: 检查是否有空文件/损坏文件
	for _, fh := range files {
		if fh.Size == 0 {
			log.Printf("[MockFaceCompare] 文件 %s 为空, 返回错误", fh.Filename)
			render.JSON(w, r, map[string]interface{}{
				"error": "compare_face_img OpenCV(4.11.0) ... !buf.empty() in function 'cv::imdecode_'",
			})
			return
		}
	}

	log.Printf("[MockFaceCompare] 收到比对请求: score=%s, images=%d", score, len(files))

	// 模拟人脸识别结果
	mockResults := [][]map[string]interface{}{
		{
			{
				"user_id":       "70239882",
				"user_name":     "黄子龙",
				"face_location": []int{110, 112, 290, 365},
				"score":         1.0,
			},
			{
				"user_id":       "70239883",
				"user_name":     "张三",
				"face_location": []int{310, 412, 290, 365},
				"score":         0.92,
			},
		},
	}

	log.Printf("[MockFaceCompare] 返回模拟结果: %d 组", len(mockResults))
	render.JSON(w, r, mockResults)
}
