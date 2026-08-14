package payload

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteOptions 写入选项
type WriteOptions struct {
	// Overwrite 允许覆盖已存在文件
	Overwrite bool
	// VerifySHA256 非空则在落盘前校验整体 SHA-256，不匹配则报错且不产生 .part
	VerifySHA256 string
}

// WriteFile 将 data 原子写入 path：
// 先写 path+".part" 临时文件，成功后 rename 为最终文件；
// 任一环节失败均清理 .part，不产生半成品最终文件。
func WriteFile(path string, data []byte, opts WriteOptions) error {
	if !opts.Overwrite {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("payload: 目标文件已存在 %q（使用 --overwrite 允许覆盖）", path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("payload: 检查目标文件 %q 失败: %w", path, err)
		}
	}

	if opts.VerifySHA256 != "" && !VerifySHA256(data, opts.VerifySHA256) {
		return fmt.Errorf("payload: SHA-256 校验失败（期望 %s，实际 %s）", opts.VerifySHA256, SHA256Hex(data))
	}

	partPath := path + ".part"
	if err := os.WriteFile(partPath, data, 0o644); err != nil {
		os.Remove(partPath)
		return fmt.Errorf("payload: 写入临时文件失败 %q（输出目录不可写？）: %w", partPath, err)
	}
	if err := os.Rename(partPath, path); err != nil {
		os.Remove(partPath)
		return fmt.Errorf("payload: 重命名临时文件失败 %q -> %q: %w", partPath, path, err)
	}
	return nil
}

// OutputPath 根据 --output 参数与原始文件名解析最终输出路径。
// output 为空 → 当前目录；以路径分隔符结尾或为已存在目录 → 视为目录拼接文件名；
// 否则视为完整文件名。
func OutputPath(output, name string) (string, error) {
	if output == "" {
		output = "."
	}
	info, err := os.Stat(output)
	if err == nil && info.IsDir() {
		return filepath.Join(output, filepath.Base(name)), nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("payload: 检查输出路径失败 %q: %w", output, err)
	}
	if strings.HasSuffix(output, string(os.PathSeparator)) || strings.HasSuffix(output, "/") {
		return filepath.Join(output, filepath.Base(name)), nil
	}
	return output, nil
}
