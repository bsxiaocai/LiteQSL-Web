// Package version 集中管理应用与 ADIF 版本号。
package version

const (
	// AppVersion 是 LiteQSL-Web 应用版本号。
	// v2 为 Go 重写版，沿用同一健康检查与 ADIF PROGRAMVERSION 字段。
	AppVersion = "2.0.0"

	// ADIFVersion 是导出 ADIF 文件时写入的 ADIF 规范版本。
	ADIFVersion = "3.1.5"
)
