package ffmpeg

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// FFmpegModule FFmpeg操作模块
type FFmpegModule struct {
	ffmpegPath  string
	ffprobePath string
	timeout     time.Duration
}

// NewFFmpegModule 创建新的FFmpeg模块
func NewFFmpegModule(ffmpegPath, ffprobePath string, timeout time.Duration) *FFmpegModule {
	if timeout == 0 {
		timeout = 300 * time.Second // 默认5分钟超时
	}
	return &FFmpegModule{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		timeout:     timeout,
	}
}

// VideoInfo 视频信息
type VideoInfo struct {
	Duration  float64 `json:"duration"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	FrameRate float64 `json:"frame_rate"`
	Bitrate   int64   `json:"bitrate"`
	Codec     string  `json:"codec"`
	Format    string  `json:"format"`
	FileSize  int64   `json:"file_size"`
}

// ExtractFramesRequest 提取帧请求
type ExtractFramesRequest struct {
	InputPath   string  // 输入文件路径或流地址
	OutputDir   string  // 输出目录
	FrameCount  int     // 提取帧数量（与Duration二选一）
	Duration    int     // 提取时长（秒）（与FrameCount二选一）
	StartTime   float64 // 开始时间（秒）
	Quality     int     // 图片质量（1-31，数值越小质量越高）
	Width       int     // 输出图片宽度（0表示保持原比例）
	Height      int     // 输出图片高度（0表示保持原比例）
	Format      string  // 输出格式（jpg, png）
	NamePattern string  // 文件名模式，默认为frame_%05d
}

// ExtractFramesResult 提取帧结果
type ExtractFramesResult struct {
	OutputDir  string   `json:"output_dir"`
	FramePaths []string `json:"frame_paths"`
	FrameCount int      `json:"frame_count"`
	Duration   float64  `json:"duration"`
	Success    bool     `json:"success"`
	Error      string   `json:"error,omitempty"`
}

// ConvertVideoRequest 转换视频请求
type ConvertVideoRequest struct {
	InputPath  string   // 输入文件路径
	OutputPath string   // 输出文件路径
	Format     string   // 输出格式（mp4, flv, etc）
	VideoCodec string   // 视频编码（h264, h265, etc）
	AudioCodec string   // 音频编码（aac, mp3, etc）
	Bitrate    string   // 比特率（例如"1000k"）
	FrameRate  float64  // 帧率
	Width      int      // 宽度（0表示保持原比例）
	Height     int      // 高度（0表示保持原比例）
	StartTime  float64  // 开始时间（秒）
	Duration   float64  // 转换时长（秒，0表示全部）
	Quality    int      // 质量参数（CRF值，0-51）
	ExtraArgs  []string // 额外参数
}

// RecordStreamRequest 录制流请求
type RecordStreamRequest struct {
	StreamURL  string // 流地址
	OutputPath string // 输出文件路径
	Duration   int    // 录制时长（秒）
	Format     string // 输出格式
	VideoCodec string // 视频编码
	AudioCodec string // 音频编码
}

// ProgressCallback 进度回调函数
type ProgressCallback func(progress float64, timeProcessed float64)

// ProbeInfo ffprobe信息结构
type ProbeInfo struct {
	Format struct {
		Filename string `json:"filename"`
		Duration string `json:"duration"`
		Size     string `json:"size"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		Index     int    `json:"index"`
		CodecName string `json:"codec_name"`
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		FrameRate string `json:"r_frame_rate"`
		BitRate   string `json:"bit_rate"`
		Duration  string `json:"duration"`
	} `json:"streams"`
}

// GetVideoInfo 获取视频信息
func (f *FFmpegModule) GetVideoInfo(ctx context.Context, inputPath string) (*VideoInfo, error) {
	if f.ffprobePath == "" {
		return nil, fmt.Errorf("ffprobe path not set")
	}

	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	}

	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.ffprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe ProbeInfo
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("parse ffprobe output failed: %w", err)
	}

	info := &VideoInfo{}

	// 解析格式信息
	if probe.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
			info.Duration = duration
		}
	}

	if probe.Format.Size != "" {
		if size, err := strconv.ParseInt(probe.Format.Size, 10, 64); err == nil {
			info.FileSize = size
		}
	}

	if probe.Format.BitRate != "" {
		if bitrate, err := strconv.ParseInt(probe.Format.BitRate, 10, 64); err == nil {
			info.Bitrate = bitrate
		}
	}

	// 解析视频流信息
	for _, stream := range probe.Streams {
		if stream.CodecType == "video" {
			info.Width = stream.Width
			info.Height = stream.Height
			info.Codec = stream.CodecName

			// 解析帧率
			if stream.FrameRate != "" {
				parts := strings.Split(stream.FrameRate, "/")
				if len(parts) == 2 {
					num, _ := strconv.ParseFloat(parts[0], 64)
					den, _ := strconv.ParseFloat(parts[1], 64)
					if den != 0 {
						info.FrameRate = num / den
					}
				}
			}
			break
		}
	}

	// 获取文件统计信息
	if stat, err := os.Stat(inputPath); err == nil {
		info.FileSize = stat.Size()
	}

	return info, nil
}

