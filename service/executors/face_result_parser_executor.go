/*
 * @module service/executors/face_result_parser_executor
 * @description 人脸识别结果处理节点 — 在推理图片上绘制人脸框+信息，输出两条企业微信消息：纯文本用户信息 + 标注图base64
 * @architecture 策略模式
 * @stateFlow 解析参数 → 解码base64图片 → 绘制人脸框(绿/红)+标签 → 编码为base64 → 输出两条消息
 * @rules score >= 阈值标绿框，低于标红框，框头部显示人员信息
 * @dependencies models, image, golang.org/x/image/font/basicfont
 */

package executors

import (
	"box-manage-service/models"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// FaceResultParserExecutor 人脸识别结果处理执行器
type FaceResultParserExecutor struct {
	*BaseExecutor
}

func NewFaceResultParserExecutor() *FaceResultParserExecutor {
	return &FaceResultParserExecutor{
		BaseExecutor: NewBaseExecutor("face_result_parser"),
	}
}

func (e *FaceResultParserExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行人脸识别结果处理"}

	// 1. 解析 face_results
	faceData := e.getParam(execCtx, "face_results")
	faceResults := e.parseFaceResults(faceData)
	if len(faceResults) == 0 {
		err := fmt.Errorf("未找到上游人脸识别结果（face_results 参数为空）")
		logs = append(logs, err.Error())
		return CreateFailureResult(err, logs), err
	}
	logs = append(logs, fmt.Sprintf("人脸识别结果: %d 条", len(faceResults)))

	// 2. 获取置信度阈值
	threshold := e.getScoreThreshold(execCtx)

	// 3. 解码base64图片
	imageData := e.getParam(execCtx, "image")
	img, err := e.decodeBase64Image(imageData)
	if err != nil {
		logs = append(logs, fmt.Sprintf("图片解码失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	// 4. 在图片上绘制人脸框
	annotated := e.drawFaceBoxes(img, faceResults, threshold)
	logs = append(logs, fmt.Sprintf("已绘制 %d 个人脸框 (阈值: %.2f)", len(faceResults), threshold))

	// 5. 标注图编码为 base64（不上传、不存盘）
	imageBase64, err := e.encodeImageToBase64(annotated)
	if err != nil {
		logs = append(logs, fmt.Sprintf("图片编码失败: %v", err))
		return CreateFailureResult(err, logs), err
	}
	logs = append(logs, fmt.Sprintf("标注图base64长度: %d", len(imageBase64)))

	// 6. 构建两条消息体（纯文本，非 markdown）
	bodyText := e.buildPlainText(faceResults)
	// 标注图 base64 加上 data:image/jpeg;base64, 前缀便于使用
	bodyImage := "data:image/jpeg;base64," + imageBase64

	// @人列表
	mentionedMobileList := e.getParam(execCtx, "mentioned_mobile_list")
	bodyTextObj := map[string]interface{}{
		"content": bodyText,
	}
	if mentionedMobileList != nil {
		switch v := mentionedMobileList.(type) {
		case []interface{}:
			bodyTextObj["mentioned_mobile_list"] = v
		case string:
			if v != "" {
				parts := strings.Split(v, ",")
				mobiles := make([]interface{}, 0, len(parts))
				for _, p := range parts {
					if trimmed := strings.TrimSpace(p); trimmed != "" {
						mobiles = append(mobiles, trimmed)
					}
				}
				bodyTextObj["mentioned_mobile_list"] = mobiles
			}
		}
	}

	outputs := map[string]interface{}{
		"body_text": bodyTextObj,
		"body_image": map[string]interface{}{
			"content": bodyImage,
		},
		"face_count": len(faceResults),
	}
	extras := map[string]interface{}{
		"face_count":      len(faceResults),
		"score_threshold": threshold,
	}

	logs = append(logs, "人脸识别结果处理完成")
	return CreateSuccessResult(CreateOutputs(outputs, extras), logs), nil
}

// getParam 从节点参数中取值：优先 Inputs，回退 Variables
func (e *FaceResultParserExecutor) getParam(execCtx *ExecutionContext, key string) interface{} {
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

// getScoreThreshold 获取置信度阈值，默认0.5
func (e *FaceResultParserExecutor) getScoreThreshold(execCtx *ExecutionContext) float64 {
	v := e.getParam(execCtx, "score")
	if v == nil {
		return 0.5
	}
	switch val := v.(type) {
	case float64:
		if val > 0 {
			return val
		}
	case string:
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err == nil && f > 0 {
			return f
		}
	case json.Number:
		if f, err := val.Float64(); err == nil && f > 0 {
			return f
		}
	}
	return 0.5
}

// decodeBase64Image 解码 base64 图片
func (e *FaceResultParserExecutor) decodeBase64Image(data interface{}) (image.Image, error) {
	s, ok := data.(string)
	if !ok || s == "" {
		return nil, fmt.Errorf("image 参数为空或格式错误")
	}

	// 去掉 data:image/xxx;base64, 前缀
	b64 := s
	if idx := strings.Index(s, ";base64,"); idx != -1 {
		b64 = s[idx+8:]
	}

	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("base64解码失败: %w", err)
	}

	// 尝试 JPEG / PNG 解码
	img, err := jpeg.Decode(bytes.NewReader(decoded))
	if err == nil {
		return img, nil
	}
	img, err = png.Decode(bytes.NewReader(decoded))
	if err == nil {
		return img, nil
	}
	// 兜底：用image.Decode
	img, _, err = image.Decode(bytes.NewReader(decoded))
	return img, err
}

// drawFaceBoxes 在图片上绘制人脸框 + 标签
func (e *FaceResultParserExecutor) drawFaceBoxes(img image.Image, faces []faceRecResult, threshold float64) image.Image {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	for _, f := range faces {
		if len(f.FaceLocation) != 4 {
			continue
		}
		x := f.FaceLocation[0]
		y := f.FaceLocation[1]
		w := f.FaceLocation[2] - x
		h := f.FaceLocation[3] - y
		if w <= 0 || h <= 0 {
			continue
		}

		// 颜色：>= threshold 绿色，否则红色
		boxColor := color.RGBA{255, 0, 0, 255} // red
		labelColor := color.RGBA{255, 255, 255, 255}
		if f.Score >= threshold {
			boxColor = color.RGBA{0, 200, 0, 255} // green
		}

		// 画框 (3px)
		for t := 0; t < 3; t++ {
			e.drawRect(rgba, x-t, y-t, w+2*t, h+2*t, boxColor)
		}

		// 标签背景
		label := fmt.Sprintf("名称:%s ID:%s score:%.2f", f.UserName, f.UserID, f.Score)
		labelW := len(label) * 7 // basicfont 7px宽
		labelH := 16
		labelY := y - labelH - 2
		if labelY < 0 {
			labelY = 0
		}
		// 半透明背景
		e.fillRect(rgba, x, labelY, labelW, labelH, color.RGBA{0, 0, 0, 180})

		// 绘文字
		e.drawText(rgba, label, x+2, labelY+13, labelColor)
	}

	return rgba
}

func (e *FaceResultParserExecutor) drawRect(rgba *image.RGBA, x, y, w, h int, c color.RGBA) {
	for dy := 0; dy < h; dy++ {
		py := y + dy
		if py < 0 || py >= rgba.Bounds().Max.Y {
			continue
		}
		// 上下边
		if dy == 0 || dy == h-1 {
			for dx := 0; dx < w; dx++ {
				px := x + dx
				if px >= 0 && px < rgba.Bounds().Max.X {
					rgba.Set(px, py, c)
				}
			}
		} else {
			// 左右边
			if x >= 0 && x < rgba.Bounds().Max.X {
				rgba.Set(x, py, c)
			}
			if x+w-1 >= 0 && x+w-1 < rgba.Bounds().Max.X {
				rgba.Set(x+w-1, py, c)
			}
		}
	}
}

func (e *FaceResultParserExecutor) fillRect(rgba *image.RGBA, x, y, w, h int, c color.RGBA) {
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

func (e *FaceResultParserExecutor) drawText(rgba *image.RGBA, text string, x, y int, c color.RGBA) {
	d := &font.Drawer{
		Dst:  rgba,
		Src:  image.NewUniform(c),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

// encodeImageToBase64 标注图编码为 base64
func (e *FaceResultParserExecutor) encodeImageToBase64(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", fmt.Errorf("JPEG编码失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func (e *FaceResultParserExecutor) parseFaceResults(data interface{}) []faceRecResult {
	if data == nil {
		return nil
	}
	var arr []faceRecResult
	switch val := data.(type) {
	case []interface{}:
		b, _ := json.Marshal(val)
		json.Unmarshal(b, &arr)
	case string:
		json.Unmarshal([]byte(val), &arr)
	}
	return arr
}

func (e *FaceResultParserExecutor) Validate(nodeInstance *models.NodeInstance) error {
	return e.BaseExecutor.Validate(nodeInstance)
}

type faceRecResult struct {
	UserID       string  `json:"user_id"`
	UserName     string  `json:"user_name"`
	FaceLocation []int   `json:"face_location"`
	Score        float64 `json:"score"`
}

// buildPlainText 构建纯文本消息体（非 markdown）
func (e *FaceResultParserExecutor) buildPlainText(faces []faceRecResult) string {
	var sb strings.Builder
	sb.WriteString("【人脸匹配通知】\n\n")
	for i, f := range faces {
		loc := ""
		if len(f.FaceLocation) == 4 {
			loc = fmt.Sprintf("[%d,%d,%d,%d]", f.FaceLocation[0], f.FaceLocation[1], f.FaceLocation[2], f.FaceLocation[3])
		}
		sb.WriteString(fmt.Sprintf("%d. 姓名: %s\n", i+1, f.UserName))
		sb.WriteString(fmt.Sprintf("   用户ID: %s\n", f.UserID))
		sb.WriteString(fmt.Sprintf("   匹配得分: %.2f\n", f.Score))
		if loc != "" {
			sb.WriteString(fmt.Sprintf("   位置: %s\n", loc))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("共匹配到 %d 人", len(faces)))
	return sb.String()
}
