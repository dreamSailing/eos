package tools

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dreamSailing/eos/internal/ai"
	"github.com/dreamSailing/eos/internal/config"
	"github.com/dreamSailing/eos/internal/pkg/utils"
)

func (m *Manager) imageGenerateStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	prompt := strings.TrimSpace(asString(params["prompt"]))
	if prompt == "" {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolImageGenerate,
			Status:  "error",
			Error:   "prompt is required",
			Display: "错误：prompt 参数为必填项",
		}
	}
	count := asIntValue(params["count"], 1)
	if count < 1 {
		count = 1
	}
	if count > 4 {
		count = 4
	}

	route, err := resolveMultimodalRoute(ai.CapabilityImageGeneration, asString(params["model"]))
	if err != nil {
		return multimodalRouteErrorResult(ToolImageGenerate, "图片生成", err)
	}
	images, err := ai.GenerateImage(ctx, route.APIBase, route.APIKey, route.Model, prompt, asString(params["size"]), count)
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolImageGenerate,
			Status:  "error",
			Error:   err.Error(),
			Display: "错误：图片生成失败：" + err.Error(),
		}
	}
	if len(images) == 0 {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolImageGenerate,
			Status:  "error",
			Error:   "image generation returned no result",
			Display: "错误：图片生成未返回结果",
		}
	}

	paths, err := saveGeneratedMediaSet(ctx, asString(params["output_path"]), images, "image", ".png")
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolImageGenerate,
			Status:  "error",
			Error:   err.Error(),
			Display: "错误：保存图片失败：" + err.Error(),
		}
	}
	revisedPrompts := make([]string, 0, len(images))
	for _, item := range images {
		if s := strings.TrimSpace(item.RevisedPrompt); s != "" {
			revisedPrompts = append(revisedPrompts, s)
		}
	}
	data := map[string]interface{}{
		"paths":           paths,
		"count":           len(paths),
		"model":           route.Model,
		"route_source":    route.Source,
		"revised_prompts": revisedPrompts,
	}
	if len(paths) == 1 {
		data["path"] = paths[0]
	}
	return ToolResult{
		Type:    "tool_result",
		Tool:    ToolImageGenerate,
		Status:  "success",
		Data:    data,
		Display: fmt.Sprintf("已生成 %d 张图片：%s", len(paths), strings.Join(paths, ", ")),
	}
}

func (m *Manager) videoGenerateStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	prompt := strings.TrimSpace(asString(params["prompt"]))
	if prompt == "" {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolVideoGenerate,
			Status:  "error",
			Error:   "prompt is required",
			Display: "错误：prompt 参数为必填项",
		}
	}

	var imageBytes []byte
	var imageMIME string
	imageInput := strings.TrimSpace(asString(params["image_input"]))
	if imageInput != "" {
		res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), imageInput)
		if !res.IsValid {
			return ToolResult{
				Type:    "tool_result",
				Tool:    ToolVideoGenerate,
				Status:  "error",
				Error:   res.ErrMsg,
				Display: "错误：输入图片路径超出工作区范围",
			}
		}
		bs, err := os.ReadFile(res.AbsPath)
		if err != nil {
			return ToolResult{
				Type:    "tool_result",
				Tool:    ToolVideoGenerate,
				Status:  "error",
				Error:   err.Error(),
				Display: "错误：读取输入图片失败：" + err.Error(),
			}
		}
		imageBytes = bs
		imageMIME = detectMediaMIME(bs, filepath.Ext(res.AbsPath))
	}

	route, err := resolveMultimodalRoute(ai.CapabilityVideoGeneration, asString(params["model"]))
	if err != nil {
		return multimodalRouteErrorResult(ToolVideoGenerate, "视频生成", err)
	}
	video, err := ai.GenerateVideo(ctx, route.APIBase, route.APIKey, route.Model, ai.VideoOptions{
		Prompt:          prompt,
		DurationSeconds: asIntValue(params["duration_seconds"], 0),
		AspectRatio:     asString(params["aspect_ratio"]),
		ImageInput:      imageBytes,
		ImageMimeType:   imageMIME,
	})
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolVideoGenerate,
			Status:  "error",
			Error:   err.Error(),
			Display: "错误：视频生成失败：" + err.Error(),
		}
	}
	paths, err := saveGeneratedMediaSet(ctx, asString(params["output_path"]), []ai.GeneratedMedia{video}, "video", ".mp4")
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolVideoGenerate,
			Status:  "error",
			Error:   err.Error(),
			Display: "错误：保存视频失败：" + err.Error(),
		}
	}
	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolVideoGenerate,
		Status: "success",
		Data: map[string]interface{}{
			"path":         paths[0],
			"model":        route.Model,
			"route_source": route.Source,
			"request_id":   strings.TrimSpace(video.RequestID),
			"mime_type":    strings.TrimSpace(video.MIMEType),
		},
		Display: fmt.Sprintf("已生成视频：%s", paths[0]),
	}
}