// ExtractFrames 提取视频帧
func (f *FFmpegModule) ExtractFrames(ctx context.Context, req *ExtractFramesRequest) (*ExtractFramesResult, error) {
	log.Printf("[FFmpegModule] ExtractFrames started - InputPath: %s, OutputDir: %s, FrameCount: %d, Duration: %d",
		req.InputPath, req.OutputDir, req.FrameCount, req.Duration)

	if f.ffmpegPath == "" {
		log.Printf("[FFmpegModule] ExtractFrames failed - ffmpeg path not set")
		return nil, fmt.Errorf("ffmpeg path not set")
	}

	// 确保输出目录存在
	log.Printf("[FFmpegModule] Creating output directory - OutputDir: %s", req.OutputDir)
	if err := os.MkdirAll(req.OutputDir, 0755); err != nil {
		log.Printf("[FFmpegModule] Failed to create output directory - OutputDir: %s, Error: %v", req.OutputDir, err)
		return nil, fmt.Errorf("create output directory failed: %w", err)
	}

	// 构建ffmpeg命令
	log.Printf("[FFmpegModule] Building FFmpeg command arguments")
	args := []string{"-y"} // 覆盖输出文件

	// 输入参数
	if req.StartTime > 0 {
		startTimeArg := fmt.Sprintf("%.3f", req.StartTime)
		args = append(args, "-ss", startTimeArg)
		log.Printf("[FFmpegModule] Added start time parameter - StartTime: %s", startTimeArg)
	}
	args = append(args, "-i", req.InputPath)
	log.Printf("[FFmpegModule] Added input path - InputPath: %s", req.InputPath)

	// 输出参数
	if req.Duration > 0 {
		args = append(args, "-t", strconv.Itoa(req.Duration))
	}

	// 视频过滤器
	var filters []string

	// 尺寸调整
	if req.Width > 0 || req.Height > 0 {
		if req.Width > 0 && req.Height > 0 {
			filters = append(filters, fmt.Sprintf("scale=%d:%d", req.Width, req.Height))
		} else if req.Width > 0 {
			filters = append(filters, fmt.Sprintf("scale=%d:-1", req.Width))
		} else {
			filters = append(filters, fmt.Sprintf("scale=-1:%d", req.Height))
		}
	}

	// 帧选择
	if req.FrameCount > 0 {
		// 按帧数提取
		filters = append(filters, fmt.Sprintf("select=not(mod(n\\,%d))", max(1, req.Duration*25/req.FrameCount)))
	} else if req.Duration > 0 {
		// 按时长提取，每秒1帧
		filters = append(filters, "fps=1")
	} else {
		// 默认每秒1帧
		filters = append(filters, "fps=1")
	}

	if len(filters) > 0 {
		args = append(args, "-vf", strings.Join(filters, ","))
	}

	// 输出格式和质量
	format := req.Format
	if format == "" {
		format = "jpg"
	}

	if req.Quality > 0 && req.Quality <= 31 {
		args = append(args, "-q:v", strconv.Itoa(req.Quality))
	}

	// 输出文件名模式
	namePattern := req.NamePattern
	if namePattern == "" {
		namePattern = "frame_%05d"
	}
	outputPattern := filepath.Join(req.OutputDir, namePattern+"."+format)
	args = append(args, outputPattern)
	log.Printf("[FFmpegModule] Output pattern set - OutputPattern: %s", outputPattern)

	// 记录完整的FFmpeg命令
	fullCommand := fmt.Sprintf("%s %s", f.ffmpegPath, strings.Join(args, " "))
	log.Printf("[FFmpegModule] Final FFmpeg command: %s", fullCommand)

	// 执行命令
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	log.Printf("[FFmpegModule] Starting FFmpeg execution - Timeout: %v", f.timeout)
	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)

	// 获取命令输出用于调试
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[FFmpegModule] FFmpeg execution failed - Command: %s, Output: %s, Error: %v",
			fullCommand, string(output), err)
		return nil, fmt.Errorf("ffmpeg extract frames failed: %w", err)
	}

	log.Printf("[FFmpegModule] FFmpeg execution completed successfully - Output: %s", string(output))

	// 扫描输出文件
	globPattern := filepath.Join(req.OutputDir, "*."+format)
	log.Printf("[FFmpegModule] Scanning output files - Pattern: %s", globPattern)
	framePaths, err := filepath.Glob(globPattern)
	if err != nil {
		log.Printf("[FFmpegModule] Failed to scan output files - Pattern: %s, Error: %v", globPattern, err)
		return nil, fmt.Errorf("scan output files failed: %w", err)
	}

	log.Printf("[FFmpegModule] Output files scanned successfully - FrameCount: %d, FramePaths: %v",
		len(framePaths), framePaths)

	result := &ExtractFramesResult{
		OutputDir:  req.OutputDir,
		FramePaths: framePaths,
		FrameCount: len(framePaths),
		Success:    true,
	}

	log.Printf("[FFmpegModule] ExtractFrames completed successfully - OutputDir: %s, FrameCount: %d",
		result.OutputDir, result.FrameCount)

	return result, nil
}

