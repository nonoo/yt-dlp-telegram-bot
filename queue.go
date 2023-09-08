package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
)

const processStartStr = "🔍 Getting information..."
const processStr = "🔨 Processing"
const uploadStr = "☁️ Uploading"
const uploadDoneStr = "🏁 Uploading"
const errorStr = "❌ Error"
const canceledStr = "❌ Canceled"

const maxProgressPercentUpdateInterval = time.Second
const progressBarLength = 10

type DownloadQueueEntry struct {
	URL    string
	Format string

	OrigEntities  tg.Entities
	OrigMsgUpdate *tg.UpdateNewMessage
	OrigMsg       *tg.Message
	FromUser      *tg.PeerUser
	FromGroup     *tg.PeerChat

	Reply    *message.Builder
	ReplyMsg *tg.UpdateShortSentMessage

	Ctx       context.Context
	CtxCancel context.CancelFunc
	Canceled  bool
}

// func (e *DownloadQueueEntry) getTypingActionDst() tg.InputPeerClass {
// 	if e.FromGroup != nil {
// 		return &tg.InputPeerChat{
// 			ChatID: e.FromGroup.ChatID,
// 		}
// 	}
// 	return &tg.InputPeerUser{
// 		UserID: e.FromUser.UserID,
// 	}
// }

func (e *DownloadQueueEntry) sendTypingAction(ctx context.Context) {
	// _ = telegramSender.To(e.getTypingActionDst()).TypingAction().Typing(ctx)
}

func (e *DownloadQueueEntry) sendTypingCancelAction(ctx context.Context) {
	// _ = telegramSender.To(e.getTypingActionDst()).TypingAction().Cancel(ctx)
}

func (e *DownloadQueueEntry) editReply(ctx context.Context, s string) {
	_, _ = e.Reply.Edit(e.ReplyMsg.ID).Text(ctx, s)
	e.sendTypingAction(ctx)
}

type currentlyDownloadedEntryType struct {
	disableProgressPercentUpdate bool
	progressPercentUpdateMutex   sync.Mutex
	lastProgressPercentUpdateAt  time.Time
	lastProgressPercent          int
	lastDisplayedProgressPercent int
	progressUpdateTimer          *time.Timer

	sourceCodecInfo string
	progressInfo    string
}

type DownloadQueue struct {
	ctx context.Context

	mutex          sync.Mutex
	entries        []DownloadQueueEntry
	processReqChan chan bool

	currentlyDownloadedEntry currentlyDownloadedEntryType
}

func (e *DownloadQueue) getQueuePositionString(pos int) string {
	return "👨‍👦‍👦 Request queued at position #" + fmt.Sprint(pos)
}

func (q *DownloadQueue) Add(ctx context.Context, entities tg.Entities, u *tg.UpdateNewMessage, url, format string) {
	q.mutex.Lock()

	var replyStr string
	if len(q.entries) == 0 {
		replyStr = processStartStr
	} else {
		fmt.Println("  queueing request at position #", len(q.entries))
		replyStr = q.getQueuePositionString(len(q.entries))
	}

	newEntry := DownloadQueueEntry{
		URL:           url,
		Format:        format,
		OrigEntities:  entities,
		OrigMsgUpdate: u,
		OrigMsg:       u.Message.(*tg.Message),
	}

	newEntry.Reply = telegramSender.Reply(entities, u)
	replyText, _ := newEntry.Reply.Text(ctx, replyStr)
	newEntry.ReplyMsg = replyText.(*tg.UpdateShortSentMessage)

	newEntry.FromUser, newEntry.FromGroup = resolveMsgSrc(newEntry.OrigMsg)

	q.entries = append(q.entries, newEntry)
	q.mutex.Unlock()

	select {
	case q.processReqChan <- true:
	default:
	}
}

func (q *DownloadQueue) CancelCurrentEntry(ctx context.Context, entities tg.Entities, u *tg.UpdateNewMessage, url string) {
	q.mutex.Lock()
	if len(q.entries) > 0 {
		q.entries[0].Canceled = true
		q.entries[0].CtxCancel()
	} else {
		fmt.Println("  no active request to cancel")
		_, _ = telegramSender.Reply(entities, u).Text(ctx, errorStr+": no active request to cancel")
	}
	q.mutex.Unlock()
}

