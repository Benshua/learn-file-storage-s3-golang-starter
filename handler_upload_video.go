package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {

	uploadLimit := 10 << 30
	r.Body = http.MaxBytesReader(w, r.Body, int64(uploadLimit))
	
	videoID := r.PathValue("videoID")
	videoUUID, err := uuid.Parse(videoID)
	if err != nil {
		respondWithError(w, 500, "Could not parse video ID: %v", err)
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

	video, err := cfg.db.GetVideo(videoUUID) 
	if err != nil {
		respondWithError(w, 500, "There was an issue retrieving video metadata", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	videoData, videoDataHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not get video data: %v", err)
		return
	}

	defer videoData.Close()

	contentType, _, err := mime.ParseMediaType(videoDataHeader.Header.Get("Content-type"))
	contentExt := strings.SplitN(contentType, "/", 2)
	if err != nil {
		respondWithError(w, http.StatusFailedDependency, "Failed to parse content header for video", err)
		return
	}

	if contentType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Bad request: %v", err)
		return
	}

	tempF, err := os.CreateTemp("", "tubely-vid.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Internal server error", err)
		return
	}

	defer os.Remove(tempF.Name())
	defer tempF.Close()



	io.Copy(tempF, videoData)
	tempF.Seek(0, io.SeekStart)

	b := make([]byte, 32)
	rand.Read(b)

	ar, err := getVideoAspectRatio(tempF.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch aspect ratio", err)
		return
	}

	vF := fmt.Sprintf("%v/%v.%v", ar, hex.EncodeToString(b), contentExt[1])

	cfg.s3Client.PutObject(
		context.TODO(),
		&s3.PutObjectInput{
			Bucket: &cfg.s3Bucket,
			Key: &vF,
			Body: tempF,
			ContentType: &contentType,
		},
	)

	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, vF)
	video.VideoURL = &videoURL
	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, 403, "could not upload video", err)
		return
	}
	fmt.Println("uploading thumbnail for video", videoID, "by user", userID, "with URL", videoURL)
	log.Printf("uploading file with URL: %s", videoURL)

	respondWithJSON(w, http.StatusOK, video)

}
