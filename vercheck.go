package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/google/go-github/v53/github"
	"github.com/wader/goutubedl"
)

const ytdlpVersionCheckTimeout = time.Second * 10

func ytdlpVersionCheck(ctx context.Context) (latestVersion, currentVersion string, err error) {
	client := github.NewClient(nil)

	release, _, err := client.Repositories.GetLatestRelease(ctx, "yt-dlp", "yt-dlp")
	if err != nil {
		return "", "", fmt.Errorf("getting latest yt-dlp version: %w", err)
	}
	latestVersion = release.GetTagName()

	currentVersion, err = goutubedl.Version(ctx)
	if err != nil {
		return "", "", fmt.Errorf("getting current yt-dlp version: %w", err)
	}
	return
}

func ytdlpVersionCheckGetStr(ctx context.Context) (res string, updateNeededOrError bool) {
	verCheckCtx, verCheckCtxCancel := context.WithTimeout(ctx, ytdlpVersionCheckTimeout)
	defer verCheckCtxCancel()

	var latestVersion, currentVersion string
	var err error
	if latestVersion, currentVersion, err = ytdlpVersionCheck(verCheckCtx); err != nil {
		return errorStr + ": " + err.Error(), true
	}

	updateNeededOrError = currentVersion != latestVersion
	res = "yt-dlp version: " + currentVersion
	if updateNeededOrError {
		res = "📢 " + res + " 📢 Update needed! Latest version is " + latestVersion + " 📢"
	} else {
		res += " (up to date)"
	}
	return
}
