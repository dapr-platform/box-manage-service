/*
 * @module service/executors/image_annotator_executor
 * @description 图片标注节点 — 在图片上绘制检测框+标签，可选上传获取URL
 * @architecture 策略模式
 * @stateFlow 解码base64 → 补充class_name → 画绿色框+标签 → 编码输出 → 可选上传
 * @rules COCO 80类名映射 class_id→class_name
 * @dependencies models, image
 */

package executors

import (
	"box-manage-service/models"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// COCO 80 类别名
var cocoNames = []string{
	"person", "bicycle", "car", "motorcycle", "airplane", "bus", "train", "truck", "boat",
	"traffic light", "fire hydrant", "stop sign", "parking meter", "bench", "bird", "cat",
	"dog", "horse", "sheep", "cow", "elephant", "bear", "zebra", "giraffe", "backpack",
	"umbrella", "handbag", "tie", "suitcase", "frisbee", "skis", "snowboard", "sports ball",
	"kite", "baseball bat", "baseball glove", "skateboard", "surfboard", "tennis racket",
	"bottle", "wine glass", "cup", "fork", "knife", "spoon", "bowl", "banana", "apple",
	"sandwich", "orange", "broccoli", "carrot", "hot dog", "pizza", "donut", "cake", "chair",
	"couch", "potted plant", "bed", "dining table", "toilet", "tv", "laptop", "mouse",
	"remote", "keyboard", "cell phone", "microwave", "oven", "toaster", "sink",
	"refrigerator", "book", "clock", "vase", "scissors", "teddy bear", "hair drier", "toothbrush",
}

// ImageAnnotatorExecutor 图片标注执行器
type ImageAnnotatorExecutor struct {
	*BaseExecutor
}

func NewImageAnnotatorExecutor() *ImageAnnotatorExecutor {
	return &ImageAnnotatorExecutor{
		BaseExecutor: NewBaseExecutor("image_annotator"),
	}
}

func (e *ImageAnnotatorExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行图片标注"}

	// 1. 参数
	imageData := e.getStringParam(execCtx, "image", "")
	detections := e.getArrayParam(execCtx, "detections")
	upload := e.getBoolParam(execCtx, "upload")
	uploadURL := e.getStringParam(execCtx, "upload_url", "")

	if imageData == "" {
		err := fmt.Errorf("缺少待标注图片 (image)")
		logs = append(logs, err.Error())
		return CreateFailureResult(err, logs), err
	}

	if len(detections) == 0 {
		// 无检测结果，直接透传
		outputs := map[string]interface{}{
			"image":     imageData,
			"image_url": "",
		}
		return CreateSuccessResult(CreateOutputs(outputs, nil), logs), nil
	}
	logs = append(logs, fmt.Sprintf("标注 %d 个检测框", len(detections)))

	// 2. 解码 base64
	img, err := e.decodeBase64Image(imageData)
	if err != nil {
		logs = append(logs, fmt.Sprintf("图片解码失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	// 3. 补充 class_name
	e.ensureClassName(detections)
	// 4. 画标注
	annotated := e.drawDetections(img, detections)

	// 5. 编码 base64
	outBase64, err := e.encodeJPEGBase64(annotated)
	if err != nil {
		logs = append(logs, fmt.Sprintf("图片编码失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	outputs := map[string]interface{}{
		"image":      outBase64,
		"image_url":  "",
		"detections": detections,
	}

	// 6. 上传
	if upload && uploadURL != "" {
		// TODO: implement HTTP upload
		logs = append(logs, fmt.Sprintf("图片上传功能待实现: %s", uploadURL))
	}

	logs = append(logs, "图片标注完成")
	return CreateSuccessResult(CreateOutputs(outputs, map[string]interface{}{
		"detection_count": len(detections),
	}), logs), nil
}

func (e *ImageAnnotatorExecutor) ensureClassName(detections []interface{}) {
	for _, d := range detections {
		m, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := m["class_name"]; ok {
			continue
		}
		cid := getClassID(m)
		m["class_name"] = getCocoName(cid)
	}
}

func getCocoName(id int) string {
	if id >= 0 && id < len(cocoNames) {
		return cocoNames[id]
	}
	return fmt.Sprintf("class_%d", id)
}

func (e *ImageAnnotatorExecutor) decodeBase64Image(s string) (image.Image, error) {
	b64 := s
	if idx := len(s); idx > 8 && s[:8] == "data:ima" {
		// find ;base64,
		for i := 0; i < len(s)-7; i++ {
			if s[i:i+8] == ";base64," {
				b64 = s[i+8:]
				break
			}
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64解码失败: %w", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(decoded))
	if err == nil {
		return img, nil
	}
	img, _, err = image.Decode(bytes.NewReader(decoded))
	return img, err
}

func (e *ImageAnnotatorExecutor) encodeJPEGBase64(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", fmt.Errorf("JPEG编码失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func (e *ImageAnnotatorExecutor) drawDetections(img image.Image, detections []interface{}) image.Image {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	for _, d := range detections {
		m, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		x, y, w, h := getBBox(m)
		if w <= 0 || h <= 0 {
			continue
		}

		// 绿框 (3px)
		green := color.RGBA{0, 255, 0, 255}
		for t := 0; t < 3; t++ {
			drawRect(rgba, x-t, y-t, w+2*t, h+2*t, green)
		}

		// 标签
		label := fmt.Sprintf("%s %.2f", getString(m, "class_name", "?"), getConfidence(m))
		labelW := len(label) * 7
		labelH := 16
		labelY := y - labelH - 2
		if labelY < 0 {
			labelY = 0
		}
		// 绿色背景
		fillRect(rgba, x, labelY, labelW, labelH, green)
		// 黑色文字
		drawText(rgba, label, x+2, labelY+13, color.RGBA{0, 0, 0, 255})
	}

	return rgba
}

func getBBox(m map[string]interface{}) (int, int, int, int) {
	// 优先 bbox
	if bb, ok := m["bbox"].(map[string]interface{}); ok {
		x := getInt(bb, "x1", getInt(bb, "x", 0))
		y := getInt(bb, "y1", getInt(bb, "y", 0))
		x2 := getInt(bb, "x2", 0)
		y2 := getInt(bb, "y2", 0)
		return x, y, x2 - x, y2 - y
	}
	// 其次 leftTopX/Y + rightBtmX/Y
	if _, ok := m["leftTopX"]; ok {
		x := getInt(m, "leftTopX", 0)
		y := getInt(m, "leftTopY", 0)
		w := getInt(m, "rightBtmX", 0) - x
		h := getInt(m, "rightBtmY", 0) - y
		return x, y, w, h
	}
	return 0, 0, 0, 0
}

func getInt(m map[string]interface{}, key string, defaultVal int) int {
	v, ok := m[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	}
	return defaultVal
}

func getString(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}

func getConfidence(m map[string]interface{}) float64 {
	if v, ok := m["confidence"]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case string:
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				return f
			}
		}
	}
	if v, ok := m["score"]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case string:
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				return f
			}
		}
	}
	return 0.0
}

func (e *ImageAnnotatorExecutor) getArrayParam(execCtx *ExecutionContext, key string) []interface{} {
	v := e.getParam(execCtx, key)
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []interface{}:
		return val
	}
	return nil
}

func (e *ImageAnnotatorExecutor) getStringParam(execCtx *ExecutionContext, key, defaultVal string) string {
	v := e.getParam(execCtx, key)
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return defaultVal
}

func (e *ImageAnnotatorExecutor) getBoolParam(execCtx *ExecutionContext, key string) bool {
	v := e.getParam(execCtx, key)
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "1"
	}
	return false
}

func (e *ImageAnnotatorExecutor) getParam(execCtx *ExecutionContext, key string) interface{} {
	if execCtx.Inputs != nil {
		if v, ok := execCtx.Inputs[key]; ok && v != nil {
			return v
		}
	}
	if execCtx.Variables != nil {
		if v, ok := execCtx.Variables[key]; ok && v != nil {
			return v
		}
	}
	return nil
}

func (e *ImageAnnotatorExecutor) Validate(nodeInstance *models.NodeInstance) error {
	return e.BaseExecutor.Validate(nodeInstance)
}

// 绘图辅助函数
func drawRect(rgba *image.RGBA, x, y, w, h int, c color.RGBA) {
	for dy := 0; dy < h; dy++ {
		py := y + dy
		if py < 0 || py >= rgba.Bounds().Max.Y {
			continue
		}
		if dy == 0 || dy == h-1 {
			for dx := 0; dx < w; dx++ {
				px := x + dx
				if px >= 0 && px < rgba.Bounds().Max.X {
					rgba.Set(px, py, c)
				}
			}
		} else {
			if x >= 0 && x < rgba.Bounds().Max.X {
				rgba.Set(x, py, c)
			}
			if x+w-1 >= 0 && x+w-1 < rgba.Bounds().Max.X {
				rgba.Set(x+w-1, py, c)
			}
		}
	}
}

func fillRect(rgba *image.RGBA, x, y, w, h int, c color.RGBA) {
	for dy := 0; dy < h; dy++ {
		py := y + dy
		if py < 0 || py >= rgba.Bounds().Max.Y {
			continue
		}
		for dx := 0; dx < w; dx++ {
			px := x + dx
			if px >= 0 && px < rgba.Bounds().Max.X {
				rgba.Set(px, py, c)
			}
		}
	}
}

func drawText(rgba *image.RGBA, text string, x, y int, c color.RGBA) {
	d := &font.Drawer{
		Dst:  rgba,
		Src:  image.NewUniform(c),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}
