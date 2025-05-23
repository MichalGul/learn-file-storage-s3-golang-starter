package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20 // bit shift 10 to the left 20 times. Same as 10 * 1024 * 1024 -> 10 MB

	r.ParseMultipartForm(maxMemory) // divide media file into parts

	file, header, err := r.FormFile("thumbnail") // get data from form by field id
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could't load file from form", err)
		return
	}
	defer file.Close()

	mediaType, _, _ := mime.ParseMediaType(header.Header.Get("Content-Type"))

	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusInternalServerError, "Incorrect media type. Should be jpeg or png", err)
		return
	}

	// imageBytes, err := io.ReadAll(file)
	// if err != nil {
	// 	respondWithError(w, http.StatusInternalServerError, "Could't parse file to bytes", err)
	// 	return
	// }

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting video from db", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Logged user does not own video", err)
		return
	}

	// videoThumbnails[videoID] = thumbnail{
	// 	data: imageBytes,
	// 	mediaType: mediaType,
	// }

	// thumbnailURL := fmt.Sprintf("http://localhost:%s/api/thumbnails/%s", cfg.port, videoIDString)
	// video.ThumbnailURL = &thumbnailURL

	// Encode image to base64 string to store in db - temporary
	// endodedImageData := base64.StdEncoding.EncodeToString(imageBytes)
	// dataURL := fmt.Sprintf("data:%s;base64,%s", mediaType, endodedImageData)
	// video.ThumbnailURL = &dataURL

	// Store file on filesystem
	// Generate random name for file
	random_key := make([]byte, 32)
	rand.Read(random_key)
	fileName := base64.RawURLEncoding.EncodeToString(random_key)

	fileWebPath := createAssetPath(fileName, mediaType, cfg.assetsRoot)
	fileDiskPath := cfg.getAssetDiskPath(fileWebPath)
	newThumbnailFile, createErr := os.Create(fileDiskPath)

	if createErr != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating thumbnail file", createErr)
		return
	}

	_, copyErr := io.Copy(newThumbnailFile, file)
	if copyErr != nil {
		respondWithError(w, http.StatusInternalServerError, "error coping thumbnail file", copyErr)
		return
	}

	defer newThumbnailFile.Close()

	thumbnailURL := cfg.getAssetURL(fileWebPath)
	video.ThumbnailURL = &thumbnailURL

	updateErr := cfg.db.UpdateVideo(video)
	if updateErr != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating video", updateErr)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
