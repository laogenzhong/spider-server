package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Exercise struct {
	ExerciseID string   `json:"exerciseId"`
	Name       string   `json:"name"`
	BodyParts  []string `json:"bodyParts"`
	Equipments []string `json:"equipments"`
}

type ExerciseData struct {
	Data []Exercise `json:"data"`
}

type Mapping struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	BodyPart  string `json:"bodyPart"`
	Equipment string `json:"equipment"`
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

func main() {
	// 所有需要读取的 json 文件路径
	files := []string{
		neck,
		lowerarms,
		shoulders,
		cardio,
		upperarms,
		chest,
		lowerlegs,
		back,
		upperlegs,
		waist,
	}

	// 字典映射
	equipmentMap := map[string]string{
		"barbell":         "杠铃",
		"dumbbell":        "哑铃",
		"smith machine":   "杠铃",
		"body weight":     "自重",
		"cable":           "器械",
		"machine":         "器械",
		"resistance band": "其他",
		"kettlebell":      "其他",
		"medicine ball":   "其他",
		"roller":          "其他",
	}

	bodyPartMap := map[string]string{
		"neck":      "颈部",
		"lowerarms": "下臂",
		"shoulders": "肩膀",
		"cardio":    "有氧运动",
		"upperarms": "上臂",
		"chest":     "胸",
		"lowerlegs": "小腿",
		"back":      "背",
		"upperlegs": "大腿",
		"waist":     "腰部",
	}

	// 去重用
	existMap := make(map[string]bool)
	var mappings []Mapping

	for _, filePath := range files {
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取文件 %s 失败: %v\n", filePath, err)
			continue
		}
		var exerciseData ExerciseData
		if err := json.Unmarshal(data, &exerciseData); err != nil {
			fmt.Fprintf(os.Stderr, "解析文件 %s 失败: %v\n", filePath, err)
			continue
		}
		for _, ex := range exerciseData.Data {
			if existMap[ex.ExerciseID] {
				continue
			}
			existMap[ex.ExerciseID] = true
			bodyPart := "其他"
			if len(ex.BodyParts) > 0 {
				if v, ok := bodyPartMap[ex.BodyParts[0]]; ok {
					bodyPart = v
				}
			}
			equipment := "其他"
			if len(ex.Equipments) > 0 {
				if v, ok := equipmentMap[ex.Equipments[0]]; ok {
					equipment = v
				}
			}
			path := fmt.Sprintf("%s/%s/%s.gif", bodyPart, equipment, ex.ExerciseID)
			mappings = append(mappings, Mapping{
				Name:      ex.Name,
				Path:      path,
				BodyPart:  bodyPart,
				Equipment: equipment,
			})
		}
	}

	// 保存映射文件
	result, _ := json.MarshalIndent(mappings, "", "  ")
	_ = os.WriteFile("exercise_mapping.json", result, 0644)

	fmt.Println("exercise_mapping.json 已更新")
}
