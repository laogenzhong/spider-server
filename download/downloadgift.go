package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type Exercise struct {
	Name       string   `json:"name"`
	GifUrl     string   `json:"gifUrl"`
	BodyParts  []string `json:"bodyParts"`
	Equipments []string `json:"equipments"`
}

type ExerciseData struct {
	Data []Exercise `json:"data"`
}

const neck = "/Users/huitailang/workdir/spider-server/download/neck.json"
const lowerarms = "/Users/huitailang/workdir/spider-server/download/lowerarms.json"
const shoulders = "/Users/huitailang/workdir/spider-server/download/shoulders.json"
const cardio = "/Users/huitailang/workdir/spider-server/download/cardio.json"
const upperarms = "/Users/huitailang/workdir/spider-server/download/upperarms.json"
const chest = "/Users/huitailang/workdir/spider-server/download/exercises_chest.json"
const lowerlegs = "/Users/huitailang/workdir/spider-server/download/lowerlegs.json"
const back = "/Users/huitailang/workdir/spider-server/download/back.json"
const upperlegs = "/Users/huitailang/workdir/spider-server/download/upperlegs.json"
const waist = "/Users/huitailang/workdir/spider-server/download/waist.json"
const barbell = "/Users/huitailang/workdir/spider-server/download/barbell.json"

func main() {
	// 读取 JSON 文件
	file, err := os.Open(barbell)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var exerciseData ExerciseData
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&exerciseData); err != nil {
		panic(err)
	}

	// 遍历每个动作
	for _, ex := range exerciseData.Data {
		// 取第一个 bodyPart 和第一个 equipment 作为文件夹名
		if len(ex.BodyParts) == 0 || len(ex.Equipments) == 0 {
			continue
		}
		bodyPart := ex.BodyParts[0]
		equipment := ex.Equipments[0]

		// 创建两级文件夹
		folderPath := filepath.Join("downloads", bodyPart, equipment)
		if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
			fmt.Println("创建文件夹失败:", folderPath, err)
			continue
		}

		// 下载 GIF
		resp, err := http.Get(ex.GifUrl)
		if err != nil {
			fmt.Println("下载失败:", ex.GifUrl, err)
			continue
		}
		defer resp.Body.Close()

		// 文件名使用动作名 + .gif
		fileName := fmt.Sprintf("%s.gif", ex.Name)
		filePath := filepath.Join(folderPath, fileName)

		outFile, err := os.Create(filePath)
		if err != nil {
			fmt.Println("创建文件失败:", filePath, err)
			continue
		}

		_, err = io.Copy(outFile, resp.Body)
		if err != nil {
			fmt.Println("写入文件失败:", filePath, err)
		}
		outFile.Close()
		fmt.Println("已下载:", filePath)
	}
}
