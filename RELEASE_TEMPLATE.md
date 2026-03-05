# VB Coding 发布模板（GitHub Release）

## 标题

`VB Coding v<版本号>`

## 概要

本版本聚焦于稳定性与发布流程完善，包含模块路径规范化、许可证完善、安装包配置更新与测试稳定性修复。

## 下载

- Windows x64 安装包（Inno Setup）  
  `vb-coding-setup-<版本号>.exe`
- Windows x64 便携版  
  `vb-coding-<版本号>-windows-amd64.zip`
- 校验文件  
  `SHA256SUMS.txt`

## 校验（SHA256）

```text
<sha256>  vb-coding-setup-<版本号>.exe
<sha256>  vb-coding-<版本号>-windows-amd64.zip
```

## 更新内容

### 新增

- 新增非商用许可证，明确个人免费使用与商用授权要求。
- 新增发布工件校验文件，便于验证下载完整性。

### 变更

- 仓库地址、安装包发布链接与文档链接统一更新为 GitHub 正式地址。
- Go 模块路径升级为 `github.com/dreamSailing/vb-coding`。

### 修复

- 修复 `internal/tools/git` 中对本地 Git 环境敏感的测试用例，提升 CI 稳定性。

## 升级说明

- 直接覆盖安装或卸载旧版本后重装均可。
- 首次启动前请配置模型参数：`VB_API_BASE`、`VB_API_KEY`、`VB_MODEL`。

## 已知事项

- Windows 终端可能出现 CRLF 提示，不影响运行。
- 若安全软件拦截，请将安装目录加入信任后重试。

## 许可证

- 个人/非商业用途免费。
- 商业用途需单独书面授权。
- 详见仓库中的 `LICENSE` 文件。
