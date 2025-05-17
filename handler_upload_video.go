package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime"
	"net/http"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func getVideoAspectRatio(filePath string) (string, error) {

	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var commandBuffer bytes.Buffer
	cmd.Stdout = &commandBuffer
	err := cmd.Run()
	if err != nil {
		log.Print("error while running ffprobe")
		return "", err
	}

	type VideoData struct {
		Streams []struct {
			Width  int `json:"width,omitempty"`
			Height int `json:"height,omitempty"`
		}
	}

	// fmt.Printf("%s\n", commandBuffer.Bytes())

	videoData := VideoData{}
	json.Unmarshal(commandBuffer.Bytes(), &videoData)

	fmt.Printf("Video data %v \n", videoData)

	if len(videoData.Streams) == 0 {
		return "", errors.New("missing video data")
	}

	aspectRatio := float64(videoData.Streams[0].Width) / float64(videoData.Streams[0].Height)

	fmt.Printf("aspect ration %f \n", aspectRatio)
	if math.Abs(aspectRatio-1.78) < 0.05 {
		return "16:9", nil
	} else if math.Abs(aspectRatio-0.5625) < 0.05 {
		return "9:16", nil
	}
	return "other", nil

}

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {

	const uploadLimit = 1 << 30 // bit shift 1 to the left 30 times.1 * 1024* 1024*1024 -> 1 GB

	http.MaxBytesReader(w, r.Body, uploadLimit)

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading video", videoID, "by user", userID)

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Video not owned by user", err)

	}

	r.ParseMultipartForm(uploadLimit) // divide media file into parts

	videoFile, header, err := r.FormFile("video") // get data from form by field id
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could't load file from form", err)
		return
	}

	defer videoFile.Close()

	mediaType, _, _ := mime.ParseMediaType(header.Header.Get("Content-Type"))

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusInternalServerError, "Incorrect media type. Should be video/mp4", err)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating temp file for upload", err)
		return
	}

	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	io.Copy(tempFile, videoFile)

	tempFile.Seek(0, io.SeekStart)

	ratio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error calculating aspect ratio for video", err)
		return
	}

	// Generate random name for file
	random_key := make([]byte, 32)
	rand.Read(random_key)
	fileName := hex.EncodeToString(random_key)
	
	if ratio == "16:9"{
		fileName = fmt.Sprintf("landscape/%s.%s", fileName, "mp4")

	} else if ratio == "9:16"{
		fileName = fmt.Sprintf("portrait/%s.%s", fileName, "mp4")
	} else {
		fileName = fmt.Sprintf("other/%s.%s", fileName, "mp4")
	}


	_, putError := cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileName,
		Body:        tempFile, // The io.Reader (your *os.File)
		ContentType: &mediaType,
	})

	if putError != nil {
		respondWithError(w, http.StatusInternalServerError, "Error putting object to s3", err)
		return
	}
	
	s3VideoUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, fileName)
	video.VideoURL = &s3VideoUrl

	cfg.db.UpdateVideo(video)

}
