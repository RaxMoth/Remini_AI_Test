package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	APIKey            = "WOW_I_HIDE_MY_SECRET_TOKEN"
	ImagePath         = "A_SEXY_PICTURE_ON_THE_BEACH"
	ContentType       = "image/jpeg"
	OutputContentType = "image/jpeg"
	BaseURL           = "https://developer.remini.ai/api"
	Timeout           = 60 * time.Second
)

func getImageMD5Content() (string, []byte) {

	content, err := os.ReadFile(ImagePath)
	if err != nil {
		panic(err)
	}

	imageMD5 := md5.Sum(content)
	imageMD5Base64 := base64.StdEncoding.EncodeToString(imageMD5[:])

	return imageMD5Base64, content
}

func main() {
	imageMD5, content := getImageMD5Content()
	client := &http.Client{
		Timeout: Timeout,
	}

	// Submit the task
	taskReqData := map[string]interface{}{
		"tools": []map[string]string{
			{"type": "face_enhance", "mode": "beautify"},
			{"type": "background_enhance", "mode": "professional"},
			{"type": "color_enhance", "mode": "madrid"},
		},
		"image_md5":           imageMD5,
		"image_content_type":  ContentType,
		"output_content_type": OutputContentType,
	}
	taskReqBody, _ := json.Marshal(taskReqData)

	taskReq, _ := http.NewRequest("POST", BaseURL+"/tasks", bytes.NewBuffer(taskReqBody))
	taskReq.Header.Set("Authorization", "Bearer "+APIKey)
	taskReq.Header.Set("Content-Type", "application/json")

	fmt.Println("Submitting the image ...")
	taskRes, err := client.Do(taskReq)
	if err != nil || taskRes.StatusCode != http.StatusOK {
		panic(err)
	}
	defer taskRes.Body.Close()

	taskResBody, _ := io.ReadAll(taskRes.Body)
	var taskResData map[string]interface{}
	json.Unmarshal(taskResBody, &taskResData)
	taskID := taskResData["task_id"].(string)

	// Upload the image
	fmt.Println("Uploading the image to Google Cloud Storage ...")
	uploadURL := taskResData["upload_url"].(string)
	uploadHeaders := taskResData["upload_headers"].(map[string]interface{})

	uploadReq, _ := http.NewRequest("PUT", uploadURL, bytes.NewBuffer(content))
	for key, value := range uploadHeaders {
		uploadReq.Header.Set(key, value.(string))
	}

	uploadRes, err := client.Do(uploadReq)
	if err != nil || uploadRes.StatusCode != http.StatusOK {
		panic(err)
	}
	defer uploadRes.Body.Close()

	// Process the image
	fmt.Println("Processing the task: " + taskID + " ...")
	processReq, _ := http.NewRequest("POST", BaseURL+"/tasks/"+taskID+"/process", nil)
	processReq.Header.Set("Authorization", "Bearer "+APIKey)

	processRes, err := client.Do(processReq)
	if err != nil || processRes.StatusCode != http.StatusAccepted {
		panic(err)
	}
	defer processRes.Body.Close()

	// Get the image
	fmt.Println("Polling for result task: " + taskID + " ...")
	for i := 0; i < 50; i++ {
		req, err := http.NewRequest("GET", BaseURL+"/tasks/"+taskID, nil)
		req.Header.Set("Authorization", "Bearer "+APIKey)

		res, err := client.Do(req)
		if err != nil || res.StatusCode != http.StatusOK {
			panic(err)
		}
		defer res.Body.Close()

		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			panic(err)
		}

		var resData map[string]interface{}
		json.Unmarshal(resBody, &resData)

		if resData["status"].(string) == "completed" {
			outputURL := resData["result"].(map[string]interface{})["output_url"].(string)
			fmt.Println("Result output image: " + outputURL)
			os.Exit(0)
		} else {
			fmt.Println("Processing, sleeping 1 second ...")
			time.Sleep(2 * time.Second)
		}
	}
	fmt.Println("Timeout reached! :(")
}
