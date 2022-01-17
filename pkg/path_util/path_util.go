package path_util

import "path/filepath"

// 获取一个无扩展的文件名
func GetFileNameWithoutExt(path string) string {
	filename := filepath.Base(path)
	return filename[:len(filename)-len(filepath.Ext(path))]
}
