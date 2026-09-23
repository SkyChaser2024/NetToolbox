# 网络工具箱

面向中山大学（SYSU）校园网使用场景的 Windows 锐捷认证与网络诊断工具。

中山大学提供的锐捷认证客户端版本较旧，在 Windows 11 上可能出现某些兼容性问题。本项目提供一个轻量、现代的替代客户端，帮助 Windows 11 用户更方便地完成校园网 802.1X 认证。

> 本项目是非官方社区工具，与中山大学及锐捷网络没有隶属或授权关系。请仅在本人有权接入的校园网络中使用，并遵守学校网络管理规定。

## 主要功能

- 锐捷 802.1X / EAP-MD5 单次认证、注销与状态日志
- 启动时识别已有认证，断线后由低功耗后台任务自动重试（最多 3 次）
- Npcap 有线网卡识别，以及物理、虚拟、隧道等网卡的分类概览与优先级管理
- 可自定义的网站 HTTP HEAD 滚动延迟测试、NAT 类型和 IPv6 连通性检测
- 原生 ICMP Ping 与路由追踪，支持 IPv4 / IPv6、实时丢包统计、逐跳结果和反向 DNS
- 可选系统托盘驻留；关闭主界面后释放 WebView，仅保留轻量后台进程

## 下载

请从仓库的 [Releases](../../releases) 页面下载 `NetToolbox-v0.21.0-windows-x64.zip`，解压后运行其中的 `NetToolbox.exe`。

## 运行要求
- [Npcap](https://npcap.com/#download)
- Microsoft Edge WebView2 Runtime（Windows 11 通常已内置）
- 管理员权限，用于访问网卡和调整系统网络设置

## 本地开发

需要 Go 1.26+、Node.js 24 LTS（至少 24.15，推荐 `.node-version` 中的版本）、Wails v2 CLI 和 Windows 开发环境。
```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
npm ci --prefix frontend
wails dev
```
生产构建：
```powershell
wails build -clean -trimpath -ldflags "-s -w -buildid="
```