// ConvertVideo 转换视频
func (f *FFmpegModule) ConvertVideo(ctx context.Context, req *ConvertVideoRequest) error {
	if f.ffmpegPath == "" {
		return fmt.Errorf("ffmpeg path not set")
	}

	// 确保输出目录存在
	outputDir := filepath.Dir(req.OutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory failed: %w", err)
	}

	// 构建ffmpeg命令
	args := []string{"-y"} // 覆盖输出文件

	// 输入参数
	if req.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", req.StartTime))
	}
	args = append(args, "-i", req.InputPath)

	// 时长限制
	if req.Duration > 0 {
		args = append(args, "-t", fmt.Sprintf("%.3f", req.Duration))
	}

	// 视频编码参数
	if req.VideoCodec != "" {
		args = append(args, "-c:v", req.VideoCodec)
	}

	// 音频编码参数
	if req.AudioCodec != "" {
		args = append(args, "-c:a", req.AudioCodec)
	}

	// 比特率
	if req.Bitrate != "" {
		args = append(args, "-b:v", req.Bitrate)
	}

	// 帧率
	if req.FrameRate > 0 {
		args = append(args, "-r", fmt.Sprintf("%.2f", req.FrameRate))
	}

	// 尺寸调整
	if req.Width > 0 || req.Height > 0 {
		var scale string
		if req.Width > 0 && req.Height > 0 {
			scale = fmt.Sprintf("scale=%d:%d", req.Width, req.Height)
		} else if req.Width > 0 {
			scale = fmt.Sprintf("scale=%d:-2", req.Width)
		} else {
			scale = fmt.Sprintf("scale=-2:%d", req.Height)
		}
		args = append(args, "-vf", scale)
	}

	// 质量参数
	if req.Quality > 0 && req.Quality <= 51 {
		args = append(args, "-crf", strconv.Itoa(req.Quality))
	}

	// 额外参数
	args = append(args, req.ExtraArgs...)

	// 输出文件
	args = append(args, req.OutputPath)

	// 执行命令
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)
	return cmd.Run()
}

// RecordStream 录制流
func (f *FFmpegModule) RecordStream(ctx context.Context, req *RecordStreamRequest) error {
	if f.ffmpegPath == "" {
		return fmt.Errorf("ffmpeg path not set")
	}

	// 确保输出目录存在
	outputDir := filepath.Dir(req.OutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory failed: %w", err)
	}

	// 构建ffmpeg命令
	args := []string{
		"-y", // 覆盖输出文件
		"-i", req.StreamURL,
	}

	// 录制时长
	if req.Duration > 0 {
		args = append(args, "-t", strconv.Itoa(req.Duration))
	}

	// 编码参数
	if req.VideoCodec != "" {
		args = append(args, "-c:v", req.VideoCodec)
	} else {
		args = append(args, "-c:v", "copy") // 默认复制流
	}

	if req.AudioCodec != "" {
		args = append(args, "-c:a", req.AudioCodec)
	} else {
		args = append(args, "-c:a", "copy") // 默认复制流
	}

	// 输出格式
	if req.Format != "" {
		args = append(args, "-f", req.Format)
	}

	args = append(args, req.OutputPath)

	// 执行命令
	ctx, cancel := context.WithTimeout(ctx, time.Duration(req.Duration+30)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)
	return cmd.Run()
}

