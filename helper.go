package main

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// Helper function to pretty-print any Telegram API object to find out which it needs to be cast to.
// https://github.com/gotd/td/blob/main/examples/pretty-print/main.go

// func formatObject(input interface{}) string {
// 	o, ok := input.(tdp.Object)
// 	if !ok {
// 		// Handle tg.*Box values.
// 		rv := reflect.Indirect(reflect.ValueOf(input))
// 		for i := 0; i < rv.NumField(); i++ {
// 			if v, ok := rv.Field(i).Interface().(tdp.Object); ok {
// 				return formatObject(v)
// 			}
// 		}

// 		return fmt.Sprintf("%T (not object)", input)
// 	}
// 	return tdp.Format(o)
// }

func getProgressbar(progressPercent, progressBarLen int) (progressBar string) {
	i := 0
	for ; i < progressPercent/(100/progressBarLen); i++ {
		progressBar += "▰"
	}
	for ; i < progressBarLen; i++ {
		progressBar += "▱"
	}
	progressBar += " " + fmt.Sprint(progressPercent) + "%"
	return
}

func resolveMsgSrc(msg *tg.Message) (fromUser *tg.PeerUser, fromGroup *tg.PeerChat) {
	fromGroup, isGroupMsg := msg.PeerID.(*tg.PeerChat)
	if isGroupMsg {
		fromUser = msg.FromID.(*tg.PeerUser)
	} else {
		fromUser = msg.PeerID.(*tg.PeerUser)
	}
	return
}

func getFromUsername(entities tg.Entities, fromUID int64) string {
	if fromUser, ok := entities.Users[fromUID]; ok {
		if un, ok := fromUser.GetUsername(); ok {
			return un
		}
	}
	return ""
}

func sendTextToAdmins(ctx context.Context, msg string) {
	for _, id := range params.AdminUserIDs {
		_, _ = telegramSender.To(&tg.InputPeerUser{
			UserID: id,
		}).Text(ctx, msg)
	}
}