func (q *DownloadQueue) updateProgress(ctx context.Context, qEntry *DownloadQueueEntry, progressStr string, progressPercent int) {
	if progressPercent < 0 {
		qEntry.editReply(ctx, progressStr+"... (no progress available)\n"+q.currentlyDownloadedEntry.sourceCodecInfo)
		return
	}
	if progressPercent == 0 {
		qEntry.editReply(ctx, progressStr+"..."+q.currentlyDownloadedEntry.progressInfo+"\n"+q.currentlyDownloadedEntry.sourceCodecInfo)
		return
	}
	fmt.Print("  progress: ", progressPercent, "%\n")
	qEntry.editReply(ctx, progressStr+": "+getProgressbar(progressPercent, progressBarLength)+q.currentlyDownloadedEntry.progressInfo+"\n"+q.currentlyDownloadedEntry.sourceCodecInfo)
	q.currentlyDownloadedEntry.lastDisplayedProgressPercent = progressPercent
}

func (q *DownloadQueue) HandleProgressPercentUpdate(progressStr string, progressPercent int) {
	q.currentlyDownloadedEntry.progressPercentUpdateMutex.Lock()
	defer q.currentlyDownloadedEntry.progressPercentUpdateMutex.Unlock()

	if q.currentlyDownloadedEntry.disableProgressPercentUpdate || q.currentlyDownloadedEntry.lastProgressPercent == progressPercent {
		return
	}
	q.currentlyDownloadedEntry.lastProgressPercent = progressPercent
	if progressPercent < 0 {
		q.currentlyDownloadedEntry.disableProgressPercentUpdate = true
		q.updateProgress(q.ctx, &q.entries[0], progressStr, progressPercent)
		return
	}

	if q.currentlyDownloadedEntry.progressUpdateTimer != nil {
		q.currentlyDownloadedEntry.progressUpdateTimer.Stop()
		select {
		case <-q.currentlyDownloadedEntry.progressUpdateTimer.C:
		default:
		}
	}

	timeElapsedSinceLastUpdate := time.Since(q.currentlyDownloadedEntry.lastProgressPercentUpdateAt)
	if timeElapsedSinceLastUpdate < maxProgressPercentUpdateInterval {
		q.currentlyDownloadedEntry.progressUpdateTimer = time.AfterFunc(maxProgressPercentUpdateInterval-timeElapsedSinceLastUpdate, func() {
			q.currentlyDownloadedEntry.progressPercentUpdateMutex.Lock()
			if !q.currentlyDownloadedEntry.disableProgressPercentUpdate {
				q.updateProgress(q.ctx, &q.entries[0], progressStr, progressPercent)
				q.currentlyDownloadedEntry.lastProgressPercentUpdateAt = time.Now()
			}
			q.currentlyDownloadedEntry.progressPercentUpdateMutex.Unlock()
		})
		return
	}
	q.updateProgress(q.ctx, &q.entries[0], progressStr, progressPercent)
	q.currentlyDownloadedEntry.lastProgressPercentUpdateAt = time.Now()
}