// ConvertVideoWithProgress 带进度回调的视频转换
func (f *FFmpegModule) ConvertVideoWithProgress(ctx context.Context, req *ConvertVideoRequest, callback ProgressCallback) error {
	if f.ffmpegPath == "" {
		return fmt.Errorf("ffmpeg path not set")
	}

	// 首先获取视频信息以计算进度
	videoInfo, err := f.GetVideoInfo(ctx, req.InputPath)
	if err != nil {
		return fmt.Errorf("get video info failed: %w", err)
	}

	totalDuration := videoInfo.Duration
	if req.Duration > 0 && req.Duration < totalDuration {
		totalDuration = req.Duration
	}

	// 确保输出目录存在
	outputDir := filepath.Dir(req.OutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output directory failed: %w", err)
	}

	// 构建ffmpeg命令（与ConvertVideo相同的逻辑）
	args := []string{"-y", "-progress", "pipe:1"} // 输出进度到stdout

	if req.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", req.StartTime))
	}
	args = append(args, "-i", req.InputPath)

	if req.Duration > 0 {
		args = append(args, "-t", fmt.Sprintf("%.3f", req.Duration))
	}

	if req.VideoCodec != "" {
		args = append(args, "-c:v", req.VideoCodec)
	}
	if req.AudioCodec != "" {
		args = append(args, "-c:a", req.AudioCodec)
	}
	if req.Bitrate != "" {
		args = append(args, "-b:v", req.Bitrate)
	}
	if req.FrameRate > 0 {
		args = append(args, "-r", fmt.Sprintf("%.2f", req.FrameRate))
	}

	if req.Width > 0 || req.Height > 0 {
		var scale string
		if req.Width > 0 && req.Height > 0 {
			scale = fmt.Sprintf("scale=%d:%d", req.Width, req.Height)
		} else if req.Width > 0 {
			scale = fmt.Sprintf("scale=%d:-2", req.Width)
		} else {
			scale = fmt.Sprintf("scale=-2:%d", req.Height)
		}
		args = append(args, "-vf", scale)
	}

	if req.Quality > 0 && req.Quality <= 51 {
		args = append(args, "-crf", strconv.Itoa(req.Quality))
	}

	args = append(args, req.ExtraArgs...)
	args = append(args, req.OutputPath)

	// 执行命令并解析进度
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe failed: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg failed: %w", err)
	}

	// 解析进度输出
	scanner := bufio.NewScanner(stdout)
	timeRegex := regexp.MustCompile(`out_time_ms=(\d+)`)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := timeRegex.FindStringSubmatch(line); len(matches) > 1 {
			timeMicros, _ := strconv.ParseInt(matches[1], 10, 64)
			timeSeconds := float64(timeMicros) / 1000000.0

			if totalDuration > 0 {
				progress := (timeSeconds / totalDuration) * 100
				if progress > 100 {
					progress = 100
				}
				if callback != nil {
					callback(progress, timeSeconds)
				}
			}
		}
	}

	return cmd.Wait()
}

// TakeScreenshot 截取单张图片
func (f *FFmpegModule) TakeScreenshot(ctx context.Context, inputURL, outputPath string) error {
	log.Printf("[FFmpegModule] TakeScreenshot started - InputURL: %s, OutputPath: %s", inputURL, outputPath)

	if f.ffmpegPath == "" {
		log.Printf("[FFmpegModule] TakeScreenshot failed - ffmpeg path not set")
		return fmt.Errorf("ffmpeg path not set")
	}

	// 确保输出目录存在
	outputDir := filepath.Dir(outputPath)
	log.Printf("[FFmpegModule] Creating output directory - OutputDir: %s", outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("[FFmpegModule] Failed to create output directory - OutputDir: %s, Error: %v", outputDir, err)
		return fmt.Errorf("create output directory failed: %w", err)
	}

	// 构建ffmpeg命令 - 专门用于单张截图
	args := []string{
		"-y",           // 覆盖输出文件
		"-i", inputURL, // 输入URL
		"-vframes", "1", // 只输出一帧
		"-q:v", "2", // 图片质量
		"-f", "image2", // 强制输出格式为单张图片
		outputPath, // 输出文件路径
	}

	// 记录完整的FFmpeg命令
	fullCommand := fmt.Sprintf("%s %s", f.ffmpegPath, strings.Join(args, " "))
	log.Printf("[FFmpegModule] Final FFmpeg screenshot command: %s", fullCommand)

	// 执行命令
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	log.Printf("[FFmpegModule] Starting FFmpeg screenshot execution - Timeout: %v", f.timeout)
	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)

	// 获取命令输出用于调试
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[FFmpegModule] FFmpeg screenshot execution failed - Command: %s, Output: %s, Error: %v",
			fullCommand, string(output), err)
		return fmt.Errorf("ffmpeg screenshot failed: %w", err)
	}

	log.Printf("[FFmpegModule] FFmpeg screenshot execution completed successfully - Output: %s", string(output))

	// 检查输出文件是否存在
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		log.Printf("[FFmpegModule] Screenshot file does not exist - OutputPath: %s", outputPath)
		return fmt.Errorf("screenshot file not created: %s", outputPath)
	}

	log.Printf("[FFmpegModule] TakeScreenshot completed successfully - OutputPath: %s", outputPath)
	return nil
}

// max 返回两个整数中的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