func (m *Manager) speechSynthesizeStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	text := strings.TrimSpace(asString(params["text"]))
	if text == "" {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolSpeechSynthesize,
			Status:  "error",
			Error:   "text is required",
			Display: "错误：text 参数为必填项",
		}
	}
	format := strings.ToLower(strings.TrimSpace(asString(params["format"])))
	if format == "" {
		format = "mp3"
	}

	route, err := resolveMultimodalRoute(ai.CapabilitySpeechSynthesis, asString(params["model"]))
	if err != nil {
		return multimodalRouteErrorResult(ToolSpeechSynthesize, "语音合成", err)
	}
	audioOut, err := ai.SynthesizeSpeech(ctx, route.APIBase, route.APIKey, route.Model, text, asString(params["voice"]), format)
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolSpeechSynthesize,
			Status:  "error",
			Error:   err.Error(),
			Display: "错误：语音合成失败：" + err.Error(),
		}
	}
	paths, err := saveGeneratedMediaSet(ctx, asString(params["output_path"]), []ai.GeneratedMedia{audioOut}, "speech", "."+format)
	if err != nil {
		return ToolResult{
			Type:    "tool_result",
			Tool:    ToolSpeechSynthesize,
			Status:  "error",
			Error:   err.Error(),
			Display: "错误：保存语音失败：" + err.Error(),
		}
	}
	return ToolResult{
		Type:   "tool_result",
		Tool:   ToolSpeechSynthesize,
		Status: "success",
		Data: map[string]interface{}{
			"path":         paths[0],
			"model":        route.Model,
			"route_source": route.Source,
			"mime_type":    strings.TrimSpace(audioOut.MIMEType),
			"format":       format,
		},
		Display: fmt.Sprintf("已生成语音：%s", paths[0]),
	}
}

type multimodalRoute struct {
	APIKey  string
	APIBase string
	Model   string
	Source  string
}

func resolveMultimodalRoute(capability ai.Capability, overrideModel string) (multimodalRoute, error) {
	cfg, _ := config.Load()
	overrideModel = strings.TrimSpace(overrideModel)
	if overrideModel != "" {
		if entry, ok := config.FindModelByName(cfg, overrideModel); ok && config.ModelEnabled(entry) {
			return multimodalRoute{
				APIKey:  firstNonEmpty(entry.APIKey),
				APIBase: firstNonEmpty(entry.APIBase),
				Model:   firstNonEmpty(entry.Model, overrideModel),
				Source:  "model_override",
			}, nil
		}
	}
	if route, err := ai.ResolveCapabilityRoute(cfg, capability); err == nil {
		resolved := multimodalRoute{
			APIKey:  firstNonEmpty(route.Entry.APIKey),
			APIBase: firstNonEmpty(route.Entry.APIBase),
			Model:   firstNonEmpty(route.Entry.Model),
			Source:  route.Source,
		}
		if overrideModel != "" {
			resolved.Model = overrideModel
			resolved.Source += "+model_id_override"
		}
		if strings.TrimSpace(resolved.APIKey) != "" && strings.TrimSpace(resolved.APIBase) != "" && strings.TrimSpace(resolved.Model) != "" {
			return resolved, nil
		}
	}
	if active, ok := config.ActiveModel(cfg); ok && config.ModelEnabled(active) {
		resolved := multimodalRoute{
			APIKey:  firstNonEmpty(active.APIKey),
			APIBase: firstNonEmpty(active.APIBase),
			Model:   firstNonEmpty(active.Model),
			Source:  "primary_fallback",
		}
		if overrideModel != "" {
			resolved.Model = overrideModel
			resolved.Source += "+model_id_override"
		}
		if strings.TrimSpace(resolved.APIKey) != "" && strings.TrimSpace(resolved.APIBase) != "" && strings.TrimSpace(resolved.Model) != "" {
			return resolved, nil
		}
	}
	apiKey, base, model := ai.ResolveAPISettings()
	if overrideModel != "" {
		model = overrideModel
	}
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(base) == "" || strings.TrimSpace(model) == "" {
		return multimodalRoute{}, fmt.Errorf("missing API settings for capability %s", capability)
	}
	return multimodalRoute{
		APIKey:  apiKey,
		APIBase: base,
		Model:   model,
		Source:  "env_fallback",
	}, nil
}

