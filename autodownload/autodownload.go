package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	baseURL     = "https://oss.exercisedb.dev/api/v1/exercises"
	downloadDir = "autodownloads"
	limit       = 50
)

type Exercise struct {
	Name       string   `json:"name"`
	GifURL     string   `json:"gifUrl"`
	BodyParts  []string `json:"bodyParts"`
	Equipments []string `json:"equipments"`
}

type ExerciseData struct {
	Data []Exercise `json:"data"`
}

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

var equipments = []string{
	"stepmill machine",
	"elliptical machine",
	"trap bar",
	"tire",
	"stationary bike",
	"wheel roller",
	"smith machine",
	"hammer",
	"skierg machine",
	"roller",
	"resistance band",
	"bosu ball",
	"weighted",
	"olympic barbell",
	"kettlebell",
	"upper body ergometer",
	"sled machine",
	"ez barbell",
	"dumbbell",
	"rope",
	"barbell",
	"band",
	"stability ball",
	"medicine ball",
	"assisted",
	"leverage machine",
	"cable",
	"body weight",
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

func main() {
	for _, bodyPart := range bodyParts {
		for _, equipment := range equipments {
			exercises, err := fetchExercises(bodyPart, equipment)
			if err != nil {
				fmt.Printf("请求失败: bodyPart=%s equipment=%s err=%v\n", bodyPart, equipment, err)
				continue
			}

			if len(exercises) == 0 {
				fmt.Printf("无数据: bodyPart=%s equipment=%s\n", bodyPart, equipment)
				continue
			}

			fmt.Printf("开始下载: bodyPart=%s equipment=%s count=%d\n", bodyPart, equipment, len(exercises))

			for _, ex := range exercises {
				if ex.Name == "" || ex.GifURL == "" {
					continue
				}

				if err := downloadExerciseGif(bodyPart, equipment, ex); err != nil {
					fmt.Printf("下载失败: bodyPart=%s equipment=%s name=%s url=%s err=%v\n", bodyPart, equipment, ex.Name, ex.GifURL, err)
					continue
				}
			}
		}
	}
}

func fetchExercises(bodyPart string, equipment string) ([]Exercise, error) {
	reqURL, err := buildExercisesURL(bodyPart, equipment)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code: %d", resp.StatusCode)
	}

	var data ExerciseData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data.Data, nil
}

func buildExercisesURL(bodyPart string, equipment string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	query := u.Query()
	query.Set("bodyParts", bodyPart)
	query.Set("equipments", equipment)
	query.Set("limit", fmt.Sprintf("%d", limit))
	u.RawQuery = query.Encode()

	return u.String(), nil
}

func downloadExerciseGif(bodyPart string, equipment string, ex Exercise) error {
	folderPath := filepath.Join(downloadDir, sanitizePathName(bodyPart), sanitizePathName(equipment))
	if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
		return err
	}

	fileName := sanitizePathName(ex.Name) + ".gif"
	filePath := filepath.Join(folderPath, fileName)

	if fileExists(filePath) {
		fmt.Printf("已存在，跳过: %s\n", filePath)
		return nil
	}

	return downloadFileWithRetry(ex.GifURL, filePath, 3)
}

func downloadFileWithRetry(fileURL string, filePath string, retryTimes int) error {
	var lastErr error

	for i := 1; i <= retryTimes; i++ {
		if err := downloadFile(fileURL, filePath); err != nil {
			lastErr = err
			fmt.Printf("第 %d 次下载失败: %s err=%v\n", i, fileURL, err)
			time.Sleep(time.Duration(i) * time.Second)
			continue
		}

		fmt.Printf("已下载: %s\n", filePath)
		return nil
	}

	return lastErr
}

func downloadFile(fileURL string, filePath string) error {
	resp, err := httpClient.Get(fileURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code: %d", resp.StatusCode)
	}

	outFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		_ = os.Remove(filePath)
		return err
	}

	return nil
}

func fileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	return err == nil && !info.IsDir()
}

func sanitizePathName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "*", "-")
	name = strings.ReplaceAll(name, "?", "-")
	name = strings.ReplaceAll(name, "\"", "-")
	name = strings.ReplaceAll(name, "<", "-")
	name = strings.ReplaceAll(name, ">", "-")
	name = strings.ReplaceAll(name, "|", "-")

	spaceRegexp := regexp.MustCompile(`\s+`)
	name = spaceRegexp.ReplaceAllString(name, " ")

	if name == "" {
		return "unknown"
	}

	return name
}
