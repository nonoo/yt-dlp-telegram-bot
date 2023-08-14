package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/wader/goutubedl"
	"golang.org/x/exp/slices"
)

type paramsType struct {
	ApiID    int
	ApiHash  string
	BotToken string

	AllowedUserIDs  []int64
	AdminUserIDs    []int64
	AllowedGroupIDs []int64
}

var params paramsType

func (p *paramsType) Init() error {
	// Further available environment variables:
	// 	SESSION_FILE:  path to session file
	// 	SESSION_DIR:   path to session directory, if SESSION_FILE is not set

	var apiID string
	flag.StringVar(&apiID, "api-id", "", "telegram api_id")
	flag.StringVar(&p.ApiHash, "api-hash", "", "telegram api_hash")
	flag.StringVar(&p.BotToken, "bot-token", "", "telegram bot token")
	flag.StringVar(&goutubedl.Path, "yt-dlp-path", "", "yt-dlp path")
	var allowedUserIDs string
	flag.StringVar(&allowedUserIDs, "allowed-user-ids", "", "allowed telegram user ids")
	var adminUserIDs string
	flag.StringVar(&adminUserIDs, "admin-user-ids", "", "admin telegram user ids")
	var allowedGroupIDs string
	flag.StringVar(&allowedGroupIDs, "allowed-group-ids", "", "allowed telegram group ids")
	flag.Parse()

	var err error
	if apiID == "" {
		apiID = os.Getenv("API_ID")
	}
	if apiID == "" {
		return fmt.Errorf("api id not set")
	}
	p.ApiID, err = strconv.Atoi(apiID)
	if err != nil {
		return fmt.Errorf("invalid env var API_ID")
	}

	if p.ApiHash == "" {
		p.ApiHash = os.Getenv("API_HASH")
	}
	if p.ApiHash == "" {
		return fmt.Errorf("api hash not set")
	}

	if p.BotToken == "" {
		p.BotToken = os.Getenv("BOT_TOKEN")
	}
	if p.BotToken == "" {
		return fmt.Errorf("bot token not set")
	}

	if goutubedl.Path == "" {
		goutubedl.Path = os.Getenv("YTDLP_PATH")
	}
	if goutubedl.Path == "" {
		goutubedl.Path = "yt-dlp"
	}
	goutubedl.Path, err = exec.LookPath(goutubedl.Path)
	if err != nil {
		return fmt.Errorf("yt-dlp not found")
	}

	if allowedUserIDs == "" {
		allowedUserIDs = os.Getenv("ALLOWED_USERIDS")
	}
	sa := strings.Split(allowedUserIDs, ",")
	for _, idStr := range sa {
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("env var ALLOWED_USERIDS contains invalid user ID: " + idStr)
		}
		p.AllowedUserIDs = append(p.AllowedUserIDs, id)
	}

	if adminUserIDs == "" {
		adminUserIDs = os.Getenv("ADMIN_USERIDS")
	}
	sa = strings.Split(adminUserIDs, ",")
	for _, idStr := range sa {
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("env var ADMIN_USERIDS contains invalid user ID: " + idStr)
		}
		p.AdminUserIDs = append(p.AdminUserIDs, id)
		if !slices.Contains(p.AllowedUserIDs, id) {
			p.AllowedUserIDs = append(p.AllowedUserIDs, id)
		}
	}

	if allowedGroupIDs == "" {
		allowedGroupIDs = os.Getenv("ALLOWED_GROUPIDS")
	}
	sa = strings.Split(allowedGroupIDs, ",")
	for _, idStr := range sa {
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("env var ALLOWED_GROUPIDS contains invalid group ID: " + idStr)
		}
		p.AllowedGroupIDs = append(p.AllowedUserIDs, id)
	}

	return nil
}
