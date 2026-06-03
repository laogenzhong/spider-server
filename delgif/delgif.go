package main

import (
	"flag"
	"fmt"
	"image/gif"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const defaultRootDir = "downloads"

type CheckResult struct {
	Path string
	OK   bool
	Err  error
}

func main() {
	rootDir := flag.String("dir", defaultRootDir, "要扫描的根目录")
	deleteInvalid := flag.Bool("delete", true, "是否删除无法展示的 gif 文件，默认只打印不删除")
	flag.Parse()

	info, err := os.Stat(*rootDir)
	if err != nil {
		fmt.Printf("目录不存在或无法访问: %s, err=%v\n", *rootDir, err)
		return
	}
	if !info.IsDir() {
		fmt.Printf("不是目录: %s\n", *rootDir)
		return
	}

	var total int
	var valid int
	var invalid int

	err = filepath.WalkDir(*rootDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			fmt.Printf("访问失败: %s, err=%v\n", path, walkErr)
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if !strings.EqualFold(filepath.Ext(path), ".gif") {
			return nil
		}

		total++
		result := checkGifFile(path)
		if result.OK {
			valid++
			fmt.Printf("✅ 可展示: %s\n", path)
			return nil
		}

		invalid++
		fmt.Printf("❌ 无法展示: %s, err=%v\n", path, result.Err)

		if *deleteInvalid {
			if err := os.Remove(path); err != nil {
				fmt.Printf("删除失败: %s, err=%v\n", path, err)
			} else {
				fmt.Printf("已删除: %s\n", path)
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("扫描失败: %v\n", err)
		return
	}

	fmt.Println("--------------------------------")
	fmt.Printf("扫描目录: %s\n", *rootDir)
	fmt.Printf("GIF 总数: %d\n", total)
	fmt.Printf("可展示: %d\n", valid)
	fmt.Printf("无法展示: %d\n", invalid)
	fmt.Printf("删除模式: %v\n", *deleteInvalid)
}

func checkGifFile(path string) CheckResult {
	file, err := os.Open(path)
	if err != nil {
		return CheckResult{Path: path, OK: false, Err: err}
	}
	defer file.Close()

	head := make([]byte, 6)
	n, err := io.ReadFull(file, head)
	if err != nil {
		return CheckResult{Path: path, OK: false, Err: fmt.Errorf("读取 GIF 文件头失败: %w", err)}
	}
	if n < 6 {
		return CheckResult{Path: path, OK: false, Err: fmt.Errorf("GIF 文件头长度不足")}
	}

	magic := string(head)
	if magic != "GIF87a" && magic != "GIF89a" {
		return CheckResult{Path: path, OK: false, Err: fmt.Errorf("不是合法 GIF 文件头: %s", magic)}
	}

	if _, err := file.Seek(0, 0); err != nil {
		return CheckResult{Path: path, OK: false, Err: fmt.Errorf("重置文件读取位置失败: %w", err)}
	}

	config, err := gif.DecodeConfig(file)
	if err != nil {
		return CheckResult{Path: path, OK: false, Err: fmt.Errorf("解析 GIF 配置失败: %w", err)}
	}
	if config.Width <= 0 || config.Height <= 0 {
		return CheckResult{Path: path, OK: false, Err: fmt.Errorf("GIF 尺寸异常: width=%d height=%d", config.Width, config.Height)}
	}

	if _, err := file.Seek(0, 0); err != nil {
		return CheckResult{Path: path, OK: false, Err: fmt.Errorf("重置文件读取位置失败: %w", err)}
	}

	_, err = gif.Decode(file)
	if err != nil {
		return CheckResult{Path: path, OK: false, Err: fmt.Errorf("解码 GIF 第一帧失败: %w", err)}
	}

	return CheckResult{Path: path, OK: true, Err: nil}
}
