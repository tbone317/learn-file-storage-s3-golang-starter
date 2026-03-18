package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

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

	const maxMemory = 10 << 20 // 10 MB
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	rawType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(rawType)
	if mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type for thumbnail", nil)
		return
	}
	ext, err := mime.ExtensionsByType(mediaType)
	if err != nil || len(ext) == 0 {
		respondWithError(w, http.StatusBadRequest, "Unsupported media type for thumbnail", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't find video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to update this video", nil)
		return
	}

	// Create file path using filepath.Join
	fileName := fmt.Sprintf("%s%s", videoID.String(), ext[0])
	filePath := filepath.Join(cfg.assetsRoot, fileName)
	//log.Printf("thumbnail file path: %s", filePath)

	// Create and save the file to disk
	newFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create thumbnail file", err)
		return
	}
	defer newFile.Close()

	// Copy file contents to disk
	_, err = io.Copy(newFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save thumbnail file", err)
		return
	}

	url := fmt.Sprintf("http://localhost:%s/assets/%s%s", cfg.port, videoID.String(), ext[0])
	video.ThumbnailURL = &url

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		//delete(videoThumbnails, videoID)
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