func (q *DownloadQueue) processQueueEntry(ctx context.Context, qEntry *DownloadQueueEntry) {
	fromUsername := getFromUsername(qEntry.OrigEntities, qEntry.FromUser.UserID)
	fmt.Print("processing request by")
	if fromUsername != "" {
		fmt.Print(" from ", fromUsername, "#", qEntry.FromUser.UserID)
	}
	fmt.Println(":", qEntry.URL)

	qEntry.editReply(ctx, processStartStr)

	downloader := Downloader{
		ConvertStartFunc: func(ctx context.Context, videoCodecs, audioCodecs, convertActionsNeeded string) {
			q.currentlyDownloadedEntry.sourceCodecInfo = "🎬 Source: " + videoCodecs
			if audioCodecs == "" {
				q.currentlyDownloadedEntry.sourceCodecInfo += ", no audio"
			} else {
				if videoCodecs != "" {
					q.currentlyDownloadedEntry.sourceCodecInfo += " / "
				}
				q.currentlyDownloadedEntry.sourceCodecInfo += audioCodecs
			}
			if convertActionsNeeded == "" {
				q.currentlyDownloadedEntry.sourceCodecInfo += " (no conversion needed)"
			} else {
				q.currentlyDownloadedEntry.sourceCodecInfo += " (converting: " + convertActionsNeeded + ")"
			}
			qEntry.editReply(ctx, "🎬 Preparing download...\n"+q.currentlyDownloadedEntry.sourceCodecInfo)
		},
		UpdateProgressPercentFunc: q.HandleProgressPercentUpdate,
	}

	r, outputFormat, title, err := downloader.DownloadAndConvertURL(qEntry.Ctx, qEntry.OrigMsg.Message, qEntry.Format)
	if err != nil {
		fmt.Println("  error downloading:", err)
		q.currentlyDownloadedEntry.progressPercentUpdateMutex.Lock()
		q.currentlyDownloadedEntry.disableProgressPercentUpdate = true
		q.currentlyDownloadedEntry.progressPercentUpdateMutex.Unlock()
		qEntry.editReply(ctx, fmt.Sprint(errorStr+": ", err))
		return
	}

	// Feeding the returned io.ReadCloser to the uploader.
	fmt.Println("  processing...")
	q.currentlyDownloadedEntry.progressPercentUpdateMutex.Lock()
	q.updateProgress(ctx, qEntry, processStr, q.currentlyDownloadedEntry.lastProgressPercent)
	q.currentlyDownloadedEntry.progressPercentUpdateMutex.Unlock()

	err = dlUploader.UploadFile(qEntry.Ctx, qEntry.OrigEntities, qEntry.OrigMsgUpdate, r, outputFormat, title)
	if err != nil {
		fmt.Println("  error processing:", err)
		q.currentlyDownloadedEntry.progressPercentUpdateMutex.Lock()
		q.currentlyDownloadedEntry.disableProgressPercentUpdate = true
		q.currentlyDownloadedEntry.progressPercentUpdateMutex.Unlock()
		r.Close()
		qEntry.editReply(ctx, fmt.Sprint(errorStr+": ", err))
		return
	}
	q.currentlyDownloadedEntry.progressPercentUpdateMutex.Lock()
	q.currentlyDownloadedEntry.disableProgressPercentUpdate = true
	q.currentlyDownloadedEntry.progressPercentUpdateMutex.Unlock()
	r.Close()

	q.currentlyDownloadedEntry.progressPercentUpdateMutex.Lock()
	if qEntry.Canceled {
		fmt.Print("  canceled\n")
		q.updateProgress(ctx, qEntry, canceledStr, q.currentlyDownloadedEntry.lastProgressPercent)
	} else if q.currentlyDownloadedEntry.lastDisplayedProgressPercent < 100 {
		fmt.Print("  progress: 100%\n")
		q.updateProgress(ctx, qEntry, uploadDoneStr, 100)
	}
	q.currentlyDownloadedEntry.progressPercentUpdateMutex.Unlock()
	qEntry.sendTypingCancelAction(ctx)
}

func (q *DownloadQueue) processor() {
	for {
		q.mutex.Lock()
		if (len(q.entries)) == 0 {
			q.mutex.Unlock()
			<-q.processReqChan
			continue
		}

		// Updating queue positions for all waiting entries.
		for i := 1; i < len(q.entries); i++ {
			q.entries[i].editReply(q.ctx, q.getQueuePositionString(i))
			q.entries[i].sendTypingCancelAction(q.ctx)
		}

		q.entries[0].Ctx, q.entries[0].CtxCancel = context.WithTimeout(q.ctx, downloadAndConvertTimeout)

		qEntry := &q.entries[0]
		q.mutex.Unlock()

		q.currentlyDownloadedEntry = currentlyDownloadedEntryType{}

		q.processQueueEntry(q.ctx, qEntry)

		q.mutex.Lock()
		q.entries[0].CtxCancel()
		q.entries = q.entries[1:]
		if len(q.entries) == 0 {
			fmt.Print("finished queue processing\n")
		}
		q.mutex.Unlock()
	}
}

func (q *DownloadQueue) Init(ctx context.Context) {
	q.ctx = ctx
	q.processReqChan = make(chan bool)
	go q.processor()
}