func multimodalRouteErrorResult(toolName, capabilityName string, err error) ToolResult {
	if err == ai.ErrCapabilityModelUnavailable {
		return ToolResult{
			Type:    "tool_result",
			Tool:    toolName,
			Status:  "error",
			Error:   err.Error(),
			Display: fmt.Sprintf("错误：未配置支持%s的模型", capabilityName),
		}
	}
	return ToolResult{
		Type:    "tool_result",
		Tool:    toolName,
		Status:  "error",
		Error:   err.Error(),
		Display: fmt.Sprintf("错误：%s路由失败：%s", capabilityName, err.Error()),
	}
}

func saveGeneratedMediaSet(ctx context.Context, requestedPath string, items []ai.GeneratedMedia, prefix, defaultExt string) ([]string, error) {
	outputs, err := planOutputPaths(ctx, requestedPath, items, prefix, defaultExt)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(outputs))
	for i, out := range outputs {
		if err := sandboxWriteError(ctx, out.AbsPath); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(out.AbsPath), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(out.AbsPath, items[i].Bytes, 0o644); err != nil {
			return nil, err
		}
		paths = append(paths, out.RelPath)
	}
	return paths, nil
}

type plannedOutputPath struct {
	AbsPath string
	RelPath string
}

func planOutputPaths(ctx context.Context, requestedPath string, items []ai.GeneratedMedia, prefix, defaultExt string) ([]plannedOutputPath, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no generated media to save")
	}
	count := len(items)
	requestedPath = strings.TrimSpace(requestedPath)
	paths := make([]plannedOutputPath, 0, count)
	if requestedPath != "" && count == 1 {
		ext := chooseOutputExt(requestedPath, items[0].MIMEType, defaultExt)
		res, err := resolveOutputPath(ctx, ensureExt(requestedPath, ext))
		if err != nil {
			return nil, err
		}
		return append(paths, res), nil
	}

	timestamp := time.Now().Format("20060102-150405")
	basePath := requestedPath
	if basePath == "" {
		basePath = filepath.ToSlash(filepath.Join("outputs", fmt.Sprintf("%s-%s", prefix, timestamp)))
	}
	baseExt := filepath.Ext(basePath)
	baseStem := strings.TrimSuffix(basePath, baseExt)
	for i, item := range items {
		ext := chooseOutputExt(basePath, item.MIMEType, defaultExt)
		candidate := fmt.Sprintf("%s-%02d%s", baseStem, i+1, ext)
		if count == 1 {
			candidate = ensureExt(basePath, ext)
		}
		res, err := resolveOutputPath(ctx, candidate)
		if err != nil {
			return nil, err
		}
		paths = append(paths, res)
	}
	return paths, nil
}

func resolveOutputPath(ctx context.Context, relPath string) (plannedOutputPath, error) {
	res := utils.ResolvePathUnder(WorkspaceRootFromContext(ctx), filepath.ToSlash(relPath))
	if !res.IsValid {
		return plannedOutputPath{}, fmt.Errorf("%s", res.ErrMsg)
	}
	return plannedOutputPath{AbsPath: res.AbsPath, RelPath: filepath.ToSlash(res.RelPath)}, nil
}

func chooseOutputExt(requestedPath, mimeType, defaultExt string) string {
	if ext := strings.ToLower(strings.TrimSpace(filepath.Ext(requestedPath))); ext != "" {
		return ext
	}
	if ext := mediaExtFromMIME(mimeType); ext != "" {
		return ext
	}
	if strings.TrimSpace(defaultExt) != "" {
		if strings.HasPrefix(defaultExt, ".") {
			return defaultExt
		}
		return "." + defaultExt
	}
	return ".bin"
}

func ensureExt(path, ext string) string {
	if strings.TrimSpace(filepath.Ext(path)) != "" {
		return path
	}
	return path + ext
}

func mediaExtFromMIME(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav":
		return ".wav"
	case "audio/flac":
		return ".flac"
	case "audio/aac":
		return ".aac"
	case "audio/ogg":
		return ".ogg"
	case "audio/mp4":
		return ".m4a"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	default:
		return ""
	}
}

func detectMediaMIME(bs []byte, ext string) string {
	if len(bs) > 0 {
		if detected := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(bs), ";")[0])); detected != "" && detected != "application/octet-stream" {
			return detected
		}
	}
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

func asIntValue(v any, fallback int) int {
	switch vv := v.(type) {
	case int:
		return vv
	case int32:
		return int(vv)
	case int64:
		return int(vv)
	case float32:
		return int(vv)
	case float64:
		return int(vv)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
			return n
		}
	}
	return fallback
}
