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
const processDoneStr = "🏁 Processing"
const errorStr = "❌ Error"
const canceledStr = "❌ Canceled"

const maxProgressPercentUpdateInterval = time.Second
const progressBarLength = 10

type DownloadQueueEntry struct {
	URL string

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

type DownloadQueue struct {
	mutex          sync.Mutex
	entries        []DownloadQueueEntry
	processReqChan chan bool
}

func (e *DownloadQueue) getQueuePositionString(pos int) string {
	return "👨‍👦‍👦 Request queued at position #" + fmt.Sprint(pos)
}

func (q *DownloadQueue) Add(ctx context.Context, entities tg.Entities, u *tg.UpdateNewMessage, url string) {
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

func (q *DownloadQueue) updateProgress(ctx context.Context, qEntry *DownloadQueueEntry, progressPercent int, sourceCodecInfo string) {
	if progressPercent < 0 {
		qEntry.editReply(ctx, processStr+"... (no progress available)\n"+sourceCodecInfo)
		return
	}
	if progressPercent == 0 {
		qEntry.editReply(ctx, processStr+"...\n"+sourceCodecInfo)
		return
	}
	fmt.Print("  progress: ", progressPercent, "%\n")
	qEntry.editReply(ctx, processStr+": "+getProgressbar(progressPercent, progressBarLength)+"\n"+sourceCodecInfo)
}

func (q *DownloadQueue) processQueueEntry(ctx context.Context, qEntry *DownloadQueueEntry) {
	fromUsername := getFromUsername(qEntry.OrigEntities, qEntry.FromUser.UserID)
	fmt.Print("processing request by")
	if fromUsername != "" {
		fmt.Print(" from ", fromUsername, "#", qEntry.FromUser.UserID)
	}
	fmt.Println(":", qEntry.URL)

	qEntry.editReply(ctx, processStartStr)

	var disableProgressPercentUpdate bool
	var progressPercentUpdateMutex sync.Mutex
	var lastProgressPercentUpdateAt time.Time
	var lastProgressPercent int
	var progressUpdateTimer *time.Timer
	var sourceCodecInfo string
	downloader := Downloader{
		ProbeStartFunc: func(ctx context.Context) {
			qEntry.editReply(ctx, "🎬 Getting video format...")
		},
		ConvertStartFunc: func(ctx context.Context, videoCodecs, audioCodecs, convertActionsNeeded string) {
			sourceCodecInfo = "🎬 Source: " + videoCodecs
			if audioCodecs == "" {
				sourceCodecInfo += ", no audio"
			} else {
				sourceCodecInfo += " / " + audioCodecs
			}
			if convertActionsNeeded == "" {
				sourceCodecInfo += " (no conversion needed)"
			} else {
				sourceCodecInfo += " (converting: " + convertActionsNeeded + ")"
			}
			qEntry.editReply(ctx, "🎬 Preparing download...\n"+sourceCodecInfo)
		},
		UpdateProgressPercentFunc: func(progressPercent int) {
			progressPercentUpdateMutex.Lock()
			defer progressPercentUpdateMutex.Unlock()

			if disableProgressPercentUpdate || lastProgressPercent == progressPercent {
				return
			}
			lastProgressPercent = progressPercent
			if progressPercent < 0 {
				disableProgressPercentUpdate = true
				q.updateProgress(ctx, qEntry, progressPercent, sourceCodecInfo)
				return
			}

			if progressUpdateTimer != nil {
				progressUpdateTimer.Stop()
				select {
				case <-progressUpdateTimer.C:
				default:
				}
			}

			timeElapsedSinceLastUpdate := time.Since(lastProgressPercentUpdateAt)
			if timeElapsedSinceLastUpdate < maxProgressPercentUpdateInterval {
				progressUpdateTimer = time.AfterFunc(maxProgressPercentUpdateInterval-timeElapsedSinceLastUpdate, func() {
					q.updateProgress(ctx, qEntry, progressPercent, sourceCodecInfo)
					lastProgressPercentUpdateAt = time.Now()
				})
				return
			}
			q.updateProgress(ctx, qEntry, progressPercent, sourceCodecInfo)
			lastProgressPercentUpdateAt = time.Now()
		},
	}

	r, err := downloader.DownloadAndConvertURL(qEntry.Ctx, qEntry.OrigMsg.Message)
	if err != nil {
		fmt.Println("  error downloading:", err)
		progressPercentUpdateMutex.Lock()
		disableProgressPercentUpdate = true
		progressPercentUpdateMutex.Unlock()
		qEntry.editReply(ctx, fmt.Sprint(errorStr+": ", err))
		return
	}

	// Feeding the returned io.ReadCloser to the uploader.
	fmt.Println("  processing...")
	progressPercentUpdateMutex.Lock()
	q.updateProgress(ctx, qEntry, lastProgressPercent, sourceCodecInfo)
	progressPercentUpdateMutex.Unlock()

	err = uploadFile(ctx, qEntry.OrigEntities, qEntry.OrigMsgUpdate, r)
	if err != nil {
		fmt.Println("  error processing:", err)
		progressPercentUpdateMutex.Lock()
		disableProgressPercentUpdate = true
		progressPercentUpdateMutex.Unlock()
		r.Close()
		qEntry.editReply(ctx, fmt.Sprint(errorStr+": ", err))
		return
	}
	progressPercentUpdateMutex.Lock()
	disableProgressPercentUpdate = true
	progressPercentUpdateMutex.Unlock()
	r.Close()

	progressPercentUpdateMutex.Lock()
	if qEntry.Canceled {
		fmt.Print("  canceled\n")
		qEntry.editReply(ctx, canceledStr+": "+getProgressbar(lastProgressPercent, progressBarLength)+"\n"+sourceCodecInfo)
	} else if lastProgressPercent < 100 {
		fmt.Print("  progress: 100%\n")
		qEntry.editReply(ctx, processDoneStr+": "+getProgressbar(100, progressBarLength)+"\n"+sourceCodecInfo)
	}
	progressPercentUpdateMutex.Unlock()
	qEntry.sendTypingCancelAction(ctx)
}

func (q *DownloadQueue) processor(ctx context.Context) {
	for {
		q.mutex.Lock()
		if (len(q.entries)) == 0 {
			q.mutex.Unlock()
			<-q.processReqChan
			continue
		}

		// Updating queue positions for all waiting entries.
		for i := 1; i < len(q.entries); i++ {
			q.entries[i].editReply(ctx, q.getQueuePositionString(i))
			q.entries[i].sendTypingCancelAction(ctx)
		}

		q.entries[0].Ctx, q.entries[0].CtxCancel = context.WithTimeout(ctx, downloadAndConvertTimeout)

		qEntry := &q.entries[0]
		q.mutex.Unlock()

		q.processQueueEntry(ctx, qEntry)

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
	q.processReqChan = make(chan bool)
	go q.processor(ctx)
}
