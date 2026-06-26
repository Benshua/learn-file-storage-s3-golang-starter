package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get thummbnail data", err)
		return
	}	

	contentType, _, err := mime.ParseMediaType(header.Header.Get("Content-type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if contentType != "image/jpeg" && contentType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Wrong MIME type", err)
		return
	}

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

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

	fileType := strings.SplitN(contentType, "/", 2)

	fmt.Println(contentType)
	fileName := fmt.Sprintf("%v.%v", videoIDString, fileType[1])
	fp := filepath.Join(cfg.assetsRoot, fileName )
	thumbnailFile, _ := os.Create(fp)
	io.Copy(thumbnailFile, file)

	defer file.Close()
	defer thumbnailFile.Close()

	videoMetadata, err := cfg.db.GetVideo(videoID) 
	if err != nil {
		respondWithError(w, 500, "There was an issue retrieving video metadata", err)
		return
	}

	if videoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized", err)
	}

	fileURL := fmt.Sprintf("http://localhost%v/assets/%v", cfg.port, fileName)
	videoMetadata.ThumbnailURL = &fileURL
	if err := cfg.db.UpdateVideo(videoMetadata); err != nil {
		respondWithError(w, 403, "could not update metadata", err)
		return
	}
	
	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	respondWithJSON(w, http.StatusOK, videoMetadata)
}
