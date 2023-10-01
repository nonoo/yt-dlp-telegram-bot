package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/wader/goutubedl"
)

const ytdlpVersionCheckTimeout = time.Second * 10

func ytdlpGetLatestRelease(ctx context.Context) (release *github.RepositoryRelease, err error) {
	client := github.NewClient(nil)

	release, _, err = client.Repositories.GetLatestRelease(ctx, "yt-dlp", "yt-dlp")
	if err != nil {
		return nil, fmt.Errorf("getting latest yt-dlp version: %w", err)
	}
	return release, nil
}

type ytdlpGithubReleaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

func ytdlpGetLatestReleaseURL(ctx context.Context) (url string, err error) {
	release, err := ytdlpGetLatestRelease(ctx)
	if err != nil {
		return "", err
	}

	assetsURL := release.GetAssetsURL()
	if assetsURL == "" {
		return "", fmt.Errorf("downloading latest yt-dlp: no assets url")
	}

	resp, err := http.Get(assetsURL)
	if err != nil {
		return "", fmt.Errorf("downloading latest yt-dlp: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("downloading latest yt-dlp: %w", err)
	}
	defer resp.Body.Close()

	var assets []ytdlpGithubReleaseAsset
	err = json.Unmarshal(body, &assets)
	if err != nil {
		return "", fmt.Errorf("downloading latest yt-dlp: %w", err)
	}

	if len(assets) == 0 {
		return "", fmt.Errorf("downloading latest yt-dlp: no release assets")
	}

	for _, asset := range assets {
		if asset.Name == "yt-dlp" {
			url = asset.URL
			break
		}
	}
	if url == "" {
		return "", fmt.Errorf("downloading latest yt-dlp: no release asset url")
	}
	return url, nil
}

func ytdlpDownloadLatest(ctx context.Context) (path string, err error) {
	url, err := ytdlpGetLatestReleaseURL(ctx)
	if err != nil {
		return "", err
	}

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading latest yt-dlp: %w", err)
	}
	defer resp.Body.Close()

	file, err := os.Create(filepath.Join(os.TempDir(), "yt-dlp"))
	if err != nil {
		return "", fmt.Errorf("downloading latest yt-dlp: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	err = os.Chmod(file.Name(), 0755)
	if err != nil {
		panic(err)
	}

	return file.Name(), nil
}

func ytdlpVersionCheck(ctx context.Context) (latestVersion, currentVersion string, err error) {
	release, err := ytdlpGetLatestRelease(ctx)
	if err != nil {
		return "", "", err
	}
	latestVersion = release.GetTagName()

	currentVersion, err = goutubedl.Version(ctx)
	if err != nil {
		return "", "", fmt.Errorf("getting current yt-dlp version: %w", err)
	}
	return
}

func ytdlpVersionCheckGetStr(ctx context.Context) (res string, updateNeeded, gotError bool) {
	verCheckCtx, verCheckCtxCancel := context.WithTimeout(ctx, ytdlpVersionCheckTimeout)
	defer verCheckCtxCancel()

	var latestVersion, currentVersion string
	var err error
	if latestVersion, currentVersion, err = ytdlpVersionCheck(verCheckCtx); err != nil {
		return errorStr + ": " + err.Error(), false, true
	}

	updateNeeded = currentVersion != latestVersion
	res = "yt-dlp version: " + currentVersion
	if updateNeeded {
		res = "📢 " + res + " 📢 Update needed! Latest version is " + latestVersion + " 📢"
	} else {
		res += " (up to date)"
	}
	return
}
