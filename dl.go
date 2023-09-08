package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/wader/goutubedl"
)

const downloadAndConvertTimeout = 5 * time.Minute

type ConvertStartCallbackFunc func(ctx context.Context, videoCodecs, audioCodecs, convertActionsNeeded string)
type UpdateProgressPercentCallbackFunc func(progressStr string, progressPercent int)

type Downloader struct {
	ConvertStartFunc          ConvertStartCallbackFunc
	UpdateProgressPercentFunc UpdateProgressPercentCallbackFunc
}

type goYouTubeDLLogger struct{}

func (l goYouTubeDLLogger) Print(v ...interface{}) {
	fmt.Print("  yt-dlp dbg:")
	fmt.Println(v...)
}

func (d *Downloader) downloadURL(dlCtx context.Context, url string) (rr *ReReadCloser, title string, err error) {
	result, err := goutubedl.New(dlCtx, url, goutubedl.Options{
		Type:     goutubedl.TypeSingle,
		DebugLog: goYouTubeDLLogger{},
		// StderrFn:          func(cmd *exec.Cmd) io.Writer { return io.Writer(os.Stdout) },
		MergeOutputFormat: "mkv",     // This handles VP9 properly. yt-dlp uses mp4 by default, which doesn't.
		SortingFormat:     "res:720", // Prefer videos no larger than 720p to keep their size small.
	})
	if err != nil {
		return nil, "", fmt.Errorf("preparing download %q: %w", url, err)
	}

	dlResult, err := result.Download(dlCtx, "")
	if err != nil {
		return nil, "", fmt.Errorf("downloading %q: %w", url, err)
	}

	return NewReReadCloser(dlResult), result.Info.Title, nil
}

func (d *Downloader) DownloadAndConvertURL(ctx context.Context, url, format string) (r io.ReadCloser, outputFormat, title string, err error) {
	rr, title, err := d.downloadURL(ctx, url)
	if err != nil {
		return nil, "", "", err
	}

	conv := Converter{
		Format:                        format,
		UpdateProgressPercentCallback: d.UpdateProgressPercentFunc,
	}

	if err := conv.Probe(rr); err != nil {
		return nil, "", "", err
	}

	if d.ConvertStartFunc != nil {
		d.ConvertStartFunc(ctx, conv.VideoCodecs, conv.AudioCodecs, conv.GetActionsNeeded())
	}

	r, outputFormat, err = conv.ConvertIfNeeded(ctx, rr)
	if err != nil {
		return nil, "", "", err
	}

	return r, outputFormat, title, nil
}
