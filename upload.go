package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"

	"github.com/dustin/go-humanize"
	"github.com/flytam/filenamify"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
)

type Uploader struct{}

var dlUploader Uploader

func (p Uploader) Chunk(ctx context.Context, state uploader.ProgressState) error {
	dlQueue.HandleProgressPercentUpdate(uploadStr, int(state.Uploaded*100/state.Total))
	return nil
}

func (p *Uploader) UploadFile(ctx context.Context, entities tg.Entities, u *tg.UpdateNewMessage, f io.ReadCloser, format, title string) error {
	// Reading to a buffer first, because we don't know the file size.
	var buf bytes.Buffer
	for {
		b := make([]byte, 1024)
		n, err := f.Read(b)
		if err != nil && err != io.EOF {
			return fmt.Errorf("reading to buffer error: %w", err)
		}
		if n == 0 {
			break
		}
		if params.MaxSize > 0 && buf.Len() > int(params.MaxSize) {
			return fmt.Errorf("file is too big, max. allowed size is %s", humanize.BigBytes(big.NewInt(int64(params.MaxSize))))
		}
		buf.Write(b[:n])
	}

	fmt.Println("  got", buf.Len(), "bytes, uploading...")
	dlQueue.currentlyDownloadedEntry.progressInfo = fmt.Sprint(" (", humanize.BigBytes(big.NewInt(int64(buf.Len()))), ")")

	upload, err := telegramUploader.FromBytes(ctx, "yt-dlp", buf.Bytes())
	if err != nil {
		return fmt.Errorf("uploading %w", err)
	}

	// Now we have uploaded file handle, sending it as styled message. First, preparing message.
	var document message.MediaOption
	filename, _ := filenamify.Filenamify(title+"."+format, filenamify.Options{Replacement: " "})
	if format == "mp3" {
		document = message.UploadedDocument(upload).Filename(filename).Audio().Title(title)
	} else {
		document = message.UploadedDocument(upload).Filename(filename).Video()
	}

	// Sending message with media.
	if _, err := telegramSender.Answer(entities, u).Media(ctx, document); err != nil {
		return fmt.Errorf("send: %w", err)
	}

	return nil
}
