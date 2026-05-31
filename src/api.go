package src

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

/*

curl https://api.groq.com/openai/v1/audio/transcriptions \
  -H "Authorization: Bearer $GROQ_API_KEY" \
  -H "Content-Type: multipart/form-data" \
  -F file="@./sample_audio.m4a" \
  -F model="whisper-large-v3"
*/

const BASE_URL string = "https://api.groq.com/openai/v1"

func GetGroqKey() (string, error) {
	key, exists := os.LookupEnv("GROQ_API_KEY")
	if !exists {
		fError := fmt.Errorf("[ERROR] Please set the Groq API key in the environment")
		return "", fError
	}
	return key, nil
}

// parsing the json response from the groq api so that I can access the values by their keys
type Response struct {
	Text   string `json:"text"`
	X_Groq struct {
		Id string `json:"id"`
	} `json:"x_groq"`
}

func Transcribe(audioPath string) (string, error) {

	// key handling
	key, err := GetGroqKey()
	if err != nil {
		return "", err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// setting up the model
	modelError := writer.WriteField("model", "whisper-large-v3")
	if modelError != nil {
		fError := fmt.Errorf("Error while setting the model for audio transcription. Error: %v", modelError)
		return "", fError
	}

	// file handling i.e. getting the file from the audioPath and adding it to the request
	file, err := os.Open(audioPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", file.Name())
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}

	writer.Close()

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/%s", BASE_URL, "audio/transcriptions"),
		body,
	)
	if err != nil {
		fError := fmt.Errorf("Error while creating new HTTP Request: %v", err)
		return "", fError
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		fmtError := fmt.Errorf("[ERROR] An error occured while making request to the GROQ API\nError: %v", err)
		return "", fmtError
	}
	defer response.Body.Close()

	var data Response
	jsonParseError := json.NewDecoder(response.Body).Decode(&data)
	if jsonParseError != nil {
		fError := fmt.Errorf("[ERROR] There was an error parsing JSON from the response: %v", jsonParseError)
		return "", fError
	}

	return data.Text, nil

}
