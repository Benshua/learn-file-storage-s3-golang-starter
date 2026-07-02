package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerVideoMetaCreate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		database.CreateVideoParams
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

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}
	params.UserID = userID

	video, err := cfg.db.CreateVideo(params.CreateVideoParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create video", err)
		return
	}

	respondWithJSON(w, http.StatusCreated, video)
}

func (cfg *apiConfig) handlerVideoMetaDelete(w http.ResponseWriter, r *http.Request) {
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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't get video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusForbidden, "You can't delete this video", err)
		return
	}

	err = cfg.db.DeleteVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't delete video", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handlerVideoGet(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid video ID", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	signedVideo, _ := cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't get video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, signedVideo)
}

func (cfg *apiConfig) handlerVideosRetrieve(w http.ResponseWriter, r *http.Request) {
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
	
	videos, err := cfg.db.GetVideos(userID)

	signedVideos := make([]database.Video, len(videos))
	for i, video := range videos {
		signed, err := cfg.dbVideoToSignedVideo(video)
		if err != nil {
			// handle error
		}
		signedVideos[i] = signed
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't retrieve videos", err)
		return
	}

	respondWithJSON(w, http.StatusOK, signedVideos)
}

func getVideoAspectRatio(filepath string) (string, error) {

	cmd := exec.Command("/usr/bin/ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	var b bytes.Buffer
	cmd.Stdout = &b

	if err := cmd.Run(); err != nil {
		return "", err
	}
	
	type arStruct struct {
			Streams[] struct {
				Width int `json:"coded_width"`
				Height int `json:"coded_height"`
			} `json:"streams"`
		}
	
	var ar arStruct
	if err := json.Unmarshal(b.Bytes(), &ar); err != nil {
		return "", err
	}

	if len(ar.Streams) == 0 {
    return "", errors.New("no streams found")
	}

	w := ar.Streams[0].Width
	h := ar.Streams[0].Height

	if w > h { return "landscape", nil }
	if w < h { return "portrait", nil }

	return "other", nil
}


func processVideoForFastStart(filepath string) (string, error) {
	log.Println(filepath)
	fPString := fmt.Sprintf("%s.processing", filepath)
	log.Println(fPString)
	cmd := exec.Command("/usr/bin/ffmpeg", 
						"-i", filepath, "-c", "copy", "-movflags", 
						"faststart", "-f", "mp4", fPString,
					)
		
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg error: %s, %v", stderr.String(), err)
	}

	return fPString, nil

}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
    	return video, nil
	}

	videoURL := video.VideoURL
	log.Printf("new video URL: %v",video)
	splitVU := strings.SplitN(*videoURL, ",", 2)

	psURL, err := generatePresignedURL(cfg.s3Client, splitVU[0], splitVU[1], 5*time.Minute)
	if err != nil {	
		return video, fmt.Errorf("Failed to generate presigned URL: %v", err)
	}

	video.VideoURL = &psURL
	log.Printf("presigned URL: %v", psURL)
	return video, err
	
}