go mod vendor 只属于本项目的包放在一起
每次 go mod tidy 后都要执行一次 go mod vendor


Go 的 import 路径不是按“文件系统绝对路径”找的，而是按：
go.mod 里的 module 名 + 项目内相对目录