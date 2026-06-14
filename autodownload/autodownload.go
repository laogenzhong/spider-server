package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	baseURL         = "https://oss.exercisedb.dev/api/v1/exercises"
	pageLimit       = 25
	outputDir       = "exercise_body_part_json"
	requestInterval = 2 * time.Second
)

var bodyParts = []string{
	"neck",
	"lower arms",
	"shoulders",
	"cardio",
	"upper arms",
	"chest",
	"lower legs",
	"back",
	"upper legs",
	"waist",
}

type ExerciseResponse struct {
	Success bool             `json:"success"`
	Meta    ExercisePageMeta `json:"meta"`
	Data    []Exercise       `json:"data"`
}

type ExercisePageMeta struct {
	Total           int  `json:"total"`
	HasNextPage     bool `json:"hasNextPage"`
	HasPreviousPage bool `json:"hasPreviousPage"`
}

type Exercise struct {
	ExerciseID       string   `json:"exerciseId"`
	Name             string   `json:"name"`
	GifURL           string   `json:"gifUrl"`
	BodyParts        []string `json:"bodyParts"`
	Equipments       []string `json:"equipments"`
	TargetMuscles    []string `json:"targetMuscles"`
	SecondaryMuscles []string `json:"secondaryMuscles"`
	Instructions     []string `json:"instructions"`
}

type BodyPartExerciseFile struct {
	BodyPart string     `json:"bodyPart"`
	Total    int        `json:"total"`
	Data     []Exercise `json:"data"`
}

// 下载 git 的详情信息
func main() {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("创建输出目录失败: %v\n", err)
		return
	}

	for _, bodyPart := range bodyParts {
		fmt.Printf("开始下载部位: %s\n", bodyPart)

		exercises, err := fetchAllExercisesByBodyPart(client, bodyPart)
		if err != nil {
			fmt.Printf("下载失败 bodyPart=%s err=%v\n", bodyPart, err)
			continue
		}

		if err := saveBodyPartExercises(bodyPart, exercises); err != nil {
			fmt.Printf("保存失败 bodyPart=%s err=%v\n", bodyPart, err)
			continue
		}

		fmt.Printf("下载完成部位: %s, count=%d\n", bodyPart, len(exercises))

		// 每个部位的首个请求之间也间隔 2 秒。
		time.Sleep(requestInterval)
	}
}

func fetchAllExercisesByBodyPart(client *http.Client, bodyPart string) ([]Exercise, error) {
	var allExercises []Exercise
	var after string

	for {
		resp, err := fetchExercisePage(client, bodyPart, after)
		if err != nil {
			return nil, err
		}

		allExercises = append(allExercises, resp.Data...)

		if !resp.Meta.HasNextPage {
			break
		}

		if len(resp.Data) == 0 {
			return nil, fmt.Errorf("hasNextPage=true 但当前页 data 为空，无法获取 after")
		}

		after = resp.Data[len(resp.Data)-1].ExerciseID
		if after == "" {
			return nil, fmt.Errorf("当前页最后一条 exerciseId 为空，无法继续分页")
		}

		// 防止请求过快触发 429，每次分页请求之间间隔 2 秒。
		time.Sleep(requestInterval)
	}

	return allExercises, nil
}

func fetchExercisePage(client *http.Client, bodyPart string, after string) (*ExerciseResponse, error) {
	reqURL, err := buildExerciseURL(bodyPart, after)
	if err != nil {
		return nil, err
	}

	if after == "" {
		fmt.Printf("请求 bodyPart=%s page=first url=%s\n", bodyPart, reqURL)
	} else {
		fmt.Printf("请求 bodyPart=%s after=%s url=%s\n", bodyPart, after, reqURL)
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "spider-server-exercise-downloader/1.0")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("请求过快，接口返回 429: %s", string(body))
			time.Sleep(time.Duration(attempt*2) * time.Second)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("接口返回异常 status=%d body=%s", resp.StatusCode, string(body))
		}

		var result ExerciseResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("解析 JSON 失败: %w, body=%s", err, string(body))
		}

		if !result.Success {
			return nil, fmt.Errorf("接口 success=false, body=%s", string(body))
		}

		fmt.Printf(
			"响应 bodyPart=%s after=%s status=%d current=%d total=%d hasNextPage=%v\n",
			bodyPart,
			after,
			resp.StatusCode,
			len(result.Data),
			result.Meta.Total,
			result.Meta.HasNextPage,
		)

		return &result, nil
	}

	return nil, fmt.Errorf("请求失败，已重试 3 次: %w", lastErr)
}

func buildExerciseURL(bodyPart string, after string) (string, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	query := parsedURL.Query()
	query.Set("bodyParts", bodyPart)
	query.Set("limit", fmt.Sprintf("%d", pageLimit))
	if after != "" {
		query.Set("after", after)
	}
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

func saveBodyPartExercises(bodyPart string, exercises []Exercise) error {
	fileData := BodyPartExerciseFile{
		BodyPart: bodyPart,
		Total:    len(exercises),
		Data:     exercises,
	}

	body, err := json.MarshalIndent(fileData, "", "  ")
	if err != nil {
		return err
	}

	fileName := safeFileName(bodyPart) + ".json"
	filePath := filepath.Join(outputDir, fileName)

	return os.WriteFile(filePath, body, 0644)
}

func safeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}
